package specimport

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	mcp "github.com/daltoniam/switchboard"
)

// These tests exercise the GraphQL spec-import pipeline against real, live
// GraphQL endpoints (GitHub, Linear, and curri) instead of an httptest mock.
// They prove that introspecting a production schema, parsing it through
// Parse, and executing a synthesized query actually round-trips end-to-end.
//
// They are skipped unless a token is available, so CI and `task test` stay
// hermetic. Tokens are sourced (in priority order) from:
//  1. an explicit env var (SPECIMPORT_LIVE_{GITHUB,LINEAR,CURRI}_TOKEN), then
//  2. the local Switchboard config at ~/.config/switchboard/config.json
//     (and its .bak), reusing the integration credentials already on disk.
//
// The flow mirrors what mcpd does at runtime:
//  1. POST the standard introspection query to the endpoint.
//  2. Parse(KindGraphQL, ...) the result into an Imported.
//  3. NewIntegration + Configure with the real credentials.
//  4. Execute a real, no-argument query field and assert a 2xx data payload.

// liveAuth describes how to authenticate against one live endpoint. scheme is
// the Authorization prefix ("Bearer" for GitHub/curri, "" for Linear, which
// expects the raw key). credConfig matches the credScheme contract used by
// Configure.
type liveAuth struct {
	name     string
	endpoint string
	token    string
	scheme   string // "" means send the raw token with no prefix
}

// introspectionQuery is the minimal introspection document needed by
// parseGraphQL: query/mutation root names, each type's name, and each
// field's args with their (possibly wrapped) type. It deliberately omits
// the parts of the full GraphQL introspection query (directives, enum
// values, input fields, deep ofType nesting) that our parser ignores.
const introspectionQuery = `query IntrospectionQuery {
  __schema {
    queryType { name }
    mutationType { name }
    types {
      kind
      name
      fields(includeDeprecated: true) {
        name
        description
        args {
          name
          description
          type { kind name ofType { kind name ofType { kind name ofType { kind name } } } }
        }
        type { kind name ofType { kind name ofType { kind name ofType { kind name } } } }
      }
    }
  }
}`

// authHeaderValue renders the Authorization header value for an auth config.
func (a liveAuth) authHeaderValue() string {
	if a.scheme == "" {
		return a.token
	}
	return a.scheme + " " + a.token
}

// errIntrospectionUnavailable signals that the endpoint accepted the request
// but refuses to satisfy introspection (e.g. Apollo Server with introspection
// disabled, or a server-side query-complexity cap). These are deliberate
// production constraints, not importer bugs, so callers skip rather than fail.
var errIntrospectionUnavailable = errors.New("endpoint does not support introspection")

// fetchIntrospection runs the introspection query against the endpoint using
// the given auth and returns the raw JSON response body. It sends an explicit
// User-Agent because some endpoints (curri, fronted by Cloudflare) reject the
// default Go agent. It returns errIntrospectionUnavailable when the endpoint
// declines to introspect so the caller can skip cleanly.
func fetchIntrospection(t *testing.T, a liveAuth) ([]byte, error) {
	t.Helper()
	reqBody, err := json.Marshal(map[string]string{"query": introspectionQuery})
	if err != nil {
		t.Fatalf("marshal introspection query: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, a.endpoint, bytes.NewReader(reqBody))
	if err != nil {
		t.Fatalf("build introspection request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", a.authHeaderValue())
	req.Header.Set("User-Agent", userAgent)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("introspection request failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 32<<20))
	if err != nil {
		t.Fatalf("read introspection response: %v", err)
	}
	if introspectionDisabled(body) {
		return nil, fmt.Errorf("%w: %s", errIntrospectionUnavailable, truncate(string(body), 256))
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("introspection returned %s: %s", resp.Status, truncate(string(body), 1024))
	}
	return body, nil
}

// introspectionDisabled reports whether the response indicates the server
// refuses introspection rather than returning a usable schema. Apollo Server
// rejects __schema with a validation error; Linear caps query complexity well
// below what the standard introspection query needs.
func introspectionDisabled(body []byte) bool {
	lower := strings.ToLower(string(body))
	switch {
	case strings.Contains(lower, "introspection is not allowed"):
		return true
	case strings.Contains(lower, "introspection") && strings.Contains(lower, "disabled"):
		return true
	case strings.Contains(lower, "query is too complex"), strings.Contains(lower, "query too complex"):
		return true
	default:
		return false
	}
}

// liveResult is the asserted outcome of executing a live query field.
type liveResult int

const (
	// wantData expects a 2xx response with the field present under "data"
	// and no top-level GraphQL errors. With default selection-set generation
	// this is achievable for object/interface/union fields too, not just
	// scalars.
	wantData liveResult = iota
)

// importLive introspects a live endpoint and returns the configured
// integration plus the parsed import, asserting basic schema invariants. It
// returns errIntrospectionUnavailable when the endpoint refuses introspection
// so the caller can skip.
func importLive(t *testing.T, a liveAuth) (*Integration, *Imported, error) {
	t.Helper()

	doc, err := fetchIntrospection(t, a)
	if err != nil {
		return nil, nil, err
	}

	im, err := Parse(KindGraphQL, a.name, doc, a.endpoint)
	if err != nil {
		t.Fatalf("Parse live %s introspection: %v", a.name, err)
	}
	tools := im.Tools()
	if len(tools) == 0 {
		t.Fatalf("live %s schema produced no tools", a.name)
	}
	t.Logf("live %s schema produced %d tools", a.name, len(tools))

	in := NewIntegration(im)

	// Confirm the parser preserved read/write semantics: at least one tool
	// from the Mutation root must be flagged as a write.
	var sawWrite bool
	for _, tool := range tools {
		if in.IsWrite(tool.Name) {
			sawWrite = true
			break
		}
	}
	if !sawWrite {
		t.Errorf("expected at least one mutation tool to be marked write")
	}

	creds := mcp.Credentials{credAPIKey: a.token}
	// Linear authenticates with the raw key (no "Bearer"); convey that to
	// Configure via an explicit empty scheme so the header matches.
	if a.scheme == "" {
		creds[credScheme] = ""
	}
	if err := in.Configure(context.Background(), creds); err != nil {
		t.Fatalf("Configure: %v", err)
	}
	return in, im, nil
}

// runLiveQuery executes one no-argument query field against a live endpoint
// and asserts the expected outcome. toolSuffix is the sanitized tool-name
// suffix (e.g. "ratelimit"); dataKey is the original GraphQL field name the
// response is keyed by (e.g. "rateLimit").
func runLiveQuery(t *testing.T, in *Integration, name, toolSuffix, dataKey string, want liveResult) {
	t.Helper()

	wantTool := mcp.ToolName(name + "_" + toolSuffix)
	op, ok := in.opByTool[wantTool]
	if !ok {
		t.Fatalf("expected tool %q not found in imported schema", wantTool)
	}
	if op.effect != effectRead {
		t.Errorf("%q effect = %q, want read", wantTool, op.effect)
	}

	res, err := in.Execute(context.Background(), wantTool, map[string]any{})
	if err != nil {
		t.Fatalf("Execute %q: %v", wantTool, err)
	}

	switch want {
	case wantData:
		if res.IsError {
			t.Fatalf("Execute %q returned error result: %s", wantTool, res.Data)
		}
		var envelope struct {
			Data   map[string]json.RawMessage `json:"data"`
			Errors []struct {
				Message string `json:"message"`
			} `json:"errors"`
		}
		if err := json.Unmarshal([]byte(res.Data), &envelope); err != nil {
			t.Fatalf("decode response %q: %v\nbody: %s", wantTool, err, truncate(string(res.Data), 1024))
		}
		if len(envelope.Errors) > 0 {
			t.Fatalf("graphql errors for %q: %s", wantTool, envelope.Errors[0].Message)
		}
		if _, ok := envelope.Data[dataKey]; !ok {
			t.Fatalf("response missing %q field: %s", dataKey, truncate(string(res.Data), 1024))
		}
		t.Logf("live %s %s returned: %s", name, dataKey, truncate(string(res.Data), 512))
	}
}

// liveToken resolves a token from an env var first, then from the local
// Switchboard config integration credentials. Returns "" when none is found,
// in which case the caller skips.
func liveToken(t *testing.T, envVar, integration string, credKeys ...string) string {
	t.Helper()
	if v := strings.TrimSpace(os.Getenv(envVar)); v != "" {
		return v
	}
	for _, name := range switchboardConfigPaths() {
		raw, err := os.ReadFile(name)
		if err != nil {
			continue
		}
		var cfg struct {
			Integrations map[string]struct {
				Credentials map[string]string `json:"credentials"`
			} `json:"integrations"`
		}
		if json.Unmarshal(raw, &cfg) != nil {
			continue
		}
		creds := cfg.Integrations[integration].Credentials
		for _, k := range credKeys {
			if v := strings.TrimSpace(creds[k]); v != "" {
				return v
			}
		}
	}
	return ""
}

// switchboardConfigPaths returns candidate Switchboard config files in
// preference order (live config first, then the most recent backup).
func switchboardConfigPaths() []string {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}
	dir := filepath.Join(home, ".config", "switchboard")
	return []string{
		filepath.Join(dir, "config.json"),
		filepath.Join(dir, "config.json.bak"),
	}
}

func TestLiveGraphQLGitHub(t *testing.T) {
	token := liveToken(t, "SPECIMPORT_LIVE_GITHUB_TOKEN", "github", "token")
	if token == "" {
		t.Skip("no GitHub token (set SPECIMPORT_LIVE_GITHUB_TOKEN or configure the github integration)")
	}
	in, _, err := importLive(t, liveAuth{
		name: "github", endpoint: "https://api.github.com/graphql",
		token: token, scheme: "Bearer",
	})
	if err != nil {
		t.Fatalf("import live github: %v", err)
	}

	// `id` is a scalar root query field — the synthesized `query { id }` is
	// valid with no selection set.
	runLiveQuery(t, in, "github", "id", "id", wantData)

	// `rateLimit` returns the RateLimit object whose fields are all plain
	// scalars readable by any token. The importer now synthesizes a default
	// selection set (e.g. `rateLimit { cost limit remaining ... }`), so the
	// query validates and returns data — this is the fix being verified.
	if got := in.opByTool["github_ratelimit"].gqlDocument; strings.Count(got, "{") < 2 {
		t.Errorf("github_ratelimit should have a generated selection set, got: %s", got)
	}
	runLiveQuery(t, in, "github", "ratelimit", "rateLimit", wantData)
}

func TestLiveGraphQLLinear(t *testing.T) {
	token := liveToken(t, "SPECIMPORT_LIVE_LINEAR_TOKEN", "linear", "api_key")
	if token == "" {
		t.Skip("no Linear token (set SPECIMPORT_LIVE_LINEAR_TOKEN or configure the linear integration)")
	}
	// Linear authenticates with the raw API key in the Authorization header,
	// NOT a Bearer token — scheme is empty.
	in, _, err := importLive(t, liveAuth{
		name: "linear", endpoint: "https://api.linear.app/graphql",
		token: token, scheme: "",
	})
	if errors.Is(err, errIntrospectionUnavailable) {
		// Linear caps query complexity (max 10000) far below what the
		// standard introspection query costs (~262144), so the live schema
		// cannot be imported via introspection. This is a Linear-side
		// constraint, not an importer bug.
		t.Skipf("linear refuses full introspection: %v", err)
	}
	if err != nil {
		t.Fatalf("import live linear: %v", err)
	}

	// `viewer` returns the User object; the generated selection set makes it
	// a valid, data-returning query against a real schema with hundreds of
	// types.
	if got := in.opByTool["linear_viewer"].gqlDocument; strings.Count(got, "{") < 2 {
		t.Errorf("linear_viewer should have a generated selection set, got: %s", got)
	}
	runLiveQuery(t, in, "linear", "viewer", "viewer", wantData)
}

func TestLiveGraphQLCurri(t *testing.T) {
	token := liveToken(t, "SPECIMPORT_LIVE_CURRI_TOKEN", "curri", "jwt")
	if token == "" {
		t.Skip("no curri token (set SPECIMPORT_LIVE_CURRI_TOKEN or configure the curri integration)")
	}
	// curri's API is fronted by Cloudflare, which rejects the default Go
	// user agent — the integration's do() and the test introspection both
	// send an explicit User-Agent so the request is allowed through. Auth is
	// a Bearer JWT.
	in, _, err := importLive(t, liveAuth{
		name: "curri", endpoint: "https://api.curri.com/graphql",
		token: token, scheme: "Bearer",
	})
	if errors.Is(err, errIntrospectionUnavailable) {
		// curri runs Apollo Server with introspection disabled in
		// production, so __schema queries are rejected and the schema
		// cannot be imported via introspection. This is a server policy,
		// not an importer bug.
		t.Skipf("curri disables introspection: %v", err)
	}
	if err != nil {
		t.Fatalf("import live curri: %v", err)
	}

	// Exercise the introspected schema's `__typename` is implicit; pick the
	// `viewer`-style root the schema exposes. curri's Query root must yield
	// at least one no-arg object field we can select; assert the importer
	// produced a selection set for it and that the query validates.
	field, dataKey := curriProbeField(t, in)
	runLiveQuery(t, in, "curri", field, dataKey, wantData)
}

// curriProbeField finds a no-argument query field whose return type is an
// object/interface/union (so the selection-set fix is exercised) and returns
// its sanitized tool suffix plus the original GraphQL field name (the data
// key). It fails the test if no such field exists.
func curriProbeField(t *testing.T, in *Integration) (toolSuffix, dataKey string) {
	t.Helper()
	type cand struct{ suffix, key string }
	var best *cand
	for tool, op := range in.opByTool {
		if op.effect != effectRead {
			continue
		}
		if len(op.tool.Required) > 0 || len(op.gqlVariables) > 0 {
			continue // needs args we cannot supply
		}
		if strings.Count(op.gqlDocument, "{") < 2 {
			continue // scalar field, doesn't exercise selection sets
		}
		name := string(tool)
		suffix := strings.TrimPrefix(name, in.Name()+"_")
		// Recover the original field name from the document: "query { <field> ...".
		doc := op.gqlDocument
		i := strings.Index(doc, "{ ")
		if i < 0 {
			continue
		}
		rest := doc[i+2:]
		key := rest
		if j := strings.IndexAny(rest, " ({"); j >= 0 {
			key = rest[:j]
		}
		c := cand{suffix: suffix, key: strings.TrimSpace(key)}
		// Prefer a field literally named "viewer"/"me" when present for a
		// deterministic, low-cost probe.
		if c.key == "viewer" || c.key == "me" {
			return c.suffix, c.key
		}
		if best == nil {
			b := c
			best = &b
		}
	}
	if best == nil {
		t.Skip("curri schema exposes no no-argument object query field to probe")
	}
	return best.suffix, best.key
}
