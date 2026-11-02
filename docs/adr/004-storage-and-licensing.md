# ADR-004: Defer storage implementation and final license

Date: 2026-11-02. Status: proposed / unresolved.

## Context

Planning named SQLite for standalone use, PostgreSQL for server mode, and Apache-2.0 as a license candidate. The repository is empty and no package, migration, or license approval was supplied.

## Proposal

Keep a storage boundary in the design; validate local persistence needs before choosing a driver. Retain PostgreSQL as a future server-mode candidate. Do not create migrations or dependency manifests merely to mirror the proposed stack. Seek an explicit maintainer license decision before open-source distribution.

## Alternatives and consequences

Requiring PostgreSQL immediately increases local setup cost. Implementing two databases now doubles compatibility work. Selecting a license without a maintainer decision would turn a candidate into a legal grant. Deferral avoids premature commitments but blocks a licensed public release and durable runtime implementation until decisions are made. No LICENSE file is issued in Phase 0.
