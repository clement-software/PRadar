# Spec pointer: the PRadar MVP

Status: accepted

Date: 2026-09-18

The MVP has no separate specification document. Its scope, outcomes and
exclusions are `docs/product/PRD.md`; its implementation decisions are
ADR-0001 to ADR-0007 and `docs/architecture/target-state.md`. This file exists
so the tracker points at those sources rather than duplicating them.

## What is already true

The demonstrator merged into `main` runs the PRD's end-to-end scenario against
a real Forgejo instance: abonnement from a repository URL, polling and
anti-rebond, leased analysis work, a restricted Claude invocation with the
pinned `show-me` guidance, `pradar.analysis.v1` validation, latest-only
publication, timeline and detail, read and archive, and restart recovery. Its
packages, tests and quality gates are production quality; its package
boundaries, schema lifecycle and interface are not.

## What the MVP tickets change

1. Align the package boundaries with `target-state.md` and keep the
   architecture test honest.
2. Give the durable store a schema lifecycle: migrations, backup and an
   explicit corruption procedure, as ADR-0002 requires.
3. Complete the PRD behaviours the demonstrator left out: wake handling and
   the engine-authorisation condition on an abonnement.
4. Rebuild the interface against the production use cases and show it in a
   native window, as ADR-0007 decided.
5. Package the application as one distributable macOS binary.
6. Score the frozen corpus on that interface, which is the PRD gate the owner
   deferred on 18 September 2026.

## Out of scope

Everything `docs/product/PRD.md` lists under "Hors périmètre", plus any change
to an accepted ADR. A contradiction found while implementing returns to an ADR
rather than being resolved inside a ticket.
