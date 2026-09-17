# Project agent constitution

This file contains project invariants, not generic engineering tutorials. Keep
it short. Put explanations in `docs/`, reusable expertise in skills, and
mechanically enforceable rules in tests or CI.

## Before changing anything

1. Locate the current phase and exit gate in `docs/agents/workflow.md`.
2. Read the artifact that authorizes that phase: the user's request and PRD for
   discovery, or the full accepted spec/ticket for delivery.
3. Read `CONTEXT.md` or the relevant entry from `CONTEXT-MAP.md` when present.
4. Read the ADRs and architecture documents that touch the area.

Missing optional context files are not a blocker. Do not invent their contents.

## Sources of truth

- The PRD and accepted spec state the desired outcome and scope.
- `CONTEXT.md` defines domain vocabulary.
- Accepted ADRs record decisions and their trade-offs.
- Architecture documents describe current and target structure.
- Code and tests describe current executable behavior.

If these disagree, surface the conflict. Do not silently pick one or rewrite an
accepted decision inside an implementation ticket.

## Architecture invariants

- Right-size the architecture. Add a layer, interface, or dependency only when
  it protects a real seam or removes demonstrated coupling.
- Keep business rules independent from transports, persistence, frameworks, and
  vendor SDKs. External systems are reached through explicit boundaries.
- Dependencies flow toward business policy. Infrastructure may depend on the
  domain; the domain must not depend on infrastructure.
- Give every mutable resource and durable state transition an explicit owner.
  Never adopt or overwrite foreign state based on name alone.
- A read does not reserve the observed state. Read-modify-write flows must define
  their concurrency boundary and handle conflicts as expected outcomes.
- External mutations must be retry-safe, idempotent, or paired with a documented
  recovery/compensation strategy. Design for a crash between any two effects.
- Initialization and dependency wiring stay explicit at the composition root.
  Avoid hidden mutable globals, service locators, and side-effectful `init()`.
- Record a chosen application architecture and DI strategy in an ADR. Until
  then, prefer the smallest explicit structure and manual constructor wiring.

Project-specific invariants belong in `docs/architecture/invariants.md` and
should become executable tests whenever practical.

## Delivery invariants

- Implement from an accepted spec or a ticket with observable acceptance
  criteria. Do not expand its scope opportunistically.
- Tickets are tracer-bullet vertical slices that remain independently
  demonstrable or verifiable. Wide mechanical refactors use expand-contract.
- Use TDD at the highest stable, pre-agreed seam. Test observable behavior, not
  private implementation structure.
- Prefer modifying an existing seam to creating another one. New seams require
  a concrete consumer or testability need.
- Research notes support a decision; they do not become the decision. Promote
  accepted conclusions to the spec, architecture docs, or an ADR.
- Prototype code is throwaway. Preserve the question, evidence, and verdict;
  move only validated production behavior into the main implementation.
- Update documentation in the same change when behavior, vocabulary, a boundary,
  or a decision changes.

## Go work

- For every Go ticket, apply `use-modern-go` and the `golang-how-to` router when
  those skills are installed; load only the additional Go skills relevant to
  the task.
- Keep `cmd/<binary>/main.go` limited to configuration, dependency wiring,
  lifecycle, and process exit.
- Use `internal/` by default. Create public packages only for real external
  consumers.
- Accept interfaces at the consuming boundary and return concrete types. Do not
  create interfaces solely to mirror implementations.
- Every external call has a cancellation or timeout policy. Every goroutine has
  a clear owner and termination path.

## Completion gate

For Go changes, run `make verify`. CI must pass formatting, dependency hygiene,
`go vet`, race-enabled tests, and `golangci-lint`; the linter configuration
includes `modernize`.

Do not claim completion without reporting the verification actually performed
and any check that could not run.

## Navigation

- Lifecycle and phase gates: `docs/agents/workflow.md`
- Skill activation policy: `docs/agents/skills.md`
- Product intent: `docs/product/PRD.md`
- Architecture index: `docs/architecture/README.md`
- Quality gates: `docs/quality/quality-gates.md`
- Local agent procedures: `.agents/workflows/`


## Git

* Propose a commit at each coherent and meaningful step, keeping changes small and incremental.
* Write Conventional Commits following the Angular convention, in UK English, clearly expressing the intent of the change, and do not add Claude as an author or co-author.
conventional commit angular sans mettre Claude en auteur en traduisant l'intention et en anglais UK

## Implementation workflow

For each coherent implementation step:

1. Read the relevant specification, ADRs and existing code before making changes.
2. Implement the smallest coherent increment with matt-pocock /implement and /modern-go-guidelines:use-modern-go.
3. Run the project's automated checks.
4. Commit the increment using a Conventional Commit angular english uk.
5. Let the Codex CLI review gate independently review the implementation.
6. For every Codex finding:
  - verify the finding against the code and specification;
  - fix it if it is valid;
  - do not blindly apply suggestions.
7. Run the automated checks again after each correction.
8. Continue until the Codex review gate reports no actionable findings.

If Claude and Codex disagree about an architectural or product decision,
stop the loop and report:
- Claude's reasoning;
- Codex's finding;
- the relevant specification or ADR;
- the decision that requires human arbitration.

Do not change the specification or an ADR merely to satisfy the reviewer.