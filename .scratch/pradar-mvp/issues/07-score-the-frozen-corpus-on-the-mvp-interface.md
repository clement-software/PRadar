# 07: Score the frozen corpus on the MVP interface

**What to build:** Run the PRD gate the product owner deferred on 18 September
2026: twenty frozen pull requests, read and scored in the MVP interface, with
a reproducible verdict that either permits or blocks the continued build.

**Blocked by:** 05: Show the interface in a native window.

**Status:** ready-for-human

- [ ] Before scoring, every automated gate passes and the worktree contains no
  unreviewed change.
- [ ] The user supplies the instance, a read-only Keychain token, two or three
  repositories, twenty pull requests and one global model, none of which is
  stored in the tracker or the repository.
- [ ] The corpus mixes small and large changes, human and agent contributions,
  and code, CI and infrastructure work, and its head SHAs are frozen before
  the run.
- [ ] The prompt, skill, engine, model, contract and interface versions are
  recorded before scoring begins.
- [ ] Each item is read in the MVP interface with a one-minute timer, and the
  evaluator records the four comprehension answers, usefulness and critical
  factual errors separately.
- [ ] The report lists passes, failures, timings, factual errors, corpus
  identity and every analysis version, and contains no credential or
  pull-request body.
- [ ] A pass records at least sixteen useful analyses understood in under one
  minute with zero critical factual errors.
- [ ] A failure blocks further product work until the analysis or its
  presentation changes, after which the whole corpus is rerun rather than
  partially rescored.
- [ ] The verdict and the resulting product decision are promoted to
  `docs/product/PRD.md`.
