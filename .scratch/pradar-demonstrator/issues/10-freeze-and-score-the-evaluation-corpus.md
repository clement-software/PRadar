# 10: Freeze and score the evaluation corpus

**What to build:** Add the local evaluation workflow that freezes a representative
twenty-pull-request corpus, times and records the user's Compréhension for every
Analyse, and produces a deterministic report whose threshold verdict can gate
production work.

**Blocked by:** 09: Complete the safe reading workflow.

**Status:** done

- [x] The user can select and freeze exactly twenty pull requests from two or
  three configured repositories before a scored run begins.
- [x] The immutable corpus manifest records repository identity, pull-request
  number, frozen head SHA, size/category, authorship kind, and inclusion reason,
  but no token or pull-request body.
- [x] Corpus validation requires a deliberate mix of small and large changes,
  human and agent contributions, and code, CI, and infrastructure work.
- [x] Each item offers a one-minute timer and records whether the user can
  explain intention, structural change, risks, and the need for deeper review.
- [x] The evaluator records usefulness and critical factual errors separately;
  presentation quality cannot conceal a factual failure.
- [x] Each score retains the exact analysis identity, elapsed comprehension
  time, analysis duration, and locally available usage provenance.
- [x] The generated report deterministically lists passes, failures, timings,
  factual errors, corpus identity, analysis versions, and the aggregate verdict.
- [x] Passing requires at least sixteen useful Analyses out of twenty, each
  understood in under one minute, and zero critical factual errors.
- [x] Changing the corpus, prompt, skill, engine, model, analysis contract, or
  presentation invalidates the previous aggregate and requires a complete rerun.
- [x] Automated tests use fixtures, never call live Forgejo or Claude, and cover
  manifest immutability, incomplete scoring, threshold edges, invalidation, and
  deterministic report generation.
- [x] Evaluation exports contain no credentials, PR bodies, or temporary source
  content.
- [x] `make verify` passes.

