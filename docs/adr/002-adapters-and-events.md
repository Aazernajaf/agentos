# ADR-002: Capability-aware adapters and normalized events

Date: 2026-11-02. Status: accepted direction; protocol implementation unverified.

## Context

Providers expose different discovery, event, and control surfaces. Observing a session does not imply authority or ability to control it.

## Decision

Translate documented provider interfaces into a versioned normalized event envelope. Keep provider-specific semantics at the edge. Record capability evidence per version, transport, and session scope; default unverified controls to disabled. Use journal/projection separation with explicit deduplication and replay gaps. Raw provider content is not the domain contract.

## Alternatives and consequences

A universal control interface with optimistic booleans would promise unsupported actions. Scraping internal files may offer visibility but creates privacy and compatibility risks. Documented interfaces still need version pinning, synthetic fixtures, and real integration experiments. Journal ordering and retention remain open; append-oriented history does not override deletion/privacy policy.

See [adapter evidence](../architecture/adapters.md) and [event draft](../architecture/events.md).
