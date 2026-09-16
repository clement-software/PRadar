# 05: Debounce and schedule each analysis identity once

**What to build:** Turn relevant observations into durable analysis work only
after the per-pull-request Anti-rebond period, always selecting the latest
candidate and creating at most one work item for an exact analysis identity.

**Blocked by:** 04: Import and reconcile real pull-request versions.

**Status:** ready-for-agent

- [ ] Every relevant observation and its scheduling decision commit in one
  SQLite transaction.
- [ ] A new candidate receives a durable ten-minute not-before deadline; a
  later candidate for the same pull request replaces both candidate and deadline.
- [ ] Restarting during Anti-rebond preserves the deadline and latest candidate
  without relying on an in-memory timer.
- [ ] Wake and reconciliation make overdue durable candidates eligible without
  creating duplicates.
- [ ] The analysis identity combines pull-request identity, input revision,
  prompt version, pinned skill version, engine, and model.
- [ ] Repeating the same observation or reconciliation is a successful no-op
  that creates neither another work item nor another history entry.
- [ ] Exactly the candidate current when its deadline expires becomes eligible
  analysis work.
- [ ] Pending counts and state remain visible across process restarts.
- [ ] Focused lifecycle tests cover replacement, duplicate observation, reopen,
  close/merge, wake, restart, and simultaneous observations under a fake clock.
- [ ] SQLite integration tests prove uniqueness and idempotence under concurrent
  scheduling attempts with the race detector enabled.
- [ ] `make verify` passes.

