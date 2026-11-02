# Product strategy and scope

Status: direction accepted for planning; features unimplemented. The latest sequence in the prior product discussion supersedes earlier broader v0.1 ideas.

**Promise:** make agent work understandable and verifiable across providers. Activity is not productivity. AgentOS should work locally without its own account and without mandatory Linear, Langfuse, AgentMonitor, Grafana, or hosted telemetry. Provider authentication and costs still apply.

## Delivery sequence

| Stage | Outcome | Acceptance target |
| --- | --- | --- |
| Phase 0 | Credible foundation and validated integration assumptions | Documents, decisions, threat model, synthetic event contract, and versioned adapter experiments; unresolved items explicit |
| v0.1 Live Fleet | Observe opted-in Claude Code and Codex sessions | Both providers visible; source/freshness shown; reconnect behavior and gaps tested; local history policy documented |
| v0.2 Workload | Understand queue, capacity, and blockers | Slot utilization separated from progress; dependency cycles rejected; every measure has a source |
| v0.3 Proof of Work | Accept results against evidence | Required checks and review bound to revision; failed/stale/missing evidence blocks acceptance |
| v0.4 Coordination | Transfer work safely | Structured handoff and capability-aware, authorized controls |
| Public beta / v1.0 | Reliable community use | Fresh macOS/Linux install, privacy and security checks, license, release/upgrade policy; stable API/SDK only when proven |

These are scope milestones, not delivery dates. First useful observation within five minutes is a usability target, not a measured result. Windows assessment, hosted multi-user mode, Kubernetes, marketplace, AI scheduling, mandatory project connectors, and universal session takeover are outside initial scope.

## Current governance slice

An [in-process permission/completion kernel](../architecture/governance.md) now gates registered host operations and revision-bound acceptance evidence. This is groundwork for v0.3/v0.4, not a claim that external CLI tools, operator UI, persistence or the full milestones are integrated.

## Additional provider queue

Antigravity CLI is a requested feasibility candidate. Verify its exact distribution/version and interfaces before adding scope or advertising support. Existing v0.1 Codex/Claude targets remain distinct from that unverified candidate.

## Metrics with explicit meaning

- Capacity utilization = active execution slots / configured parallel slots. Undefined if capacity is unknown or zero; not model intelligence or CPU usage.
- Task progress = verified completed scope weight / total agreed scope weight. Undefined for absent/zero scope; version the scope and show changes.
- Queue depth and blocked duration come from known assignments and explicit state transitions. A silent agent is not automatically blocked.
- Outcome measures include accepted work, time to acceptance, rework, and cost per accepted task when cost is actually available. Do not rank unlike tasks or fabricate missing cost.

## Proof of Work

Draft lifecycle: `todo → ready → running → submitted → verifying → accepted`; explicit alternatives include `blocked`, `failed`, `cancelled`, and `changes_requested`. Acceptance requires all configured criteria and human approval where required. Record actor, revision, evidence digest, check command/environment/exit result, and time. Changed revision invalidates affected evidence. Agent text, process exit, a commit, and a passing test are distinct facts; none alone guarantees correctness.

Pilot work on another repository requires a separate scope and that repository's audit. This foundation does not authorize changes to Fenix or any infrastructure.
