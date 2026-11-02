# Claude Code instructions

Read and follow [AGENTS.md](AGENTS.md) before working in this repository. It is the canonical cross-agent policy; do not maintain a second conflicting rule set here.

Read [README.md](README.md) for current implementation status and [architecture](docs/architecture/overview.md) for boundaries. Claude Code integration is a feasibility target, not an installed plugin. Do not install hooks, change user settings, or read private session history as part of repository setup.

Use the [repository skill catalog](docs/agents/skills.md). Local `.claude/skills/` entries link to canonical `.agents/skills/` folders; read the selected skill and its own SOP. If native discovery is unavailable, read the canonical paths directly. Follow [session continuity](docs/agents/operating-policy.md): answer incidental questions, preserve the active task, suggest a separate chat for unrelated additions when useful, and honor explicit stops/priority changes. Do not install global configuration to work around discovery.
