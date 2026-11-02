#!/usr/bin/env python3
"""Bounded, opt-in provider feasibility probe. No SDK or raw transcript export."""
import argparse
import collections
import datetime
import json
import os
from pathlib import Path
import queue
import signal
import shlex
import subprocess
import sys
import tempfile
import threading
import time
import uuid

MAX_LINE = 1024 * 1024
TIMEOUT = 90
MARKER = 'AGENTOS_SYNTHETIC_OK'


def decode_line(line):
    if len(line.encode('utf-8')) > MAX_LINE:
        raise ValueError('oversized event')
    value = json.loads(line)
    if not isinstance(value, dict):
        raise ValueError('event must be an object')
    return value


def safe_shape(event):
    """Fixed allowlist only: no provider text, IDs, paths, or URLs."""
    known = {
        'method': {
            'remoteControl/status/changed', 'thread/started', 'thread/status/changed',
            'turn/started', 'turn/completed', 'warning', 'error', 'item/started',
            'item/completed', 'item/agentMessage/delta', 'thread/tokenUsage/updated',
            'thread/goal/cleared', 'thread/closed', 'deprecationNotice',
        },
        'type': {'system', 'assistant', 'user', 'result', 'stream_event',
                 'tool_progress', 'tool_use_summary', 'rate_limit_event'},
        'subtype': {'init', 'success', 'error_during_execution', 'error_max_turns',
                    'error_max_budget_usd', 'hook_started', 'hook_response',
                    'hook_progress', 'status', 'compact_boundary'},
    }
    result = {}
    for key, allowed in known.items():
        if isinstance(event.get(key), str):
            result[key] = event[key] if event[key] in allowed else 'unknown'
    error = event.get('error')
    if isinstance(error, dict) and isinstance(error.get('code'), int):
        result['error_code'] = error['code']
    return result


def classify_error(value):
    """Reduce errors to fixed categories; never persist message text or URLs."""
    text = json.dumps(value).lower()
    categories = []
    for category, needles in {
        'authentication': ('unauthorized', 'authentication', 'not logged in', 'invalid api key', '401'),
        'authorization': ('forbidden', '403'),
        'rate_limit': ('rate limit', '429'),
        'model_unavailable': ('model not found', 'model_not_found', 'unsupported model'),
        'network': ('connection', 'connect error', 'dns', 'stream disconnected'),
        'budget': ('budget', 'credit balance', 'insufficient_quota'),
    }.items():
        if any(needle in text for needle in needles):
            categories.append(category)
    return categories or (['unclassified'] if value else [])


def environment():
    # Do not inherit the parent Codex session, tool bridges, hooks or telemetry context.
    keep = ('PATH', 'HOME', 'USER', 'LOGNAME', 'SHELL', 'TMPDIR', 'LANG', 'LC_ALL',
            'SSL_CERT_FILE', 'SSL_CERT_DIR', 'HTTPS_PROXY', 'HTTP_PROXY', 'NO_PROXY',
            'OPENAI_API_KEY', 'OPENAI_BASE_URL', 'ANTHROPIC_API_KEY',
            'ANTHROPIC_BASE_URL', 'ANTHROPIC_AUTH_TOKEN')
    return {k: os.environ[k] for k in keep if k in os.environ}


def stop(proc):
    """Only stop the process group created by this probe."""
    if proc.poll() is None:
        os.killpg(proc.pid, signal.SIGTERM)
        try:
            proc.wait(timeout=5)
        except subprocess.TimeoutExpired:
            os.killpg(proc.pid, signal.SIGKILL)
            proc.wait(timeout=5)


class Rpc:
    def __init__(self, cwd, env):
        self.proc = subprocess.Popen(
            ['codex', 'app-server', '--stdio'], cwd=cwd, env=env,
            stdin=subprocess.PIPE, stdout=subprocess.PIPE, stderr=subprocess.DEVNULL,
            text=True, start_new_session=True, bufsize=1)
        self.events = []
        self.responses = {}
        self.incoming = queue.Queue(maxsize=4096)
        self.next_id = 0
        self.failure = None
        self.on_notification = None
        self.reader = threading.Thread(target=self._read, daemon=True)
        self.reader.start()

    def _read(self):
        try:
            total_bytes = 0
            while True:
                line = self.proc.stdout.readline(MAX_LINE + 2)
                total_bytes += len(line.encode('utf-8'))
                if total_bytes > 4 * MAX_LINE:
                    raise ValueError('capture limit')
                if not line:
                    self.incoming.put(None, timeout=2)
                    return
                self.incoming.put(decode_line(line), timeout=2)
        except (ValueError, queue.Full, OSError):
            self.failure = 'invalid_or_overflowed_stream'

    def send(self, message):
        self.proc.stdin.write(json.dumps(message) + '\n')
        self.proc.stdin.flush()

    def receive(self, deadline):
        if self.failure:
            raise RuntimeError(self.failure)
        remaining = deadline - time.monotonic()
        if remaining <= 0:
            raise TimeoutError('protocol timeout')
        try:
            message = self.incoming.get(timeout=min(remaining, TIMEOUT))
        except queue.Empty:
            raise TimeoutError('protocol timeout') from None
        if message is None:
            raise RuntimeError('provider stream closed')
        if 'id' in message and ('result' in message or 'error' in message):
            self.responses[message['id']] = message
        elif 'id' in message:
            # Reject provider approval/tool requests; never auto-approve permissions.
            self.send({'id': message['id'], 'error': {
                'code': -32601, 'message': 'Probe does not authorize tool requests'}})
        else:
            self.events.append(message)
            if self.on_notification:
                self.on_notification(message)
            if len(self.events) > 4096:
                raise RuntimeError('event buffer limit')
        return message

    def call(self, method, params, timeout=15):
        self.next_id += 1
        request_id = self.next_id
        self.send({'id': request_id, 'method': method, 'params': params})
        deadline = time.monotonic() + timeout
        while request_id not in self.responses:
            self.receive(deadline)
        return self.responses.pop(request_id)

    def initialize(self):
        response = self.call('initialize', {'clientInfo': {
            'name': 'agentos_feasibility_probe', 'version': '0.0.1'}})
        self.send({'method': 'initialized', 'params': {}})
        return 'result' in response

    def wait_turn(self, turn_id, timeout=TIMEOUT):
        deadline = time.monotonic() + timeout
        while True:
            for event in self.events:
                p = event.get('params', {})
                if event.get('method') == 'turn/completed' and p.get('turn', {}).get('id') == turn_id:
                    return p['turn']
            self.receive(deadline)

    def close(self):
        if self.proc.stdin:
            self.proc.stdin.close()
        stop(self.proc)
        self.reader.join(timeout=2)
        self.proc.stdout.close()


def result_ok(response):
    return isinstance(response, dict) and 'result' in response


def run_codex(root, live, native_auth=False, interrupt_only=False):
    workspace = root / 'codex-workspace'
    workspace.mkdir()
    provider_home = root / 'codex-home'
    provider_home.mkdir(mode=0o700)
    # Fresh configuration/history; native auth link is separately opt-in.
    env = environment()
    env['CODEX_HOME'] = str(provider_home)
    if native_auth:
        auth = Path.home() / '.codex/auth.json'
        if not auth.is_file():
            return {'version': version('codex'), 'skipped': 'native_auth_unavailable'}
        # Only the official CLI reads the existing credential; never copy it into evidence.
        (provider_home / 'auth.json').symlink_to(auth)
        env.pop('OPENAI_API_KEY', None)
        env.pop('OPENAI_BASE_URL', None)
    out = {'version': version('codex'), 'transport': 'stdio', 'checks': {}}
    out['auth_mode'] = 'native_cli_auth_link' if native_auth else 'inherited_provider_env_only'
    checks = out['checks']
    clients = []
    try:
        rpc = Rpc(workspace, env)
        clients.append(rpc)
        checks['reject_before_initialize'] = 'error' in rpc.call('thread/list', {})
        checks['initialize'] = rpc.initialize()
        checks['reject_duplicate_initialize'] = 'error' in rpc.call('initialize', {
            'clientInfo': {'name': 'agentos_feasibility_probe', 'version': '0.0.1'}})
        response = rpc.call('thread/list', {'cwd': str(workspace), 'limit': 10})
        checks['isolated_list_empty'] = result_ok(response) and response['result'].get('data') == []
        checks['unknown_method_rejected'] = 'error' in rpc.call('agentos/unknown', {})
        checks['malformed_request_rejected'] = 'error' in rpc.call('thread/read', {})
        start = rpc.call('thread/start', {
            'cwd': str(workspace), 'sandbox': 'read-only',
            'approvalPolicy': 'untrusted',
            'baseInstructions': 'Synthetic protocol test. Do not use any tools. Reply briefly.',
            'developerInstructions': 'No file reads, shell commands, network tools or subagents.'})
        checks['thread_start'] = result_ok(start)
        if not result_ok(start):
            out['failure_shape'] = safe_shape(start)
            return out
        thread_id = start['result']['thread']['id']
        checks['thread_read_before_turn'] = result_ok(rpc.call('thread/read', {'threadId': thread_id}))
        if live:
            prompt = ('Write the integers from 1 through 500, one per line. Do not use tools.'
                      if interrupt_only else 'Reply exactly ' + MARKER + '. Do not use tools.')
            interrupt_request = []
            if interrupt_only:
                def interrupt_on_started(event):
                    params = event.get('params', {})
                    if (event.get('method') == 'turn/started' and
                            params.get('threadId') == thread_id and not interrupt_request):
                        rpc.next_id += 1
                        interrupt_request.append(rpc.next_id)
                        rpc.send({'id': rpc.next_id, 'method': 'turn/interrupt', 'params': {
                            'threadId': thread_id, 'turnId': params['turn']['id']}})
                rpc.on_notification = interrupt_on_started
            turn = rpc.call('turn/start', {'threadId': thread_id,
                'input': [{'type': 'text', 'text': prompt}],
                'effort': 'low'}, timeout=20)
            checks['turn_start'] = result_ok(turn)
            if result_ok(turn):
                completed = rpc.wait_turn(turn['result']['turn']['id'])
                if interrupt_only:
                    checks['interrupt_sent_on_turn_started'] = bool(interrupt_request)
                    if interrupt_request:
                        deadline = time.monotonic() + 15
                        while interrupt_request[0] not in rpc.responses:
                            rpc.receive(deadline)
                        interrupt = rpc.responses.pop(interrupt_request[0])
                        checks['interrupt_acknowledged'] = result_ok(interrupt)
                        out['interrupt_response_shape'] = safe_shape(interrupt)
                    rpc.on_notification = None
                out['first_turn_status'] = completed.get('status')
                out['failure_categories'] = classify_error(completed.get('error'))
                checks['turn_completed'] = completed.get('status') == 'completed'
                if interrupt_only:
                    checks['interrupt_completed_as_interrupted'] = completed.get('status') == 'interrupted'
                # Inspect synthetic output in memory, never export full content.
                checks['synthetic_reply'] = any(
                    e.get('method') == 'item/completed' and
                    e.get('params', {}).get('item', {}).get('type') == 'agentMessage' and
                    e.get('params', {}).get('item', {}).get('text', '').strip() == MARKER
                    for e in rpc.events)
        first_events = list(rpc.events)
        # Reconnect to the SAME isolated provider home, never the user's store.
        rpc.close()
        rpc = Rpc(workspace, env)
        clients.append(rpc)
        checks['reconnect_initialize'] = rpc.initialize()
        listing = rpc.call('thread/list', {'cwd': str(workspace), 'limit': 10})
        checks['discover_own_persisted_thread'] = result_ok(listing) and any(
            t.get('id') == thread_id for t in listing['result'].get('data', []))
        read = rpc.call('thread/read', {'threadId': thread_id, 'includeTurns': True})
        checks['history_read_after_reconnect'] = result_ok(read)
        if not result_ok(read):
            out['history_error_shape'] = safe_shape(read)
        resume = rpc.call('thread/resume', {'threadId': thread_id, 'cwd': str(workspace)})
        checks['resume_owned_thread'] = result_ok(resume)
        if live and not interrupt_only and result_ok(resume) and checks.get('turn_completed'):
            turn = rpc.call('turn/start', {'threadId': thread_id, 'input': [{
                'type': 'text', 'text': 'Reply with the same synthetic marker from the prior turn. Do not use tools.'}],
                'effort': 'low'})
            if result_ok(turn):
                turn_id = turn['result']['turn']['id']
                end = rpc.wait_turn(turn_id)
                out['continued_turn_status'] = end.get('status')
                checks['continue_context_recall'] = end.get('status') == 'completed' and any(
                    e.get('method') == 'item/completed' and
                    e.get('params', {}).get('item', {}).get('type') == 'agentMessage' and
                    e.get('params', {}).get('item', {}).get('text', '').strip() == MARKER
                    for e in rpc.events)
        all_events = first_events + rpc.events
        out['stream_error_categories'] = sorted(set(
            category for event in all_events if event.get('method') == 'error'
            for category in classify_error(event.get('params', {}).get('error'))))
        out['notification_counts'] = dict(collections.Counter(
            safe_shape(e).get('method', 'unknown') for e in all_events))
        checks['item_notifications'] = any(e.get('method') == 'item/started' for e in all_events)
        checks['usage_notification'] = any(e.get('method') == 'thread/tokenUsage/updated' for e in all_events)
    except (TimeoutError, RuntimeError, OSError, KeyError) as exc:
        out['failure_class'] = type(exc).__name__
        if clients:
            out['observed_shapes'] = [safe_shape(e) for e in clients[-1].events[-12:]]
    finally:
        for client in clients:
            if client.proc.poll() is None:
                client.close()
    return out


def collect_events(proc, timeout):
    """Bounded JSONL reader with an overall deadline and a four-MiB capture ceiling."""
    incoming = queue.Queue(maxsize=4096)

    def reader():
        try:
            while True:
                line = proc.stdout.readline(MAX_LINE + 2)
                incoming.put(line if line else None, timeout=2)
                if not line:
                    return
        except (OSError, ValueError, queue.Full):
            return

    worker = threading.Thread(target=reader, daemon=True)
    worker.start()
    deadline = time.monotonic() + timeout
    events = []
    total_bytes = 0
    try:
        while True:
            remaining = deadline - time.monotonic()
            if remaining <= 0:
                raise TimeoutError('provider deadline')
            try:
                line = incoming.get(timeout=remaining)
            except queue.Empty:
                raise TimeoutError('provider deadline') from None
            if line is None:
                proc.wait(timeout=max(0.01, deadline - time.monotonic()))
                return events
            total_bytes += len(line.encode('utf-8'))
            if total_bytes > 4 * MAX_LINE or len(events) >= 4096:
                raise ValueError('capture limit')
            if line.strip():
                events.append(decode_line(line))
    finally:
        stop(proc)
        worker.join(timeout=3)
        proc.stdout.close()


HOOK_COLLECTOR = '''import json, sys
from pathlib import Path
raw = sys.stdin.buffer.read(1048577)
if len(raw) > 1048576:
    sys.exit(1)
event = json.loads(raw)
name = event.get('hook_event_name')
allowed = ('SessionStart', 'PreToolUse', 'PostToolUse', 'Stop', 'SessionEnd')
if name not in allowed:
    sys.exit(1)
record = {'hook_event_name': name}
if event.get('tool_name') == 'Read':
    record['tool_name'] = 'Read'
with (Path(__file__).parent / 'hook-events.jsonl').open('a') as f:
    f.write(json.dumps(record) + '\\n')
'''


def version(command):
    return subprocess.check_output([command, '--version'], text=True, timeout=10).strip()


def run_claude(root, live):
    workspace = root / 'claude-workspace'
    workspace.mkdir()
    (workspace / 'fixture.txt').write_text(MARKER + '\n')
    collector = workspace / 'collect_hook.py'
    collector.write_text(HOOK_COLLECTOR)
    hooks = {}
    for name in ('SessionStart', 'PreToolUse', 'PostToolUse', 'Stop', 'SessionEnd'):
        hooks[name] = [{'hooks': [{'type': 'command',
            'command': shlex.quote(sys.executable) + ' ' + shlex.quote(str(collector)), 'timeout': 5}]}]
    settings = workspace / 'probe-settings.json'
    settings.write_text(json.dumps({'hooks': hooks}))
    out = {'version': version('claude'), 'transport': 'stream-json', 'checks': {},
           'max_budget_usd_per_call': 0.25, 'live_requested': live}
    if not live:
        out['skipped'] = 'Claude inference requires --live; settings generated and removed only'
        return out
    status = subprocess.run(['claude', 'auth', 'status', '--json'], env=environment(),
                            text=True, capture_output=True, timeout=15)
    try:
        ready = json.loads(status.stdout).get('loggedIn') is True
    except (ValueError, AttributeError):
        ready = False
    out['native_auth_ready'] = ready
    if not ready:
        out['skipped'] = 'native_auth_not_ready; no model call attempted'
        return out
    session_id = str(uuid.uuid4())
    base = ['claude', '-p', '--output-format', 'stream-json', '--verbose',
            '--include-partial-messages', '--restricted', '--tools', 'Read',
            '--allowedTools', 'Read', '--strict-mcp-config', '--mcp-config', '{"mcpServers":{}}',
            '--setting-sources', '', '--settings', str(settings),
            '--max-budget-usd', '0.25', '--effort', 'low']
    prompts = [(['--session-id', session_id],
                'Read only fixture.txt with the Read tool. Reply with its single line exactly. No other tools.'),
               (['--resume', session_id],
                'Reply with the same synthetic marker from the prior turn. Do not use tools.')]
    for index, (extra, prompt) in enumerate(prompts, 1):
        proc = subprocess.Popen(base + extra + [prompt], cwd=workspace,
            env=environment(), stdin=subprocess.DEVNULL, stdout=subprocess.PIPE,
            stderr=subprocess.DEVNULL, text=True, start_new_session=True)
        try:
            events = collect_events(proc, TIMEOUT)
            final = next((e for e in reversed(events) if e.get('type') == 'result'), {})
            out['call_' + str(index)] = {
                'exit_code': proc.returncode,
                'event_counts': dict(collections.Counter(
                    '/'.join(safe_shape(e).get(k, '') for k in ('type', 'subtype')).rstrip('/')
                    for e in events)),
                'result_error': final.get('is_error'),
                'result_subtype': safe_shape(final).get('subtype'),
                'has_usage': isinstance(final.get('usage'), dict),
                'estimated_cost_usd': final.get('total_cost_usd'),
                'synthetic_reply': final.get('result', '').strip() == MARKER,
                'same_owned_session': final.get('session_id') == session_id,
                'failure_categories': classify_error(final.get('result')) if final.get('is_error') else [],
            }
            out['checks']['call_' + str(index) + '_success'] = (
                proc.returncode == 0 and final.get('is_error') is False and
                final.get('result', '').strip() == MARKER)
            if not out['checks']['call_' + str(index) + '_success']:
                break
        except (subprocess.TimeoutExpired, TimeoutError):
            out['failure_class'] = 'TimeoutError'
            break
        except (ValueError, RuntimeError, OSError) as exc:
            out['failure_class'] = type(exc).__name__
            break
        finally:
            stop(proc)
    hook_file = workspace / 'hook-events.jsonl'
    observed = [decode_line(line) for line in hook_file.read_text().splitlines()] if hook_file.exists() else []
    out['hook_event_counts'] = dict(collections.Counter(e['hook_event_name'] for e in observed))
    for name in hooks:
        out['checks']['hook_' + name] = any(e['hook_event_name'] == name for e in observed)
    out['temporary_settings_cleanup'] = 'TemporaryDirectory on probe exit'
    return out


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--provider', choices=('codex', 'claude', 'both'), default='both')
    parser.add_argument('--codex-interrupt-only', action='store_true',
                        help='Use one longer synthetic turn to isolate interruption race')
    parser.add_argument('--native-codex-auth', action='store_true',
                        help='Let official CLI read existing auth via temporary private link')
    parser.add_argument('--live', action='store_true', help='Authorize bounded synthetic inference')
    parser.add_argument('--output', type=Path, required=True, help='Sanitized summary JSON, no raw streams')
    args = parser.parse_args()
    report = {'observed_at': datetime.datetime.now(datetime.timezone.utc).isoformat(),
              'live_requested': args.live, 'timeout_seconds_per_call': TIMEOUT,
              'max_model_turns_per_provider': 2, 'providers': {}}
    with tempfile.TemporaryDirectory(prefix='agentos-probe-') as tmp:
        root = Path(tmp)
        for name, probe in (('codex', run_codex), ('claude', run_claude)):
            if args.provider in (name, 'both'):
                report['providers'][name] = (probe(root, args.live, args.native_codex_auth, args.codex_interrupt_only)
                                              if name == 'codex' else probe(root, args.live))
    args.output.parent.mkdir(parents=True, exist_ok=True)
    args.output.write_text(json.dumps(report, indent=2) + '\n')
    print(json.dumps(report, indent=2))


if __name__ == '__main__':
    main()
