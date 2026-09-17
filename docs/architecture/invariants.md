# Project-specific architecture invariants

**Status:** Accepted for the MVP on 16 September 2026

The test names are required production evidence. Their exact package may change
with implementation, but each behavior must remain executable.

| Rule | Failure prevented | Owner | Enforcement | Evidence |
| --- | --- | --- | --- | --- |
| Only an analysis whose identity still matches the latest observed identity may update the visible card. | An old subprocess overwrites a newer PR version. | `internal/pullrequest` policy; SQLite transaction applies it. | Conditional update in the completion transaction. | `TestCompleteAnalysis_DoesNotPublishStaleIdentity`; prototype `2007e82`. |
| A result is appended to history before, or atomically with, changing the visible projection. | A visible card points to a missing analysis after a crash. | Analysis store transaction. | One SQLite transaction for result and projection. | `TestCompleteAnalysis_CommitsResultAndProjectionAtomically`. |
| One analysis identity creates at most one durable work item and one durable result. | Polling and retries multiply model spend and history rows. | Collection/analysis store. | Unique constraints plus idempotent upsert. | `TestObserve_DuplicateIdentityIsIdempotent`. |
| At most one worker owns a work item, and completion requires its live lease token. | Two Claude processes publish the same work or a late worker commits after recovery. | Analysis store. | Atomic leased claim and compare-on-complete. | `TestClaimAnalysis_GrantsSingleLease`; prototype `2007e82`. |
| Automatic analysis stops after three failed attempts and exposes `unavailable`. | Infinite retries and hidden PRs. | `internal/analyze`. | Durable attempt count and terminal transition. | `TestAnalysis_ThirdFailurePublishesUnavailable`; prototype `2007e82`. |
| A new observed version is unread immediately; an archived PR reappears only when that version has a publishable result. | Empty/stale cards reappear during debounce or analysis. | `internal/pullrequest`. | State transition and conditional publication. | `TestArchivedPR_ReappearsAfterLatestAnalysis`; prototype `471a8d2`. |
| Unsubscribing makes queued/debounced work ineligible and prevents an in-flight result from mutating the active projection. | Work continues after the user stops collection. | Subscription generation and analysis completion policy. | Transactional subscription version check plus root cancellation. | `TestUnsubscribe_InvalidatesOutstandingWork`. |
| PR content, diffs, Markdown, and Mermaid are untrusted data and never change the analyzer's tool or permission policy. | Prompt injection, file mutation, secret access, or unsafe rendering. | Claude adapter and UI renderer. | Fixed CLI arguments, isolated workspace, schema validation, sanitization, strict Mermaid mode. | `TestClaudeAnalyzer_UsesFixedReadOnlyToolsAndThePinnedSkill`; renderer security tests. |
| The engine's granted surface is verified at runtime and stays within the pinned guidance plus read-only discovery tools. | An engine, plugin, or operator installation silently widens the analysis surface. | Claude adapter. | Startup-event verification of tools, skills, slash commands, plugins, MCP servers, and permission mode. | `TestClaudeAnalyzer_RejectsAWiderGrantedSurface`. |
| Forgejo tokens exist only in macOS Keychain and are never persisted in SQLite, logs, prompts, or analysis provenance. | Credential disclosure. | Keychain and Forgejo adapters. | Types exclude tokens from durable/request models; log redaction test. | `TestPersistenceSchema_HasNoCredentialColumn`; `TestLogs_RedactForgejoToken`. |
| Temporary code, patches, and workspaces are deleted after use and scavenged at startup; durable storage keeps only metadata, results, state, work, and provenance. | Source leakage and unbounded local storage. | Workspace adapter. | Owned temp root and startup scavenger. | `TestWorkspace_CleansAfterSuccessFailureAndRestart`. |
| Business policy imports no desktop, HTTP, SQL, Keychain, process, or vendor package. | Framework and vendor coupling leaks inward. | Package dependency graph. | Import-boundary architecture test. | `TestArchitecture_DependenciesPointInward`. |

No daily analysis ceiling is an MVP product choice, not permission for an
unbounded in-memory queue. Durable work may grow with observed PR activity;
goroutines, subprocesses, response bodies, diff inputs, and temporary files
remain bounded.
