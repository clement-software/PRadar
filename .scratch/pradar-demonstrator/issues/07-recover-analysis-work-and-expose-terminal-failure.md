# 07: Recover analysis work and expose terminal failure

**What to build:** Make analysis ownership recoverable across crashes and make
technical failure visible: live leases prevent competing completion, expired
work is reclaimed, transient failures receive bounded durable retries, and the
third failure produces an Analyse indisponible with a safe Rejeu path.

**Blocked by:** 06: Analyse one due version with restricted Claude.

**Status:** done

- [x] Concurrent claim attempts grant exactly one live lease for a work item.
- [x] Completion and retry require the matching unexpired lease token; a late
  worker that lost ownership cannot mutate the result or Carte.
- [x] Running work whose lease expires after interruption becomes claimable
  again on startup or by the worker.
- [x] A technical analysis failure schedules at most three durable attempts with
  increasing delay and no in-memory retry authority.
- [x] Invalid envelopes, invalid or mismatched schemas, non-zero exit, timeout,
  output overflow, and workspace failure follow the same bounded failure policy.
- [x] The third failed attempt publishes Analyse indisponible while keeping the
  Forgejo link and a visible Rejeu action.
- [x] Rejeu requires a changed prompt, skill, engine, or model identity and
  preserves all earlier results and failures in history.
- [x] Intentional shutdown or Désabonnement cancellation hands ownership back
  without consuming another failure attempt.
- [x] Workspace cleanup is observable after success, failure, cancellation, and
  startup scavenging; malicious names and symlinks cannot escape the owned root.
- [x] The visualizer and structured logs expose pending, running, retry, lease
  recovery, terminal failure, and cleanup outcomes without PR bodies or secrets.
- [x] Race-enabled integration tests cover one-winner claim, lost-lease
  rejection, expiry recovery, retry exhaustion, cancellation, and restart.
- [x] `make verify` passes.

