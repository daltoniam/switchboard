package specimport

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	mcp "github.com/daltoniam/switchboard"
	"github.com/daltoniam/switchboard/netpolicy"
)

// allowPrivateOutboundHosts is test-only: httptest servers listen on loopback.
var allowPrivateOutboundHosts bool

// SetAllowPrivateOutboundHostsForTest enables loopback/private base URLs. Must
// only be called from tests.
func SetAllowPrivateOutboundHostsForTest(allow bool) {
	allowPrivateOutboundHosts = allow
}

// defaultTimeout caps every upstream HTTP request the integration makes.
const defaultTimeout = 30 * time.Second

// maxResponseBytes bounds how much of an upstream response we read into
// memory before truncating. Imported APIs are arbitrary, so we protect the
// runtime from a hostile or buggy endpoint streaming unbounded data.
const maxResponseBytes = 5 << 20 // 5 MiB

// userAgent identifies spec-import traffic to upstream APIs. A blank/default
// Go user agent is rejected by some bot-protection layers (e.g. Cloudflare),
// so we always send an explicit one.
const userAgent = "Switchboard-SpecImport/1.0"

// Credential keys understood by Configure. They are injected host-side into
// every outbound request and never appear in tool definitions or results.
const (
	// credAPIKey is sent as a bearer token unless credHeader overrides it.
	credAPIKey = "api_key"
	// credHeader names the header the api_key is placed in (default
	// "Authorization"). When set without "Bearer", the raw key is used.
	credHeader = "auth_header"
	// credScheme is the auth scheme prefix (default "Bearer"). Set to an
	// empty string via the form to send the raw key with no prefix.
	credScheme = "auth_scheme"
	// credBaseURL overrides the spec's server URL at configure time.
	credBaseURL = "base_url"
)

// Integration is a runtime mcp.Integration produced from an imported spec.
// One instance wraps one Imported document and is safe for concurrent use
// after Configure returns (its fields are only mutated during Configure).
type Integration struct {
	im     *Imported
	client *http.Client

	// resolved at Configure time
	baseURL    string
	authHeader string
	authValue  string

	// opByTool indexes operations by their tool name for O(1) Execute.
	opByTool map[mcp.ToolName]*operation
}

// NewIntegration builds a runtime integration from a parsed import.
func NewIntegration(im *Imported) *Integration {
	idx := make(map[mcp.ToolName]*operation, len(im.operations))
	for i := range im.operations {
		idx[im.operations[i].tool.Name] = &im.operations[i]
	}
	return &Integration{
		im:       im,
		client:   outboundHTTPClient(im.BaseURL, "", ""),
		baseURL:  im.BaseURL,
		opByTool: idx,
	}
}

func outboundHTTPClient(baseURL, authHeader, authValue string) *http.Client {
	var origin *url.URL
	if u, err := url.Parse(strings.TrimSpace(baseURL)); err == nil {
		origin = u
	}
	return netpolicy.NewUserHTTPClient(netpolicy.UserHTTPClientConfig{
		Timeout:          defaultTimeout,
		AllowPrivate:     allowPrivateOutboundHosts,
		ValidateURL:      validateURL,
		AuthHeader:       authHeader,
		AuthValue:        authValue,
		AuthOrigin:       origin,
		DisableRedirects: !allowPrivateOutboundHosts,
	})
}

func (in *Integration) rebuildClient() {
	in.client = outboundHTTPClient(in.baseURL, in.authHeader, in.authValue)
}

// Name returns the integration identifier (the sanitized import name).
func (in *Integration) Name() string { return in.im.Name }

// Configure resolves credentials and any base-URL override. Credentials are
// stored on the instance and attached to outbound requests; they are never
// exposed to the model.
func (in *Integration) Configure(_ context.Context, creds mcp.Credentials) error {
	in.baseURL = in.im.BaseURL
	if in.im.Kind != KindGraphQL {
		in.baseURL = strings.TrimRight(in.baseURL, "/")
	}
	if override := strings.TrimSpace(creds[credBaseURL]); override != "" {
		if err := validateURL(override); err != nil {
			return fmt.Errorf("specimport: invalid base_url: %w", err)
		}
		if in.im.Kind == KindGraphQL {
			in.baseURL = override
		} else {
			in.baseURL = strings.TrimRight(override, "/")
		}
	}
	if in.baseURL == "" {
		return fmt.Errorf("specimport: no base url; set base_url credential or include a server in the spec")
	}
	if err := validateURL(in.baseURL); err != nil {
		return fmt.Errorf("specimport: invalid base url: %w", err)
	}

	key := strings.TrimSpace(creds[credAPIKey])
	if key == "" {
		// Public / unauthenticated API — valid, just no auth header.
		in.authHeader, in.authValue = "", ""
		in.rebuildClient()
		return nil
	}
	header := strings.TrimSpace(creds[credHeader])
	if header == "" {
		header = "Authorization"
	}
	scheme, hasScheme := creds[credScheme]
	scheme = strings.TrimSpace(scheme)
	if !hasScheme {
		scheme = "Bearer"
	}
	in.authHeader = header
	if scheme == "" {
		in.authValue = key
	} else {
		in.authValue = scheme + " " + key
	}
	in.rebuildClient()
	return nil
}

// Tools returns every imported operation as an MCP tool definition.
func (in *Integration) Tools() []mcp.ToolDefinition { return in.im.Tools() }

// Healthy reports whether the integration is configured. We intentionally do
// not probe the upstream: an imported spec may have no safe, argument-free
// endpoint to hit, and a failing probe would wrongly hide a usable
// integration. Configuration success is the health signal.
func (in *Integration) Healthy(_ context.Context) bool { return in.baseURL != "" }

// Execute dispatches a tool call to the correct transport.
func (in *Integration) Execute(ctx context.Context, toolName mcp.ToolName, args map[string]any) (*mcp.ToolResult, error) {
	op, ok := in.opByTool[toolName]
	if !ok {
		return mcp.ErrResult(fmt.Errorf("specimport: unknown tool %q", toolName))
	}
	if in.baseURL == "" {
		return mcp.ErrResult(fmt.Errorf("specimport: integration not configured"))
	}
	if err := requireArgs(op, args); err != nil {
		return mcp.ErrResult(err)
	}
	switch in.im.Kind {
	case KindGraphQL:
		return in.executeGraphQL(ctx, op, args)
	default:
		return in.executeOpenAPI(ctx, op, args)
	}
}

// requireArgs validates that every required parameter is present and
// non-empty before we build a request. Catches model mistakes early with a
// clear, non-retryable error.
func requireArgs(op *operation, args map[string]any) error {
	var missing []string
	for _, r := range op.tool.Required {
		v, ok := args[r]
		if !ok || v == nil {
			missing = append(missing, r)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("specimport: missing required argument(s): %s", strings.Join(missing, ", "))
	}
	return nil
}

// executeOpenAPI builds and sends an HTTP request for an OpenAPI operation:
// substitute path params, append query params, and send remaining args
// (write methods only) as a JSON body.
func (in *Integration) executeOpenAPI(ctx context.Context, op *operation, args map[string]any) (*mcp.ToolResult, error) {
	path := op.pathTemplate
	pathSet := bindingArgSet(op.pathBindings)
	querySet := bindingArgSet(op.queryBindings)
	q := url.Values{}
	for _, b := range op.queryBindings {
		if v, ok := args[b.arg]; ok && v != nil {
			formatted, err := formatParamValue(v)
			if err != nil {
				return mcp.ErrResult(fmt.Errorf("specimport: query parameter %q: %w", b.arg, err))
			}
			q.Set(b.wire, formatted)
		}
	}
	for _, b := range op.pathBindings {
		v, ok := args[b.arg]
		if !ok {
			return mcp.ErrResult(fmt.Errorf("specimport: missing path parameter %q", b.arg))
		}
		formatted, err := formatParamValue(v)
		if err != nil {
			return mcp.ErrResult(fmt.Errorf("specimport: path parameter %q: %w", b.arg, err))
		}
		path = strings.ReplaceAll(path, "{"+b.wire+"}", url.PathEscape(formatted))
	}

	target, err := joinOpenAPIURL(in.baseURL, path, q)
	if err != nil {
		return mcp.ErrResult(err)
	}

	body, contentType, bodyErr := openAPIWriteBody(op, args, pathSet, querySet)
	if bodyErr != nil {
		return mcp.ErrResult(bodyErr)
	}

	req, err := http.NewRequestWithContext(ctx, op.httpMethod, target, body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	for _, b := range op.headerBindings {
		v, ok := args[b.arg]
		if !ok || v == nil {
			continue
		}
		formatted, err := formatParamValue(v)
		if err != nil {
			return mcp.ErrResult(fmt.Errorf("specimport: header parameter %q: %w", b.arg, err))
		}
		req.Header.Set(b.wire, formatted)
	}
	for _, b := range op.cookieBindings {
		v, ok := args[b.arg]
		if !ok || v == nil {
			continue
		}
		formatted, err := formatParamValue(v)
		if err != nil {
			return mcp.ErrResult(fmt.Errorf("specimport: cookie parameter %q: %w", b.arg, err))
		}
		req.AddCookie(&http.Cookie{Name: b.wire, Value: formatted})
	}
	return in.do(req)
}

func joinOpenAPIURL(base, path string, opQuery url.Values) (string, error) {
	u, err := url.Parse(strings.TrimSpace(base))
	if err != nil {
		return "", fmt.Errorf("specimport: invalid base url: %w", err)
	}
	joined, err := url.JoinPath(u.Path, path)
	if err != nil {
		return "", fmt.Errorf("specimport: join url path: %w", err)
	}
	u.Path = joined
	q := u.Query()
	for k, vals := range opQuery {
		for _, v := range vals {
			q.Add(k, v)
		}
	}
	u.RawQuery = q.Encode()
	return u.String(), nil
}

func openAPIWriteBody(op *operation, args map[string]any, pathSet, querySet map[string]bool) (io.Reader, string, error) {
	if op.effect != effectWrite {
		return nil, "", nil
	}
	if bodyVal, explicitBody := args["body"]; explicitBody {
		m, ok := bodyVal.(map[string]any)
		if !ok {
			return nil, "", fmt.Errorf("specimport: body must be a JSON object")
		}
		raw, err := json.Marshal(m)
		if err != nil {
			return nil, "", fmt.Errorf("specimport: marshal body: %w", err)
		}
		return bytes.NewReader(raw), "application/json", nil
	}
	payload := bodyArgs(args, pathSet, querySet, op.headerBindings, op.cookieBindings)
	if len(payload) == 0 {
		return nil, "", nil
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return nil, "", fmt.Errorf("specimport: marshal body: %w", err)
	}
	return bytes.NewReader(raw), "application/json", nil
}

// executeGraphQL POSTs the synthesized document with the relevant args
// forwarded as GraphQL variables.
func (in *Integration) executeGraphQL(ctx context.Context, op *operation, args map[string]any) (*mcp.ToolResult, error) {
	vars := map[string]any{}
	for _, name := range op.gqlVariables {
		if v, ok := args[name]; ok {
			vars[name] = v
		}
	}
	payload := map[string]any{"query": op.gqlDocument}
	if len(vars) > 0 {
		payload["variables"] = vars
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return mcp.ErrResult(fmt.Errorf("specimport: marshal graphql request: %w", err))
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, in.baseURL, bytes.NewReader(raw))
	if err != nil {
		return mcp.ErrResult(err)
	}
	req.Header.Set("Content-Type", "application/json")
	res, err := in.do(req)
	if err != nil || res == nil || res.IsError {
		return res, err
	}
	// GraphQL reports query-level failures as HTTP 200 with a top-level
	// "errors" array, so a 2xx status alone does not mean success. Promote a
	// response that carries errors (and no usable data) to an error result so
	// the policy layer and the model see the failure instead of treating the
	// error envelope as a successful payload.
	if msg, failed := graphQLError(res.Data); failed {
		return mcp.ErrResult(fmt.Errorf("specimport: graphql error: %s", msg))
	}
	return res, nil
}

// graphQLError inspects a GraphQL response envelope and reports whether it
// represents a failed operation. A response is treated as failed when it
// contains a non-empty top-level "errors" array AND no usable "data" (data
// absent or null) — partial successes (data present alongside errors) are
// left intact so the caller still receives whatever the server resolved.
func graphQLError(body string) (string, bool) {
	var env struct {
		Data   json.RawMessage `json:"data"`
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}
	if err := json.Unmarshal([]byte(body), &env); err != nil {
		// Not a JSON envelope we recognize — leave the raw body untouched.
		return "", false
	}
	if len(env.Errors) == 0 {
		return "", false
	}
	if len(env.Data) > 0 && string(env.Data) != "null" {
		// Partial success: keep the data, do not flag as a hard error.
		return "", false
	}
	msg := env.Errors[0].Message
	if msg == "" {
		msg = "request failed"
	}
	if len(env.Errors) > 1 {
		msg = fmt.Sprintf("%s (and %d more)", msg, len(env.Errors)-1)
	}
	return msg, true
}

// do attaches auth + Accept headers, sends the request, and returns a
// bounded, truncation-aware ToolResult. Non-2xx responses are surfaced as
// non-retryable errors carrying the upstream body so the model can react.
func (in *Integration) do(req *http.Request) (*mcp.ToolResult, error) {
	if req.Header.Get("Accept") == "" {
		req.Header.Set("Accept", "application/json")
	}
	if req.Header.Get("User-Agent") == "" {
		// Some upstreams (e.g. Cloudflare-fronted APIs) reject the default
		// Go user agent as a suspected bot. Send an explicit, identifiable
		// one so imported integrations reach those endpoints.
		req.Header.Set("User-Agent", userAgent)
	}
	// #nosec G704 -- The request URL is the customer's own imported API
	// (validated http/https at Configure/Parse time via url.ParseRequestURI).
	// Spec import is a deliberate "fetch your own API" feature, the same
	// trust model as the webfetch integration; the per-org policy layer and
	// write-approval gating bound what an agent can actually invoke.
	resp, err := in.client.Do(req)
	if err != nil {
		return mcp.ErrResult(fmt.Errorf("specimport: request failed: %w", err))
	}
	defer func() { _ = resp.Body.Close() }()

	data, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes+1))
	if err != nil {
		return mcp.ErrResult(fmt.Errorf("specimport: read response: %w", err))
	}
	if int64(len(data)) > maxResponseBytes {
		return mcp.ErrResult(fmt.Errorf("specimport: response exceeds %d byte limit", maxResponseBytes))
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return mcp.ErrResult(fmt.Errorf("specimport: upstream returned %s: %s", resp.Status, truncate(string(data), 2048)))
	}
	return mcp.RawResult(data)
}

// bodyArgs returns the args that are neither path nor query params — these
// form the JSON request body for write methods. An explicit "body" arg, if
// present, takes precedence and is used verbatim.
func bodyArgs(args map[string]any, pathSet, querySet map[string]bool, headers, cookies []openAPIParamBinding) map[string]any {
	if b, ok := args["body"]; ok {
		if m, ok := b.(map[string]any); ok {
			return m
		}
	}
	out := map[string]any{}
	for k, v := range args {
		if k == "body" || pathSet[k] || querySet[k] {
			continue
		}
		if paramBindingContains(headers, k) || paramBindingContains(cookies, k) {
			continue
		}
		out[k] = v
	}
	return out
}

func bindingArgSet(bindings []openAPIParamBinding) map[string]bool {
	m := make(map[string]bool, len(bindings))
	for _, b := range bindings {
		m[b.arg] = true
	}
	return m
}

func paramBindingContains(bindings []openAPIParamBinding, arg string) bool {
	for _, b := range bindings {
		if b.arg == arg {
			return true
		}
	}
	return false
}

// validateURL enforces that an imported endpoint is an absolute http(s) URL
// with an allowed host (same policy as hosted user-supplied outbound URLs).
func validateURL(raw string) error {
	if allowPrivateOutboundHosts {
		u, err := url.Parse(strings.TrimSpace(raw))
		if err != nil {
			return err
		}
		if u.Scheme != "http" && u.Scheme != "https" {
			return fmt.Errorf("url must be http or https, got %q", u.Scheme)
		}
		if u.Host == "" {
			return fmt.Errorf("url must include a host")
		}
		return nil
	}
	return netpolicy.ValidateUserHTTPURL(raw)
}

// formatParamValue stringifies a path or query argument without scientific
// notation for whole-number floats (JSON decodes integers as float64).
func formatParamValue(v any) (string, error) {
	return mcp.ArgStr(map[string]any{"v": v}, "v")
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…(truncated)"
}

// --- Optional mcp interfaces ---

// PlainTextKeys marks credential keys that are non-secret configuration so
// the UI renders them as plain inputs rather than password fields.
func (in *Integration) PlainTextKeys() []string {
	return []string{credBaseURL, credHeader, credScheme}
}

// OptionalKeys reports credentials that are not required — every spec-import
// credential is optional (a public API needs none).
func (in *Integration) OptionalKeys() []string {
	return []string{credAPIKey, credHeader, credScheme, credBaseURL}
}

// Placeholders gives the UI helpful example values for each credential field.
func (in *Integration) Placeholders() map[string]string {
	return map[string]string{
		credAPIKey:  "your API key (optional for public APIs)",
		credHeader:  "Authorization",
		credScheme:  "Bearer",
		credBaseURL: in.im.BaseURL,
	}
}

// compile-time check that we satisfy the primary port.
var _ mcp.Integration = (*Integration)(nil)
