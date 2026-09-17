# ADR-0005: Isolate untrusted pull-request content

- **Status:** Accepted; its skill-delivery mechanism is superseded by the proposed [ADR-0006](0006-deliver-the-pinned-skill-as-a-system-prompt.md)
- **Date:** 2026-09-16
- **Owners:** PRadar maintainers
- **Related:** `docs/product/PRD.md`, `docs/architecture/failure-model.md`

## Context

PR titles, descriptions, diffs, repository files, Markdown, and Mermaid may be
controlled by an attacker. Claude CLI requires its own authentication, Forgejo
uses a read-only token, and rendered analysis can contain active constructs.
Passing untrusted content to an agent with broad tools or to an unsanitized
renderer would turn a reading aid into a credential or code-execution boundary.

## Decision

Materialize each analysis under an application-owned temporary root. Give the
Claude invocation only the pinned `show-me` skill and read-only file discovery
tools (`Read`, `Glob`, and `Grep`) in `dontAsk` mode. Expose no write, shell,
browser, network, MCP, or subagent tool. Disable session persistence. Construct
all command arguments in Go without a shell.

The Forgejo token stays in macOS Keychain and is never passed to Claude. Fetch
required content before analysis. Bound input, output, workspace size, and
duration. Validate structured output, sanitize Markdown, and render Mermaid in
strict mode. Remove the workspace after every outcome and scavenge orphans at
startup.

## Consequences

- Some `show-me` behaviors that depend on shell or network access are
  intentionally unavailable; the demonstrator must prove the remaining output
  is useful.
- Read-only agent tools reduce blast radius but do not make model output trusted;
  schema validation and renderer sanitization remain required.
- Temporary workspace ownership and cleanup become explicit runtime concerns.

## Alternatives considered

- Running Claude in the main repository was rejected because it exposes user
  files and writable Git state.
- Relying only on prompt instructions was rejected because untrusted content can
  contain prompt injection.
- Passing the Forgejo token through the analyzer environment was rejected
  because analysis does not need it.
- Accepting arbitrary HTML was rejected by the PRD and would enlarge the
  rendering attack surface.

## Verification

- Process-boundary tests inspect the exact available tools and arguments.
- Security tests use malicious PR text that asks for tools, secrets, writes, and
  unsafe Markdown; no such effect occurs and rendering stays inert.
- Workspace cleanup tests cover success, failure, cancellation, and restart.

Primary technical evidence:

- [Claude Code CLI tool restriction](https://code.claude.com/docs/en/cli-reference)
- [Claude Code permission behavior](https://code.claude.com/docs/en/permissions)
- [Claude Code session persistence](https://code.claude.com/docs/en/sessions)
