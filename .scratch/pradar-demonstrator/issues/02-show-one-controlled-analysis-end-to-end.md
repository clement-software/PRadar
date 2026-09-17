# 02: Show one controlled analysis end to end

**What to build:** Establish the thinnest complete demonstrator path: observe one
controlled pull request, schedule and complete a deterministic controlled
Analyse through real temporary SQLite storage, and show its Carte and detail in
the loopback visualizer. This slice establishes the application seam on which
later real adapters will be substituted.

**Blocked by:** 01: Activate the reproducible Go delivery gates.

**Status:** done

- [x] One manually wired Go process owns configuration, SQLite, the application
  use cases, the visualizer, root cancellation, and orderly process exit.
- [x] A controlled pull-request observation is persisted, scheduled, claimed,
  completed, and projected without bypassing the application use-case surface.
- [x] The visualizer binds only to loopback on an ephemeral port and shows one
  Carte plus a detail containing the deterministic Analyse.
- [x] Restarting the process against the same database restores the Carte and
  its Analyse without inserting duplicate work or history.
- [x] SQLite runs in WAL mode with an explicit busy timeout, validated schema
  version, short transactions, and no external I/O inside a transaction.
- [x] The primary automated test seam uses a real temporary SQLite database and
  controlled substitutes for all external capabilities.
- [x] Tests assert visible state and durable outcomes rather than private calls
  or concrete implementation structure.
- [x] An architecture test rejects dependencies from domain and application
  policy towards SQL, HTTP, process, Keychain, UI, or adapter code.
- [x] Background work has an explicit owner and terminates when the root context
  is cancelled.
- [x] `make verify` passes.

