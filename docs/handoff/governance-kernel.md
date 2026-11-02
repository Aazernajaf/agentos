# Permission and completion kernel handoff

Date: 2026-11-02. Scope: enforce named permission/evidence rules in AgentOS-managed callbacks; preserve the existing read-only Fleet.

## Actual changes

- New `internal/governance` package: host-owned contracts/operation registry, separate Operator authority, pinned skill/SOP/root-policy hashes and current revision checks.
- Deny-before-callback operation dispatch, exact single-use protected approvals, sensitive-resource classification, one in-flight operation, request replay protection and optional completed-result reuse.
- Host-run required acceptance checks, opaque internal receipts, explicit review/automatic modes, expiry/drift rejection and terminal accepted state.
- Cooperative pause/cancel, held uncertain effects, host reconciliation, action/check retry budgets preserved across pause/rebase and bounded metadata audit.
- Strict request decoding and bounded canonical policy-file reads, with no imported agent approvals, check claims or arbitrary commands.
- Runnable `agentos-policy-demo` proves denied and approved flows using synthetic in-memory effects only. No real migration, provider control or model call occurs.

## Verification

- `GOTOOLCHAIN=local GOPROXY=off go test -race ./...`: all 41 Go test functions passed, including 29 new governance tests with negative/concurrency subcases. Existing Fleet tests were included and reused valid cache entries where inputs were unchanged.
- `GOTOOLCHAIN=local GOPROXY=off go vet ./...`: passed.
- `GOTOOLCHAIN=local GOPROXY=off go run ./cmd/agentos-policy-demo`: unapproved action blocked, one approved callback, replay blocked, claimed completion blocked, review required and final synthetic accepted state confirmed. No real migration or external operation performed.
- Foundation links, five skill/SOP packages, seven Python package tests and Git whitespace checks passed.
- Frontend build/browser checks were not rerun: no frontend, HTTP or observation code changed. No live provider probe or model inference was run.

The tests exercise denied effects and permitted completion, concurrency, actual file-content drift, replay, false claims, cancellation, timeout/panic, reconciliation, repeated-evidence rejection, identifier redaction and retry-budget boundaries. Checks do not establish external CLI interception or human authentication.

## Limits and next work

The kernel enforces only calls routed through its Runtime. It does not intercept native provider/shell tools, parse every SOP sentence, authenticate people, persist state across host restarts or expose an approval UI/API. Session/task IDs are identifiers; the integrating host must authenticate the caller, retain Operator authority and register truthful, isolated/cancellable handlers and real verifier functions. Skill hashes do not prove model understanding.

Next: connect an authenticated owned-session tool broker and verifier runner without exposing operator methods to agents; then choose persistence for restart-safe budgets/receipts/audit and add operator review UI. Antigravity identity/capability evaluation remains queued. No dependency, migration, deployment or release was added. Existing source license/security-contact decisions remain open.

## Rollback

Revert the source change to remove this kernel/demo. No database rollback, provider settings cleanup or service deployment is involved. Demo state and receipts are in memory and disappear on exit.
