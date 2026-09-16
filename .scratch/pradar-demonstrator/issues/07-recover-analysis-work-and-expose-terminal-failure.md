# 07: Recover analysis work and expose terminal failure

**What to build:** Make analysis ownership recoverable across crashes and make
technical failure visible: live leases prevent competing completion, expired
work is reclaimed, transient failures receive bounded durable retries, and the
third failure produces an Analyse indisponible with a safe Rejeu path.

**Blocked by:** 06: Analyse one due version with restricted Claude.

**Status:** ready-for-agent

- [ ] Concurrent claim attempts grant exactly one live lease for a work item.
- [ ] Completion and retry require the matching unexpired lease token; a late
  worker that lost ownership cannot mutate the result or Carte.
- [ ] Running work whose lease expires after interruption becomes claimable
  again on startup or by the worker.
- [ ] A technical analysis failure schedules at most three durable attempts with
  increasing delay and no in-memory retry authority.
- [ ] Invalid envelopes, invalid or mismatched schemas, non-zero exit, timeout,
  output overflow, and workspace failure follow the same bounded failure policy.
- [ ] The third failed attempt publishes Analyse indisponible while keeping the
  Forgejo link and a visible Rejeu action.
- [ ] Rejeu requires a changed prompt, skill, engine, or model identity and
  preserves all earlier results and failures in history.
- [ ] Intentional shutdown or Désabonnement cancellation hands ownership back
  without consuming another failure attempt.
- [ ] Workspace cleanup is observable after success, failure, cancellation, and
  startup scavenging; malicious names and symlinks cannot escape the owned root.
- [ ] The visualizer and structured logs expose pending, running, retry, lease
  recovery, terminal failure, and cleanup outcomes without PR bodies or secrets.
- [ ] Race-enabled integration tests cover one-winner claim, lost-lease
  rejection, expiry recovery, retry exhaustion, cancellation, and restart.
- [ ] `make verify` passes.

