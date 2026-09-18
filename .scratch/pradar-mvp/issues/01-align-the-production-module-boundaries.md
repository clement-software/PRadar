# 01: Align the production module boundaries

**What to build:** Move the demonstrator's use cases behind the ownership
boundaries recorded in `docs/architecture/target-state.md`, so collection,
analysis and reading own their own contracts instead of sharing one package,
without changing observable behaviour.

**Blocked by:** None (can start immediately).

**Status:** done

- [x] Collection, analysis and reading each own their use cases and the
  interfaces they consume, as named by the target architecture.
- [x] Types shared by more than one boundary live where their owner is
  explicit, and no package exists only to hold shared structs.
- [x] Adapters implement the interfaces of the packages that consume them and
  keep returning concrete types.
- [x] The architecture test rejects any dependency from domain or application
  policy towards SQL, HTTP, process, Keychain, UI or adapter code, and names
  the new boundaries.
- [x] Every behaviour test that existed before the move still passes, asserting
  the same observable outcomes.
- [x] The move happens by expand and contract: the new form lands, callers
  migrate, then the old form is removed.
- [x] No behaviour, schema or contract changes in this ticket.
- [x] `make verify` passes.
