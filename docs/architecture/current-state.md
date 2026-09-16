# Current architecture

**Status:** Assessed on 16 September 2026, updated as the demonstrator lands

The repository now contains the disposable vertical demonstrator specified in
`.scratch/pradar-demonstrator/spec.md`. Its package layout is evidence for the
PRD gate, not the production design (see `target-state.md`).

## Runtime shape

`cmd/pradar` is one macOS process. `pradar run` opens one SQLite database in
WAL mode, scavenges the owned workspace root, starts one polling collector and
one analysis worker under a root context, and serves a loopback visualizer.
`--controlled` substitutes a fixture Forgejo and a deterministic analyzer.

## Modules and seams

| Package | Owns | Depends on |
| --- | --- | --- |
| `internal/pullrequest` | Identity, input revision, analysis identity, `pradar.analysis.v1`, the pure `Reconcile` lifecycle decision | standard library only |
| `internal/app` | Abonnements and reconciliation (`Collector`), the single leased worker (`Worker`), reading use cases (`Timeline`); declares the `Forge`, `CollectionStore`, `WorkStore`, `Workspace`, `Analyzer` and `ReadModel` interfaces it consumes | `internal/pullrequest` |
| `internal/adapter/sqlite` | Schema, observation+scheduling transaction, atomic leased claim, completion with latest-only publication, read model, user state | `database/sql`, `modernc.org/sqlite`, `internal/app` |
| `internal/adapter/workspace` | Owned temporary root, safe materialisation, cleanup and startup scavenging | `os` |
| `internal/adapter/forgejo` | Instance and repository URL validation (HTTP client follows in ticket 03) | `net/url` |
| `internal/controlled` | Deterministic Forgejo and analyzer substitutes for `--controlled` | `internal/app` |
| `internal/ui` | Loopback HTTP visualizer, safe Markdown rendering, embedded assets | `net/http`, `html/template`, `goldmark`, `internal/app` |

`internal/architecture_test.go` rejects outward imports from the policy
packages. Application tests use a real temporary SQLite file and controlled
substitutes at the interfaces above.

## Data and control flow

1. `Collector.reconcile` lists open pull requests, applies author and draft
   exclusions, and calls `ObservePullRequest`, which runs `pullrequest.Reconcile`
   inside one transaction: upsert the observed version, mark it unread,
   supersede the queued candidate and enqueue the analysis identity with its
   anti-rebond not-before time.
2. `Worker.RunOne` claims one due item with a lease token, materialises the
   description and diff in an owned workspace, calls the analyzer, validates
   the contract and completes in one transaction that appends the result and
   publishes the carte only when the identity still matches the latest
   observed identity of an active abonnement generation.
3. The visualizer reads cartes, details, status and posts read, archive,
   replay and abonnement actions to the use cases.

## Friction and hot spots

- The demonstrator database has a single schema version and refuses older
  files instead of migrating; acceptable for disposable evidence only.
- Retry delay, lease duration and polling are process configuration; there is
  no daily ceiling by product decision.

## Unknowns

- Live Forgejo payload compatibility and real `show-me` usefulness remain
  unverified until the twenty-pull-request demonstrator runs.
- The production macOS UI toolkit is still not selected; the visualizer only
  proves rendering of cartes, Markdown and Mermaid.
