# 06: Package the application for distribution

**What to build:** Turn the binary into an application a user can install and
launch on macOS, reproducibly, from a clean checkout.

**Blocked by:** 05: Show the interface in a native window.

**Status:** done

- [x] One documented command produces the application bundle from a clean
  checkout, embedding every asset the interface needs.
- [x] The bundle carries its identifier, version, icon and the minimum macOS
  version, and the version is visible from the application.
- [x] The application is signed and notarised, and the procedure records which
  secrets the operator supplies without storing them in the repository.
- [x] A first launch on a machine that never built the project starts, creates
  its data directory and shows an empty timeline with an actionable next step.
- [x] Continuous integration builds the bundle and fails when packaging breaks,
  without requiring signing secrets for a pull request.
- [x] The release procedure is documented, including what to check before
  distributing.
- [x] `make verify` passes.
