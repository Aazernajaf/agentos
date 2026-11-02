# Real project artifact and test review

Status: local Go submission verifier, operator terminal review and optional Fleet review panel. No provider/model call is required. Fleet observation remains read-only; only explicitly configured project acceptance has a write endpoint. This slice checks an already produced project result; it does not launch an unrestricted coding agent or intercept its filesystem tools.

## Contract and execution

The operator supplies a trusted manifest outside the project workspace. It names the task/session, all meaningful input paths, exact output paths with operator-selected expected SHA-256 values, and required test IDs/argument vectors. It is executable host configuration: never load it from a provider response, generated task artifact or unreviewed repository file. Expected hashes must represent independently reviewed intended outcomes; computing them blindly from an agent's output does not establish correctness.

`internal/projecttask.New` copies this configuration, opens an `os.Root`, fingerprints the actual input trees, output bytes/modes and manifest, and pins the current AgentOS skill/SOP/policy. Canonical relative paths only; no Git metadata, symlinks or special files. Selected trees are bounded to 4096 entries and 64 MiB. Artifacts must already exist at submission time. The host must declare every relevant source, fixture, test, manifest and configuration path. Hidden dependencies and undeclared files are outside the evidence contract; HEAD alone is never used as the revision.

The fixed `inspect-artifacts` operation passes through governance scope/admission. Verification then checks exact artifact digests and runs all required real test processes with host-selected argv, a two-minute overall deadline, bounded output and owned process-group cancellation. No shell interpolation is used by the runner; explicitly selecting a shell/script still executes code trusted by the operator. Tests may have side effects and are **not sandboxed**. Do not select installs, migrations, network mutations or protected operations as tests without their separate authorization. The environment omits provider token/session variables but retains HOME for normal tool operation; it does not prevent project code from reading host files. External executable identity, dependencies outside declared paths, network state and malicious change-and-restore races are not attested. Use an appropriately isolated trusted test environment for those threats.

Raw stdout/stderr are hashed while streaming and discarded. Reports contain test IDs, actual process pass/fail, output byte counts/digests and artifact digests. Imported test logs, provider success messages and user-supplied pass booleans are never accepted as evidence. Failed processes, changed inputs/tests/outputs, missing files, bad digests, cancellation or policy drift prevent a valid receipt. A passing script only establishes the criteria that script actually tests; the host must choose meaningful checks.

## Explicit human review

Successful verification produces `awaiting_user_approval`, never automatic acceptance. The governance mode is always `OperatorReview`. Task and Reviewer are separate object capabilities; the receipt stays private. The host must keep Reviewer out of agent tools and authenticate the actual operator action. A random review ID identifies the exact report; it is not authentication or a portable receipt.

The local command can wait on `/dev/tty` for `approve <review_id>` after displaying evidence. There is no auto-approve option or approval through stdin/provider JSON. Rejection, EOF or no controlling terminal leaves work unaccepted. Immediately before acceptance the kernel rechecks current content/policy and receipt TTL (15 minutes). File changes invalidate approval; the same approval cannot be replayed.

This local terminal is a trusted single-user operator interface, not proof of human identity against an agent that already controls the same OS account/TTY. Do not expose the command or Reviewer through a coding agent.

## Fleet review panel

The existing Fleet screen shows the task ID, revision, output filenames/digests, bounded text previews (up to 8 KiB per operator-selected artifact), actual test results and output digests. No arbitrary filesystem browsing or provider transcript route exists. Text renders escaped; binary artifacts have metadata only. The panel polls for freshness and disables acceptance on stale/expired evidence. Tests themselves still require host startup authorization; the browser cannot register commands, upload manifests or rerun checks.

Start the server with `--review-manifest /operator-controlled/task.json --review-workspace /absolute/project --policy-repo /absolute/agentos --run-review-checks` in addition to its normal observation/demo options. The trusted manifest must reside outside the project workspace. A failed verification aborts startup rather than displaying a successful review. Do not restart to refill logical task budgets.

`GET /v0/review` requires the existing HttpOnly SameSite browser session and provides the bounded report plus a separate CSRF nonce. `POST /v0/review/approve` requires that cookie, exact configured Host and Origin, same-origin Fetch Metadata, JSON content type, the nonce in a custom header and one exact review ID. Duplicate/unknown fields, wrong proof, stale files, expiry and replay are rejected. The nonce is not a receipt. These routes do not exist unless the host supplies a verified Task and its Reviewer. Only the explicit button handler sends the POST; reading/refreshing the screen never accepts work.

This is single-user loopback browser authorization, not independent human identity against another process controlling the same OS account/browser. Agents must not be given operator browser authority; do not advertise protection against a compromised local account. Operator review changes only the project-task report. Provider execution observations and synthetic demo session states remain separate. In-memory acceptance/evidence is lost on server restart.

## Run on an actual project

Create the operator manifest outside the workspace. Example schema (replace the expected hash with an independently reviewed artifact digest):

```json
{
  "task_id": "project-outcome",
  "session_id": "owned-project-session",
  "inputs": ["src", "tests", "go.mod"],
  "artifacts": [{"path": "result.json", "sha256": "0000000000000000000000000000000000000000000000000000000000000000"}],
  "tests": [{"id": "project-tests", "argv": ["go", "test", "-count=1", "./..."]}]
}
```

```sh
go run ./cmd/agentos-task-review \
  --workspace /absolute/project \
  --policy-repo /absolute/agentos \
  --manifest /operator-controlled/task.json \
  --run-checks --review
```

`--run-checks` explicitly permits execution of the selected tests; without it no test starts. Without `--review`, verification prints a pending report and exits without acceptance. Receipts and budgets are in memory only. Restarting loses them; an exported JSON report cannot later approve a task. Do not restart the host to silently refill an existing logical task's budget. No database/migration, durable evidence store, provider task executor or portable signature is included.

Offline integration tests use actual temporary result files and test processes. They cover mandatory review, correct/incorrect review IDs, approval replay, changed source/output/test files, failing/mutating tests, missing outputs, symlinks and configuration/report aliasing. They do not contact Codex or Claude. Previously verified synthetic adapter evidence is not repeated by this slice.
