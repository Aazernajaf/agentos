# Adapter feasibility handoff

Date: 2026-11-02. Scope: dedicated Claude Code and Codex feasibility experiments following the original Phase 0 foundation. Production adapter implementation has not begun.

## Actual changes

- Added an opt-in, standard-library Python probe with synthetic workspaces, limited tools, deadlines, native-auth separation, structured observations, and sanitized evidence export.
- Added offline safety/correlation tests and an executed capability matrix with aggregate JSON evidence.
- Updated README, agent/contributor checks, and adapter planning to distinguish observed capabilities from assumptions.
- Corrected the foundation checker to skip macOS AppleDouble `._*.md` metadata, which caused a real UTF-8 failure on the external project drive; a regression test covers this.
- The selected project directory was initially empty, so the existing remote main branch was cloned into it. Native macOS metadata/private scratch ignores are local Git exclusions rather than repository-wide changes.

## Observed results

Codex CLI 0.150.1: successful synthetic reply; item/turn/delta and token-usage notifications; owned-session persistence/list/read after reconnect; thread resume with verified synthetic context recall; successful interrupt when dispatched on turn/started. No unrelated session discovery, control, approval-flow coverage, replay guarantees, or production support is claimed.

Claude Code 2.1.316: native authentication ready; successful synthetic Read, exact reply, structured partial events, usage/cost fields, all five configured hooks, and context recall when resuming the same owned session. Earlier auth failures used 2.1.296; that version emitted subtype success with is_error true and exit 1. Adapters must not trust subtype alone. This probe did not upgrade the CLI; login state and version had both changed before success. General session discovery/history APIs and active control remain unverified.

Full matrix, caveats, commands, and [sanitized evidence](../architecture/adapter-probe-results.json) are in [executed experiments](../architecture/adapter-experiments.md).

## Verification

- `/usr/bin/python3 -m unittest discover -s scripts/probes -p 'test_*.py' -v`: 11 offline tests passed.
- `/usr/bin/python3 scripts/check_foundation.py`: passed after documentation changes.
- `git diff --check`: passed before commit.
- Live probes: completed with mixed scenario-specific results as recorded; no blanket “all passed” assertion.
- No Go/React tests, lint configuration, production adapter contract tests, migrations, or CI exist yet. No dependencies were installed. No production services or original agent sessions were changed.

## Remaining decisions and next step

Native authentication and both providers' basic owned-session paths are now verified. Next, define the smallest read-only Live Fleet slice. Before writing production adapters, add versioned fixture coverage, explicit capabilities, history/replay semantics, and privacy/authorization boundaries. Keep automatic discovery of unrelated sessions and live controls outside current product promises. License, toolchain/dependency selection, and public security contact remain open decisions.

Work follows the requested main branch. Commit/push state is reported separately after final verification; this document does not imply a deployed service or release.
