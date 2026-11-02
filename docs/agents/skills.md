# AgentOS skills and SOP catalog

Canonical skills live in `.agents/skills/`. Each has a short `SKILL.md` and its own adjacent `SOP.md` covering inputs, permitted/prohibited actions, procedure, stopping conditions and completion evidence. Read only the applicable skill/SOP; shared scope and message rules live in [operating policy](operating-policy.md).

| Skill | Use when | Own SOP |
| --- | --- | --- |
| [agentos-session-flow](../../.agents/skills/agentos-session-flow/SKILL.md) | A multi-step task receives questions, corrections or separate ideas | [Session flow SOP](../../.agents/skills/agentos-session-flow/SOP.md) |
| [agentos-implementation](../../.agents/skills/agentos-implementation/SKILL.md) | Implementing/fixing scoped AgentOS code | [Implementation SOP](../../.agents/skills/agentos-implementation/SOP.md) |
| [agentos-adapter-feasibility](../../.agents/skills/agentos-adapter-feasibility/SKILL.md) | Evaluating Codex, Claude Code, Antigravity or another requested adapter | [Adapter SOP](../../.agents/skills/agentos-adapter-feasibility/SOP.md) |
| [agentos-verification](../../.agents/skills/agentos-verification/SKILL.md) | Selecting checks and assessing an AgentOS change's acceptance evidence | [Verification SOP](../../.agents/skills/agentos-verification/SOP.md) |
| [agentos-handoff](../../.agents/skills/agentos-handoff/SKILL.md) | Resuming or handing off existing AgentOS work | [Handoff SOP](../../.agents/skills/agentos-handoff/SOP.md) |

Session continuity applies to every task through AGENTS.md; the session-flow skill gives extra guidance when steering occurs. Select the domain skill for the actual work. Verification and handoff are used when their phase applies, rather than loading five skills at startup.

## Discovery and provider boundaries

- **Codex:** isolated CLI 0.150.1 App Server `skills/list` discovered all five exact repository paths without model calls. This verifies metadata discovery, not behavior in a running model session. Repository `.agents/skills/` is the documented native location. Invoke, for example, `$agentos-implementation`, or let matching task descriptions select it. Discovery may require refreshing/restarting the host.
- **Claude Code local sessions:** `.claude/skills/<name>` symlinks point to the same canonical folders; `/agentos-implementation` and description-based selection are documented host mechanisms. No duplicate SOP copies or global settings/hooks are installed. Bare/safe modes, managed policies and some cloud contexts can disable repository customization. If a host does not follow symlinks, read the canonical paths explicitly through CLAUDE.md; do not silently copy and diverge the rules.
- **Antigravity CLI:** included in the policy and adapter queue, but executable/distribution, skill-discovery paths and behavior are unverified. Supply the canonical policy/skill/SOP as explicit context when supported. Do not claim native integration or create guessed provider configuration directories.

The local catalog, path checks and SOP review do not prove that every model follows instructions. These are authoring/discovery controls. Live Fleet remains read-only and does not execute policy gates, prevent model drift, install skills in running sessions or enforce provider permissions. The [permission/completion kernel](../architecture/governance.md) now enforces a named subset for registered host operations; it is not wired into Fleet observations or every external tool. Authenticated broker/UI integration and durable state remain future work.

## Authoring and validation

Keep one canonical skill folder per distinct workflow, with frontmatter name matching its directory and a focused description. Link the own SOP from SKILL.md. Put provider-specific behavior in a scoped SOP, not invented universal capabilities. Follow the existing permission boundaries rather than duplicating or relaxing them.

Run `python3 scripts/check_agent_skills.py` and `python3 scripts/check_foundation.py`. The first checks local package/SOP/Claude-alias integrity; the second checks document links. Frontmatter was also checked using the existing skill-creator validator during authoring. Neither test establishes behavioral conformance.

See [steering scenarios](scenarios.md) for manual acceptance cases. Real independent model evaluation remains unperformed; no model invocation or delegated agent evaluation was performed for this instruction-only change.

## Primary host references

Reviewed 2026-11-02: [Codex skill authoring and local discovery](https://learn.chatgpt.com/docs/build-skills), [Claude Code skill locations and symlink support](https://code.claude.com/docs/en/skills). These describe host mechanisms, not a guarantee of local invocation in every environment.
