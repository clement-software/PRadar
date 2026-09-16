# 09: Complete the safe reading workflow

**What to build:** Complete the demonstrator's reading experience so the user
can understand, inspect, filter, mark Lu, and archive each current Carte while
all repository and model-controlled presentation remains inert and accessible.

**Blocked by:** 08: Publish only the current abonnement state.

**Status:** ready-for-agent

- [ ] The Timeline shows at most one Carte per non-Archivé pull request, ordered
  by recent activity.
- [ ] Every Carte shows repository, number, title, intention, author, update
  time, pull-request state, Importance, Risques, and Lu state.
- [ ] The detail shows the Forgejo header, complete Analyse, change from the
  prior analysed version, provenance, and ordered Historique de pull request.
- [ ] Timeline filters cover Lu state, repository, pull-request state,
  Importance, and Risque without mutating durable data.
- [ ] Carte and detail provide a safe direct link to the original Forgejo pull
  request.
- [ ] Marking the current analysed version Lu and marking the Carte Archivé are
  durable use cases and survive restart.
- [ ] Markdown is sanitised, arbitrary HTML is rejected, links use safe schemes,
  and Mermaid renders in strict mode from embedded pinned assets.
- [ ] Hostile Markdown, Mermaid directives, links, HTML, titles, and model output
  stay inert under automated renderer security tests.
- [ ] The visualizer supports light and dark themes, keyboard traversal, visible
  focus, semantic labels, and WCAG AA contrast.
- [ ] Last successful synchronisation, pending/running counts, retries, and
  blocked Abonnements are understandable without reading logs.
- [ ] The visualizer remains loopback-only, sends no remote telemetry, and
  introduces no notification, daemon, webhook, or daily analysis ceiling.
- [ ] Acceptance tests verify observable reading behavior without snapshotting
  incidental styling.
- [ ] `make verify` passes.

