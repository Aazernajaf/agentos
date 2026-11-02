# Owned provider check gates — experimental, pinned synthetic goals verified

Updated 2026-11-02. This slice connects the governance kernel to actual CLI process runners, without SDKs or dependencies. Both pinned providers passed the fixed synthetic goal. It is not a production adapter or general coding executor. OpenClaw remains an optional future integration, outside the core dependency graph. Antigravity remains unverified.

## Implemented boundary

`cmd/agentos-adapter-check` registers a fixed `new_session` operation. No CLI process starts before `Runtime.Dispatch` receives the host's exact single-use approval. Agent input cannot select commands, prompts, grants or required checks. The host-only command uses a fixed synthetic reply criterion and explicit `TrustedChecks` acceptance; it does not approve real project work or expose Operator authority through Fleet.

Default invocation is offline and proves denial with zero runner calls:

```sh
go run ./cmd/agentos-adapter-check --provider codex
go run ./cmd/agentos-adapter-check --provider claude
```

Live invocation requires both `--live` and `--approve-new-session`, authorizing one new disposable check only. It has a 90-second deadline and Claude's additional $0.25 CLI budget. Do not run live commands in routine verification or reset a session's agreed experiment budget by rerunning the executable.

The runners pin Codex 0.150.1 and Claude Code 2.1.316. They use private temporary workspaces, bounded output, a minimal environment, native authentication and no imported history. Codex's temporary auth symlink is read only by the official CLI; AgentOS never reads/copies its contents. Claude disables tools/customizations and persistence; Codex disables listed tool features and uses read-only mode. These settings are not a proven universal tool sandbox. Unexpected tool events fail the check, which is detection rather than prevention of an already executed tool. Never use this harness for untrusted arbitrary prompts.

Required checks use host-held process evidence: successful exit, valid terminal stream, session attribution, exact expected reply and an unchanged empty workspace. Codex attribution is the dedicated owned process plus its first thread ID, not ambient discovery. Missing, malformed, oversized, duplicate-terminal, tool-bearing or wrong-outcome streams fail closed. The pinned Codex version's completed `error` items represent nonfatal warnings/deprecations; they are recorded as metadata and never constitute proof. Top-level fatal errors and explicit dropped/lagged event warnings still invalidate evidence. The revision digest binds the fixed synthetic job contract, not a Git commit or arbitrary project artifacts. Skill/SOP/policy hashes are reloaded by the kernel. Receipts remain in memory; no restart durability is claimed. Only the fixed synthetic goal may be accepted.

## Executed evidence and blocker

Offline process integration tests exercise approval denial before launch, approved fake CLI execution, replay denial, forged completion denial, trusted acceptance and wrong-outcome rejection. Protocol fixtures cover both providers and misleading Claude `subtype: success` with `is_error: true`.

Initial live testing used two attempts per provider. A subsequent user-authorized budget allowed two additional attempts per provider, all used. No task was accepted. Raw output and private account data were not saved. The follow-up established:

| Provider | Follow-up evidence | Status |
| --- | --- | --- |
| Codex 0.150.1 | First run produced no events. A separate offline `debug prompt-input` configuration check (no authentication/model call) returned the exact fixed error that `approval_policy = "untrusted"` is no longer supported. Changed to `never`, retaining read-only mode and disabled tool features. Second run emitted recognized event types but failed strict parsing. | Startup configuration defect fixed; remaining stream mismatch unresolved |
| Claude Code 2.1.316 | Native auth preflight succeeded. First run emitted recognized events but failed validation. Second run contained a provider result with `is_error: true`. | Provider error confirmed; underlying error category unresolved |

Diagnostics now export fixed classifications only. In particular, nonzero exit after a parser cancellation is not evidence of an independent provider startup failure. The new `parser_cancelled` field makes that distinction explicit for future runs; it was added after the follow-up and has only offline validation. The available attempts did not capture enough allowlisted detail to identify the remaining Codex mismatch or Claude error reason. No further inference was attempted, and no push was made.

## Second follow-up: bounded diagnostics and Codex success

The user authorized another two attempts per provider, all used. Output is now collected in bounded memory through process completion before parsing, so parser rejection cannot mask the native exit status by prematurely killing the child. Only fixed event labels, ownership booleans and words from an explicit diagnostic vocabulary leave memory. No raw stream is retained.

- Codex attempt 1: native exit 0, exact expected reply and terminal completion, but two `item.completed/error` items caused strict rejection. The [official source pinned to rust-v0.150.1](https://github.com/openai/codex/blob/rust-v0.150.1/codex-rs/exec/src/event_processor_with_jsonl_output.rs) maps warnings/config warnings/deprecations to these item events and fatal errors to top-level `error`/`turn.failed`. The parser now follows that distinction while rejecting explicit event gaps.
- Codex attempt 2: exit 0, valid stream, exact outcome and empty workspace; unapproved launch blocked, forged completion blocked, trusted check receipt accepted. This verifies only the fixed prompt-only synthetic goal, not general coding work or all external tools.
- Claude attempts 1 and 2: native auth preflight passed, owned init, rate-limit event, assistant response and `result/success` with `is_error: true`; native exit nonzero. Allowlisted error vocabulary contained `limit`. This is a limit-related provider failure, not accepted work. The captured vocabulary does not establish the exact quota type, reset time or billing cause. No credential change, account switching or bypass was attempted.

## Final follow-up: Claude success after reset

A read-only official Usage-page check established that the current session allowance was exhausted while the weekly allowance was not. No credentials were extracted and no model call was made for this lookup; personal quota details are not retained in the repository.

After the displayed reset, the user authorized exactly one further Claude call. At 2026-11-02 14:24 Asia/Baku, Claude Code 2.1.316 passed: native exit 0, owned init/result session, `is_error: false`, valid stream, exact synthetic outcome and empty workspace. The real governance path blocked unapproved launch and forged completion before issuing and accepting a trusted-check receipt. The prior Codex passing run was not repeated. Both installed versions therefore have positive live evidence for this fixed scope.

Final checks: `go test -race ./...`, `go vet ./...`, policy demo, foundation/skill checks, both existing Python unittest suites and `git diff --check`. Frontend and HTTP behavior are unchanged; frontend build/typecheck and browser smoke are outside this diff. Fleet controls remain disabled. Further work is a separately scoped broker for real project operations/artifact criteria and explicit operator review, not another repetition of these synthetic calls.

Protocol references: [Codex non-interactive mode](https://learn.chatgpt.com/docs/non-interactive-mode), [Claude Code CLI](https://code.claude.com/docs/en/cli-reference). Installed CLI help was checked; online documentation does not establish runtime success.
