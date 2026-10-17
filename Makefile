.PHONY: build generate node-version npm-version check-node check-npm tsc styles styles-watch proto proto-check test test-race test-web test-browser test-ui ui-install vet lint fmt security gosec govulncheck ci clean install deploy help wasm-build wasm-test rust-awm compose-check compose-up compose-status compose-endpoints compose-logs compose-down compose-destroy

BIN        := dist/switchboard
NODE_VERSION := 26.8.1
NPM_VERSION := 11.19.0
TYPESCRIPT_VERSION := 7.0.2
PLAYWRIGHT_VERSION := 1.64.0
TSC_FLAGS ?= --version
UNOCSS_VERSION := 66.10.5
LUCIDE_VERSION := 1.2.142
SIMPLE_ICONS_VERSION := 1.2.100
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

generate: proto styles ## Generate protobuf bindings, templ templates, and CSS
	go generate .

node-version: ## Print the required Node version for tooling and CI
	@echo $(NODE_VERSION)

npm-version: ## Print the required npm version
	@echo $(NPM_VERSION)

check-node: ## Require the Node version pinned in this Makefile
	@test "$$(node --version 2>/dev/null)" = "v$(NODE_VERSION)" || \
		{ echo "Node $(NODE_VERSION) is required. Install and activate it before running this target." >&2; exit 1; }

check-npm: check-node ## Require the npm version pinned in this Makefile
	@test "$$(npm --version 2>/dev/null)" = "$(NPM_VERSION)" || \
		{ echo "npm $(NPM_VERSION) is required. Run npm install --global npm@$(NPM_VERSION)." >&2; exit 1; }

tsc: check-npm ## Run the pinned TypeScript compiler (override TSC_FLAGS for options)
	npx --yes --ignore-scripts --package=typescript@$(TYPESCRIPT_VERSION) tsc $(TSC_FLAGS)

styles: check-npm ## Generate web CSS with pinned npx tools
	npx --yes --ignore-scripts \
		--package=@unocss/cli@$(UNOCSS_VERSION) \
		--package=@unocss/preset-wind3@$(UNOCSS_VERSION) \
		--package=@unocss/preset-icons@$(UNOCSS_VERSION) \
		--package=@iconify-json/lucide@$(LUCIDE_VERSION) \
		--package=@iconify-json/simple-icons@$(SIMPLE_ICONS_VERSION) \
		unocss 'web/templates/**/*.templ' 'web/templates/layouts/themes.json' 'web/static/*.mjs' \
		--config web/style.js --out-file web/static/style.css $(STYLE_FLAGS)

styles-watch: ## Rebuild CSS when templates or style configuration change
	$(MAKE) styles STYLE_FLAGS=--watch

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

test-web: check-node ## Run browser module unit tests with the pinned Node version
	node --test 'web/tests/unit/*.test.{mjs,ts,mts}'

test-browser: ## Build and run isolated Chromium tests in Docker
	docker build -f web/tests/browser/Dockerfile -t switchboard-browser-tests \
		--build-arg NODE_VERSION=$(NODE_VERSION) --build-arg NPM_VERSION=$(NPM_VERSION) \
		--build-arg PLAYWRIGHT_VERSION=$(PLAYWRIGHT_VERSION) .
	docker run --rm --init --network none --shm-size=256m switchboard-browser-tests

test: test-web ## Run tests
	go test ./...
	bash scripts/compose-dev_test.sh

test-race: test-web ## Run tests with race detector
	go test -race -coverprofile=coverage.out ./...
	bash scripts/compose-dev_test.sh

ui-install: ## Install the pinned Playwright driver and Chromium (UI_INSTALL_ARGS=--with-deps on Linux)
	bash scripts/ui-install.sh $(UI_INSTALL_ARGS)

test-ui: build ## Run mandatory isolated Chromium acceptance tests and save masked screenshots
	SWITCHBOARD_UI_BINARY="$(CURDIR)/$(BIN)" SWITCHBOARD_UI_ARTIFACTS="$(CURDIR)/dist/ui-artifacts" go test -race -count=1 -timeout=5m -v ./web -run '^TestUI'

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

ci: styles proto-check build vet test-race test-ui lint security ## Run all CI checks locally

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
