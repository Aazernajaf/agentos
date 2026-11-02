"""Offline safety and correlation checks; never contact a provider."""
import json
import importlib.util
from pathlib import Path
import queue
import subprocess
import sys
import tempfile
import time
import unittest
from unittest import mock

import adapter_probe as probe


class StreamSafetyTests(unittest.TestCase):
    def test_rejects_invalid_nonobject_and_oversized_unicode(self):
        for value in ('{bad}', '[]', 'null', json.dumps({'x': 'ə' * probe.MAX_LINE})):
            with self.subTest(value_length=len(value)):
                with self.assertRaises(ValueError):
                    probe.decode_line(value)

    def test_secret_canaries_not_exported(self):
        raw = {'method': 'turn/completed', 'params': {'text': 'SECRET_CANARY'},
               'token': 'SECRET_CANARY', 'session_id': 'SECRET_CANARY',
               'error': {'code': -32600, 'message': 'SECRET_CANARY'}}
        safe = probe.safe_shape(raw)
        self.assertNotIn('SECRET_CANARY', json.dumps(safe))
        self.assertEqual(safe['error_code'], -32600)

    def test_unknown_metadata_never_reflects_content(self):
        self.assertEqual(probe.safe_shape({'method': 'SECRET_CANARY'}), {'method': 'unknown'})
        self.assertEqual(probe.classify_error('SECRET_CANARY 401'), ['authentication'])

    def test_bounded_reader_handles_exit_and_timeout(self):
        command = [sys.executable, '-c', 'print(\'{"type":"result"}\')']
        proc = subprocess.Popen(command, stdout=subprocess.PIPE, text=True, start_new_session=True)
        self.assertEqual(probe.collect_events(proc, 2), [{'type': 'result'}])
        proc = subprocess.Popen([sys.executable, '-c', 'import time; time.sleep(10)'],
                                stdout=subprocess.PIPE, text=True, start_new_session=True)
        with self.assertRaises(TimeoutError):
            probe.collect_events(proc, 0.02)
        self.assertIsNotNone(proc.poll())

    def test_missing_auth_preflight_does_not_start_inference(self):
        with tempfile.TemporaryDirectory() as tmp:
            auth = subprocess.CompletedProcess([], 1, '{"loggedIn":false}', '')
            with mock.patch.object(probe, 'version', return_value='fixture'), \
                    mock.patch.object(probe.subprocess, 'run', return_value=auth), \
                    mock.patch.object(probe.subprocess, 'Popen') as start:
                result = probe.run_claude(Path(tmp), live=True)
                self.assertFalse(result['native_auth_ready'])
                self.assertIn('no model call', result['skipped'])
                start.assert_not_called()

    def test_notification_callback_runs_before_pending_response(self):
        rpc = object.__new__(probe.Rpc)
        rpc.failure = None
        rpc.events = []
        rpc.responses = {}
        rpc.incoming = queue.Queue()
        rpc.incoming.put({'method': 'turn/started', 'params': {'turn': {'id': 'synthetic'}}})
        seen = []
        rpc.on_notification = lambda event: seen.append(event['method'])
        rpc.receive(time.monotonic() + 1)
        self.assertEqual(seen, ['turn/started'])
        self.assertEqual(rpc.responses, {})

    def test_foundation_check_skips_appledouble_metadata(self):
        source = Path(__file__).resolve().parents[1] / 'check_foundation.py'
        spec = importlib.util.spec_from_file_location('foundation', source)
        module = importlib.util.module_from_spec(spec)
        spec.loader.exec_module(module)
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            for name in module.REQUIRED:
                target = root / name
                target.parent.mkdir(parents=True, exist_ok=True)
                target.write_text('# Synthetic document\n')
            (root / '._README.md').write_bytes(b'\x00\xff\x01')
            (root / 'node_modules').mkdir()
            (root / 'node_modules' / 'README.md').write_bytes(b'\xff')
            self.assertEqual(module.check(root)[1], [])
            (root / 'README.md').write_text('[broken](missing.md)\n')
            self.assertTrue(module.check(root)[1])

    def test_turn_correlation_ignores_duplicates_old_and_unknown_events(self):
        rpc = object.__new__(probe.Rpc)
        rpc.events = [
            {'method': 'turn/completed', 'params': {'turn': {'id': 'old', 'status': 'completed'}}},
            {'method': 'turn/completed', 'params': {'turn': {'id': 'old', 'status': 'completed'}}},
            {'method': 'unknown', 'params': {}},
            {'method': 'turn/started', 'params': {'turn': {'id': 'current'}}},
            {'method': 'turn/completed', 'params': {'turn': {'id': 'current', 'status': 'interrupted'}}},
        ]
        self.assertEqual(rpc.wait_turn('current')['status'], 'interrupted')

    def test_missing_completion_is_timeout_not_success(self):
        rpc = object.__new__(probe.Rpc)
        rpc.events = []
        rpc.failure = None
        rpc.on_notification = None
        rpc.incoming = queue.Queue()
        with self.assertRaises(TimeoutError):
            rpc.wait_turn('missing', timeout=0.01)

    def test_closed_stream_is_failure(self):
        rpc = object.__new__(probe.Rpc)
        rpc.failure = None
        rpc.on_notification = None
        rpc.incoming = queue.Queue()
        rpc.incoming.put(None)
        with self.assertRaises(RuntimeError):
            rpc.receive(time.monotonic() + 1)

    def test_hook_collector_redacts_and_rejects_oversized_input(self):
        with tempfile.TemporaryDirectory() as tmp:
            script = Path(tmp) / 'collector.py'
            script.write_text(probe.HOOK_COLLECTOR)
            raw = json.dumps({'hook_event_name': 'PostToolUse', 'tool_name': 'Read',
                              'tool_input': {'secret': 'SECRET_CANARY'},
                              'transcript_path': '/private/SECRET_CANARY',
                              'session_id': 'SECRET_CANARY'})
            result = subprocess.run([sys.executable, str(script)], input=raw,
                                    text=True, capture_output=True, timeout=5)
            self.assertEqual(result.returncode, 0)
            output = (Path(tmp) / 'hook-events.jsonl').read_text()
            self.assertNotIn('SECRET_CANARY', output)
            self.assertEqual(json.loads(output), {'hook_event_name': 'PostToolUse', 'tool_name': 'Read'})
            result = subprocess.run([sys.executable, str(script)], input='x' * (probe.MAX_LINE + 1),
                                    text=True, capture_output=True, timeout=5)
            self.assertNotEqual(result.returncode, 0)


if __name__ == '__main__':
    unittest.main()
