# 06: Analyse one due version with restricted Claude

**What to build:** Claim one due Version de pull request, materialise only its
required content in an owned workspace, run the pinned HumanLayer `show-me`
skill through a restricted Claude CLI process, validate `pradar.analysis.v1`,
and publish the resulting Analyse with complete provenance.

**Blocked by:** 05: Debounce and schedule each analysis identity once.

**Status:** ready-for-agent

- [ ] Exactly one application-owned analysis worker runs and claims one eligible
  item through an atomic SQLite mutation with a unique lease token and expiry.
- [ ] The exact HumanLayer `show-me` source revision is pinned, project-managed,
  and recorded as part of analysis provenance.
- [ ] Each invocation receives only the selected pull request's bounded content
  beneath an application-owned temporary root, with no Forgejo token or
  unrelated user checkout available.
- [ ] Claude CLI is invoked directly without a shell, session persistence, or
  ambient secrets, and uses structured output, a bounded turn count, timeout,
  output limit, and the single configured model.
- [ ] Claude can use only the pinned skill and read-only file discovery tools in
  `dontAsk` mode; shell, write, browser, network, MCP, and subagent tools are not
  exposed.
- [ ] The adapter validates the outer CLI envelope, `pradar.analysis.v1`, schema
  version, pull-request identity, and head SHA before persistence.
- [ ] A valid result contains intention, structural importance, risks, visual
  explanation, and change from the prior analysed SHA when one exists.
- [ ] Prompt, skill, engine, model, input identity, duration, and locally
  available usage data are stored as provenance.
- [ ] Result insertion and visible Carte projection commit atomically.
- [ ] Temporary content is removed after a successful invocation.
- [ ] Deterministic subprocess contract tests verify arguments, standard input,
  environment isolation, output bounds, cancellation, and schema validation
  without spending model tokens.
- [ ] A malicious pull-request fixture cannot broaden the configured tool or
  permission policy.
- [ ] `make verify` passes.

