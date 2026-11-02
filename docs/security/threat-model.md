# Initial threat model

Status: design requirements; no runtime controls implemented.

Assets: local repositories, provider credentials, private prompts/transcripts, task evidence, workspace identity, browser session, and command approval authority. Trust boundaries: browser/API, provider adapter/core, workspace/filesystem, imported evidence/verifier, and public contribution/release workflow.

| Threat | Required design response | Future verification |
| --- | --- | --- |
| Malicious website calls loopback API / DNS rebinding | Validate Origin and Host, protect browser session and mutations, restrictive CORS, loopback binding | Cross-origin, forged Host, missing-token requests rejected |
| Provider/task text injects instructions | Treat content as data; typed commands and separate authorization path | Event payload cannot trigger command execution |
| Hooks or transcripts leak secrets | Explicit opt-in; metadata allowlist; redact before logs/storage; restrictive permissions | Synthetic secrets absent from journal, errors, UI and exports |
| Path traversal or symlink crosses workspace | Canonical paths, allowlisted roots, identity checks at use time | Escape and symlink fixtures denied |
| Unsupported control / wrong session | Versioned capability evidence, ownership and identity validation, explicit authorization, preserve provider approvals | Unsupported or foreign-session operation denied |
| Verification executes hostile repository code | Trusted execution policy, reviewed commands, restricted environment and budget; no automatic test execution on import | Import cannot run scripts; time/resource limits enforced |
| Fake or stale Proof of Work | Bind evidence and approvals to revision/criteria; record actor and provenance | Modified revision cannot reuse stale acceptance |
| Event flood / replay / disconnect | Bounded queues and payloads, deduplication, source correlation, stale state and resync | Flood and replay tests preserve correctness and responsiveness |
| Dependency / release supply chain | Approved packages, provenance/license review, minimal CI permissions and pinned reviewed tooling | Dependency and release review before publication |

No external telemetry by default; explicit opt-in and field review would be required. Establish retention/deletion/export policies before persisting private data. Local-first does not protect against a fully compromised user account. Remote and multi-user operation need a separate authorization design.

Open security decisions: local auth bootstrap, private reporting contact, retention, verifier isolation, supported provider versions, and release signing/update trust. See [security policy](../../SECURITY.md).
