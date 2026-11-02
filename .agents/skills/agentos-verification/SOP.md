# Verification SOP

## Inputs and entry

Acceptance criteria, actual diff, current revision/inputs, prior command results and existing check tooling. Establish which prior evidence remains valid before rerunning anything.

## Permitted actions

Run existing relevant checks, review the diff and artifacts, exercise synthetic negative cases, and report coverage gaps. Correct task-scoped failures when authorized; a report-only review does not grant edit permission.

## Prohibited actions

No invented pass, skipped-failure concealment, weakened assertion, real private fixture, paid inference as routine lint, unapproved test package or blanket production-readiness claim. Do not rerun all suites after an unrelated wording-only change merely to show activity.

## Procedure

1. Map acceptance criteria to actual observable evidence. Include failure/privacy/permission boundaries when affected; avoid tests mirroring implementation text.
2. Reuse prior checks only for unchanged relevant inputs/revision. Invalidate affected evidence after code, configuration, dependency or source-version changes.
3. Select the smallest relevant existing checks: documentation/skill packages use their link/integrity checks; Go boundaries use tests/vet; frontend changes use strict typecheck/build and appropriate UI checks; ingress/server changes use synthetic built-server smoke.
4. For broad runtime changes, apply the repository's full relevant check set. Mark unavailable tooling and unconfigured CI explicitly instead of silently installing tools.
5. Inspect outcomes and artifacts. Fix authorized failures, then repeat affected checks once there is a material reason. Keep total budgets across retries.
6. Before an authorized push, inspect staged scope, exclude private scratch/build/credential material, and verify current remote state. Push only within the session's authorization and risk limits.

## Stop or hold

Hold acceptance on failed/missing required evidence. Hold protected actions on missing permission. A useful partial finding can be reported, but do not label the active task complete while required independent work remains.

## Exit and evidence

Report exact commands, results, relevant revision/inputs, skipped checks with reasons and material unverified behavior. Tests, lint, review, provider completion and user acceptance are separate facts. Record what a later agent can reuse without repeating work.
