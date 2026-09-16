# 11: Execute the live demonstrator gate

**What to build:** Run the completed demonstrator against the user's real
Forgejo corpus, collect the human Compréhension assessment for all twenty pull
requests, and record the reproducible verdict that either permits or blocks the
next production phase.

**Blocked by:** 10: Freeze and score the evaluation corpus.

**Status:** ready-for-human

- [ ] Before live execution, all automated verification gates pass and the
  worktree contains no unreviewed demonstrator change.
- [ ] The user supplies the selected Instance Forge, read-only Keychain token,
  two or three repositories, twenty pull requests, and one global Claude model
  without storing any credential in the tracker or repository.
- [ ] The pinned `show-me`, prompt, engine, model, analysis contract, and
  presentation versions are recorded before scoring begins.
- [ ] A separately invoked live smoke test validates Forgejo access, Keychain
  lookup, workspace isolation, one real Claude invocation, schema validation,
  cleanup, and visual rendering before the corpus run.
- [ ] The frozen corpus satisfies the agreed breadth criteria and remains
  unchanged throughout the scored run.
- [ ] The evaluator completes all twenty one-minute Compréhension assessments
  and records usefulness and critical factual errors for every item.
- [ ] The final report contains all twenty outcomes and enough provenance to
  reproduce the run without exposing credentials or pull-request bodies.
- [ ] A pass records at least sixteen useful Analyses understood in under one
  minute and zero critical factual errors.
- [ ] A failing verdict explicitly blocks production implementation; any prompt,
  skill, model, contract, corpus, or presentation correction is followed by a
  complete new scored run rather than partial rescoring.
- [ ] The accepted verdict and any resulting product decision are promoted to
  the durable product documentation rather than remaining only in this ticket.

