.PHONY: all build test test-unit test-lsp test-e2e test-security test-durability test-scale test-contrib test-all fmt fmt-check line-check vet verify clean help

SHELL := /bin/bash
BIN_DIR := bin

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

build:
	@mkdir -p $(BIN_DIR)
	@echo "==> Building Foundation CLI..."
	go build -o $(BIN_DIR)/foundation ./cmd/foundation
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
	rm -rf $(BIN_DIR)
