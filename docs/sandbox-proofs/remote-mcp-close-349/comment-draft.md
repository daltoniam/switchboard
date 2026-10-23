Sandbox E2E @ c118818: 30 PASS / 0 FAIL / 3 SKIPPED. Stand-in streamable-HTTP MCP server (official Go SDK v1.7.0), not the hosted vendors.

- ✅ Linear, Figma and Slack MCP: Close() sends DELETE, the old session gets 404 server-side, a second Close is a no-op, and re-Configure opens a fresh session id that works.
- ✅ Linear mcp_access_token → api_key ends the remote session; api_key REST still wired (fake key → api.linear.app 401).
- ✅ Figma base_url A→B ends the A session, both in the harness and via the local Switchboard web Save. Slack MCP Close ends both identities' sessions.
- ✅ On main 5b82ef2, the same runs leave 8 sessions open (0 DELETEs). The guard test fails on main naming all three adapters.
- ✅ go build/vet, `go test -race ./...`, golangci-lint and gosec pass. Not run on the box: full `make ci` (no make, Node 26, buf, Docker), govulncheck.

Merge-ready from the sandbox side. Notes: the local server never calls Close (not even on shutdown); Linear Execute after Close hits api.linear.app unauthenticated (401) instead of "not configured". Details and video: docs/sandbox-proofs/remote-mcp-close-349/ on sandbox-e2e-proofs.
