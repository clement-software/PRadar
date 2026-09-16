# Quality gates

`make verify` is the local contract for Go changes. CI expresses the same checks
with a separate lint action so tool installation and caching remain explicit.

| Gate | Local command | Enforcement |
| --- | --- | --- |
| Harness integrity | `make verify-harness` | Required even before `go.mod` exists |
| Formatting | `make fmt-check` | `gofmt` reports no changed files |
| Module hygiene | `make tidy-check` | `go mod tidy -diff` is empty |
| Static analysis | `make vet` | `go vet ./...` passes |
| Tests and races | `make test` | `go test -race -shuffle=on ./...` passes |
| Lint and modernization | `make lint` | `.golangci.yml`, including `modernize`, passes |

## Policy

- Run the narrowest relevant test during red/green iterations and the full gate
  before claiming a ticket complete.
- A linter suppression names the linter and explains why the warning is not
  actionable. Do not use blanket `//nolint`.
- Do not lower a gate to land unrelated work. Change the policy in a dedicated,
  reviewed change with evidence.
- CI pins `golangci-lint` to `v2.13.2`; update the pin and configuration together.
- The template does not impose a coverage percentage. Set a threshold only when
  it represents useful behavior coverage rather than a vanity metric.

## Local prerequisites after Go initialization

- A Go toolchain compatible with the `go` directive in `go.mod`.
- `golangci-lint v2.13.2`.
- Race-detector prerequisites for the target platform.
