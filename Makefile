# Makefile for Go module

# Variables
PKG := ./...
PKG_SIGNALS := ./signals
PKG_EXAMPLES := ./examples/...
GOCACHE_DIR := $(CURDIR)/.gocache
FUZZTIME ?= 30s

# Default target
.PHONY: all
all: test

# Run tests with race detector and verbose output
.PHONY: test test-race test-signals test-race-signals cover fuzz clean-cache
test:
	@echo "Running tests (signals, race)..."
	@GOCACHE="$(GOCACHE_DIR)" go test -race $(PKG_SIGNALS)

test-race:
	@echo "Running tests (all, race)..."
	@GOCACHE="$(GOCACHE_DIR)" go test -race $(PKG)

test-signals:
	@echo "Running tests (signals)..."
	@GOCACHE="$(GOCACHE_DIR)" go test $(PKG_SIGNALS)

test-race-signals:
	@echo "Running tests (signals, race)..."
	@GOCACHE="$(GOCACHE_DIR)" go test -race $(PKG_SIGNALS)

cover:
	@echo "Computing coverage for signals..."
	@GOCACHE="$(GOCACHE_DIR)" go test -coverprofile=coverage.out -covermode=atomic -count=1 $(PKG_SIGNALS)
	@go tool cover -func=coverage.out | tail -n1

fuzz:
	@echo "Fuzzing signals for $(FUZZTIME) each (StateMachine, PolicyShutdown, DefaultVsInstance)"
	@GOCACHE="$(GOCACHE_DIR)" go test $(PKG_SIGNALS) -run FuzzStateMachine -fuzz=StateMachine -fuzztime=$(FUZZTIME) -count=1
	@GOCACHE="$(GOCACHE_DIR)" go test $(PKG_SIGNALS) -run FuzzPolicyShutdown -fuzz=PolicyShutdown -fuzztime=$(FUZZTIME) -count=1
	@GOCACHE="$(GOCACHE_DIR)" go test $(PKG_SIGNALS) -run FuzzDefaultVsInstance -fuzz=DefaultVsInstance -fuzztime=$(FUZZTIME) -count=1

.PHONY: test-examples
test-examples:
	@echo "Running example tests..."
	@GOCACHE="$(GOCACHE_DIR)" go test $(PKG_EXAMPLES) -count=1 -v

# Format the code
.PHONY: fmt
fmt:
	@echo "Formatting code..."
	@go fmt $(PKG)

# Lint the code using golangci-lint
.PHONY: lint
lint:
	@echo "Linting code..."
	@golangci-lint run

# Clean build artifacts and coverage reports
.PHONY: clean
clean:
	@echo "Cleaning build artifacts..."
	@go clean
	@rm -f coverage.txt coverage.out

clean-cache:
	@echo "Removing local GOCACHE..."
	@rm -rf "$(GOCACHE_DIR)"

# Display help
.PHONY: help
help:
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@echo "  all                 Run tests (default)"
	@echo "  test                Run race tests for ./signals"
	@echo "  test-race           Run race tests for ./..."
	@echo "  test-signals        Run tests for ./signals"
	@echo "  test-race-signals   Alias to 'test'"
	@echo "  test-examples       Run tests under ./examples/..."
	@echo "  cover               Generate coverage.out for ./signals and print summary"
	@echo "  fuzz                Run fuzzers sequentially; set FUZZTIME=30s"
	@echo "  fmt           Format the code"
	@echo "  lint          Lint the code using golangci-lint"
	@echo "  clean         Remove build artifacts and coverage reports"
	@echo "  clean-cache   Remove local GOCACHE"
	@echo "  help          Display this help message"
