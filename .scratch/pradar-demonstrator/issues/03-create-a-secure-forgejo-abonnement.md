# 03: Create a secure Forgejo abonnement

**What to build:** Let the user configure one Instance Forge and one repository
by URL, provide its read-only credential through macOS Keychain, and activate an
Abonnement only after the demonstrator proves that the repository is accessible.
Invalid or unauthorised configurations remain visible and actionable.

**Blocked by:** 02: Show one controlled analysis end to end.

**Status:** ready-for-agent

- [ ] The user can configure exactly one Forgejo instance and add a repository
  using its normal Forgejo URL rather than API identifiers.
- [ ] Live mode accepts HTTPS instance URLs only; automated tests may use an
  explicit loopback HTTP origin.
- [ ] Malformed, unsupported, ambiguous, or cross-origin repository URLs are
  rejected before any credential is sent.
- [ ] Redirect handling never forwards the Forgejo credential to a different
  origin.
- [ ] The Forgejo token is stored and retrieved through macOS Keychain and is
  absent from SQLite, configuration exports, prompts, provenance, and logs.
- [ ] Repository access is checked before the Abonnement becomes active.
- [ ] Authentication, authorisation, missing-repository, timeout, and network
  failures leave the Abonnement blocked with an actionable visible reason.
- [ ] The direct Forgejo HTTP adapter has contract tests for authentication,
  URL mapping, redirects, bounded bodies, malformed responses, and cancellation.
- [ ] Structured logging proves that credentials and response bodies containing
  credentials are redacted.
- [ ] Automated tests use controlled HTTP and Keychain substitutes and never
  require a live account.
- [ ] `make verify` passes.

