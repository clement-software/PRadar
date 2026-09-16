# Current architecture

**Status:** Assessed on 16 September 2026

PRadar has no production runtime yet. The current repository contains product
intent, domain vocabulary, workflow policy, UI references, and architecture
templates. Target modules named elsewhere do not yet exist.

## Runtime shape

There is no `go.mod`, entry point, long-running process, database, or external
integration on `main`. `docs/product/PRD.md` is the executable-design input,
not an implementation.

## Modules and seams

No production modules or interfaces exist. The only stable seams are product
contracts:

- Forgejo is the sole forge for the MVP.
- `pradar.analysis.v1` is the required analysis output.
- storage and analysis engine are required to remain replaceable boundaries.
- `CONTEXT.md` defines lifecycle vocabulary.

## Data and control flow

No end-to-end production flow exists. Two throwaway branches provide evidence
without being merge candidates:

1. `codex/prototype/pradar-state-machine` at `471a8d2` models polling,
   debounce, analysis, reading, archiving, retry, and stale results.
2. `codex/prototype/pradar-integration-spike` at `2007e82` exercises a fake
   Forgejo endpoint, an actual SQLite file, a subprocess matching Claude's JSON
   envelope, and race-enabled concurrency tests.

## Friction and hot spots

There is no production code to review for coupling or hot spots. The main risk
is accidental architecture: copying either prototype into production would
also copy its disposable schema, flat package, and test substitutes.

## Unknowns

- The concrete macOS UI toolkit is intentionally not selected by the target
  architecture; it must satisfy the UI acceptance criteria behind the UI seam.
- The concrete Go SQLite driver is an implementation choice behind
  `database/sql`; the spike's cached driver is not a decision.
- Live Forgejo payload compatibility and real `show-me` usefulness remain
  unverified until the twenty-pull-request demonstrator runs.
