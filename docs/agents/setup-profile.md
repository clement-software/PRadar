# Setup profile for Matt Pocock engineering skills

Use this file as the repository-specific brief when running
`/setup-matt-pocock-skills`. It complements that setup; it does not replace its
exploration and confirmation steps.

## Preserve

- `AGENTS.md` is the canonical project constitution. Do not copy generic skill
  instructions into it and do not replace it.
- `CLAUDE.md` is the Claude Code adapter. Update only its `## Agent skills`
  block when setup needs to change repository integration.
- Existing ADRs, domain vocabulary, workflow documents, and user edits remain
  authoritative unless the user explicitly changes them.

## Repository defaults

- Issue tracker: prefer the repository's real remote tracker when one exists;
  otherwise use local Markdown under `.scratch/<feature>/`.
- Local Markdown artifacts are intended to be versioned. They are the active
  delivery log, not a substitute for durable architecture documentation.
- Triage vocabulary: keep the five defaults unless the selected tracker already
  has an established mapping.
- Domain layout: use single-context (`CONTEXT.md` plus `docs/adr/`) unless actual
  monorepo or bounded-context evidence justifies `CONTEXT-MAP.md`.

## Context the setup should expose

- Lifecycle router: `docs/agents/workflow.md`
- Skill policy: `docs/agents/skills.md`
- Product discovery: `docs/product/PRD.md`
- Architecture index: `docs/architecture/README.md`
- Quality gate: `docs/quality/quality-gates.md`

## Done when

- Exactly one `## Agent skills` block exists in the selected adapter.
- `docs/agents/issue-tracker.md`, `domain.md`, and (when triage is installed)
  `triage-labels.md` describe this project rather than the template in general.
- The setup did not choose an application architecture or DI framework. Those
  are phase-4 decisions backed by the PRD, demonstrator, and ADRs.
