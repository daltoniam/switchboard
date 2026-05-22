package remotemcp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	mcp "github.com/daltoniam/switchboard"
	"github.com/daltoniam/switchboard/version"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	defaultTimeout = 30 * time.Second
)

// TokenSink is invoked whenever a refresh succeeds and produces new token
// material that callers may want to persist. The supplied Credentials map
// contains only the keys that changed: always "access_token" after a
// successful refresh, plus "refresh_token" when the upstream rotated it.
// Implementations should be safe to call from arbitrary goroutines.
type TokenSink func(creds mcp.Credentials)

type remote struct {
	name      string
	serverURL string
	tokenSink TokenSink

	// connectMu serializes connection setup. It MUST NOT be held while
	// holding mu — client.Connect() issues HTTP requests through
	// refreshingTransport which needs to RLock mu to read the token, so
	// holding mu across Connect() would deadlock.
	connectMu sync.Mutex

	mu           sync.RWMutex
	token        string
	refreshToken string
	clientID     string
	clientSecret string
	session      *mcpsdk.ClientSession
	client       *mcpsdk.Client
	cachedTools  []mcp.ToolDefinition
	toolsFetched bool
}

// Option configures a remote MCP integration constructed via New.
type Option func(*remote)

// WithTokenSink registers a callback that receives refreshed tokens whenever
// the transport performs a refresh. Required for refresh tokens to survive
// process restarts — without a sink, refreshed access tokens live only in
// memory and the next startup hits 401 again and re-refreshes.
func WithTokenSink(sink TokenSink) Option {
	return func(r *remote) { r.tokenSink = sink }
}

// New creates a remote MCP integration that proxies to the given server URL.
func New(name, serverURL string, opts ...Option) mcp.Integration {
	r := &remote{
		name:      name,
		serverURL: serverURL,
	}
	for _, opt := range opts {
		opt(r)
	}
	return r
}

func (r *remote) Name() string { return r.name }

func (r *remote) DeferStartupToolDiscovery() bool { return true }

func (r *remote) Configure(_ context.Context, creds mcp.Credentials) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	token := creds["access_token"]
	if token == "" {
		return fmt.Errorf("%s: access_token is required", r.name)
	}

	if token != r.token {
		if r.session != nil {
			_ = r.session.Close()
			r.session = nil
		}
		r.token = token
		r.toolsFetched = false
		r.cachedTools = nil
	}

	r.refreshToken = creds["refresh_token"]
	r.clientID = creds["client_id"]
	r.clientSecret = creds["client_secret"]
	return nil
}

// canRefresh reports whether we have enough material to attempt a refresh.
// Must be called with r.mu held (read lock is sufficient).
func (r *remote) canRefresh() bool {
	return r.refreshToken != "" && r.clientID != ""
}

// refresh exchanges the stored refresh_token for a new access_token, updates
// the cached credentials, and notifies the token sink. Returns the new access
// token on success. Caller MUST NOT hold r.mu.
func (r *remote) refresh(ctx context.Context) (string, error) {
	r.mu.RLock()
	refreshToken := r.refreshToken
	clientID := r.clientID
	clientSecret := r.clientSecret
	serverURL := r.serverURL
	sink := r.tokenSink
	r.mu.RUnlock()

	if refreshToken == "" || clientID == "" {
		return "", fmt.Errorf("refresh not configured (need refresh_token + client_id)")
	}

	tokens, err := RefreshAccessToken(ctx, serverURL, clientID, clientSecret, refreshToken)
	if err != nil {
		return "", err
	}

	r.mu.Lock()
	r.token = tokens.AccessToken
	updated := mcp.Credentials{"access_token": tokens.AccessToken}
	if tokens.RefreshToken != "" && tokens.RefreshToken != r.refreshToken {
		r.refreshToken = tokens.RefreshToken
		updated["refresh_token"] = tokens.RefreshToken
	}
	r.mu.Unlock()

	if sink != nil {
		sink(updated)
	}
	return tokens.AccessToken, nil
}

// refreshingTransport injects an Authorization header into every request and,
// on a 401, attempts to exchange the stored refresh_token for a new access
// token and retries the request exactly once. Request bodies are buffered so
// the retry can replay them — MCP payloads are small JSON-RPC blobs, so the
// memory cost is negligible.
type refreshingTransport struct {
	remote *remote
	base   http.RoundTripper
}

func (t *refreshingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	base := t.base
	if base == nil {
		base = http.DefaultTransport
	}

	// Buffer the body once so we can replay on a 401 retry. http.Request.Body
	// is a stream that's consumed by the first RoundTrip; without buffering,
	// the retry would send an empty body.
	var bodyBytes []byte
	if req.Body != nil && req.Body != http.NoBody {
		var err error
		bodyBytes, err = io.ReadAll(req.Body)
		_ = req.Body.Close()
		if err != nil {
			return nil, fmt.Errorf("buffer request body: %w", err)
		}
		req.Body = io.NopCloser(bytes.NewReader(bodyBytes))
	}

	t.remote.mu.RLock()
	token := t.remote.token
	t.remote.mu.RUnlock()

	resp, err := t.do(req, token, bodyBytes, base)
	if err != nil {
		return resp, err
	}
	if resp.StatusCode != http.StatusUnauthorized {
		return resp, nil
	}

	t.remote.mu.RLock()
	canRefresh := t.remote.canRefresh()
	t.remote.mu.RUnlock()
	if !canRefresh {
		return resp, nil
	}

	// Best-effort drain so the underlying connection can be reused.
	_, _ = io.Copy(io.Discard, resp.Body)
	_ = resp.Body.Close()

	newToken, refreshErr := t.remote.refresh(req.Context())
	if refreshErr != nil {
		// Surface the original 401 status to the caller — refresh failures
		// are typically auth-fatal (expired refresh_token), and returning a
		// distinct error would confuse the MCP client's error handling.
		return t.do(req, token, bodyBytes, base)
	}

	return t.do(req, newToken, bodyBytes, base)
}

func (t *refreshingTransport) do(req *http.Request, token string, bodyBytes []byte, base http.RoundTripper) (*http.Response, error) {
	clone := req.Clone(req.Context())
	clone.Header.Set("Authorization", "Bearer "+token)
	if bodyBytes != nil {
		clone.Body = io.NopCloser(bytes.NewReader(bodyBytes))
		clone.GetBody = func() (io.ReadCloser, error) {
			return io.NopCloser(bytes.NewReader(bodyBytes)), nil
		}
		clone.ContentLength = int64(len(bodyBytes))
	}
	return base.RoundTrip(clone)
}

// bearerTransport is retained for tests that exercise the simple bearer
// path. Production callers go through refreshingTransport via connect().
type bearerTransport struct {
	token string
	base  http.RoundTripper
}

func (t *bearerTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	r := req.Clone(req.Context())
	r.Header.Set("Authorization", "Bearer "+t.token)
	base := t.base
	if base == nil {
		base = http.DefaultTransport
	}
	return base.RoundTrip(r)
}

func (r *remote) connect(ctx context.Context) (*mcpsdk.ClientSession, error) {
	r.mu.RLock()
	if r.session != nil {
		sess := r.session
		r.mu.RUnlock()
		return sess, nil
	}
	hasToken := r.token != ""
	r.mu.RUnlock()

	// Fast-fail when there's no access token. Without this the MCP SDK will
	// try to open a connection and block on network/HTTP retries — which
	// hides "not configured" misuse behind multi-minute hangs on Healthy()
	// or Execute() probes.
	if !hasToken {
		return nil, fmt.Errorf("%s: not configured (call Configure first)", r.name)
	}

	// Serialize connection setup with connectMu, NOT mu. Holding mu across
	// client.Connect() deadlocks: Connect() makes HTTP requests through
	// refreshingTransport, which RLocks mu to read the token.
	r.connectMu.Lock()
	defer r.connectMu.Unlock()

	r.mu.RLock()
	if r.session != nil {
		sess := r.session
		r.mu.RUnlock()
		return sess, nil
	}
	r.mu.RUnlock()

	client := mcpsdk.NewClient(&mcpsdk.Implementation{
		Name:    "switchboard",
		Version: version.String(),
	}, nil)

	transport := &mcpsdk.StreamableClientTransport{
		Endpoint: r.serverURL + "/mcp",
		HTTPClient: &http.Client{
			Transport: &refreshingTransport{remote: r},
			Timeout:   defaultTimeout,
		},
		DisableStandaloneSSE: true,
		// MaxRetries must be negative to disable MCP SDK reconnect loops (0 means
		// default 5 retries with exponential backoff, which can hang startup for
		// minutes when the upstream is slow or misconfigured).
		MaxRetries: -1,
	}

	session, err := client.Connect(ctx, transport, nil)
	if err != nil {
		return nil, fmt.Errorf("connect to %s: %w", r.serverURL, err)
	}

	r.mu.Lock()
	r.client = client
	r.session = session
	r.mu.Unlock()
	return session, nil
}

func (r *remote) disconnect() {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.session != nil {
		_ = r.session.Close()
		r.session = nil
	}
}

func (r *remote) Healthy(ctx context.Context) bool {
	// Fast-path: if we have no access token, there's nothing to authenticate
	// with — skip the upstream round-trip entirely. Otherwise unconfigured
	// integrations would hang for the full HTTP client timeout on every
	// health probe (and the SDK retries can extend that further).
	r.mu.RLock()
	hasToken := r.token != ""
	r.mu.RUnlock()
	if !hasToken {
		return false
	}
	session, err := r.connect(ctx)
	if err != nil {
		return false
	}
	_, err = session.ListTools(ctx, &mcpsdk.ListToolsParams{})
	if err != nil {
		r.disconnect()
		return false
	}
	return true
}

func (r *remote) Tools() []mcp.ToolDefinition {
	r.mu.RLock()
	if r.toolsFetched {
		tools := r.cachedTools
		r.mu.RUnlock()
		return tools
	}
	r.mu.RUnlock()

	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()

	session, err := r.connect(ctx)
	if err != nil {
		return nil
	}

	result, err := session.ListTools(ctx, &mcpsdk.ListToolsParams{})
	if err != nil {
		return nil
	}

	tools := convertTools(r.name, result.Tools)

	r.mu.Lock()
	r.cachedTools = tools
	r.toolsFetched = true
	r.mu.Unlock()

	return tools
}

func (r *remote) Execute(ctx context.Context, toolName mcp.ToolName, args map[string]any) (*mcp.ToolResult, error) {
	session, err := r.connect(ctx)
	if err != nil {
		return &mcp.ToolResult{Data: err.Error(), IsError: true}, nil
	}

	remoteName := strings.TrimPrefix(string(toolName), r.name+"_")

	result, err := session.CallTool(ctx, &mcpsdk.CallToolParams{
		Name:      remoteName,
		Arguments: args,
	})
	if err != nil {
		r.disconnect()
		return &mcp.ToolResult{Data: err.Error(), IsError: true}, nil
	}

	return convertResult(result), nil
}

func convertTools(prefix string, tools []*mcpsdk.Tool) []mcp.ToolDefinition {
	var defs []mcp.ToolDefinition
	for _, t := range tools {
		params := extractParams(t.InputSchema)
		required := extractRequired(t.InputSchema)

		defs = append(defs, mcp.ToolDefinition{
			Name:        mcp.ToolName(prefix + "_" + t.Name),
			Description: t.Description,
			Parameters:  params,
			Required:    required,
		})
	}
	return defs
}

func extractParams(schema any) map[string]string {
	params := make(map[string]string)
	schemaMap, ok := toMap(schema)
	if !ok {
		return params
	}
	props, ok := toMap(schemaMap["properties"])
	if !ok {
		return params
	}
	for k, v := range props {
		desc := ""
		if vMap, ok := toMap(v); ok {
			if d, ok := vMap["description"].(string); ok {
				desc = d
			}
		}
		params[k] = desc
	}
	return params
}

func extractRequired(schema any) []string {
	schemaMap, ok := toMap(schema)
	if !ok {
		return nil
	}
	req, ok := schemaMap["required"]
	if !ok {
		return nil
	}
	reqArr, ok := req.([]any)
	if !ok {
		return nil
	}
	var result []string
	for _, v := range reqArr {
		if s, ok := v.(string); ok {
			result = append(result, s)
		}
	}
	return result
}

func toMap(v any) (map[string]any, bool) {
	if m, ok := v.(map[string]any); ok {
		return m, true
	}
	data, err := json.Marshal(v)
	if err != nil {
		return nil, false
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, false
	}
	return m, true
}

func convertResult(result *mcpsdk.CallToolResult) *mcp.ToolResult {
	if result == nil {
		return &mcp.ToolResult{Data: "no result", IsError: true}
	}

	var parts []string
	for _, c := range result.Content {
		if tc, ok := c.(*mcpsdk.TextContent); ok {
			parts = append(parts, tc.Text)
		} else {
			data, err := json.Marshal(c)
			if err == nil {
				parts = append(parts, string(data))
			}
		}
	}

	return &mcp.ToolResult{
		Data:    strings.Join(parts, "\n"),
		IsError: result.IsError,
	}
}
