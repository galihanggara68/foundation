.PHONY: all build test test-unit test-lsp test-e2e test-security test-durability test-scale test-contrib test-all fmt fmt-check line-check vet verify clean help version changelog-draft release-check tag release snapshot

SHELL := /bin/bash
BIN_DIR := bin

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null | sed 's/^v//')
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null)
DATE    ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDVARS  := -X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.date=$(DATE)
RELDIR  := dist

all: build

## help: Display available make targets
help:
	@echo "Foundation Compiler Development Makefile"
	@echo "Usage: make [target]"
	@echo ""
	@echo "Build Targets:"
	@echo "  build              Build Foundation CLI and reference generator plugins into bin/"
	@echo "  clean              Remove compiled binaries and build artifacts"
	@echo ""
	@echo "Test Targets:"
	@echo "  test               Run all unit and integration tests"
	@echo "  test-unit          Run fast unit tests (cmd/ and pkg/)"
	@echo "  test-lsp           Run Foundation LSP language server unit & E2E suite"
	@echo "  test-e2e           Run end-to-end integration tests (test/e2e/)"
	@echo "  test-security      Run sandboxing & adversarial security suite (test/security/)"
	@echo "  test-durability    Run live HTTP durability & crash recovery suite (test/durability/)"
	@echo "  test-scale         Run heavy compiler scaling & stress suite (test/scale/)"
	@echo "  test-contrib       Run contrib Go and TypeScript runtime test suites"
	@echo "  test-all           Run all test suites including race detector and contrib runtimes"
	@echo ""
	@echo "Quality & Verification:"
	@echo "  fmt                Format all Go source files with gofmt"
	@echo "  fmt-check          Verify all Go files are formatted"
	@echo "  line-check         Verify file size discipline (<= 450 lines per file)"
	@echo "  vet                Run go vet across all packages"
	@echo "  verify             Run full pre-commit gate (fmt-check, line-check, vet, test-unit)"
	@echo ""
	@echo "Release & Versioning:"
	@echo "  version            Print the version the current tree reports"
	@echo "  changelog-draft    Print commit subjects since the last tag, for the CHANGELOG"
	@echo "  release-check      Gate a release (clean tree, fresh semver tag, CHANGELOG entry, verify)"
	@echo "  tag                Validate, then create and push the annotated release tag (CI publishes)"
	@echo "  release            Alias for tag"
	@echo "  snapshot           Cross-compile release archives into dist/ without tagging"

build:
	@mkdir -p $(BIN_DIR)
	@echo "==> Building Foundation CLI..."
	go build -trimpath -ldflags="$(LDVARS)" -o $(BIN_DIR)/foundation ./cmd/foundation
	@echo "==> Building reference generator plugins..."
	go build -o $(BIN_DIR)/foundation-gen-rich-ddd ./cmd/foundation-gen-rich-ddd
	go build -o $(BIN_DIR)/foundation-gen-otel ./cmd/foundation-gen-otel
	go build -o $(BIN_DIR)/foundation-gen-python-fastapi ./cmd/foundation-gen-python-fastapi
	@echo "==> Binaries built in $(BIN_DIR)/"

test:
	go test ./...

test-unit:
	go test -v ./cmd/... ./pkg/...

test-lsp:
	go test -v ./pkg/compiler/internal/lsp/...
	go test -v ./test/e2e -run TestE2E_FoundationLSP

test-e2e:
	go test -v ./test/e2e/...

test-security:
	go test -v ./test/security/...

test-durability:
	go test -v ./test/durability/...

test-scale:
	go test -v ./test/scale/...

test-contrib:
	@echo "==> Running contrib/go/runtime tests..."
	(cd contrib/go/runtime && go test -v -race ./...)
	@echo "==> Running contrib/ts/runtime tests..."
	(cd contrib/ts/runtime && npm test)

test-all: test-unit test-e2e test-security test-durability test-scale test-contrib

fmt:
	gofmt -w cmd/ pkg/ contrib/ test/

fmt-check:
	@unformatted=$$(gofmt -l cmd/ pkg/ contrib/ test/); \
	if [ -n "$$unformatted" ]; then \
		echo "The following files are not formatted with gofmt:"; \
		echo "$$unformatted"; \
		exit 1; \
	fi
	@echo "gofmt: all files clean"

line-check:
	@oversized=$$(find cmd/ pkg/ test/ -name "*.go" -exec wc -l {} + | awk '$$1 > 450 && $$2 != "total" {print $$1, $$2}'); \
	if [ -n "$$oversized" ]; then \
		echo "The following files exceed 450 lines (CODING_STANDARDS.md):"; \
		echo "$$oversized"; \
		exit 1; \
	fi
	@echo "line-check: all files <= 450 lines"

vet:
	go vet ./...
	(cd contrib/go/runtime && go vet ./...)

verify: fmt-check line-check vet test-unit
	@echo "==> Pre-commit verification passed!"

clean:
	rm -rf $(BIN_DIR) $(RELDIR)

version:
	@echo $(VERSION)

changelog-draft:
	@last=$$(git describe --tags --abbrev=0 2>/dev/null); \
	git log $${last:+$$last..HEAD} --no-merges --pretty='- %s'

release-check:
	@echo "$(VERSION)" | grep -Eq '^[0-9]+\.[0-9]+\.[0-9]+$$' \
		|| { echo "VERSION must be x.y.z (got '$(VERSION)'); pass VERSION=<x.y.z> explicitly"; exit 1; }
	@git diff --quiet && git diff --cached --quiet \
		|| { echo "working tree is dirty; commit before tagging"; exit 1; }
	@! git rev-parse -q --verify refs/tags/v$(VERSION) >/dev/null \
		|| { echo "tag v$(VERSION) already exists"; exit 1; }
	@grep -q "^## \[$(VERSION)\]" CHANGELOG.md \
		|| { echo "CHANGELOG.md has no [$(VERSION)] section"; exit 1; }
	@$(MAKE) --no-print-directory verify

tag: release-check
	git tag -a v$(VERSION) -m "Release v$(VERSION)"
	git push origin v$(VERSION)

release: tag

snapshot:
	@mkdir -p $(RELDIR)
	@for target in linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64; do \
		GOOS=$${target%/*}; GOARCH=$${target#*/}; ext=""; \
		if [ "$$GOOS" = "windows" ]; then ext=".exe"; fi; \
		name="foundation-$${GOOS}-$${GOARCH}$${ext}"; \
		echo "==> $$name"; \
		GOOS=$$GOOS GOARCH=$$GOARCH CGO_ENABLED=0 \
			go build -trimpath -ldflags="-s -w $(LDVARS)" -o $(RELDIR)/$$name ./cmd/foundation || exit 1; \
	done
	@echo "==> Snapshots written to $(RELDIR)/"
