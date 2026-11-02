# Live Fleet preview handoff

Date: 2026-11-02. Scope: the first read-only local Fleet slice after scoped adapter feasibility.

## Delivered

- Runnable Go backend with no third-party Go modules: metadata normalization, bounded session projection, authenticated local snapshot and SSE endpoints, explicit stdin source scope, and synthetic demo.
- React/TypeScript UI: observed sessions, execution/attention counts, provider/search filters, helpful empty/error states, freshness and disconnect labels, keyboard-accessible controls, mobile layout, and offline recovery.
- Exact frontend dependency versions and lockfile, limited to the seven explicitly approved direct packages. No database, migration, provider SDK, component library, test framework or CI dependency was added.
- Reproducible standard-library built-server smoke check, Go boundary tests, run/API/security documentation, ADR-005, and a screenshot of the synthetic demo.
- Foundation checker now prunes dependencies, build output, private scratch and macOS metadata; regression coverage prevents binary metadata/third-party READMEs being mistaken for project documents.

## Verification

- `go test -race ./...`: all 12 Go tests passed across projection, normalizer and HTTP/SSE boundary packages.
- `go vet ./...`: passed. No ESLint configuration or CI execution is claimed.
- Frontend strict TypeScript checking and production build: passed. npm audit reported zero vulnerabilities at verification time; this is not a general security guarantee.
- `python3 scripts/smoke_fleet.py`: passed for synthetic Codex and Claude input through the built process, auth bootstrap, foreign-session rejection, EOF, error semantics and output redaction.
- Existing 11 offline Python probe tests and the foundation check: passed.
- Browser smoke using already-installed Playwright/Chrome: live two-provider demo, search/filter/clear, empty state, connection help, mobile page overflow, offline warning/recovery, no page errors. Desktop and mobile screenshots inspected; no browser package was added to the project.
- No new live inference or real session observation was performed in this implementation task. Earlier version-scoped feasibility evidence remains separate.

## Boundaries and remaining work

The demo has two synthetic providers, while real mode accepts one explicitly selected stdin session per process. Concurrent source connection, real-provider launch/bridge, durable history/replay, Workload, Proof of Work, universal discovery and active control remain unimplemented. Completion is an execution observation, never task acceptance. Current state becomes unknown on disconnect; last-known execution is separately retained as a historical fact.

Only numeric loopback listening is supported. Browser bootstrap establishes a local HttpOnly/Strict cookie, exact Host/origin/Fetch Metadata checks protect reads, and SSE resynchronizes by full snapshot. Another native process under the same OS user is outside this boundary. Remote deployment and multi-user operation are unsupported.

License selection and a private security contact remain outstanding public-release decisions. Dependencies beyond the approved frontend set and any migrations still require explicit permission.

## Run and rollback

See [Live Fleet setup and contract](../architecture/live-fleet.md). Stop the local preview to discard its in-memory state. Reverting this source change requires no database rollback or provider configuration changes. The demo does not access agent accounts. Git commit/push state and any running review URL are reported separately after verification.
