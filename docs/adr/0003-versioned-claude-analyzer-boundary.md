# ADR-0003: Isolate Claude behind a versioned analyzer boundary

- **Status:** Accepted; live `show-me` quality remains a PRD gate
- **Date:** 2026-09-16
- **Owners:** PRadar maintainers
- **Related:** `docs/product/PRD.md`, `pradar.analysis.v1`, prototype branch `codex/prototype/pradar-integration-spike` at `2007e82`

## Context

The MVP requires Claude CLI and `show-me`, while the engine, model, prompt, and
skill can evolve independently from PR lifecycle policy. CLI output, timeouts,
cancellation, and invalid JSON are failure-prone process-boundary concerns.
Replays must retain previous results and explain exactly which inputs produced
each analysis.

## Decision

The analysis application package consumes an `Analyzer` interface. The Claude
adapter invokes the CLI non-interactively, requests structured output using a
JSON Schema, bounds duration and output, disables session persistence, and
validates `pradar.analysis.v1` before returning.

The analysis identity contains PR identity, the input revision defined by the
PRD, prompt version, skill version, engine, and model. Prompt and skill assets
are pinned and versioned; the adapter never relies on an unversioned personal
installation as durable behavior. A changed identity creates another
append-only result.

The selected [`show-me`](https://github.com/humanlayer/skills/tree/main/plugins/show-me/skills/show-me)
skill is owned by HumanLayer, not by the Matt Pocock repository. Its exact
source/version and distribution method must be recorded when the demonstrator
installs it.

## Consequences

- Business policy does not parse Claude's outer JSON envelope or spawn
  processes.
- Invalid or mismatched output is an analysis failure and follows the same
  three-attempt policy.
- Model use may be duplicated after a crash between subprocess completion and
  database commit; the system favors recoverability over exactly-once claims.
- The twenty-PR corpus must still validate usefulness and factual accuracy.

## Alternatives considered

- Calling a model API directly was rejected for the MVP because the PRD chooses
  Claude CLI.
- Persisting free-form Markdown alone was rejected because cards, filtering,
  provenance, replay, and migration need a versioned contract.
- Letting each caller invoke Claude was rejected because it would duplicate
  security, limits, decoding, and retry behavior.

## Verification

- Contract tests reject unknown schema versions and mismatched PR/SHA values.
- Adapter tests assert fixed CLI flags, cancellation, output bounds, and no
  session persistence.
- The PRD corpus reaches at least 16 useful analyses out of 20 in under one
  minute each with no critical factual error before implementation proceeds.

Primary technical evidence:

- [Claude Code non-interactive mode](https://code.claude.com/docs/en/headless)
- [Claude Code CLI reference](https://code.claude.com/docs/en/cli-reference)
- [Claude Code skills](https://code.claude.com/docs/en/skills)
