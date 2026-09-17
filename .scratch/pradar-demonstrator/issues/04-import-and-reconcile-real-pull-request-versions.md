# 04: Import and reconcile real pull-request versions

**What to build:** Make an active Abonnement discover real open pull requests,
import the requested initial set, and continuously reconcile each relevant
Version de pull request while exposing synchronisation and pending state in the
visualizer.

**Blocked by:** 03: Create a secure Forgejo abonnement.

**Status:** done

- [x] Creating an Abonnement lets the user import none, the ten most recently
  updated, or all currently open pull requests; ten is the default.
- [x] Draft pull requests are excluded by default.
- [x] Bot- and agent-authored pull requests are included unless their authors
  are explicitly excluded.
- [x] Pagination is complete and deterministic for every import mode.
- [x] Launch and wake perform a complete reconciliation before normal polling;
  normal polling runs every five minutes under a controllable clock.
- [x] Opening, reopening, head, title, or description changes create a distinct
  persisted input revision using the exact observed head SHA, normalised text,
  and reopen generation.
- [x] Closing or merging updates pull-request state without scheduling a new
  Analyse.
- [x] Older observations never replace newer durable state.
- [x] The visualizer reports last successful synchronisation, blocked
  Abonnements, and the count of pull requests awaiting later processing.
- [x] Forgejo contract tests cover pagination, drafts, author exclusions,
  reopen, close, merge, changed title/body/head, rate limiting, transient server
  failure, malformed JSON, cancellation, and bounded response bodies.
- [x] Automated tests make no live Forgejo request.
- [x] `make verify` passes.

