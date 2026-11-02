# ADR-001: Local-first modular monolith

Date: 2026-11-02. Status: accepted direction from product planning.

## Context

The first user is a developer observing multiple coding agents on one machine. Setup friction and external service requirements would weaken the initial value.

## Decision

Use a Go backend and React/TypeScript UI in one repository, with domain modules inside one backend process. Target macOS/Linux single-user local operation first. Use REST and SSE as the initial transport direction. No AgentOS account or mandatory hosted dependency.

## Alternatives and consequences

Microservices add deployment and consistency overhead before demand exists. A hosted-first application introduces identity and data-export requirements. A terminal-only interface could be simpler but does not meet the intended fleet/workload overview. The chosen shape still requires local browser security, packaging, and storage work. Tool versions, frontend build tooling, and libraries remain subject to approval.
