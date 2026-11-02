#!/usr/bin/env python3
"""Exercise the built local server with synthetic stdin, never a real provider."""
import argparse
import http.cookiejar
import json
from pathlib import Path
import signal
import subprocess
import time
import urllib.error
import urllib.request

ROOT = Path(__file__).resolve().parents[1]


def probe(binary, provider, records, expected):
    process = subprocess.Popen(
        [str(binary), '--listen', '127.0.0.1:0', '--web-dir', str(ROOT / 'apps/web/dist'),
         '--provider', provider, '--provider-version', 'synthetic', '--session', 'owned'],
        cwd=ROOT, stdin=subprocess.PIPE, stdout=subprocess.PIPE, stderr=subprocess.DEVNULL,
        text=True)
    try:
        url = process.stdout.readline().strip().split(' — ')[1]
        client = urllib.request.build_opener(urllib.request.HTTPCookieProcessor(http.cookiejar.CookieJar()))
        try:
            client.open(url + '/v0/fleet', timeout=3)
            raise AssertionError('unauthenticated API exposed')
        except urllib.error.HTTPError as error:
            assert error.code == 401
        with client.open(url, timeout=3) as response:
            assert response.status == 200
        for record in records:
            process.stdin.write(json.dumps(record) + '\n')
        process.stdin.close()
        deadline = time.monotonic() + 5
        while True:
            with client.open(url + '/v0/fleet', timeout=3) as response:
                text = response.read().decode()
            snapshot = json.loads(text)
            if snapshot['sessions'] and not snapshot['sessions'][0]['source_connected']:
                break
            if time.monotonic() > deadline:
                raise AssertionError('stdin EOF not observed')
            time.sleep(0.02)
        assert snapshot['mode'] == 'observe'
        assert len(snapshot['sessions']) == 1
        row = snapshot['sessions'][0]
        assert row['execution_state'] == 'unknown'
        assert row['last_known_execution_state'] == expected
        assert 'SECRET_CANARY' not in text and 'accepted' not in row.values()
    finally:
        process.send_signal(signal.SIGINT)
        try:
            process.wait(timeout=5)
        except subprocess.TimeoutExpired:
            process.kill()
            process.wait(timeout=3)
        process.stdout.close()


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--binary', type=Path, default=ROOT / 'work/agentos')
    args = parser.parse_args()
    binary = args.binary.resolve()
    probe(binary, 'codex', [
        {'method': 'turn/started', 'params': {'threadId': 'owned', 'turn': {'id': 'one'}, 'text': 'SECRET_CANARY'}},
        {'method': 'turn/completed', 'params': {'threadId': 'owned', 'turn': {'id': 'one', 'status': 'completed'}}},
        {'method': 'turn/completed', 'params': {'threadId': 'foreign', 'turn': {'id': 'other', 'status': 'failed'}}},
    ], 'completed')
    probe(binary, 'claude', [
        {'type': 'system', 'subtype': 'init', 'session_id': 'owned'},
        {'type': 'stream_event', 'session_id': 'owned', 'event': {'type': 'message_start'}},
        {'type': 'result', 'subtype': 'success', 'session_id': 'owned', 'is_error': True, 'result': 'SECRET_CANARY'},
    ], 'failed')
    print('PASS: synthetic Codex/Claude stdin → scoped projection → authenticated HTTP; EOF, error semantics and redaction')


if __name__ == '__main__':
    main()
