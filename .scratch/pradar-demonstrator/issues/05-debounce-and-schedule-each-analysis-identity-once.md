# 05: Debounce and schedule each analysis identity once

**What to build:** Turn relevant observations into durable analysis work only
after the per-pull-request Anti-rebond period, always selecting the latest
candidate and creating at most one work item for an exact analysis identity.

**Blocked by:** 04: Import and reconcile real pull-request versions.

**Status:** done

- [x] Every relevant observation and its scheduling decision commit in one
  SQLite transaction.
- [x] A new candidate receives a durable ten-minute not-before deadline; a
  later candidate for the same pull request replaces both candidate and deadline.
- [x] Restarting during Anti-rebond preserves the deadline and latest candidate
  without relying on an in-memory timer.
- [x] Wake and reconciliation make overdue durable candidates eligible without
  creating duplicates.
- [x] The analysis identity combines pull-request identity, input revision,
  prompt version, pinned skill version, engine, and model.
- [x] Repeating the same observation or reconciliation is a successful no-op
  that creates neither another work item nor another history entry.
- [x] Exactly the candidate current when its deadline expires becomes eligible
  analysis work.
- [x] Pending counts and state remain visible across process restarts.
- [x] Focused lifecycle tests cover replacement, duplicate observation, reopen,
  close/merge, wake, restart, and simultaneous observations under a fake clock.
- [x] SQLite integration tests prove uniqueness and idempotence under concurrent
  scheduling attempts with the race detector enabled.
- [x] `make verify` passes.

