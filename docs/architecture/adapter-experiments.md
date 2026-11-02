# Adapter experiments — 2026-11-02

Status: executed feasibility probes, not production adapters. Successful machine-local results apply to Codex CLI 0.150.1 and Claude Code 2.1.316. Earlier Claude startup/auth-error observations used 2.1.296; authentication readiness and the installed version changed before the successful run. This probe did not upgrade either CLI. [Sanitized aggregate evidence](adapter-probe-results.json) accompanies this report; no raw prompt/output streams, credentials, session IDs, account details, or local paths are exported there.

## Confirmed capabilities

| Capability | Codex | Claude Code |
| --- | --- | --- |
| Transport startup and structured messages | Verified: dedicated App Server stdio | Verified: stream-json init, partial events, tool output and success result |
| Lifecycle observations | Thread, turn, item, text-delta notifications | SessionStart, PreToolUse, PostToolUse, Stop and SessionEnd hooks |
| Successful synthetic model result | Verified with native CLI authentication | Verified: synthetic file Read and exact marker reply |
| Usage observation | Token usage notification observed; billing accuracy not validated | Usage and client-estimated cost fields observed on successful results; billing accuracy unvalidated |
| Discover owned persisted session | Verified after a turn, in isolated history | Unverified |
| Read history after process reconnect | Verified for own synthetic thread | Explicit CLI resume recalled earlier synthetic marker; general history/list API unverified |
| Continue context | Verified: after process reconnect and thread/resume, a new turn recalled the earlier synthetic marker without it being repeated in the new input | Verified: second process resumed the same owned session and recalled the earlier marker |
| Interrupt active work | Verified when sent on turn/started; terminal status interrupted | Unverified |
| Tool observations | Not tested; no tools requested | Read tool execution plus PreToolUse/PostToolUse verified |
| Discover/control unrelated existing sessions | Not tested or authorized | Not tested or authorized |

## Important findings

1. Codex thread creation alone did not persist a discoverable thread across process restart in the no-inference experiment. After a turn, list/read/resume worked. Session creation and durable observation are different capabilities.
2. The environment-only Codex probe reached a failed terminal turn. That early run did not retain a sanitized root-cause category, so its exact failure cause is unknown. Using the official CLI's existing native authentication through a temporary private auth-file link completed the synthetic marker request. The probe itself never read or copied credential contents into fixtures/evidence.
3. Codex interruption is timing-sensitive. Waiting for the turn/start RPC response resulted in an already-completed turn and error -32600, including with a longer synthetic response. Sending turn/interrupt immediately upon the matching turn/started notification yielded an acknowledged request and interrupted terminal state. Do not map “already finished” to “interrupt unsupported,” or an interrupt acknowledgement alone to terminal cancellation.
4. Claude emitted result subtype success while is_error was true and the process exit code was 1. A consumer must inspect error/exit semantics and actual acceptance checks, not subtype alone. SessionStart/SessionEnd delivery does not establish successful model or tool execution.
5. Claude initially reported loggedIn false in normal and interactive shells. After native CLI login was completed, auth status returned loggedIn true, and both synthetic calls passed. The successful run used 2.1.316 rather than the earlier 2.1.296. Do not attribute the changed result to a version fix: login state changed too. No credentials were requested or printed.

The normal Codex request produced an exact synthetic marker match; the final normal probe also confirmed context recall after process reconnect. The interrupted request correctly did not produce that success marker or completed state. False assertions in raw experiment summaries can therefore be expected for the selected scenario; they are not a universal adapter capability verdict.

## Isolation and budget

- Fresh temporary workspaces, synthetic content, owned test session IDs only. Original agent histories were never listed/read. No existing session was interrupted.
- Codex configuration/history used a fresh CODEX_HOME per experiment. Native authentication retry used a temporary link consumed by the official CLI; no credential extraction, copy, public export, or permission bypass was performed.
- Claude used an explicitly scoped synthetic session, Read as its only available tool, restricted mode, empty external MCP configuration, and explicit probe-only settings. Hook collector retained only fixed lifecycle/tool names, never hook input bodies or transcript paths. Provider-managed policy/auth behavior still applies.
- Transient hook/settings/workspace files and Codex history were removed by temporary-directory cleanup. Claude's own native store may retain the newly created synthetic session history needed for resume; the probe does not bulk-delete provider data.
- 90-second CLI/terminal-wait deadlines with short protocol request timeouts; Claude has a $0.25 per-call CLI budget cap. Codex has no dollar cap enforced by this probe. Each run makes at most two model-turn attempts; the focused interruption mode makes one. Initial failures and targeted interruption diagnosis required multiple runs; the evidence records them separately. Across the recorded runs there were eight Codex turn-start attempts (two initial failures) and four Claude calls (two failed before the auth-readiness guard, then two successful after login); auth-only readiness checks did not start inference. No benchmark or cost comparison is claimed.
- Only probe-owned process groups are terminated for cleanup/timeouts. No global provider settings or repository migrations were changed. No dependencies were installed.

## Reproduce safely

Use a working Python 3 interpreter and already installed provider CLIs. Live commands use provider access and may incur usage; do not run automatically in CI. Output files must be kept private until reviewed for publication. The current Claude probe checks native auth readiness first and skips inference when loggedIn is false. Earlier recorded runs predate that guard and demonstrate error-stream/lifecycle behavior.

```sh
# Offline checks; no provider requests.
python3 -m unittest discover -s scripts/probes -p 'test_*.py'
python3 scripts/check_foundation.py

# Codex protocol-only probe; no model turn.
python3 scripts/probes/adapter_probe.py --provider codex --output /tmp/agentos-codex-local.json

# Explicitly authorized synthetic inference using native Codex CLI authentication.
python3 scripts/probes/adapter_probe.py --provider codex --native-codex-auth --live --output /tmp/agentos-codex-live.json

# Interrupt on turn-start notification, not on turn/start RPC response.
python3 scripts/probes/adapter_probe.py --provider codex --native-codex-auth --codex-interrupt-only --live --output /tmp/agentos-codex-interrupt.json

# Requires native Claude CLI authentication; no API key/token should be pasted into chat.
python3 scripts/probes/adapter_probe.py --provider claude --live --output /tmp/agentos-claude-live.json
```

On Claude 2.1.316, both calls returned exit 0, is_error false, exact synthetic marker matches, and the same owned session identity. Hook counts were SessionStart 2, PreToolUse 1, PostToolUse 1, Stop 2, and SessionEnd 2. The final resumed result reported a cumulative client-side estimate of $0.0191124; it must not be added to the first call's cumulative estimate or treated as a bill.

The normal probe now uses its second turn to test context recall; older evidence records the earlier late-interrupt diagnosis. The probe's zero exit status means it wrote a report, not that all capability checks passed. Inspect scenario-specific checks, terminal state, errors, and missing evidence. The code is a feasibility harness, not a reusable production adapter or general replay engine.

## Verification and remaining work

Offline tests cover malformed/non-object/oversized events, canary redaction, unknown metadata, duplicate/stale/unknown turn observations, missing completion, EOF, bounded read timeout/cleanup, notification timing, hook collector output, and AppleDouble metadata handling in the foundation checker. These establish probe behavior only; they do not verify a future application journal, SSE replay, global discovery, control authorization, or crash recovery.

Next: define the smallest read-only Live Fleet contract using these proven owned-session paths. Claude universal discovery, active interruption, and cross-client control remain unverified. Keep live controls disabled outside the proven owned-session scope. License and dependency choices remain separate decisions.

Provider protocol references: [Codex App Server](https://learn.chatgpt.com/docs/app-server), [Claude hooks](https://code.claude.com/docs/en/hooks), [programmatic Claude](https://code.claude.com/docs/en/headless). Runtime findings above are local experimental observations, not guarantees derived from documentation.
