# ADR-0006: Deliver the pinned skill as a system prompt

- **Status:** Accepted on 18 September 2026; supersedes the delivery mechanism of ADR-0005
- **Date:** 2026-09-17
- **Owners:** PRadar maintainers
- **Related:** [ADR-0005](0005-isolate-untrusted-pull-request-content.md), `docs/product/PRD.md`, `.scratch/pradar-demonstrator/issues/06-analyse-one-due-version-with-restricted-claude.md`

## Context

ADR-0005 decided that an analysis runs with "only the pinned `show-me` skill
and read-only file discovery tools (`Read`, `Glob`, and `Grep`) in `dontAsk`
mode". It assumed the skill is delivered through the CLI's `Skill` tool, as
the integration prototype did.

Two captured runs of the demonstrator's exact arguments on 17 September 2026
showed that this mechanism cannot satisfy the decision:

- The startup event listed every skill installed for the operator
  (`update-config`, `schedule`, `loop`, `code-review` and others), all
  reachable through the `Skill` tool. The analysis therefore depended on the
  machine's personal configuration, which `--restricted` does not remove.
- A plugin skill is namespaced `show-me:show-me`, so the `/show-me` prefix in
  the prompt never expanded. No analysis had used the pinned guidance, while
  provenance recorded its version.

Disabling the operator's skills without disabling the pinned one is not
possible: the CLI switch that removes skill discovery removes all of them, and
the mode that skips personal configuration also skips the CLI's own login.

## Decision

Deliver the pinned `show-me` source as system-prompt content rather than as an
invocable skill. The analysis invocation disables every skill and slash
command, exposes exactly `Read`, `Glob` and `Grep`, and appends the pinned
`SKILL.md` verbatim, so its guidance always applies.

Verify the granted surface at runtime instead of trusting the arguments. Read
the CLI's event stream and reject the invocation as a technical failure when
the reported tools, skills, slash commands, plugins, MCP servers or permission
mode are wider than this policy. Record the tools the engine actually called
and the number of denied permission requests as analysis provenance.

Everything else in ADR-0005 stands: an application-owned temporary workspace,
no credential in the engine's environment, bounded input, output, duration and
turns, schema validation, Markdown sanitisation and strict Mermaid rendering.

## Consequences

- The pinned guidance applies deterministically and no longer depends on the
  operator's installation, so an analysis is reproducible from its provenance.
- `show-me` behaviours that need the skill machinery, an HTML artefact or a
  shell are unavailable. The corpus run must show that the remaining output is
  useful; the first live analyses used Markdown, Mermaid, trees and targeted
  diff excerpts.
- The prompt version changes, so analyses produced before this decision keep a
  distinct identity and are not comparable with later ones.
- A CLI release that renames or adds a startup field can fail every analysis
  until the verification is updated. That is the intended direction: a wider
  surface must stop the analysis rather than pass silently.
- Provenance grows by the list of tools used and the denial count, which the
  evaluation can weigh against usefulness.

## Alternatives considered

- Keeping the `Skill` tool and accepting the operator's skills was rejected
  because it makes an analysis depend on unversioned local configuration and
  contradicts ADR-0005's own reasoning about blast radius.
- Vendoring the skill as a plugin and naming it `show-me:show-me` in the
  prompt was rejected because it would still expose every other skill.
- Running the CLI with an isolated configuration directory was rejected
  because the engine then loses its own authentication.
- Rewriting the guidance in PRadar's own words was rejected because the PRD
  chooses `show-me`, and a paraphrase could not be pinned to an upstream
  revision.

## Verification

- `TestClaudeAnalyzer_UsesFixedReadOnlyToolsAndThePinnedSkill` asserts the
  fixed arguments, the verbatim appended guidance and the recorded tool use.
- `TestClaudeAnalyzer_RejectsAWiderGrantedSurface` rejects an extra tool, an
  operator skill, a slash command, a loaded plugin, an MCP server, a wrong
  permission mode and a missing startup event.
- `TestShowMePin_MatchesTheEmbeddedSkill` ties the recorded skill version to
  the vendored source and its pinned upstream revision.
- A live smoke run reports the granted surface and the tools actually used.
