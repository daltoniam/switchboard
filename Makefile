.PHONY: build generate proto proto-check test test-race vet lint fmt security gosec govulncheck ci clean install deploy help wasm-build wasm-test rust-awm compose-check compose-up compose-status compose-endpoints compose-logs compose-down compose-destroy

BIN        := dist/switchboard
INSTALL_DIR := $(HOME)/.local/bin
INSTALL_BIN := $(INSTALL_DIR)/switchboard
VERSION    := $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT     := $(shell git rev-parse --short HEAD 2>/dev/null || echo none)
DATE       := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS    := -X github.com/daltoniam/switchboard/version.Version=$(VERSION) \
              -X github.com/daltoniam/switchboard/version.Commit=$(COMMIT) \
              -X github.com/daltoniam/switchboard/version.Date=$(DATE)

## Build

build: ## Build the binary
	@mkdir -p dist
	go build -ldflags '$(LDFLAGS)' -o $(BIN) ./cmd/server

generate: proto ## Generate protobuf bindings and templ templates
	go generate .

proto: ## Generate AWM gRPC protobuf bindings
	buf generate

proto-check: ## Lint the AWM protobuf API
	buf lint

clean: ## Remove build artifacts
	rm -rf dist/ coverage.out

## WASM

wasm-build: ## Build WASM modules (requires Rust with wasm32-wasip1 target)
	cd wasm/guest-rust && cargo build --target wasm32-wasip1 --release -p example-wasm
	cp wasm/guest-rust/target/wasm32-wasip1/release/example_wasm.wasm wasm/testdata/example.wasm

wasm-test: wasm-build ## Build WASM modules and run WASM tests
	go test -v ./wasm/

rust-awm: ## Test and package-verify the consumable switchboard-awm Rust client crate
	cargo test --manifest-path rust/switchboard-awm/Cargo.toml
	rust/switchboard-awm/scripts/package-verify.sh

## Test

test: ## Run tests
	go test ./...
	bash scripts/compose-dev_test.sh

test-race: ## Run tests with race detector
	go test -race -coverprofile=coverage.out ./...
	bash scripts/compose-dev_test.sh

## Analysis

vet: ## Run go vet
	go vet ./...

lint: ## Run golangci-lint
	go tool golangci-lint run

fmt: ## Format Go source files
	gofmt -w .

gosec: ## Run security scanner
	go tool gosec -exclude-dir=gen -exclude=G101,G104,G115,G117,G119,G120,G304,G505,G704,G706 ./...

govulncheck: ## Run vulnerability checker
	go tool govulncheck ./...

security: gosec govulncheck ## Run all security checks

## CI

ci: proto-check build vet test-race lint security ## Run all CI checks locally

## Stacklane compose (dev)

compose-check: ## Fail-closed Stacklane compose contract check
	bash scripts/compose-dev.sh check

compose-up: ## check + build + start DEV compose stack
	bash scripts/compose-dev.sh up

compose-status: ## Compose ps + FQDN / loopback endpoints
	bash scripts/compose-dev.sh status

compose-endpoints: ## Print Stacklane FQDNs + direct loopback URLs
	bash scripts/compose-dev.sh endpoints

compose-logs: ## Follow compose logs (Ctrl-C leaves the stack running)
	bash scripts/compose-dev.sh logs

compose-down: ## Stop compose stack (volumes preserved)
	bash scripts/compose-dev.sh down

compose-destroy: ## Remove compose stack AND volumes (CONFIRM=switchboard-<instance>-destroy)
	bash scripts/compose-dev.sh destroy

## Install & Deploy

install: build ## Build, install to ~/.local/bin, and set up systemd user service
	@mkdir -p $(INSTALL_DIR)
	install -m 0755 $(BIN) $(INSTALL_BIN)
	$(INSTALL_BIN) daemon install --verbose
	$(INSTALL_BIN) daemon start
	@sleep 1
	@systemctl --user is-active switchboard.service >/dev/null 2>&1 && \
		echo "Installed and started. Logs: journalctl --user -u switchboard -f" || \
		echo "Service installed but failed to start. Check: systemctl --user status switchboard"

deploy: build ## Build, install to ~/.local/bin, and restart the daemon (requires make install first)
	@if ! systemctl --user is-enabled switchboard.service >/dev/null 2>&1; then \
		echo "Error: systemd service not installed. Run 'make install' first."; \
		exit 1; \
	fi
	systemctl --user stop switchboard
	@sleep 1
	@if systemctl --user is-active switchboard.service >/dev/null 2>&1; then \
		echo "Error: switchboard did not stop. Check: systemctl --user status switchboard"; \
		exit 1; \
	fi
	install -m 0755 $(BIN) $(INSTALL_BIN)
	$(INSTALL_BIN) daemon install --verbose
	systemctl --user start switchboard
	@sleep 1
	@if systemctl --user is-active switchboard.service >/dev/null 2>&1; then \
		echo "Deployed and restarted."; \
	else \
		echo "Error: switchboard failed to start. Check: journalctl --user -u switchboard -n 20"; \
		exit 1; \
	fi

## Help

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-14s\033[0m %s\n", $$1, $$2}'

.DEFAULT_GOAL := help
