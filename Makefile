.PHONY: build generate test test-race vet lint security gosec govulncheck ci clean install deploy help

BIN        := dist/switchboard
INSTALL_DIR := $(HOME)/.local/bin
INSTALL_BIN := $(INSTALL_DIR)/switchboard

## Build

build: ## Build the binary
	@mkdir -p dist
	go build -o $(BIN) ./cmd/server

generate: ## Generate templ templates
	go generate .

clean: ## Remove build artifacts
	rm -rf dist/ coverage.out

## Test

test: ## Run tests
	go test ./...

test-race: ## Run tests with race detector
	go test -race -coverprofile=coverage.out ./...

## Analysis

vet: ## Run go vet
	go vet ./...

lint: ## Run golangci-lint
	go tool golangci-lint run

gosec: ## Run security scanner
	go tool gosec -exclude=G101,G104,G115,G117,G119,G120,G304,G505,G704,G706 ./...

govulncheck: ## Run vulnerability checker
	go tool govulncheck ./...

security: gosec govulncheck ## Run all security checks

## CI

ci: build vet test-race lint security ## Run all CI checks locally

## Install & Deploy

install: build ## Build, install to ~/.local/bin, and set up systemd user service
	@mkdir -p $(INSTALL_DIR)
	cp $(BIN) $(INSTALL_BIN)
	$(INSTALL_BIN) daemon install
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
	cp $(BIN) $(INSTALL_BIN)
	systemctl --user start switchboard
	@echo "Deployed and restarted."

## Help

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-14s\033[0m %s\n", $$1, $$2}'

.DEFAULT_GOAL := help
