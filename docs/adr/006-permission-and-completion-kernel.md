# ADR-006: Separate agent intent from host authority

Date: 2026-11-02. Status: implemented in-process kernel and synthetic executable; provider/tool/UI integration pending.

## Context

Instruction-only skills cannot block unauthorized callbacks or establish that work passed actual checks. The current Fleet is intentionally read-only, so enabling broad remote write/control routes would expand its trust boundary prematurely.

## Decision

Add a standard-library Go kernel with a scoped agent Runtime and a separate host-held Operator capability. Admit only host-registered operations after task, session, revision, scope, skill/SOP/root-policy and approval checks. Protected resource categories cannot be downgraded by a request's claimed kind. Agent requests carry no commands, resources, grants or check results.

Run host-owned required checks and issue process-local, generation-bound receipts. Require an explicit acceptance mode; operator review is required when selected. Preserve retry counts across rebases/pauses and hold uncertain failed effects for reconciliation. Keep provider completion observations separate from task acceptance.

## Alternatives and consequences

A JSON `approved` flag or agent-reported test result would let the requesting agent create its own authority/evidence. A shell-text classifier cannot establish all command effects. Pure advisory checks would leave a bypass between a verdict and handler entry. The chosen dispatcher owns handler admission and acceptance state, while trusting registered host code to declare/perform scoped effects and obey cancellation.

This library is not an OS sandbox, human authentication system or cross-process permission service. In-memory state avoids a premature storage dependency but cannot preserve budgets/audit through host restart. No provider control/mutation HTTP endpoint is added. See [implementation contract](../architecture/governance.md) for current limits and the authenticated broker integration requirement.
