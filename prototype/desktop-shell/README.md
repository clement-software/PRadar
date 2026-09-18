# PRadar desktop shell prototype

**Question:** can one Go binary show the PRadar interface in a native macOS
window, with Markdown, Mermaid, keyboard focus and the system theme working,
without an npm toolchain or a second language?

This branch is disposable evidence, not the production layout. It deliberately
contains a single file and its own module so the experiment does not add a
dependency to the main module by accident.

## What the prototype does

It opens a WKWebView window on a running PRadar visualizer, waits for the page
to settle, then asks the page itself what rendered and prints the answer on
standard output. The measurement is made by the page in the real window, not
guessed from the outside.

## Run

```sh
pradar run --controlled --listen 127.0.0.1:47860 &
go run . --url 'http://127.0.0.1:47860/pr/controlled/demo%231' --close-after 15s
```

## Verdict

**Validated on 18 September 2026.** Measured on macOS 26.5 with the SDK and
WebKit already present, against the demonstrator's own pages:

| Page | Result |
| --- | --- |
| Detail | `mermaid=1 focusable=11 title="Add retry with jittered backoff to the Forgejo client · PRadar"` |
| Timeline | `cards=1 focusable=23 theme=light background=rgb(246,248,251)` |

- Mermaid rendered inside the native window, so the strict-mode diagrams of an
  analysis need no browser and no bundler.
- The page reported the system theme and the light palette token, so
  `prefers-color-scheme` works through WKWebView.
- Focusable elements are the real links, buttons, inputs and selects, so
  keyboard traversal and visible focus keep working.
- The shell binary is 3.4 MB and builds with `go build`; it needs cgo and the
  macOS frameworks, both already required for a macOS desktop application.

Not answered here: window chrome and menus, application packaging, signing and
notarisation, deep links back into Forgejo, and how the production UI should be
structured. Those belong to the MVP tickets.
