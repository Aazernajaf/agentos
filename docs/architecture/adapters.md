# Adapter feasibility

An experimental [owned-check gate integration](adapter-gates.md) has offline process-boundary tests and passing fixed synthetic goals for Codex 0.150.1 and Claude Code 2.1.316. This is not general supported provider control or coding-task acceptance.

Updated 2026-11-02. The original surface inspection has been followed by scoped synthetic experiments; see [executed results](adapter-experiments.md) and [sanitized evidence](adapter-probe-results.json). No production adapter is implemented. Existing private sessions were not inspected or controlled.

## Evidence and limits

| Provider | Candidate observation path | Candidate control path | Evidence today | Unproven |
| --- | --- | --- | --- | --- |
| Codex | App Server thread queries and notifications | Start, continue own thread; interrupt on turn-start notification | CLI 0.150.1: synthetic reply, events, usage notification, persisted list/read/resume, successful interruption | Unrelated-client session visibility, approval flows, replay/gap recovery, version ranges |
| Claude Code | Explicitly installed opt-in hooks; JSON stream for AgentOS-launched CLI runs | CLI run/resume candidates | CLI 2.1.316: synthetic Read, structured/partial events, all five test hooks, context recall on explicit resume and usage/cost fields; earlier 2.1.296 auth-error observations retained | General discovery/history API, billing accuracy, active interruption and cross-client controls |

**Requested candidate: Antigravity CLI.** Exact distribution/executable, version, native skill discovery, observation and control capabilities remain unverified. Keep it in the feasibility queue; no installation, integration, SDK or support claim is implied. Apply the [adapter feasibility SOP](../../.agents/skills/agentos-adapter-feasibility/SOP.md) before experiments.

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

Documentation can change independently of installed versions. Revalidate before implementation. These results establish feasibility only for the tested synthetic scope; neither provider is advertised as a supported production adapter.
