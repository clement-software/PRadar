# 03: Complete the collection behaviours the PRD requires

**What to build:** Close the two PRD collection behaviours the demonstrator
left out: a full reconciliation when the machine wakes, and an abonnement that
cannot be active when the configured engine is not allowed to process the
repository's content.

**Blocked by:** 01: Align the production module boundaries.

**Status:** done

- [x] Waking from sleep triggers a complete reconciliation without waiting for
  the next poll, and a long sleep produces exactly one catch-up.
- [x] The wake source is behind an interface, so tests drive it without
  sleeping the machine.
- [x] Creating or activating an abonnement checks that the configured analysis
  engine is authorised for that repository's content, and a refusal leaves the
  abonnement blocked with an actionable visible reason.
- [x] The check is re-evaluated when the engine or model configuration
  changes, and an abonnement blocked for that reason recovers without being
  recreated.
- [x] Usage measurements stay local and can be exported on demand by the user.
- [x] Tests cover wake, long sleep, an unauthorised engine, a recovered
  authorisation and a restart in each state.
- [x] `make verify` passes.
