# ADR-0001: Use a modular monolith and manual wiring

- **Status:** Accepted
- **Date:** 2026-09-16
- **Owners:** PRadar maintainers
- **Related:** `docs/product/PRD.md`, `docs/architecture/target-state.md`

## Context

PRadar is one macOS desktop application for one user. It has meaningful seams
at Forgejo, local persistence, Claude CLI, Keychain, temporary workspaces, and
the desktop UI, but no evidence for multiple deployable services or a general
plugin platform. Business lifecycle rules must remain testable without any of
those technologies.

## Decision

Build one process as a modular monolith. Put domain policy and application use
cases in focused `internal/` packages. Define narrow interfaces at the packages
that consume external capabilities; implement them in adapter packages.

Use manual constructor injection in `cmd/pradar/main.go`. That file owns
configuration, resource construction, root cancellation, goroutine lifecycle,
and process exit. Do not use a DI framework, service locator, hidden mutable
global, side-effectful `init()`, generic repository layer, or event bus.

## Consequences

- Domain and use-case tests can use in-memory fakes at real seams.
- The UI toolkit, SQLite driver, and vendor clients can change without moving
  business policy.
- Wiring is explicit but `main` will contain deliberate constructor ceremony.
- Interfaces are added only for a concrete consumer; small domain functions
  remain concrete.

## Alternatives considered

- A flat package was rejected because it would couple five volatile external
  technologies directly to lifecycle policy.
- Layered clean-architecture scaffolding was rejected because generic entities,
  gateways, and presenters would add indirection without more use cases.
- Microservices were rejected because the MVP has one process, user, machine,
  and local data owner.
- A DI container was rejected because the expected dependency graph is small
  and has no scoped or dynamic resolution requirement.

## Verification

- `TestArchitecture_DependenciesPointInward` rejects forbidden imports.
- `cmd/pradar/main.go` contains all production constructors and owned shutdown.
- No domain or application package imports an adapter or UI package.
