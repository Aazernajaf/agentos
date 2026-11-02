# ADR-003: Separate activity, workload, and acceptance

Date: 2026-11-02. Status: accepted direction.

## Context

Running processes, token use, and an agent's completion message do not establish that a user's requested outcome is correct.

## Decision

Expose capacity, queue, blocked time, verified scope progress, and quality separately. Unknown measures remain unknown. Tasks enter accepted state only under explicit acceptance policy with revision-bound evidence and required review. Provider completion is observation, not acceptance.

## Alternatives and consequences

A single “productivity percentage” is easy to present but misleading. Accepting all successful process exits loses acceptance context. Evidence adds collection and review overhead, and passing checks still cannot prove every property. Criteria must name what was verified and what remains outside coverage. See [product scope](../product/strategy.md).
