#!/usr/bin/env bash
# `make` is not installed on the box (no root); this runs the `make ci` recipe commands directly.
cd /workspace/sb-remote-mcp-349
L=/workspace/sb-sandbox-proofs/remote-mcp-close-349/logs
step(){ n=$1; shift; s=$(date +%s); "$@" >$L/ci-$n.log 2>&1; rc=$?; echo "$n rc=$rc $(( $(date +%s)-s ))s" | tee -a $L/ci-summary.txt; }
: > $L/ci-summary.txt
step new-tests go test -count=1 -v -run 'TestRemoteMCPAdapters_ImplementClose|TestClose_|TestConfigure_SwitchingToAPIKeyEndsRemoteSession|TestConfigure_ChangingServerEndsOldSession' . ./integrations/linear/ ./integrations/figma/ ./integrations/slackmcp/
step pkg-tests-race go test -count=1 -race . ./remotemcp/ ./integrations/linear/ ./integrations/figma/ ./integrations/slackmcp/
step build go build -ldflags "-X github.com/daltoniam/switchboard/version.Commit=$(git rev-parse --short HEAD)" -o /tmp/sb349 ./cmd/server
step vet go vet ./...
step test-race-all go test -race -coverprofile=/tmp/cov349.out ./...
step compose-dev-test bash scripts/compose-dev_test.sh
step test-web-node20 node --test 'web/tests/unit/*.test.{mjs,ts,mts}'
step lint go tool golangci-lint run
step gosec go tool gosec -exclude-dir=gen -exclude=G101,G104,G115,G117,G119,G120,G304,G505,G704,G706 ./...
step govulncheck go tool govulncheck ./...
echo DONE >> $L/ci-summary.txt
