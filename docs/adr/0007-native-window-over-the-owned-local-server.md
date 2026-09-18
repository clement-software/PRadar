# ADR-0007: Show the MVP interface in a native window over an owned local server

- **Status:** Proposed
- **Date:** 2026-09-18
- **Owners:** PRadar maintainers
- **Related:** `docs/product/PRD.md` (open question 4), [ADR-0001](0001-modular-monolith-and-manual-wiring.md), [ADR-0005](0005-isolate-untrusted-pull-request-content.md), prototype branch `codex/prototype/pradar-desktop-shell` at `b6fc87f`

## Context

The PRD requires a macOS desktop application distributed as a single Go binary
whose interface renders Markdown, Mermaid, targeted diffs and pull-request
metadata, follows the system theme, keeps WCAG AA contrast, shows visible
focus and allows keyboard traversal. It deliberately left the toolkit open.

Analysis bodies are Markdown with Mermaid produced by a model, so the
interface must render an evolving document format, not a fixed widget tree.
The demonstrator already renders exactly that surface with sanitised HTML,
strict Mermaid and pinned embedded assets, and its renderer security tests
are the executable evidence for the untrusted-content invariant.

A prototype answered the remaining question on 18 September 2026. A WKWebView
window opened on the demonstrator's own pages reported, from inside the
window, a rendered Mermaid diagram, the system theme, the light palette token
and the real focusable elements, from a 3.4 MB binary built with `go build`.

## Decision

Keep rendering the interface in Go as sanitised HTML served by an
application-owned loopback server, and show it in a native macOS window backed
by WKWebView. The window is a shell: it owns the frame, the title and the
lifecycle, and navigates to the owned server. No Node, npm or JavaScript
bundler enters the build.

The binding surface between the shell and the page stays minimal: create the
window, size it, navigate, and expose no Go function to the page beyond what a
named ticket justifies. Developer tools stay disabled in release builds. The
server keeps binding to loopback on an ephemeral port and keeps the
content-security policy of the demonstrator.

The production interface is rewritten against the production use cases in its
own ticket. The demonstrator's templates are evidence that the surface is
renderable, not a layout to copy.

## Consequences

- One rendering path serves both the MVP window and any later evaluation
  harness, so the renderer security tests keep their value.
- The build needs cgo and the macOS frameworks, which a macOS desktop
  application requires anyway; cross-compiling from another platform is out.
- Accessibility comes from semantic HTML, which the toolkit alternatives would
  have had to reimplement.
- A web view can render arbitrary content, so sanitisation, the strict Mermaid
  mode and the content-security policy remain load-bearing, not conveniences.
- `webview_go` is a thin binding whose last release is 2024. The three calls
  used are replaceable by direct AppKit bindings without touching the
  interface, and that risk is recorded rather than hidden.
- Packaging, signing, notarisation and the application bundle are separate
  work and do not change this decision.

## Alternatives considered

- Fyne and Gio were rejected because they draw widgets: Markdown, Mermaid and
  diff rendering would have to be reimplemented, and the accessibility and
  focus behaviour the PRD requires would be rebuilt rather than inherited.
- Wails v3 was rejected for the MVP because it is a beta release and brings a
  frontend pipeline PRadar does not need: the interface is already server
  rendered in Go.
- Direct AppKit bindings were rejected as the starting point because they need
  more code to reach the same WKWebView, and remain the fallback.
- Keeping the browser-based visualizer was rejected because the PRD asks for a
  desktop application, not a tab whose lifecycle the user manages.

## Verification

- The prototype reports, from inside the native window, the rendered Mermaid
  count, the focusable elements, the system theme and the computed background.
- Renderer security tests keep proving that hostile Markdown, links and
  Mermaid directives stay inert.
- A production ticket asserts that the shell exposes no binding to the page
  and that developer tools are disabled in a release build.
