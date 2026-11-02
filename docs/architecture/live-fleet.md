# Live Fleet — read-only preview

Status: runnable local preview. Go standard-library backend and React/TypeScript UI; no database, provider process launcher, task acceptance engine, or agent control routes. This is a thin implementation slice, not the complete v0.1 milestone.

## Run locally

Tested with Go 1.26.4, Node 24.16.0, npm 11.13.0 and Chrome. Requirements: Go 1.26+, Node 22.12+ and npm. Source license selection remains pending; package metadata is private/UNLICENSED.

```sh
npm --prefix apps/web ci
npm --prefix apps/web run build
go run ./cmd/agentos --demo
```

Open the printed loopback URL, normally `http://127.0.0.1:8080`. The Go process serves the built frontend and API on one origin. Demo mode contains two clearly labeled synthetic sessions cycling through execution and disconnect states; it neither accesses provider accounts nor runs inference.

For frontend iteration, `npm --prefix apps/web run dev` rebuilds on changes. Reload the Go-served page; there is no second-origin development proxy or HMR setup. Rebuild the backend after Go changes.

## Explicit source input

The observation process reads **one explicitly selected session stream** from standard input. It never scans provider histories, reads provider authentication, starts an agent, installs hooks, or sends controls. The two-provider demo does not imply concurrent real-provider sources are implemented.

If you already have an authorized producer emitting the selected session's structured events, connect its output as follows:

```sh
# Replace only the session/version labels with the selected owned stream's values.
# Supply JSONL on stdin from that trusted, explicitly scoped producer.
go run ./cmd/agentos --provider codex --provider-version 0.150.1 --session owned-session --workspace my-project
```

Use `--provider claude` for Claude stream-json (successful feasibility tested with 2.1.316). `--installation` and `--workspace` are explicit labels, not filesystem paths; identities allow bounded ASCII letters, digits, dot, dash, underscore and colon. The session ID must match incoming events. Raw provider errors and content are never echoed into logs or API output. A rejected input logs a generic message only.

The normalizer recognizes Codex turn-start/completion and Claude init/message-start/result metadata. It strips prompts, tool input/output, transcript paths, and other raw fields. Claude error semantics are taken from `is_error`, with success requiring the explicit success subtype; a success label cannot override an error flag. Other event types are ignored. These are version-scoped metadata mappings, not full production adapters.

Standard-input EOF or oversized/unreadable input marks the source disconnected, current state unknown. Last known execution state remains separately labeled as a historical observation. It is never task acceptance. A 30-second UI age marker describes stale state observation, not proof a provider is dead. No progress, token totals, capacity or cost is fabricated in this preview.

## API and browser boundary

- `GET /v0/fleet`: complete current metadata snapshot with process-local cursor and demo/observe mode.
- `GET /v0/events`: SSE `fleet` events containing the same complete snapshot, with comment heartbeats. Every connection starts with a fresh snapshot; Last-Event-ID is deliberately not replayed. Coalescing can omit intermediate transitions. No durable event journal or historical replay is claimed.
- No HTTP ingest endpoint or mutation/control routes. Non-GET methods are rejected.
- Numeric loopback bind only; exact Host and same-origin checks; cross-site and same-site sibling-origin browser requests rejected by Fetch Metadata; no CORS allowances.
- Opening `/` through direct/same-origin local navigation establishes an HttpOnly, SameSite=Strict cookie scoped to `/v0/`, with a per-origin cookie name so parallel local instances do not overwrite each other. API requests need that secret cookie. It is never put in URLs or printed. Static serving is limited to the index and non-hidden asset files; CSP disallows external scripts and framing.
- Bootstrap intentionally trusts local native clients. It does not defend against a compromised OS user or malicious local process; this is not remote or multi-user authentication. Reverse proxy exposure is unsupported.
- After backend restart, click Reconnect/reload to obtain the new browser session and snapshot. Browser offline/online events also update the connection warning and reopen the stream.

State is bounded to 64 identities, 64 SSE subscribers, 32 retired turn IDs per session, and 1 MiB per encoded source line. Slow viewers cannot stall ingestion: notifications are coalesced and each viewer reads the current snapshot. Local receive sequence and recent retired-turn correlation reject duplicates/stale transitions; this is not a general out-of-order network journal. All state disappears on restart. There is no automatic data retention on disk and no migration.

## Checks

```sh
go test -race ./...
go vet ./...
npm --prefix apps/web run typecheck
npm --prefix apps/web run build
python3 scripts/check_foundation.py
python3 -m unittest discover -s scripts/probes -p 'test_*.py'
mkdir -p work
go build -o work/agentos ./cmd/agentos
python3 scripts/smoke_fleet.py
```

The final command exercises synthetic Codex/Claude stdin through the built server and authenticated HTTP, including EOF, foreign-session rejection and payload redaction. It requires built web assets and does not contact a provider. Browser smoke was separately run with an already-installed Playwright/Chrome environment; Playwright is not a project dependency or required routine check.

Go uses no third-party modules. The explicitly approved frontend packages are React, React DOM, TypeScript, Vite, the Vite React plugin, and React/DOM type definitions, pinned with a lockfile. No additional framework, component kit, analytics, database driver or test package was added.

## Next slice

Add explicit concurrent source connection and versioned provider ingress fixtures before claiming a real multi-provider fleet. Durable journal/replay, Workload, Proof of Work, universal discovery, provider launch/control, and packaging remain separate work. Dependencies beyond the approved frontend list and any migrations still require explicit approval.
