# Implementation SOP

## Inputs and entry

Requested behavior, acceptance criteria, affected paths/contracts, current revision/tree and existing approvals. Inspect the relevant implementation and tests before proposing replacement code. Recover current state from handoff when continuing prior work.

## Permitted actions

Make task-scoped reversible code/documentation changes, add meaningful tests using approved tools, correct failures caused by the change, and run relevant existing checks. Reuse valid earlier code, fixtures and evidence.

## Prohibited actions

No unrelated refactor, speculative feature, invented task status, silent dependency/tool upgrade, migration without explicit permission, hidden provider-control path or private transcript fixture. Do not change the agreed stack because a skill prefers a different one. Do not weaken tests or browser/provider security to pass a check.

## Procedure

1. Identify the smallest complete slice and its observable acceptance criteria; distinguish implemented behavior from planned modules.
2. Check repository status, relevant contracts and previous results. Record why any completed area needs revisiting.
3. Implement within scope. Ask before a new dependency or protected action; continue independent work while awaiting it.
4. Preserve explicit session/workspace attribution, unknown/stale semantics and metadata-only privacy boundaries. Keep observation separate from control and execution separate from task acceptance.
5. Select affected checks with the verification SOP. Inspect the final diff, correct actual failures and rerun only affected checks.
6. Maintain documentation/API behavior and handoff to match what actually ships. Before an authorized push, check the staged scope and remote state. Do not treat this skill as push/deploy/release authorization.

## Stop or hold

Hold a dependency/migration/security-sensitive expansion until required authorization exists. Report an incompatible requirement rather than silently broadening scope. Explicit stop/priority changes override continued implementation.

## Exit and evidence

Requested slice implemented, relevant checks reported, final diff inspected, limitations explicit, remaining work queued and Git/push state accurate. No claim of full product/adapter support based only on a synthetic preview.
