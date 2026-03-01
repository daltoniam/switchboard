.PHONY: build generate test test-race vet lint security gosec govulncheck ci clean help

BIN := switchboard

## Build

build: ## Build the binary
	go build -o $(BIN) ./cmd/server

generate: ## Generate templ templates
	go generate .

clean: ## Remove build artifacts
	rm -f $(BIN) coverage.out

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
	go tool gosec -exclude=G101,G104,G115,G117,G119,G120,G304,G505,G704 ./...

govulncheck: ## Run vulnerability checker
	go tool govulncheck ./...

security: gosec govulncheck ## Run all security checks

## CI

ci: build vet test-race lint security ## Run all CI checks locally

## Help

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-14s\033[0m %s\n", $$1, $$2}'

.DEFAULT_GOAL := help
