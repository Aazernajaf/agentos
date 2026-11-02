# Adapter feasibility

Reviewed 2026-11-02. This is documentation and local CLI surface inspection, not an end-to-end integration result. No private sessions, credentials, provider settings, or hooks were read or changed; no model runs were started.

## Evidence and limits

| Provider | Candidate observation path | Candidate control path | Evidence today | Unproven |
| --- | --- | --- | --- | --- |
| Codex | App Server thread queries and notifications | App Server turn operations | Official protocol docs; local CLI 0.150.1 help and generated JSON schemas | Cross-client session visibility, permissions, notification coverage, reconnect/replay, real execution |
| Claude Code | Explicitly installed opt-in hooks; JSON stream for AgentOS-launched CLI runs | New CLI runs and documented resume flows in a controlled workspace | Official hooks/programmatic docs; local Claude Code 2.1.296 help | Universal discovery, visibility into unrelated terminals, safe live control, event reliability |

Codex generated schema inspection found ThreadListParams/Response, ThreadReadParams/Response, TurnStartParams/Response, TurnInterruptParams/Response, ItemStartedNotification, and TurnCompletedNotification. Generation alone proves neither successful requests nor permission to control existing threads. The installed CLI labels App Server tooling experimental.

Claude hooks offer lifecycle/tool observations including SessionStart, PreToolUse, PostToolUse, and Stop. Programmatic CLI output supports structured streaming. These are opt-in integration paths, not proof of an ambient fleet API. Resuming conversation context is not equivalent to pausing/resuming a running process. Hook delivery can be incomplete; source silence must produce stale/unknown state.

## Capability contract direction

Represent each capability as `unsupported`, `unverified`, or `verified`, scoped to provider/version/transport/session ownership with evidence and observation time. Separate discovery, live events, history, start, interrupt, continue-context, pause-running, send-input, and usage metrics. Disable controls unless runtime capability is verified and the user is authorized. No generic pause button based on signals or terminal injection.

Prefer documented transports. Avoid scraping private internal databases or transcript directories as the default adapter. No SDK dependency is required merely to investigate CLI protocols. Reject unknown fields safely, bound input sizes, and expose degraded operation.

## Next experiment: disposable workspace only

1. Record exact CLI versions and generated protocol shape; compare with current official docs.
2. Use synthetic repository and task content, dedicated provider session, explicit test budget and authorization before model execution.
3. Codex: initialize a dedicated server; exercise list/read against synthetic threads, observe item/turn completion, disconnect/reconnect, and interrupt only its own disposable run. Record permissions and gaps.
4. Claude: review a temporary project-scoped hook configuration before installation; collect synthetic lifecycle events, remove configuration after test, and compare structured CLI stream coverage. Test failure/timeout and incomplete shutdown.
5. Validate duplicate, out-of-order, missing, unknown, and oversized events; redact fixtures. Confirm unknown usage stays unknown.
6. Publish versioned capability results and supported scope. Never generalize launched-session support to all pre-existing sessions.

## Official sources

- [Codex App Server](https://learn.chatgpt.com/docs/app-server): thread/turn protocol and notifications.
- [Claude Code hooks reference](https://code.claude.com/docs/en/hooks): lifecycle integration and hook inputs.
- [Run Claude Code programmatically](https://code.claude.com/docs/en/headless): CLI execution and structured output.

Documentation can change independently of installed versions. Revalidate before implementation. No provider is advertised as runtime-supported yet.
