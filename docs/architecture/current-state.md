# Current architecture

**Status:** Assessed on 16 September 2026, updated as the demonstrator lands

The repository now contains the disposable vertical demonstrator specified in
`.scratch/pradar-demonstrator/spec.md`. Its package layout is evidence for the
PRD gate, not the production design (see `target-state.md`).

## Runtime shape

`cmd/pradar` is one macOS process. `pradar run` opens one SQLite database in
WAL mode, scavenges the owned workspace root, starts one polling collector and
one analysis worker under a root context, and serves a loopback visualizer.
`--controlled` substitutes a fixture Forgejo and a deterministic analyzer and
sets the anti-rebond to zero so the demo shows a carte within seconds; live
mode keeps the ten-minute window of ADR-0004.
`--instance <https url> --model <model>` uses the real instance with the
token read from the Keychain (`pradar token set`) and the restricted Claude
CLI analyzer.

## Modules and seams

| Package | Owns | Depends on |
| --- | --- | --- |
| `internal/pullrequest` | Identity, input revision, analysis identity, `pradar.analysis.v1`, the pure `Reconcile` lifecycle decision | standard library only |
| `internal/collect` | Abonnements, polling reconciliation, durable anti-rebond; declares the `Forge` and `CollectionStore` interfaces it consumes | `internal/pullrequest` |
| `internal/analyse` | The single leased worker, workspace materialisation, engine invocation, validation, retry and the live smoke check; declares the `WorkStore`, `Workspace`, `Analyzer` and `ContentFetcher` interfaces | `internal/pullrequest` |
| `internal/timeline` | Timeline and detail queries, read, archive and replay, corpus freezing and scoring; declares the `ReadModel` and `EvaluationStore` interfaces | `internal/pullrequest`, `internal/evaluation` |
| `internal/evaluation` | Corpus manifest validation and content id, per-item score, deterministic threshold report | `internal/pullrequest` |
| `internal/adapter/sqlite` | Schema, observation and scheduling transaction, atomic leased claim, completion with latest-only publication, read model, user state, corpus and scores | `database/sql`, `modernc.org/sqlite`, the three use-case packages |
| `internal/adapter/forgejo` | Instance and repository URL validation, read-only bounded HTTP client, self-redacting `Token` | `net/http`, `internal/pullrequest` |
| `internal/adapter/claudecli` | Restricted `claude -p` invocation, pinned `show-me` guidance, runtime verification of the granted surface, stream decoding and contract validation | `os/exec`, `internal/analyse` |
| `internal/adapter/keychain` | Token lookup and storage through `/usr/bin/security` | `os/exec`, `internal/adapter/forgejo` |
| `internal/adapter/desktop` | The native macOS window over the owned origin, and handing an external link to the browser | `webview`, `os/exec` |
| `internal/adapter/wake` | Resume detection from a wall-clock jump, without a platform framework | `time` |
| `internal/adapter/workspace` | Owned temporary root, safe materialisation, cleanup and startup scavenging | `os` |
| `internal/controlled` | Deterministic forge and analyzer substitutes for `--controlled` | `internal/collect`, `internal/analyse` |
| `internal/ui` | Reading interface served on loopback: timeline, detail, filters, reading actions, abonnement management, evaluation and usage export, with safe Markdown rendering and embedded pinned assets | `net/http`, `html/template`, `goldmark`, `internal/collect`, `internal/timeline` |
| `internal/apptest` | Cross-boundary application tests over a real temporary database with controlled substitutes | the packages under test |

`internal/architecture_test.go` rejects outward imports from the policy
packages. Application tests use a real temporary SQLite file and controlled
substitutes at the interfaces above.

## Data and control flow

0. Launch, every poll tick and every resume from sleep run the same complete
   reconciliation; a resume drops a pending tick so waking costs one catch-up.
   An abonnement whose authorised engine no longer matches the configured one
   is deactivated with a visible reason until the user authorises it again.
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
   replay and abonnement actions to the use cases. `/evaluation` shows the
   frozen corpus, the scorecard timer lives on each corpus item's detail and
   `/evaluation/report.json` exports the deterministic report.

## Packaging

`make app` builds `PRadar.app` from a clean checkout: the icon is generated by
`tools/mkicon`, the version comes from git and is reported by the application
itself, and `Info.plist` carries the identifier, the version, the icon and the
minimum macOS version. Signing and notarisation are separate targets that read
credentials from the operator's keychain; `docs/release.md` holds the
procedure.

## Friction and hot spots

- The database has an ordered migration ladder applied at startup, a copy
  taken before any migration, and an explicit refusal for a newer or
  unreadable file. A released migration's statements are frozen; a change to
  the durable shape is a new step.
- Retry delay, lease duration and polling are process configuration; there is
  no daily ceiling by product decision.

## Engine surface

The analysis invocation follows
[ADR-0006](../adr/0006-deliver-the-pinned-skill-as-a-system-prompt.md): every
skill and slash command is disabled, the tools are `Read`, `Glob` and `Grep`,
and the pinned `SKILL.md` is appended verbatim to the system prompt. That
decision came from two captured runs on 17 September 2026, which showed the
`Skill` tool also exposed every skill installed for the operator and that a
plugin skill is namespaced `show-me:show-me`, so the `/show-me` prefix never
expanded and no analysis had used the pinned guidance.

Each invocation verifies the startup event and fails the analysis when the
reported tools, skills, slash commands, plugins, MCP servers or permission
mode are wider than that policy. Provenance records the tools the engine
actually called and the number of denied permission requests.

## Unknowns

- Live Forgejo payload compatibility and real `show-me` usefulness remain
  unverified until the twenty-pull-request demonstrator runs.
- The production macOS UI toolkit is still not selected; the visualizer only
  proves rendering of cartes, Markdown and Mermaid.
