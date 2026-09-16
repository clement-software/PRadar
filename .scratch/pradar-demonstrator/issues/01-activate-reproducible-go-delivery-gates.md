# 01: Activate the reproducible Go delivery gates

**What to build:** Turn the existing local engineering harness into a versioned,
reproducible Go delivery gate so every subsequent demonstrator slice can prove
its formatting, dependency hygiene, static analysis, race safety, linting, and
continuous-integration status from a clean checkout.

**Blocked by:** None (can start immediately).

**Status:** ready-for-agent

- [ ] The repository initialises a Go module using the canonical repository
  identity and a supported Go toolchain version.
- [ ] The project constitution, workflow, quality policy, build entry points,
  and other files required by the harness are versioned rather than depending
  on ignored local state.
- [ ] The CI workflow runs the same formatting, module hygiene, static analysis,
  race-enabled test, lint, and modernisation checks required locally.
- [ ] The lint configuration pins the agreed linter release and enables the
  modernisation checks described by the quality policy.
- [ ] `make doctor` succeeds from a clean checkout with the documented tool
  prerequisites installed.
- [ ] `make verify` succeeds with a minimal Go package and reports a failure if
  any required gate is deliberately broken.
- [ ] Generated test artefacts do not leave the worktree dirty after a
  successful verification run.
- [ ] No product architecture, framework, database driver, or UI toolkit is
  selected merely to complete this prefactor.

