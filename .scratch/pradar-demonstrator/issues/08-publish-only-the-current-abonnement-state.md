# 08: Publish only the current abonnement state

**What to build:** Preserve trustworthy visible state while pull requests change
or collection stops: every valid Analyse remains auditable, but only a result
matching the latest Version de pull request and active Abonnement may update or
restore its Carte.

**Blocked by:** 07: Recover analysis work and expose terminal failure.

**Status:** ready-for-agent

- [ ] Completing an Analyse whose identity no longer matches the latest observed
  identity appends it to the Historique de pull request but never updates the
  visible Carte.
- [ ] A result and any eligible visible-projection change commit together or not
  at all.
- [ ] A new observed Version de pull request becomes non-Lu immediately.
- [ ] An Archivé pull request stays absent while its new version is pending and
  reappears as non-Lu only after that latest version has a publishable result.
- [ ] Closing or merging schedules no Analyse and leaves an existing Carte
  visible until the user marks it Lu or Archivé.
- [ ] Désabonnement stops future collection, invalidates queued or debouncing
  work, cancels the active invocation, and prevents late completion from
  changing the active projection.
- [ ] Désabonnement retains pull-request state, provenance, analyses, and
  Historique de pull request.
- [ ] Repository data deletion is a separate destructive action that requires
  explicit confirmation and cannot be triggered by Désabonnement.
- [ ] Restart preserves read, archive, subscription generation, history, and
  latest-only publication decisions.
- [ ] Application-surface tests with real temporary SQLite exercise stale
  completion, close/merge, Désabonnement, late completion, deletion confirmation,
  and Archivé réapparition.
- [ ] `make verify` passes.

