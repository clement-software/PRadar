# 02: Give the local store a schema lifecycle

**What to build:** Replace the demonstrator's single-version schema with a
durable lifecycle: ordered migrations applied at startup, a backup before a
migration, and an explicit procedure when the database is unreadable, as
ADR-0002 requires.

**Blocked by:** 01: Align the production module boundaries.

**Status:** ready-for-agent

- [ ] Startup applies pending migrations in order inside a transaction and
  records the applied version durably.
- [ ] A database created by an older application version is migrated rather
  than refused; a database created by a newer one is refused with an
  actionable message.
- [ ] A migration failure leaves the database at its previous version and the
  application refuses to mutate data afterwards.
- [ ] A copy of the database is taken before a migration and its location is
  reported to the user.
- [ ] Corruption or an unreadable database stops mutation, preserves the
  files, and points at a documented backup and rebuild procedure; nothing is
  silently recreated.
- [ ] Migration tests cover an empty directory, an up-to-date database, an
  older one, a newer one, a failing migration and a restart after each case.
- [ ] The procedure is documented where an operator will find it.
- [ ] `make verify` passes.
