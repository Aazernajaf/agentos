# Event contract draft v0

Status: design sketch, not a stable API or database schema. The [Fleet preview](live-fleet.md) implements a narrower metadata projection and full-snapshot stream; this draft journal/envelope is not yet implemented. No migrations are authorized by this document.

A normalized envelope needs: `schema_version`, unique `event_id`, `provider`, `provider_version`, `installation_id`, `workspace_id`, `session_id`, optional `task_id`, `event_type`, `source_timestamp` (nullable), `received_at`, source event ID/sequence when available, and a redacted typed payload. Keep provider facts separate from inferred state and attach provenance to inferences.

Synthetic example:

```json
{
  "schema_version": 0,
  "event_id": "synthetic-event-001",
  "provider": "example",
  "provider_version": "fixture",
  "installation_id": "synthetic-installation",
  "workspace_id": "synthetic-workspace",
  "session_id": "synthetic-session",
  "task_id": null,
  "event_type": "session.observed",
  "source_timestamp": null,
  "received_at": "2026-11-02T00:00:00Z",
  "source_event_id": null,
  "payload": {"execution_state": "unknown"}
}
```

Candidate types: session observed/state changed, tool started/completed, usage observed, connection lost/restored, evidence recorded, verification completed, and task acceptance decided. Provider adapter events cannot impersonate verifier or human decisions.

## Delivery semantics

- Do not promise exactly-once input. Deduplicate using provider IDs where available; document collision behavior when only a derived key is possible.
- Preserve source order only where guaranteed. Use a local journal cursor for SSE replay, never wall-clock time as a total order.
- A stale/out-of-order completion must not overwrite a newer running turn. Correlate turn and item IDs.
- Missing/disconnected streams mean unknown/stale, not idle or accepted. Record replay gaps and require snapshot resynchronization when a cursor expires.
- Bound payload size, queue length, and subscriber buffering; slow consumers must not stall providers. Reject unsupported schema versions explicitly.
- Redact before persistence; default payloads exclude prompts, tool arguments/results, environment, credentials, and transcript bodies.

Future acceptance tests: duplicate delivery leaves one effect; out-of-order events preserve current state; disconnect is visible; expired replay cursor triggers resync; cross-workspace IDs are rejected; changed revision makes evidence stale; no provider completion directly accepts a task.
