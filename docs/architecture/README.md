# Architecture

This directory separates observations from decisions. Do not describe a target
as if it already exists, and do not turn an accidental demonstrator structure
into a rule.

**MVP target status:** Accepted on 16 September 2026. Production implementation
has not started. The target is therefore a constraint on future code, not a
description of code already present.

| File | Purpose |
| --- | --- |
| `current-state.md` | Evidence-backed structure, seams, coupling, and hot spots observed in the demonstrator/codebase |
| `target-state.md` | Intended modules, dependency direction, composition root, and migration shape |
| `invariants.md` | Project-specific rules that must survive refactors and language/framework changes |
| `failure-model.md` | Ownership, concurrency, retry, crash, partial-failure, and recovery behavior |
| `../adr/` | One accepted or rejected durable decision per record |

## Decision discipline

1. Observe the current state with `/improve-codebase-architecture`.
2. Use `/grill-with-docs` to make target trade-offs explicit.
3. Use `/research` for uncertain facts and `/prototype` for uncertain behavior
   or interaction.
4. Record load-bearing decisions as ADRs.
5. Convert important invariants into architecture or behavior tests.

Phase 4 of `docs/agents/workflow.md` completed the architecture and dependency
injection decisions listed below. Reopen them through a superseding ADR rather
than silently changing this overview.

## Accepted decisions

- [ADR-0001](../adr/0001-modular-monolith-and-manual-wiring.md): modular
  monolith, explicit external ports, and manual constructor wiring.
- [ADR-0002](../adr/0002-sqlite-and-leased-analysis-work.md): SQLite local
  state with idempotent, leased analysis work.
- [ADR-0003](../adr/0003-versioned-claude-analyzer-boundary.md): Claude CLI
  behind a versioned analyzer contract.
- [ADR-0004](../adr/0004-durable-polling-and-latest-only-publication.md):
  durable polling/debounce and latest-only publication.
- [ADR-0005](../adr/0005-isolate-untrusted-pull-request-content.md): isolate
  untrusted pull-request content and keep credentials outside analysis.

## Prototype evidence

- `codex/prototype/pradar-state-machine` at `471a8d2` validated lifecycle and
  stale-result behavior with the user.
- `codex/prototype/pradar-integration-spike` at `2007e82` validated atomic
  claims, lease recovery, retry exhaustion, stale publication protection, and
  the Claude JSON process boundary with a deterministic substitute.

The second prototype did not invoke a live Forgejo instance or `show-me`.
Those checks remain part of the PRD demonstrator gate and cannot be represented
as completed implementation evidence.
