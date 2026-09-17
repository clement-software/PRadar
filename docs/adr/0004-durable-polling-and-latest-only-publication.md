# ADR-0004: Persist polling/debounce and publish latest only

- **Status:** Accepted
- **Date:** 2026-09-16
- **Owners:** PRadar maintainers
- **Related:** `CONTEXT.md`, `docs/product/PRD.md`, prototype branch `codex/prototype/pradar-state-machine` at `471a8d2`

## Context

Forgejo webhooks are outside the MVP. Polls repeat observations and commits may
arrive while a previous version is debouncing or being analysed. A late result
must not make an archived PR reappear with obsolete content.

## Decision

Poll every five minutes and reconcile fully on launch and wake. Persist the
latest observation and a ten-minute per-PR debounce deadline. A newer relevant
observation replaces the candidate and its deadline; only the candidate current
when the deadline expires becomes analysis work.

A new observation marks the PR unread immediately but does not unarchive it.
Completion stores every valid result in history. It updates the visible card
and clears `archived` only when the result identity still equals the latest
observed identity. Close and merge update state without scheduling analysis and
keep an existing card visible until read or archive.

## Consequences

- Polling, restart, and wake are naturally idempotent.
- Users do not see a stale card reappear while a new head is still pending.
- Observation, analysis, and publication are distinct durable states, so the UI
  must communicate pending and degraded states.
- Title, description, and reopen changes participate in the input revision even
  when the head SHA is unchanged, so every PRD analysis trigger receives a
  distinct, idempotent analysis identity.

## Alternatives considered

- Immediate analysis of every poll result was rejected because commit bursts
  multiply spend and obsolete work.
- Hiding stale results entirely was rejected because history and provenance are
  required.
- Reappearing an archived PR on observation was rejected because it would show
  no current analysis; the validated state prototype reappears it on latest
  analysis completion.

## Verification

- Port the validated scenarios from `471a8d2` into domain/use-case tests.
- Use a fake clock for poll, wake, debounce replacement, and restart tests.
- Prove stale completion changes history but not the timeline projection.
