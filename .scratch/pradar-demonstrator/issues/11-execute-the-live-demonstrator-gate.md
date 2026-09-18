# 11: Execute the live demonstrator gate

**What to build:** Run the completed demonstrator against the user's real
Forgejo corpus, collect the human Compréhension assessment for all twenty pull
requests, and record the reproducible verdict that either permits or blocks the
next production phase.

**Blocked by:** 10: Freeze and score the evaluation corpus.

**Status:** deferred (ready-for-human)

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
5. Before scoring, run the live smoke check once:
   `pradar smoke --instance https://<forge> --pull-request owner/name#N --model <model>`.
   It validates the Keychain token, repository access, pull-request metadata,
   one real restricted Claude analysis with contract validation, workspace
   cleanup and rendering, and writes nothing to the database.
6. `pradar run --instance https://<forge> --model <model>`, subscribe the same
   repositories from the timeline (import "all" so the frozen heads are
   analysed), let the anti-rebond and the worker run, then score each item
   from its detail page and read `/evaluation` and `/evaluation/report.json`.

2026-09-17 — First live evidence (agent, on the user's test repository).

- Instance `git.mbvsi.fr` runs Forgejo 14.0.2; the read-only token is stored in
  the Keychain only.
- `pradar corpus candidates` listed 50 pull requests of `merlin/docs` across
  pages with correct states (open, merged, closed), head SHAs and change sizes.
- `pradar smoke` on `merlin/docs#187` passed every step with model `sonnet`:
  repository access 89 ms, metadata 49 ms, analysis and contract validation
  23 s (4 turns, no web request, estimated cost 0.10 USD), cleanup, rendering.
  The body used Mermaid, a file tree and a table, with no HTML.
- `merlin/docs` alone cannot form the scored corpus: it is one repository,
  documentation only, and every recent author is human. The frozen corpus
  still needs a second or third repository with agent-authored, CI and
  infrastructure pull requests.
- Not yet proven: that the `show-me` skill itself was invoked (session
  persistence is disabled, so the tool trace is not kept).

2026-09-18 — The PRD's end-to-end scenario ran live on `merlin/docs`.

`pradar run --instance https://git.mbvsi.fr --model sonnet --debounce 0` with
the abonnement created from the repository URL through the visualizer:

- Launch reconciliation imported the open pull requests and scheduled both.
- The single worker claimed and completed each one in turn (about 30 s each,
  roughly 0.09 USD each); no retry, no unavailable card.
- The timeline showed one carte per pull request with intention, importance
  and risks; the detail showed the Forgejo link, two Mermaid diagrams, the
  change-since-previous section and provenance naming the model, the pinned
  skill revision and the duration.
- Marking the carte read removed it from the unread filter; archiving removed
  it from the timeline and the detail reported it archived.
- A restart against the same database restored one visible carte and the
  archived state, scheduled no work and spent nothing. The workspace
  directory was empty after every analysis.

Still not exercised live: réapparition after a new commit, which needs a push
to the repository, and the three-failure unavailable path. Both are covered by
the automated tests.

2026-09-18 — Deferred by the product owner. The scored corpus run happens on
the MVP interface rather than the disposable visualizer, so this ticket waits
for that interface. The threshold and the protocol are unchanged, and the
decision is recorded in `docs/product/PRD.md`.
