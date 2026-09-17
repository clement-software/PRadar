# Spec: Validate the PRadar vertical demonstrator

Status: ready-for-agent

Date: 2026-09-16

Sources: validated PRD, PRadar glossary, ADR-0001 through ADR-0005, lifecycle
prototype `471a8d2`, and integration prototype `2007e82`.

## Problem Statement

The user needs evidence that PRadar can turn real Forgejo pull requests into
useful, trustworthy explanations before investing in the production desktop
application. Architecture alone cannot prove that `show-me` understands the
actual repositories, that the end-to-end integration is reliable, or that the
result can be understood in under one minute.

The current prototypes validate lifecycle logic and local SQLite coordination,
but they substitute the live Forgejo and Claude boundaries. Without a vertical
demonstrator, the highest product risk remains unresolved: PRadar could be
technically sound while producing explanations that are slow, misleading, or
too difficult to read.

## Solution

Build a disposable but runnable macOS demonstrator that follows selected pull
requests from one real Forgejo instance through polling, durable anti-rebond,
leased SQLite work, a restricted Claude CLI invocation using a pinned
`show-me`, validation of `pradar.analysis.v1`, and a local visualizer containing
the timeline and detail views needed for evaluation.

Run the demonstrator against a frozen corpus of twenty real pull requests from
two or three repositories. Record the time to comprehension and the user's
assessment for every pull request. The gate passes only when at least sixteen
of twenty analyses enable comprehension in under one minute and no analysis
contains a critical factual error. A failure changes the prompt, skill input,
contract use, or presentation and then reruns the full frozen corpus.

This spec validates the demonstrator. It does not authorise production UI work
or copying disposable demonstrator structure into the MVP.

## User Stories

1. As the user, I want to configure one Forgejo instance, so that the corpus is
   collected from the forge I actually use.
2. As the user, I want the demonstrator to reject malformed or unsupported
   instance URLs, so that it never sends a token to an unintended destination.
3. As the user, I want to supply a read-only Forgejo token without placing it in
   source control or SQLite, so that the evaluation does not create a credential
   leak.
4. As the user, I want the demonstrator to validate the token and repository
   access before activating an abonnement, so that failures are immediate and
   actionable.
5. As the user, I want to add repositories by URL, so that I do not need to
   reconstruct Forgejo API identifiers manually.
6. As the user, I want an abonnement to import the ten most recently updated
   open pull requests by default, so that the first run stays bounded.
7. As the user, I want to choose none, ten, or all currently open pull requests
   when creating an abonnement, so that corpus preparation remains explicit.
8. As the user, I want draft pull requests excluded by default, so that the
   demonstrator follows the agreed MVP behavior.
9. As the user, I want bot- and agent-authored pull requests included unless I
   exclude their authors, so that the corpus represents the volume PRadar is
   intended to handle.
10. As the user, I want every imported pull request associated with its exact
    head SHA and input revision, so that an analysis can be reproduced and
    attributed correctly.
11. As the user, I want the corpus manifest to freeze twenty pull requests from
    two or three repositories before evaluation, so that prompt or presentation
    changes are measured against the same evidence.
12. As the user, I want the corpus to contain small and large changes, human and
    agent contributions, and code, CI, and infrastructure work, so that passing
    the gate demonstrates useful breadth.
13. As the user, I want a complete reconciliation at startup, so that a stopped
    demonstrator catches up without webhooks.
14. As the user, I want Forgejo polled every five minutes while the demonstrator
    runs, so that new or changed pull requests are discovered automatically.
15. As the user, I want opening, reopening, head, title, and description changes
    to create a new input revision, so that every agreed analysis trigger is
    represented.
16. As the user, I want close and merge changes to update the pull-request state
    without scheduling another analysis, so that model work matches the PRD.
17. As the user, I want changes to the same pull request grouped for ten minutes,
    so that a commit burst analyses only its latest candidate.
18. As the user, I want anti-rebond state to survive restart, so that closing the
    demonstrator does not lose or duplicate pending work.
19. As the user, I want duplicate polling observations to remain idempotent, so
    that they do not multiply Claude usage or history entries.
20. As the user, I want exactly one analysis session to run at a time, so that
    local resource and model use remain predictable.
21. As the user, I want a claimed analysis to have an expiring durable lease, so
    that work becomes recoverable after interruption.
22. As the user, I want an expired lease to be reclaimed on restart, so that a
    crash cannot strand a pull request permanently.
23. As the user, I want a late worker to be unable to complete work after losing
    its lease, so that competing results cannot corrupt the visible state.
24. As the user, I want Claude CLI invoked without session persistence and with
    only the pinned skill and read-only discovery tools, so that pull-request
    content cannot broaden its privileges.
25. As the user, I want Forgejo content materialized in an owned temporary
    workspace without credentials, so that analysis can inspect what it needs
    without seeing unrelated files or secrets.
26. As the user, I want temporary code and diffs removed after success, failure,
    cancellation, and restart, so that source copies do not become durable data.
27. As the user, I want every analysis to use a recorded prompt, skill, engine,
    and model version, so that provenance is sufficient to explain a result.
28. As the user, I want Claude output validated against
    `pradar.analysis.v1`, so that malformed prose cannot silently become a card.
29. As the user, I want an analysis to state its intention, structural
    importance, risks, visual explanation, and change since the prior analysed
    SHA, so that I can decide whether deeper review is necessary.
30. As the user, I want Markdown, Mermaid, pseudocode, and targeted diffs but no
    arbitrary HTML, so that results remain visual without accepting an unsafe
    rendering surface.
31. As the user, I want an analysis failure retried three times with increasing
    delay, so that transient engine problems recover automatically.
32. As the user, I want the third failure to create a visible analyse
    indisponible state with the Forgejo link and manual replay, so that a failed
    model call never hides the pull request.
33. As the user, I want manual replay with a changed analysis identity to retain
    earlier results, so that improvements remain comparable and auditable.
34. As the user, I want a stale result kept in the historique de pull request
    without replacing the current carte, so that useful provenance survives
    without misleading the timeline.
35. As the user, I want the result and its visible projection committed
    atomically, so that a crash cannot create a card without its analysis.
36. As the user, I want the timeline to show one carte per non-archived pull
    request ordered by recent activity, so that the evaluation resembles the
    intended reading experience.
37. As the user, I want each carte to show repository, number, title, intention,
    author, update time, state, importance, risks, and read state, so that I can
    understand the change before opening the detail.
38. As the user, I want the detail to show the Forgejo header, complete analysis,
    change from the prior version, provenance, and historique de pull request,
    so that I can verify the explanation rather than trust a summary blindly.
39. As the user, I want rendered Markdown sanitised and Mermaid in strict mode,
    so that hostile model or repository output remains inert.
40. As the user, I want the visualizer to respect light and dark themes, visible
    focus, keyboard navigation, and WCAG AA contrast, so that evaluation is not
    distorted by an unusable presentation.
41. As the user, I want to open the original pull request in Forgejo from the
    carte or detail, so that deeper review remains one action away.
42. As the user, I want to mark the latest analysed version as lu, so that the
    timeline distinguishes what I have understood.
43. As the user, I want to archive a carte without deleting its historique, so
    that the active timeline stays focused.
44. As the user, I want a newly observed version to become non-lu immediately
    but an archived pull request to reappear only after its latest analysis is
    publishable, so that no empty or stale card returns early.
45. As the user, I want close or merge to leave an existing carte visible until
    I read or archive it, so that lifecycle changes do not remove unread context.
46. As the user, I want désabonnement to stop pending and future collection
    without deleting history, so that stopping work is distinct from erasure.
47. As the user, I want repository data deletion to be a separate confirmed
    action, so that désabonnement cannot destroy evidence accidentally.
48. As the user, I want the visualizer to show last successful synchronization,
    pending and running work, retries, and blocked abonnements, so that degraded
    behavior is diagnosable without reading logs.
49. As the user, I want local structured logs that exclude tokens and PR bodies,
    so that technical failures can be investigated safely.
50. As the user, I want no remote telemetry or daily analysis ceiling in this
    version, so that the demonstrator matches the accepted privacy and scope.
51. As the evaluator, I want a one-minute timer and a scorecard for each corpus
    item, so that the comprehension criterion is measured consistently.
52. As the evaluator, I want to record whether I can explain intention,
    structural change, risks, and the need for deeper review, so that usefulness
    is based on the PRD outcome rather than aesthetic preference.
53. As the evaluator, I want to record critical factual errors separately, so
    that a visually persuasive but incorrect analysis cannot pass.
54. As the evaluator, I want duration and locally available usage provenance
    captured for every analysis, so that quality improvements can be weighed
    against cost and latency.
55. As the evaluator, I want a final deterministic report showing passes,
    failures, timings, factual errors, and the exact corpus and analysis
    versions, so that the verdict can be reproduced and promoted to the PRD.
56. As the product owner, I want a failed threshold to stop production
    implementation, so that the team improves analysis or presentation before
    building the application around an unproven result.

## Implementation Decisions

- The deliverable is a vertical demonstrator and evaluation harness, not the
  production desktop application. Its internal package layout and schema are
  disposable evidence and must not be copied into production without a later
  ticket and tests.
- The demonstrator is one Go process on macOS. It owns one polling scheduler,
  one analysis worker, one SQLite connection pool, one local visualizer, and one
  root cancellation context.
- Use manual constructor wiring. Domain decisions remain independent of HTTP,
  SQL, process execution, Keychain, rendering, and the local visualizer.
- Use direct Forgejo HTTP calls rather than a broad vendor SDK. Accept only an
  HTTPS instance URL, except a loopback HTTP URL used by automated tests.
- Store the Forgejo token in macOS Keychain. Durable models, prompts, logs,
  provenance, fixtures, and evaluation exports must not contain it.
- Use one SQLite database in WAL mode for metadata, input revisions, durable
  anti-rebond, leased work, append-only analyses, visible projection, user
  state, provenance, and evaluation records.
- The demonstrator may use the SQLite driver already exercised by the
  integration prototype. That choice is demonstrator-only and does not select
  the production driver.
- Represent every analysis trigger as a deterministic input revision containing
  the head SHA, normalised title and description, and a persisted reopen
  generation. Combine it with prompt, skill, engine, and model versions to form
  the idempotency identity.
- Make observation plus scheduling one transaction. Duplicate identities are a
  successful no-op.
- Persist a not-before deadline for anti-rebond. Replacing the candidate also
  replaces its deadline; no in-memory timer is authoritative.
- Claim work with one atomic SQLite mutation that assigns a unique lease token,
  expiry, and incremented attempt. Completion and retry require the same live
  token.
- Never keep a SQLite transaction open during Forgejo, filesystem, renderer, or
  Claude I/O.
- Pin the HumanLayer `show-me` source and record its immutable revision. Load it
  from a project-managed location for the demonstrator rather than depending on
  an unversioned personal installation.
- Invoke Claude CLI non-interactively with structured JSON output, the analysis
  schema, no session persistence, a bounded number of turns, a configurable
  timeout, bounded output, and a single globally configured model.
- Limit Claude's available tools to the skill plus read-only file discovery.
  Use `dontAsk`; expose no shell, write, browser, network, MCP, or subagent tool.
- Create each analysis workspace below one demonstrator-owned temporary root.
  Materialize only the selected pull request input, never credentials or the
  user's unrelated checkout. Scavenge abandoned workspaces at startup.
- Treat titles, descriptions, diffs, repository files, Markdown, Mermaid, and
  model output as untrusted. Validate the JSON contract before persistence,
  sanitise Markdown output, and use strict Mermaid rendering.
- Keep valid stale results in history. Publish or unarchive only when the
  completed identity still matches the latest observed identity and active
  abonnement generation.
- Commit result insertion and visible-projection change in the same transaction.
- Retry technical analysis failure at most three times with increasing durable
  delay. The third failure publishes `unavailable`; cancellation caused by
  shutdown or désabonnement hands ownership back without consuming an attempt.
- A local-only visualizer binds to loopback on an ephemeral port and uses
  embedded, pinned assets. It renders the evaluation timeline and detail but
  makes no production toolkit decision.
- A test clock controls poll, anti-rebond, lease, retry, and wake behavior in
  automated tests. Live mode uses monotonic-aware wall time and the PRD
  intervals.
- The corpus manifest records repository identity, pull-request number, frozen
  head SHA, category, authorship kind, and inclusion reason. It contains no
  token and is not mutated during a scored run.
- The evaluation report records each analysis identity, elapsed comprehension
  time, the four comprehension answers, usefulness, critical factual errors,
  duration, locally available usage, and the final threshold verdict.
- No A/B test, blind evaluation, relevance score, topic taxonomy, notification,
  daemon, webhook, remote telemetry, or daily usage limit is introduced.

## Testing Decisions

- The primary test seam is the complete application use-case surface backed by
  a real temporary SQLite database and controlled substitutes for Forgejo,
  Claude CLI, Keychain, clock, workspace, and visualizer notification. Tests
  assert user-visible state and durable outcomes, not private function calls.
- Lifecycle policy receives focused table-driven tests for observation,
  anti-rebond replacement, latest-only publication, read/archive, close/merge,
  désabonnement, replay, and unavailable transitions. Port the behavioral cases
  validated by prototype `471a8d2` rather than copying its implementation.
- SQLite integration tests run with the race detector and prove duplicate
  scheduling, one-winner concurrent claim, lost-lease rejection, expired-lease
  recovery, atomic result/publication, three-attempt exhaustion, and restart
  recovery. Prototype `2007e82` is prior evidence, not reusable production code.
- Forgejo contract tests use an HTTP test server and representative payload
  fixtures for authentication, pagination, drafts, bots, reopen, close, merge,
  changed title/body/head, rate limiting, server failure, malformed JSON, and
  bounded response bodies.
- Claude contract tests execute a deterministic subprocess that verifies fixed
  arguments, stdin shape, cancellation, timeout, output cap, invalid envelope,
  invalid schema, mismatched PR/SHA, non-zero exit, and secret-free environment.
- Workspace tests use malicious names, symlinks, oversized inputs, cancellation,
  and simulated restart. All filesystem access stays under the owned root and
  cleanup is observable after every outcome.
- Renderer tests use hostile Markdown, Mermaid directives, links, and embedded
  HTML. Tests assert sanitised output and strict Mermaid behavior without
  snapshotting incidental styling.
- Visualizer acceptance tests cover timeline ordering, one carte per pull
  request, filters, detail content, Forgejo link, lu, archive, réapparition,
  light/dark themes, keyboard traversal, visible focus, and contrast.
- Automated tests use fixtures and never call the live Forgejo instance or spend
  model tokens. A separately invoked live smoke run validates the configured
  instance, Keychain access, and one real Claude analysis before corpus scoring.
- The scored twenty-item corpus run is a human acceptance test, not a CI test.
  It never compares model prose byte-for-byte. It validates contract fields and
  records the user's comprehension outcome and factual-error assessment.
- The gate passes only with at least sixteen useful analyses out of twenty, each
  understood in under one minute, and zero critical factual errors. Any corpus,
  prompt, skill, model, or presentation change invalidates the prior aggregate
  and requires a complete rerun.
- No implementation is complete until formatting, module hygiene, static
  analysis, race-enabled tests, linting, and the repository verification target
  pass. A missing repository harness prerequisite is reported rather than
  bypassed.

## Out of Scope

- Production desktop toolkit selection, application packaging, signing,
  notarisation, auto-update, or distribution.
- Reusing the demonstrator schema or package layout as the production design.
- Multiple Forgejo instances or any GitHub, GitLab, Bitbucket, or generic forge
  adapter.
- Forgejo webhooks.
- Claude API integration, multiple engines, per-abonnement model selection, or
  parallel analysis sessions.
- Relevance personalisation, “Pour moi”, topic threads, topic taxonomy, and
  cross-repository semantic grouping.
- Full-text search.
- Clone, checkout, worktrees, IDE opening, or modifying a repository.
- Automatic approval, rejection, merge, comment, review, or write operation on
  Forgejo.
- Arbitrary HTML analysis artefacts.
- System notifications, menu-bar operation, daemon mode, or analysis after the
  application closes.
- Remote telemetry, shared accounts, collaboration, or multi-user state.
- Linux and Windows support.
- An A/B comparison, blind study, automatic factuality score, or coverage
  percentage target.
- A daily analysis ceiling; a configurable ceiling may be specified later.

## Further Notes

- Live execution requires the user to provide the selected Forgejo instance,
  read-only token through Keychain, two or three repositories, twenty frozen
  pull requests, and the global Claude model. None of these values belongs in
  this tracker document.
- `show-me` is supplied by HumanLayer rather than the Matt Pocock skills
  repository. The demonstrator must pin and record the exact upstream revision
  before the first scored analysis.
- ADR-0001 through ADR-0005 are constraints. If implementation evidence
  contradicts one, return to architecture design and supersede the ADR rather
  than silently changing the spec inside a ticket.
- The repository harness currently lacks its configured CI workflow, so the
  bootstrap verification gate remains independently unresolved. Demonstrator
  work must not weaken or bypass that gate.
