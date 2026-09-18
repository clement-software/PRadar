GO_PACKAGES ?= ./...
APP_NAME ?= PRadar
BUNDLE_ID ?= com.clement-software.pradar
MIN_MACOS ?= 13.0
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
BUILD_DIR ?= build
APP = $(BUILD_DIR)/$(APP_NAME).app
GO_FILES = $(shell find . -type f -name '*.go' -not -path './vendor/*')

.PHONY: doctor verify-harness require-go fmt fmt-check tidy-check vet test lint modernize verify icon app sign notarise release clean-build

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

# Packaging. `make app` is the whole build from a clean checkout; signing and
# notarisation need credentials the operator supplies and the repository never
# stores.
icon: require-go
	@test "$$(uname)" = Darwin || { echo "the application bundle is built on macOS"; exit 1; }
	go run ./tools/mkicon --out $(BUILD_DIR)/$(APP_NAME).iconset
	iconutil -c icns $(BUILD_DIR)/$(APP_NAME).iconset -o $(BUILD_DIR)/$(APP_NAME).icns

app: icon
	mkdir -p $(APP)/Contents/MacOS $(APP)/Contents/Resources
	go build -trimpath -ldflags "-s -w -X main.version=$(VERSION)" -o $(APP)/Contents/MacOS/pradar ./cmd/pradar
	cp $(BUILD_DIR)/$(APP_NAME).icns $(APP)/Contents/Resources/$(APP_NAME).icns
	printf '%s\n' \
	  '<?xml version="1.0" encoding="UTF-8"?>' \
	  '<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">' \
	  '<plist version="1.0"><dict>' \
	  '  <key>CFBundleName</key><string>$(APP_NAME)</string>' \
	  '  <key>CFBundleDisplayName</key><string>$(APP_NAME)</string>' \
	  '  <key>CFBundleIdentifier</key><string>$(BUNDLE_ID)</string>' \
	  '  <key>CFBundleExecutable</key><string>pradar</string>' \
	  '  <key>CFBundleIconFile</key><string>$(APP_NAME)</string>' \
	  '  <key>CFBundlePackageType</key><string>APPL</string>' \
	  '  <key>CFBundleShortVersionString</key><string>$(VERSION)</string>' \
	  '  <key>CFBundleVersion</key><string>$(VERSION)</string>' \
	  '  <key>LSMinimumSystemVersion</key><string>$(MIN_MACOS)</string>' \
	  '  <key>NSHighResolutionCapable</key><true/>' \
	  '</dict></plist>' > $(APP)/Contents/Info.plist
	@$(APP)/Contents/MacOS/pradar version
	@echo "built $(APP)"

# DEVELOPER_ID is the "Developer ID Application: ..." identity in the operator's
# keychain. It is never stored in the repository.
sign: app
	@test -n "$(DEVELOPER_ID)" || { echo "set DEVELOPER_ID to your Developer ID Application identity"; exit 1; }
	codesign --force --options runtime --timestamp --sign "$(DEVELOPER_ID)" $(APP)
	codesign --verify --strict --verbose=2 $(APP)

# NOTARY_PROFILE is a notarytool keychain profile the operator created with
# `xcrun notarytool store-credentials`. No credential is read from the repository.
notarise: sign
	@test -n "$(NOTARY_PROFILE)" || { echo "set NOTARY_PROFILE to your notarytool keychain profile"; exit 1; }
	ditto -c -k --keepParent $(APP) $(BUILD_DIR)/$(APP_NAME).zip
	xcrun notarytool submit $(BUILD_DIR)/$(APP_NAME).zip --keychain-profile "$(NOTARY_PROFILE)" --wait
	xcrun stapler staple $(APP)
	xcrun stapler validate $(APP)

release: notarise
	ditto -c -k --keepParent $(APP) $(BUILD_DIR)/$(APP_NAME)-$(VERSION).zip
	@echo "release archive: $(BUILD_DIR)/$(APP_NAME)-$(VERSION).zip"

clean-build:
	rm -rf $(BUILD_DIR)
