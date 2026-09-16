# Engineering workflow

This is the lifecycle router for the repository. A phase advances only when its
exit gate is satisfied. `/research` and `/prototype` are branches used to answer
a specific open question, not mandatory ceremony.

| Phase | Invocation | Durable output | Exit gate |
| --- | --- | --- | --- |
| 0. Bootstrap | Installation plus `.agents/workflows/bootstrap.md` | Agent adapters, skill inventory, tool versions | `make doctor` passes |
| 1. Configure skills | `/setup-matt-pocock-skills` | `CLAUDE.md` or `AGENTS.md` integration plus `docs/agents/*` | Tracker, labels, and domain layout are explicit |
| 2. Discover | `/grill-with-docs` | `docs/product/PRD.md`, domain vocabulary, decision notes, runnable demonstrator | Problem, users, outcomes, constraints, and demo question are clear |
| 3. Understand current design | `/improve-codebase-architecture` | Review report and `docs/architecture/current-state.md` | Current seams and friction are evidence-backed |
| 4. Freeze target design | `/grill-with-docs`; `/research` or `/prototype` only for open questions | `target-state.md`, `invariants.md`, `failure-model.md`, accepted ADRs | Architecture style, dependency direction, ownership, failure behavior, and DI choice are decided |
| 5. Specify | `/to-spec` | A spec in the configured issue tracker | Scope, implementation decisions, testing seams, and exclusions are accepted |
| 6. Slice | `/to-tickets` | Tracer-bullet tickets with blocking edges | Every ticket is independently demonstrable and fits one fresh context window |
| 7. Implement | `use-modern-go`, then `/implement` | Production change, tests, review findings, doc updates | Ticket criteria and `make verify` pass; `/code-review` is resolved |
| 8. Integrate | CI | Reproducible quality signal | Required checks are green |
| 9. Maintain | `/improve-codebase-architecture` periodically | Prioritized deepening opportunities and ADR updates | Only evidence-backed changes enter the next spec/ticket cycle |

## Flow rules

1. The demonstrator may reveal the current design, but it does not define the
   target architecture by accident.
2. Phase 4 turns design choices into durable decisions before `/to-spec`.
3. A research note cites primary sources. If its conclusion is accepted, copy
   the decision into the spec or an ADR.
4. A prototype answers one question. Keep it off the main branch; retain a
   pointer to its branch and its verdict.
5. Architecture changes discovered during implementation return to phase 4 or
   receive a follow-up ticket. They are not hidden inside code review fixes.

## Artifacts and ownership

| Artifact | Owner | What belongs there |
| --- | --- | --- |
| `AGENTS.md` | Project maintainers | Short non-negotiable invariants |
| `docs/product/PRD.md` | Product discovery | Problem, outcomes, scope, constraints |
| `CONTEXT.md` | Domain modeling | Canonical terms and meanings |
| `docs/architecture/current-state.md` | Architecture review | Observed structure and friction |
| `docs/architecture/target-state.md` | Architecture decisions | Intended modules, seams, and dependency direction |
| `docs/adr/` | Decision owners | One durable decision and trade-offs per record |
| Configured tracker | Delivery workflow | Active spec, tickets, status, and blocking edges |
| Tests and CI | Implementation owners | Executable behavior and enforcement |
