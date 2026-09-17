# 06: Analyse one due version with restricted Claude

**What to build:** Claim one due Version de pull request, materialise only its
required content in an owned workspace, run the pinned HumanLayer `show-me`
skill through a restricted Claude CLI process, validate `pradar.analysis.v1`,
and publish the resulting Analyse with complete provenance.

**Blocked by:** 05: Debounce and schedule each analysis identity once.

**Status:** done

- [x] Exactly one application-owned analysis worker runs and claims one eligible
  item through an atomic SQLite mutation with a unique lease token and expiry.
- [x] The exact HumanLayer `show-me` source revision is pinned, project-managed,
  and recorded as part of analysis provenance.
- [x] Each invocation receives only the selected pull request's bounded content
  beneath an application-owned temporary root, with no Forgejo token or
  unrelated user checkout available.
- [x] Claude CLI is invoked directly without a shell, session persistence, or
  ambient secrets, and uses structured output, a bounded turn count, timeout,
  output limit, and the single configured model.
- [x] Claude can use only the pinned skill and read-only file discovery tools in
  `dontAsk` mode; shell, write, browser, network, MCP, and subagent tools are not
  exposed.
- [x] The adapter validates the outer CLI envelope, `pradar.analysis.v1`, schema
  version, pull-request identity, and head SHA before persistence.
- [x] A valid result contains intention, structural importance, risks, visual
  explanation, and change from the prior analysed SHA when one exists.
- [x] Prompt, skill, engine, model, input identity, duration, and locally
  available usage data are stored as provenance.
- [x] Result insertion and visible Carte projection commit atomically.
- [x] Temporary content is removed after a successful invocation.
- [x] Deterministic subprocess contract tests verify arguments, standard input,
  environment isolation, output bounds, cancellation, and schema validation
  without spending model tokens.
- [x] A malicious pull-request fixture cannot broaden the configured tool or
  permission policy.
- [x] `make verify` passes.


## Comments

2026-09-17 — Live evidence corrected two defects the fixture substitute hid.

Two real captured runs (model `sonnet`, about 0.04 USD each) showed that:

- the `Skill` tool exposed every skill installed for the user, not only the
  pinned one, and the `/show-me` prompt prefix never expanded, because a
  plugin skill is namespaced `show-me:show-me`. No analysis had used the
  pinned guidance, although provenance recorded its version.
- the startup event reports the surface actually granted, which nothing was
  checking.

The adapter now passes `--disable-slash-commands`, exposes `Read,Glob,Grep`
only, appends the pinned `SKILL.md` verbatim through `--append-system-prompt`,
and reads `--output-format stream-json` to verify the init event (tools,
skills, slash commands, plugins, MCP servers, permission mode) and to record
the tools actually called plus the permission-denial count as provenance.
`PromptVersion` moves to `pradar-prompt-v2`, so earlier analyses keep a
distinct identity.

Verified live on `merlin/docs#188`: init accepted, tools used `Read` and
`StructuredOutput`, no denial, 30 s, 0.087 USD, Mermaid body without HTML.
