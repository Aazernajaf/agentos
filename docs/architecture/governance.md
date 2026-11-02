# Permission and completion kernel

Status: implemented Go in-process kernel and runnable synthetic demo. This is the first enforcement slice, not universal interception of Codex, Claude Code or Antigravity tools. Live Fleet observation remains read-only and does not expose grant or provider-control endpoints. Optional project-result acceptance is described below.

The [project submission verifier](project-task-review.md) binds real artifact bytes and executed project tests to mandatory operator review, including an optional same-origin Fleet approval panel. These local interfaces do not authenticate human identity against software controlling the same OS account. The default Fleet server still has no approval endpoint unless explicitly configured.

## Run the enforced flow

```sh
go run ./cmd/agentos-policy-demo
```

The demo loads the current repository's canonical implementation skill/SOP and root policy. It exercises actual gate methods with in-memory effects: an unapproved migration-shaped operation is denied before its callback runs, one exact operator approval permits it once, replay is blocked, a completion claim is denied, trusted checks produce a receipt, operator review is required, and the synthetic task is accepted. No migration directory/file, database, provider session, model call or external service is touched. The receipt token is not printed.

## Trust and integration contract

`internal/governance.New(Config)` returns two separate object capabilities:

- **Runtime:** agent-facing `Contract`, `Dispatch`, `Submit`, `Verify`, `Accept` and metadata `Snapshot`.
- **Operator:** trusted host-only `Approve`, `Review`, `Pause`, `Resume`, `Cancel` and `Rebase`.

Keep Operator inside authenticated host code. Do not expose it as an agent tool, create it from provider events, or let an agent supply Config, handlers, checks or approvals. A host must authenticate/route the actual calling session; task/session strings are scope identifiers, not authentication credentials. Runtime is scoped to one assigned task/session. A copy of Contract provides metadata, not operator authority.

The host registers fixed operation IDs, real operation kinds, exact logical resources and trusted handlers. An agent request contains only request ID, task/session, revision, pinned skill hashes and registered operation ID. `DecodeRequest` rejects unknown/duplicate/case-aliased keys, appended JSON, malformed/oversized input, claimed grants/check results and arbitrary command/resource fields. Dispatch calls a registered handler only after admission checks. No shell-command evaluator or arbitrary plugin execution is added.

Handlers/check functions are trusted code: they must accurately declare resources, preserve provider approvals, obey cancellation and use suitable filesystem/subprocess isolation. The kernel cannot infer hidden effects of a mislabeled handler or sandbox native code. A capability-verified marker is a host-owned prerequisite, not an automatic provider probe; version/ownership evidence is still the host/adapter's responsibility.

## Operation admission

Each request must match the immutable task/session contract, current trusted revision callback and pinned instructions/SOP/policy hashes. `LoadSkill` reads only bounded canonical regular files under an `os.Root`: selected SKILL.md, own SOP.md, AGENTS.md and docs/agents/operating-policy.md. Symlinked canonical components, missing and oversized files are rejected. A matching hash acknowledges a version; it does not prove a model read or understood the text.

Task scope and the operation allowlist are fixed at construction. Scope uses exact relative logical resources or descendants, with traversal, absolute paths, backslashes and Git metadata rejected. Logical resource checks are not an OS filesystem sandbox. The host must derive revision from all meaningful current inputs, including dirty worktree/artifacts where relevant; a constant or HEAD-only callback cannot establish unchanged working content.

Safe read/write/check operations still need explicit task allowlisting and scope. Protected kinds require a separate host approval: dependency, migration, provider control, push, deploy, release, new session, external message and policy edit. Sensitive resources also add categories even if the host labeled the operation Write:

- Any migrations path component requires migration approval.
- Known Go/JS manifests, lock/config files and node_modules/vendor writes require dependency approval.
- AGENTS.md, CLAUDE.md, canonical skill/native-alias roots and docs/agents require policy-edit approval.

Classification handles case aliases conservatively. It is defense in depth for known resource patterns, not discovery of arbitrary command effects. Every applicable category is required; dependency approval cannot authorize a migration. Approval binds one exact request/operation/revision/skill contract and is consumed before execution. Unknown or unverified provider controls remain denied.

Only one operation may run per Runtime at once. Used request IDs cannot be replayed. A host can set `OncePerRevision` for an operation whose successful result should be reused: a new request ID cannot repeat it under the same revision/skill binding, including after an unchanged rebase. Do not apply this option to observations that legitimately need refreshing.

## Completion and evidence

```text
active → submitted → verifying → verified → accepted
```

Submission is not acceptance. An unexpired verified result cannot be resubmitted merely to repeat the same checks; reuse the current receipt. Expired evidence can be refreshed within the remaining verification budget. Verify runs every required, host-registered check. Required checks cannot be empty, duplicated, selected by the agent or supplied as claimed pass results. Failure, panic, cancelled context, changed revision/policy or changed generation prevents receipt issuance. Current checks must represent real acceptance criteria; an always-successful host callback is not meaningful evidence.

A successful batch issues a random, process-local receipt retained by the Runtime and bound to its task, generation, revision, skill/SOP/root-policy content and timestamp. Accept rejects unknown/stale/expired receipts, clock rollback, altered current inputs and work still in flight. Receipt tokens are omitted from snapshots/audits and cannot be imported from another Runtime. They are not portable signed evidence.

Acceptance mode must be explicit:

- `OperatorReview`: the trusted Operator must review this exact current receipt before Accept.
- `TrustedChecks`: the host explicitly permits automatic acceptance after all trusted checks. An omitted/unknown mode is rejected.

Operator review establishes host approval, not an independently authenticated human identity inside this library. The integrating application must enforce that distinction and retain the user's actual authorization. Accepted tasks are terminal and remain bound to their accepted revision; later work needs a new contract rather than treating old evidence as current.

## Stop, failure and budgets

Pause/cancel invalidates pending approvals and proof and signals cancellation to in-flight work/checks. No new operation/completion can pass while paused/cancelled. Cancel is terminal. Resume requires an explicit prior pause and does not refill budgets. Cancellation is cooperative; it cannot undo effects already performed or forcibly stop native callbacks that ignore context.

A failed/timed-out/panicking operation enters `needs_reconciliation`. Further dispatch/submission is held until the trusted host inspects uncertain effects and calls Rebase against actual current inputs. Resume alone does not remove that hold. Failed verification returns to active work without a receipt; it does not accept the task.

Rebase acknowledges current revision and policy content, invalidates grants/proof and preserves action/check counts and completed once-per-revision results. It cannot expand scope/operation/check lists. MaxActions and MaxVerifications count admitted attempts across retries, pauses and rebases; failed callbacks/checks do not refund them. Context deadlines bound cooperative execution; they are not dollar caps or forced process termination.

State is intentionally in memory: a host must retain the Runtime across provider retries. Budgets, grants and audit do not survive host restart. Do not use a new Runtime to silently reset an existing logical task's budget. Durable identity/receipt storage, restart-safe budgets and revocation require a later persistence design; no migrations or database driver were introduced.

Audit retains the latest 256 metadata decisions, including operation admission, check outcomes, review and acceptance. Request IDs are hashed for correlation and unknown operation names are replaced with a fixed label. It excludes raw output/error text, commands and receipt secrets. This is a bounded volatile decision log, not tamper-proof durable audit.

## Checks and current limits

```sh
go test -race ./...
go vet ./...
go run ./cmd/agentos-policy-demo
```

Tests cover deny-before-effect, protected-resource relabeling, exact/single-use approval, concurrent replay, all-category consent, wrong task/session/revision/skill, file-content drift, failed/panicking checks, missing/expired/forged/replayed receipts, explicit review policy, cancellation/in-flight completion, unchanged-result reuse, reconciliation, preserved retry budgets, strict request decoding, canonical file bounds and input changes during verification.

This kernel does not parse natural-language goals, guarantee semantic SOP obedience, create chats, schedule agents or mark existing Fleet observations accepted. It does not intercept unmediated external CLI/shell activity. Next integration work is an authenticated broker that registers real owned-session operations and trusted verifier runners through this kernel, followed by durable state and an operator review UI. Permission rules and SOP guidance remain complementary.

The canonical policy reader uses Go's [os.Root](https://pkg.go.dev/os#Root) as available in the project's installed Go toolchain; handler sandboxing is a separate responsibility.
