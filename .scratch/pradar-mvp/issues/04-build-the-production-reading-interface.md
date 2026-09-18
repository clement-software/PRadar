# 04: Build the production reading interface

**What to build:** Rebuild the timeline, the detail and the reading actions
against the production use cases, with the layout direction of `ui-model/`,
replacing the demonstrator's disposable visualizer.

**Blocked by:** 03: Complete the collection behaviours the PRD requires.

**Status:** done

- [x] The timeline shows one carte per non-archived pull request ordered by
  recent activity, with repository, number, title, intention, author, update
  time, state, importance, risks and read state.
- [x] The detail shows the Forgejo header, the complete analysis, the change
  since the previous analysed version, the historique and provenance in a
  secondary section.
- [x] Filters cover read state, repository, pull-request state, importance and
  risk without mutating durable data.
- [x] Marking the current version read, archiving, opening in Forgejo and
  replaying are available from the carte or the detail.
- [x] Degraded state is understandable without logs: last successful
  synchronisation, pending and running work, retries and blocked abonnements.
- [x] Rendering stays sanitised, links use safe schemes, Mermaid renders in
  strict mode from pinned embedded assets, and arbitrary HTML is rejected.
- [x] The interface follows the system theme, keeps WCAG AA contrast, shows
  visible focus and allows keyboard traversal of the timeline and the detail.
- [x] The aesthetic direction of `ui-model/` is followed without copying the
  demonstrator's layout.
- [x] The server binds loopback on an ephemeral port, sends no remote
  telemetry and exposes no write operation to Forgejo.
- [x] Acceptance tests assert observable reading behaviour and hostile content
  staying inert, without snapshotting incidental styling.
- [x] `make verify` passes.
