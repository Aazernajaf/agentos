# Architecture overview

Status: architecture direction; the [read-only Fleet preview](live-fleet.md) implements the first in-memory slice, with the wider architecture still planned. See [ADRs](../adr/README.md).

## Shape and boundaries

One Go process hosts a modular monolith with a React/TypeScript browser UI. Keep provider-specific behavior behind adapters. REST serves queries and deliberate commands; SSE carries read-side updates. WebSocket, separate workers, and SDK packages require demonstrated need.

Planned layout (create source directories only when they hold real implementation):

```text
cmd/agentos/          Go entry point
internal/api/        HTTP boundary, validation, local session security
internal/governance/ Implemented in-process operation and acceptance gates
internal/agents/     Registry, sessions, capability observations
internal/events/     Normalization, journal, projections, replay
internal/tasks/      Scope, assignments, dependencies, transitions
internal/workload/   Source-backed capacity and queue measures
internal/verification/ Revision-bound evidence and acceptance
internal/storage/    Persistence interface
adapters/codex/      Codex protocol translation
adapters/claude/     Claude hook/stream translation
apps/web/            React + TypeScript UI
```

Domain services own invariants; adapters never write task acceptance directly. UI state cannot grant capabilities. Storage and HTTP are replaceable boundaries, not the domain model. Avoid a generic plugin runtime until two adapters establish a useful contract.

## Data flow

Provider observation → size/schema validation → workspace attribution → redaction → normalized event → durable journal → projection → REST/SSE → UI. Record source and receive times separately. Unattributed events must not silently enter another workspace. Preserve normalized history subject to retention/deletion policy; do not promise permanent raw transcript retention.

Core concepts: workspace, project, repository identity, agent definition, agent instance, provider session, task, assignment, dependency, event, evidence, verification run, and acceptance decision. Session identity is provider + installation + workspace + provider session ID; PID alone is unsafe. Distinguish provider execution state from task lifecycle.

## Local-first operation

Default design is loopback-only, single-user, with workspace allowlisting, local browser session protection, Origin/Host validation, and restrictive file permissions. Loopback is not authentication. No automatic external telemetry. Provider credentials stay with provider tooling. Remote exposure and multi-user authorization are future work.

SQLite is the local candidate; PostgreSQL belongs to a later server-mode decision. Do not add either driver, schema, or migrations in Phase 0. Retention, backup, crash recovery, journal ordering, and replay performance must be designed and tested before claiming persistence support.

## Open decisions

Toolchain versions, frontend tooling beyond React/TS, storage driver, API schema tooling, event retention, local auth bootstrap, supported provider version ranges, and distribution packaging remain unselected. Each dependency requires approval. No install script, container image, CI platform, or runtime skeleton is implied by this document.

The [permission/completion kernel](governance.md) is an implemented, separate host-facing boundary. It does not change Fleet observation semantics or expose mutation endpoints.
