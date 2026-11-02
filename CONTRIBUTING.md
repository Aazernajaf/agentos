# Contributing to AgentOS

AgentOS is in Phase 0. Useful early contributions are corrections to the domain model, documented adapter limitations, synthetic fixtures, and narrowly scoped design proposals. Read [AGENTS.md](AGENTS.md), [the skill/SOP catalog](docs/agents/skills.md), and [the roadmap](docs/product/strategy.md) first.

## Workflow

1. Discuss substantial behavior or API changes in an issue before implementation. Describe the user outcome and acceptance criteria.
2. Use a focused branch from current `main`; preserve unrelated changes.
3. Get maintainer approval before dependencies or tooling additions. Any `migrations/` change requires explicit permission.
4. Update architecture or ADRs when contracts or decisions change. Label proposals and unverified claims.
5. Run the Go tests/vet and frontend typecheck/build in [Live Fleet checks](docs/architecture/live-fleet.md), plus `python3 scripts/check_foundation.py`, `python3 -m unittest discover -s scripts/probes -p 'test_*.py'`, and `git diff --check`. Offline probe tests do not run inference. Never run live probes as routine lint. Include relevant application tests/lint once they exist.
6. Open a focused pull request with evidence, limitations, and rollback considerations. Do not include private agent data or secrets.

Do not submit third-party code without clear provenance and compatible licensing. The repository license remains undecided; resolve contribution/distribution terms with the maintainer before submitting reusable code. No CLA or DCO process is currently configured.

## Testing direction

Future runtime changes need targeted tests: normalization and duplicate/out-of-order events; disconnect/reconnect and replay gaps; unsupported controls; path and workspace isolation; stale revision evidence; task acceptance policy. UI checks must distinguish unknown, stale, blocked, failed, and verified states. Synthetic fixtures must identify provider/version and expected mapping.

Do not invent placeholder test commands or claim application checks pass when there is no application. New tooling requires approval.

Skill changes require an own SOP and canonical host aliases; run `python3 scripts/check_agent_skills.py` and `python3 -m unittest discover -s scripts/agents -p 'test_*.py'`. Do not claim structural checks enforce runtime behavior.
