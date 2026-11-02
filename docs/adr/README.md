# Architecture decision records

“Accepted direction” means planning agreement, not implemented software or permission to add packages/migrations.

| ADR | Status | Decision |
| --- | --- | --- |
| [001](001-local-first-modular-monolith.md) | Accepted direction | Go + React/TS, local-first modular monolith |
| [002](002-adapters-and-events.md) | Accepted direction | Capability-aware adapters and normalized events |
| [003](003-outcomes-and-evidence.md) | Accepted direction | Separate activity, workload, progress, acceptance |
| [004](004-storage-and-licensing.md) | Proposed / unresolved | Storage implementation and license selection |
| [005](005-read-only-fleet-preview.md) | Implemented preview | Bounded read-only local Fleet slice |
| [006](006-permission-and-completion-kernel.md) | Implemented kernel | Host authority, registered operations and completion receipts |

New ADRs should state context, decision/status, alternatives, consequences, evidence, and unresolved questions. Supersede rather than silently rewrite historical decisions.
