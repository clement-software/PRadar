# Skill policy

Skills provide generic expertise and workflow behavior. Project facts and
decisions stay in this repository so they remain reviewable and versioned.

## Recommended stack

| Layer | Source | Use |
| --- | --- | --- |
| Workflow | `mattpocock/skills` | grilling, domain modeling, research, prototyping, specification, tickets, TDD, implementation, review |
| Go language | `JetBrains/go-modern-guidelines` | `use-modern-go` on every Go implementation ticket |
| Go routing | `samber/cc-skills-golang` | `golang-how-to` selects task-relevant Go guidance |
| Architecture | Project choice such as `wshobson/agents` | Generic patterns when an architecture question actually needs them |

Do not install both the managed Claude plugin and copied `mattpocock/skills` for
the same agent: that exposes duplicate skills. For Codex or another file-based
agent, use the project-copy installation route. For Claude Code, use the managed
plugin route.

## Activation policy

- `setup-matt-pocock-skills` is human-invoked and run once per repository setup
  or tracker/layout change.
- Workflow commands such as `/grill-with-docs`, `/to-spec`, `/to-tickets`, and
  `/implement` are invoked at the phase shown in `workflow.md`.
- `use-modern-go` applies to every Go implementation ticket.
- `golang-how-to` routes to the smallest relevant set of Go skills; do not force
  the entire catalog into every context window.
- Architecture skills are consulted for design decisions or reviews. Their
  generic advice never overrides an accepted project ADR.

## Reproducibility

- Record skill source and version changes in the same pull request that changes
  their generated repository configuration.
- Prefer tagged releases or managed-plugin versions over unpinned source heads.
- Review generated changes. A setup skill may edit an adapter, but it does not
  have authority to rewrite product or architecture decisions.
- Keep custom project procedures under `.agents/workflows/`; do not fork an
  upstream skill merely to add one project-specific sentence.

## Deliberate exclusions

- Do not install an entire architecture catalog by default. Start with
  `architecture-patterns`; add `microservices-patterns` only for a distributed
  system and language-specific concurrency guidance only when that language is
  selected.
- Do not stack `obra/superpowers` on top of the complete Matt Pocock delivery
  chain by default. Both prescribe design, TDD, debugging, and verification
  behavior. Choose one workflow owner, then add only genuinely missing skills.
- Do not force every Go skill to load on every turn. The router plus
  `use-modern-go` gives stronger task fit with a smaller context cost.
