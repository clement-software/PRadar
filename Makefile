GO_PACKAGES ?= ./...
GO_FILES = $(shell find . -type f -name '*.go' -not -path './vendor/*')

.PHONY: doctor verify-harness require-go fmt fmt-check tidy-check vet test lint modernize verify

doctor: verify-harness
	@if [ -f go.mod ]; then \
		go version; \
		if command -v golangci-lint >/dev/null 2>&1; then golangci-lint version; \
		else echo "golangci-lint is not installed (required before make verify)"; fi; \
	else \
		echo "Harness ready. Go checks activate after go.mod is created."; \
	fi

verify-harness:
	@test -f AGENTS.md
	@test -f docs/agents/workflow.md
	@test -f docs/agents/setup-profile.md
	@test -f docs/architecture/README.md
	@test -f docs/quality/quality-gates.md
	@test -f .github/workflows/ci.yml
	@echo "Harness structure: OK"

require-go:
	@test -f go.mod || { echo "go.mod is missing; initialize the Go module after project discovery."; exit 1; }
	@test -n "$(GO_FILES)" || { echo "No Go source files found."; exit 1; }

fmt: require-go
	gofmt -w $(GO_FILES)

fmt-check: require-go
	@unformatted="$$(gofmt -l $(GO_FILES))"; \
	if [ -n "$$unformatted" ]; then \
		echo "Files requiring gofmt:"; echo "$$unformatted"; exit 1; \
	fi

tidy-check: require-go
	go mod tidy -diff

vet: require-go
	go vet $(GO_PACKAGES)

test: require-go
	go test -race -shuffle=on -coverprofile=coverage.out $(GO_PACKAGES)

lint: require-go
	golangci-lint run $(GO_PACKAGES)

modernize: require-go
	golangci-lint run --enable-only modernize $(GO_PACKAGES)

verify: fmt-check tidy-check vet test lint
