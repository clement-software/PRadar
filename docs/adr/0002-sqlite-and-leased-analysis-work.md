# ADR-0002: Use SQLite and leased analysis work

- **Status:** Accepted for the MVP coordination model
- **Date:** 2026-09-16
- **Owners:** PRadar maintainers
- **Related:** `docs/architecture/failure-model.md`, prototype branch `codex/prototype/pradar-integration-spike` at `2007e82`

## Context

PRadar needs local durable metadata, analysis history, user state, provenance,
debounce deadlines, and recoverable work. The UI must remain readable while a
single worker commits short state changes. A process can stop between any two
effects, and polling may observe the same PR repeatedly.

The integration spike passed race-enabled tests for a single atomic claim,
expired-lease recovery, three-attempt exhaustion, stale-result protection, and
subprocess completion. It used one SQLite driver as disposable evidence; the
driver itself was not evaluated.

## Decision

Use one local SQLite database as the MVP system of record and work queue, behind
interfaces owned by the collection, analysis, and timeline packages. Enable WAL
mode on a local filesystem, configure a busy timeout, keep write transactions
short, and permit only one application analysis worker.

Each analysis identity is unique. Claiming is one atomic write that sets a
unique lease token and expiry and increments the durable attempt count.
Completion requires the live lease. Result insertion and visible-projection
update share one transaction; the projection update additionally compares the
current observed identity.

Select the concrete `database/sql` driver during implementation using build,
distribution, maintenance, and vulnerability evidence. Do not copy the
prototype schema or driver choice.

## Consequences

- Restart recovery does not need a daemon or external broker.
- Duplicate polling and expired workers are ordinary idempotent outcomes.
- SQLite remains a single-writer system; long transactions or network/process
  I/O inside transactions are prohibited.
- WAL, database, and shared-memory files must remain together on local storage.
- A schema migration and backup/recovery policy become mandatory.

## Alternatives considered

- An in-memory channel was rejected because work would be lost on application
  exit and could not support durable debounce or retries.
- A separate queue product was rejected because it creates another service for
  one local worker.
- Multiple database files were rejected because atomic result/publication
  updates would become cross-store coordination.

## Verification

- Production equivalents of the tests on prototype commit `2007e82` pass under
  `go test -race`.
- Integration tests kill a worker after claim and verify lease recovery.
- Transaction tests prove a stale identity is stored but never published.
- SQLite configuration is checked at startup rather than assumed.

Primary technical evidence:

- [SQLite write-ahead logging](https://sqlite.org/wal.html)
- [SQLite transactions](https://sqlite.org/lang_transaction.html)
- [SQLite `RETURNING`](https://sqlite.org/lang_returning.html)
