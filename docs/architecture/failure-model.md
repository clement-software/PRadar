# Failure model

**Status:** Accepted for the MVP on 16 September 2026

Complete this before the target architecture is considered frozen.

## Ownership

| Resource | Owner and proof |
| --- | --- |
| Subscription and observed PR state | SQLite collection transaction, keyed by Forgejo instance/repository/PR identity. |
| Debounce deadline and analysis work | SQLite analysis store. A unique analysis identity prevents duplicate ownership. |
| Running analysis | One worker holding a durable, expiring lease token. The token is required to complete or retry. |
| Analysis history and visible projection | One SQLite completion transaction. History is append-only; the projection changes only on an identity match. |
| Forgejo credential | macOS Keychain. SQLite stores only a non-secret credential reference if required. |
| Temporary workspace | The active analysis invocation under an application-owned temp root. |
| UI selection, open detail, and scroll position | Desktop UI memory; never authoritative for durable read/archive state. |

## Concurrency

The MVP has a UI reader, a polling scheduler, and one analysis worker in one
process. SQLite uses WAL mode so readers may continue while the single writer
commits; SQLite still permits only one writer at a time. Writes are short and
never wrap Forgejo or Claude I/O.

Work claiming is one atomic update that chooses an eligible row, writes a lease
token and expiry, increments the attempt, and returns the claimed payload. It is
not a read followed by a write. Completion verifies both job ID and lease token.

Publication uses optimistic comparison: the completed analysis identity must
equal the PR's current observed identity. A mismatch is an expected stale
outcome; the result enters history but does not change the card.

## Retries and idempotence

| Operation | Identity | Repetition behavior |
| --- | --- | --- |
| Poll/reconcile PR | Forgejo instance + repository + PR number + observed revision | Upsert metadata; never regress to an older observation. |
| Schedule analysis | PR + input revision + prompt version + skill version + engine + model | Duplicate is success with no second job. |
| Claim work | Durable job ID + unique lease token | Only an eligible or expired row is claimable. |
| Complete analysis | Analysis identity + live lease token | Duplicate result is ignored; lost lease cannot complete. |
| Manual replay | A newly selected identity; at minimum a changed prompt, skill, engine, or model version | Creates a new history entry while retaining older results. |

Forgejo calls have a bounded timeout and retry only transient transport, 429,
and 5xx outcomes with jittered exponential backoff. Authentication and
authorization failures block the subscription and require user action. Claude
gets three durable attempts with increasing delay; the third failure records
`unavailable`. Cancellation does not consume a failure attempt when ownership
is intentionally handed back through lease expiry.

## Crash and partial failure

| Crash point | Recovery |
| --- | --- |
| Before the observation transaction commits | Nothing is durable; the next poll repeats the observation. |
| After observation/job commit, before claim | The job remains eligible. |
| After claim, before or during Claude | The job remains `running`; startup or the worker reclaims it after lease expiry. |
| After Claude returns, before completion commits | No result is durable; the expired job repeats. Duplicate model spend is accepted. |
| During result/publication persistence | The transaction commits both or neither. |
| After completion commit, before UI refresh | The next query shows the durable result. |
| After completion, before workspace deletion | Startup scavenging removes the orphan. No durable source copy is adopted by name alone. |

Closing or merging a PR updates its state without creating new analysis work.
An already-running analysis may finish and remain visible because the PRD keeps
the card until read or archive. Unsubscribing increments a subscription version,
makes pending work ineligible, cancels the active process, and prevents a late
completion from changing the active projection while retaining history.

## Degradation and recovery

- Poll every five minutes, with a full reconciliation on launch and wake.
- Persist a ten-minute per-PR debounce deadline; a later observation replaces
  the candidate and deadline.
- Run one Claude subprocess at a time. Bound process duration, output bytes,
  diff/workspace size, HTTP bodies, and shutdown grace time in configuration.
- Keep work durable rather than mirroring it in an unbounded channel.
- Emit local structured logs for poll, schedule, claim, retry, stale result,
  completion, cleanup, and blocked subscription. Never log tokens or PR bodies.
- Show last successful sync, pending/running counts, and actionable degraded
  states in the UI. Send no remote telemetry.
- On SQLite corruption or migration failure, stop mutation, preserve the files,
  and offer an explicit backup/rebuild procedure; never silently recreate data.

## Executable evidence

Required production evidence is listed in `invariants.md`. Prototype evidence:

- `codex/prototype/pradar-state-machine` at `471a8d2`.
- `codex/prototype/pradar-integration-spike` at `2007e82`, whose race-enabled
  tests cover concurrent claim, lease recovery, stale completion, subprocess
  integration, and retry exhaustion.
