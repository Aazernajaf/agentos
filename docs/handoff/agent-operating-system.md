# Agent rules and SOP handoff

Date: 2026-11-02. Scope: repository-local operating discipline and reusable skills. No runtime authorization engine or provider control is implemented by this change.

## Delivered

- Shared operating policy for scope, incidental questions, corrections, separate ideas, explicit stop/priority changes, permission holds, duplicate-work avoidance, cross-retry budgets and evidence-bound handoff.
- Five focused canonical skills, each with its own SOP: session flow, implementation, adapter feasibility, verification and handoff.
- AGENTS.md and CLAUDE.md route applicable work to the policy and selected skill/SOP; Claude local skill aliases share the canonical content rather than diverging copies.
- A small task-brief template and manual steering scenarios, plus a standard-library package/SOP/alias integrity checker and negative filesystem tests.
- Antigravity CLI recorded as an explicit adapter candidate with unverified identity/version/native discovery/capabilities. No guessed integration, installation or support claims were added.

## Verification

- Skill-creator bundled `quick_validate.py`: all five skill frontmatter/packages valid using the existing system Python/YAML runtime; no package installation needed.
- `python3 scripts/check_agent_skills.py`: five canonical skills, own SOP links and Claude aliases passed.
- `python3 -m unittest discover -s scripts/agents -p 'test_*.py' -v`: seven package/link boundary tests passed, including missing/borrowed SOP and incorrect/missing/unregistered aliases.
- `python3 scripts/check_foundation.py` and `git diff --check`: passed after final documentation changes.
- Codex CLI 0.150.1 App Server `skills/list`, isolated CODEX_HOME and exact repository skill paths: all five discovered; no model call, private transcript read or live-session control.
- Claude alias targets were verified on disk and use the documented local host mechanism. Native invocation/behavior was not independently model-tested. Antigravity discovery remains unverified.
- Go/frontend runtime code did not change; their prior checks belong to revision 40bbeb3 and were not needlessly rerun for this instruction-only slice.

## Semantics and remaining work

An intermediate question gets a concise answer while the active assignment continues. A separate topic can prompt a new-chat suggestion; that suggestion never automatically creates a chat/agent or abandons the current task. Explicit stop, cancellation, priority change and real blocking permissions still apply. A user choosing to keep related work in the same chat is respected.

Skills and SOPs guide agents but are not sandbox enforcement, a behavioral guarantee or a task-acceptance engine. The manual scenarios are designed evaluation cases, not a claimed independent model evaluation. Future runtime policy gates need explicit permission/capability checks and revision-bound evidence; this change does not claim to implement them.

Next: evaluate actual skill use on scoped tasks and investigate Antigravity's exact CLI/native skill mechanism before integration. Keep the current read-only product boundaries and existing dependency/migration/privacy rules. Git/push state is reported after final verification.
