# Agent operating policy

Applies to AgentOS work by Codex, Claude Code, Antigravity CLI and future adapters. Follow higher-priority instructions and the user's current authorization. This policy and the skills are instructions, not a guarantee of model behavior. The [in-process kernel](../architecture/governance.md) now enforces named operation and acceptance checks for host-routed calls; it does not intercept all external tools or interpret every policy sentence.

## Keep the active assignment explicit

For multi-step work, maintain a short task brief in the chat: objective, scope, acceptance criteria, authorized actions, completed evidence, next action, pending decisions and queued ideas. Use [the task brief template](task-brief-template.md) when a durable handoff helps. Do not create paperwork for a one-line edit.

Before each material action, check: does it advance the active objective, is it authorized, and is the result already available? If not needed, do not do it. Do not turn a useful suggestion into an unrequested implementation. A skill narrows a workflow; it never grants extra permissions.

## Handle messages without losing the assignment

| New user message | Required behavior |
| --- | --- |
| Question, explanation request or status check | Answer briefly, preserve the objective and resume the next authorized step in the same session. Do not silently finish/abandon the assignment because the question was answered. |
| Correction or constraint | Apply it to ongoing work. Revisit only affected work/checks. Preserve compatible completed work. |
| Related addition | Include it if the request authorizes it and priorities are clear; otherwise queue the decision while continuing independent work. |
| Separate idea or unrelated work | Acknowledge and suggest a separate chat when it would preserve focus. Keep the current assignment moving; do not open a chat, launch another agent or start the extra work merely because it was mentioned. |
| Explicit request to create another chat | Use the available chat tool under its access rules; retain an accurate handoff and the current assignment's status. If unavailable, provide the handoff and state the limitation. |
| “Do this instead” / explicit reprioritization | Follow the new priority. Record the previous assignment as superseded or queued; do not continue incompatible old work. |
| Explicit stop, pause or cancel | Stop the affected work promptly and report its state. Do not interpret “keep going through questions” as permission to ignore a stop. Resume only when instructed. |
| Blocking approval or missing input | Ask one precise question, stop dependent actions, and continue only genuinely independent authorized work. Never treat elapsed time as consent. |

Offer another chat only when it is useful. Do not require a new chat for every small question, and do not repeatedly offer one after the user chooses to stay here. A suggestion is not a new thread, approval, delegation or an automatic task switch.

Example: while implementing Fleet, “Do you like this idea?” calls for a brief opinion followed by Fleet progress. “Set up a CRM too” is separate work: propose a new chat and preserve Fleet unless the user changes priority. “Stop Fleet” must stop Fleet.

## Avoid duplicate and speculative work

- Inspect current files, Git state, existing handoff, relevant ADRs and prior check results before rebuilding anything. Reuse evidence only when its inputs/revision and scope still apply.
- Record what was actually done, what remains, and why a repeated action is necessary. Freshness matters for deployments, network/auth state, provider versions and remote state before push.
- After passing checks, repeat only those affected by new changes, failures or unresolved concerns. Do not repeat model probes, installs, audits or full test suites merely to appear active.
- Track a budget across retries, not just per process/run. For live probes, record total allowed calls/time/cost and stop at that boundary. Authentication failure is not a reason for repeated inference attempts; wait for changed readiness.
- Prefer existing project tools and the smallest relevant skill. Load that skill's own SOP; do not load the whole catalog into every task.
- Keep unrelated ideas in a small pending list with a next decision. Do not drop them, count them as completed, or implement them silently.

## Permission and evidence boundaries

The dependency, migrations, privacy and provider-control rules in [AGENTS.md](../../AGENTS.md) remain authoritative. Existing approval persists for its stated scope; do not ask again without a material change. Ask before expanding it. Never substitute a skill name, a provider capability, a completion message or a successful tool call for authorization or acceptance evidence.

Do not invent installed skills, provider compatibility, test success, runtime enforcement or completed work. Separate verified, unverified, blocked and explicitly unsupported capabilities. Antigravity is a requested adapter candidate; identity/version, native skill discovery and capabilities still require verification.

## Completion and handoff

Finish the active acceptance criteria, inspect the diff, run applicable checks and correct failures. Report changes, evidence, limitations, open decisions and Git/push state. Link reusable results. A final answer to an incidental question is not a completion handoff for an unfinished active task.

When handing off, include the last verified revision and inputs, completed work/checks, active objective, next concrete step, pending approvals, queued ideas and any explicit pause/stop. Exclude secrets, private transcripts and raw provider content. Instructions do not authorize messaging others, creating chats or delegating work unless the user or applicable instructions explicitly do so.
