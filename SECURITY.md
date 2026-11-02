# Security policy

## Supported versions

There are no releases or supported production versions. The local read-only preview implements the browser/API boundary described in [Live Fleet](docs/architecture/live-fleet.md); wider threat-model controls remain planned. It is not a trusted control service. Remote exposure and multi-user access are unsupported.

## Report a vulnerability

Use the repository's **Security → Advisories → Report a vulnerability** option if GitHub private vulnerability reporting is enabled. Its availability has not been verified. Do not post vulnerabilities, credentials, private transcripts, or exploit details in a public issue.

If private reporting is unavailable, open a minimal public issue asking the maintainer to establish a private reporting channel, without sensitive details. A dedicated security contact and response SLA have not yet been established and are release blockers. Never send secrets while arranging that channel.

## Development boundaries

See the [threat model](docs/security/threat-model.md). Observation does not authorize control. Provider permission prompts must remain intact. Verification commands execute repository-controlled code and require an explicitly trusted execution context. Redact before persistence, and keep real credentials and session content out of test fixtures and logs.

The [governance kernel](docs/architecture/governance.md) separates agent intent from host authority for registered callbacks. Integrating hosts must authenticate/route callers, keep Operator private, supply real input revisions/checks and isolate executors. It is not an OS sandbox or restart-safe permission store. Live Fleet exposes no operator/grant endpoints.
