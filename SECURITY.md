# Security policy

## Supported versions

There are no releases or supported production versions. Phase 0 documents intended security boundaries; those controls are not implemented yet. Do not deploy AgentOS as a trusted control service.

## Report a vulnerability

Use the repository's **Security → Advisories → Report a vulnerability** option if GitHub private vulnerability reporting is enabled. Its availability has not been verified. Do not post vulnerabilities, credentials, private transcripts, or exploit details in a public issue.

If private reporting is unavailable, open a minimal public issue asking the maintainer to establish a private reporting channel, without sensitive details. A dedicated security contact and response SLA have not yet been established and are release blockers. Never send secrets while arranging that channel.

## Development boundaries

See the [threat model](docs/security/threat-model.md). Observation does not authorize control. Provider permission prompts must remain intact. Verification commands execute repository-controlled code and require an explicitly trusted execution context. Redact before persistence, and keep real credentials and session content out of test fixtures and logs.
