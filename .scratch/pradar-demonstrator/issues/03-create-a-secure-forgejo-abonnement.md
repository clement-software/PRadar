# 03: Create a secure Forgejo abonnement

**What to build:** Let the user configure one Instance Forge and one repository
by URL, provide its read-only credential through macOS Keychain, and activate an
Abonnement only after the demonstrator proves that the repository is accessible.
Invalid or unauthorised configurations remain visible and actionable.

**Blocked by:** 02: Show one controlled analysis end to end.

**Status:** done

- [x] The user can configure exactly one Forgejo instance and add a repository
  using its normal Forgejo URL rather than API identifiers.
- [x] Live mode accepts HTTPS instance URLs only; automated tests may use an
  explicit loopback HTTP origin.
- [x] Malformed, unsupported, ambiguous, or cross-origin repository URLs are
  rejected before any credential is sent.
- [x] Redirect handling never forwards the Forgejo credential to a different
  origin.
- [x] The Forgejo token is stored and retrieved through macOS Keychain and is
  absent from SQLite, configuration exports, prompts, provenance, and logs.
- [x] Repository access is checked before the Abonnement becomes active.
- [x] Authentication, authorisation, missing-repository, timeout, and network
  failures leave the Abonnement blocked with an actionable visible reason.
- [x] The direct Forgejo HTTP adapter has contract tests for authentication,
  URL mapping, redirects, bounded bodies, malformed responses, and cancellation.
- [x] Structured logging proves that credentials and response bodies containing
  credentials are redacted.
- [x] Automated tests use controlled HTTP and Keychain substitutes and never
  require a live account.
- [x] `make verify` passes.

