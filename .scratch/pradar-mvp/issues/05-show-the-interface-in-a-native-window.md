# 05: Show the interface in a native window

**What to build:** Replace the browser with the native macOS window decided by
ADR-0007: one window over the owned local server, owning the application
lifecycle, with no capability handed to the page.

**Blocked by:** 04: Build the production reading interface.

**Status:** ready-for-agent

- [ ] Launching the application opens one native window showing the timeline,
  with a title, a restorable size and the system appearance.
- [ ] The window navigates only to the owned loopback origin; an analysis link
  to Forgejo opens in the user's browser instead.
- [ ] No Go function is exposed to the page, and developer tools are disabled
  in a release build.
- [ ] Closing the window stops collection and analysis, cancels the running
  invocation, removes temporary workspaces and exits within a bounded grace
  period, as the PRD requires.
- [ ] Keyboard traversal, visible focus and the system theme keep working in
  the window, proven by a check made from inside the window rather than
  assumed.
- [ ] The application emits no system notification and runs no daemon.
- [ ] `make verify` passes.
