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


## Comments

2026-09-17 — Procedure prepared by the agent; every value below is the user's.

1. `printf '%s' "$TOKEN" | pradar token set --instance https://<forge>` (read-only token, Keychain only).
2. `pradar corpus candidates --instance https://<forge> owner/a owner/b [owner/c] > manifest.json`
   lists the fifteen most recently updated pull requests per repository with a
   guessed `size` (large from 200 changed lines) and `authorship` (bot-like
   logins); `category` is empty and `reason` starts with `TODO` plus the title,
   author, state, size and URL to help the choice.
3. Trim to exactly twenty items over two or three repositories, keep the mix
   (small/large, human/agent, code/ci/infra), fill `category` and `reason`,
   and remove drafts. Head SHAs are frozen as listed.
4. `pradar corpus freeze manifest.json` validates the breadth rules and stores
   the manifest under its content id; any later change is a new corpus.
5. `pradar run --instance https://<forge> --model <model>`, subscribe the same
   repositories from the timeline (import "all" so the frozen heads are
   analysed), let the anti-rebond and the worker run, then score each item
   from its detail page and read `/evaluation` and `/evaluation/report.json`.
