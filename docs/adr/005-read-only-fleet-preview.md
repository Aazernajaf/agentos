# ADR-005: A bounded read-only Fleet slice

Date: 2026-11-02. Status: implemented preview, not a stable public API.

## Context

Scoped provider experiments established useful observation paths. The next increment needs an inspectable local product without prematurely choosing storage or enabling provider controls.

## Decision

Use Go's standard library for a bounded metadata projection, same-origin local HTTP/SSE, and a React/TypeScript UI built with the approved minimal frontend packages. Offer a clearly synthetic two-provider demo and one explicitly selected JSONL stdin source per real observation process. Do not launch providers, read ambient private histories, or expose write/control routes.

Keep process-local cursors and full-snapshot SSE. Each reconnect resynchronizes from current state; intermediate transitions may be coalesced. Preserve last-known execution as a historical fact while current state becomes unknown after disconnect. Execution completion never implies verified task acceptance.

## Alternatives and consequences

Adding SQLite now would require an unselected driver and migration design. A journal and generic plugin runtime would increase scope before ingestion semantics are proven. An unauthenticated loopback API would expose private metadata to browser-origin attacks. The selected navigation bootstrap protects the local browser boundary, but deliberately does not isolate malicious local OS processes. Remote exposure is unsupported.

Memory-only state, bounded recent turn correlation, one real source, and no durable replay are explicit limitations. This slice demonstrates the product and ingress boundaries without claiming the full v0.1 feature set. See [implementation contract](../architecture/live-fleet.md).
