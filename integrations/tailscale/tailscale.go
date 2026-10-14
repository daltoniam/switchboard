package tailscale

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	mcp "github.com/daltoniam/switchboard"
	"github.com/daltoniam/switchboard/compact"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/clientcredentials"
)

//go:embed compact.yaml
var compactYAML []byte

var compactResult = compact.MustLoadWithOverlay("tailscale", compactYAML, compact.Options{Strict: false})
var fieldCompactionSpecs = compactResult.Specs
var maxBytesByTool = compactResult.MaxBytes

const (
	maxResponseSize    = 10 * 1024 * 1024 // 10 MB
	defaultBaseURL     = "https://api.tailscale.com"
	apiPrefix          = "/api/v2"
	defaultTailnet     = "-"
	requestTimeout     = 30 * time.Second
	contentTypeJSON    = "application/json"
	contentTypeHuJSON  = "application/hujson"
	emptySuccessResult = `{"status":"success"}`
)

type tailscale struct {
	client       *http.Client
	httpClient   *http.Client
	apiKey       string
	baseURL      string
	tailnet      string
	configureErr error
}

var (
	_ mcp.Integration                = (*tailscale)(nil)
	_ mcp.FieldCompactionIntegration = (*tailscale)(nil)
	_ mcp.ToolMaxBytesIntegration    = (*tailscale)(nil)
	_ mcp.PlainTextCredentials       = (*tailscale)(nil)
	_ mcp.PlaceholderHints           = (*tailscale)(nil)
	_ mcp.OptionalCredentials        = (*tailscale)(nil)
)

func New() mcp.Integration {
	return &tailscale{client: &http.Client{Timeout: requestTimeout}}
}

func (t *tailscale) Name() string { return "tailscale" }

func (t *tailscale) PlainTextKeys() []string {
	return []string{"oauth_client_id", "tailnet", "base_url"}
}

func (t *tailscale) Placeholders() map[string]string {
	return map[string]string{
		"api_key":             "API access token (tskey-api-...). Leave blank when using an OAuth client",
		"oauth_client_id":     "OAuth client ID (use with oauth_client_secret instead of api_key)",
		"oauth_client_secret": "OAuth client secret (tskey-client-...)",
		"tailnet":             "Tailnet ID or organization name (default: - for the credential's own tailnet)",
		"base_url":            "https://api.tailscale.com (default)",
	}
}

func (t *tailscale) OptionalKeys() []string {
	return []string{"api_key", "oauth_client_id", "oauth_client_secret", "tailnet", "base_url"}
}

// Configure records its error so Execute can tell the caller why the
// integration is unusable. Hosts often only log Configure failures, which
// leaves an agent with no way to learn that, for example, a device auth key
// was pasted where an API token belongs.
func (t *tailscale) Configure(ctx context.Context, creds mcp.Credentials) error {
	t.configureErr = t.configure(ctx, creds)
	return t.configureErr
}

func (t *tailscale) configure(_ context.Context, creds mcp.Credentials) error {
	apiKey := strings.TrimSpace(creds["api_key"])
	clientID := strings.TrimSpace(creds["oauth_client_id"])
	clientSecret := strings.TrimSpace(creds["oauth_client_secret"])

	t.baseURL = defaultBaseURL
	if v := strings.TrimSpace(creds["base_url"]); v != "" {
		t.baseURL = strings.TrimRight(v, "/")
	}
	t.tailnet = defaultTailnet
	if v := strings.TrimSpace(creds["tailnet"]); v != "" {
		t.tailnet = v
	}
	if t.client == nil {
		t.client = &http.Client{Timeout: requestTimeout}
	}
	t.apiKey = ""
	t.httpClient = nil

	switch {
	case apiKey != "" && (clientID != "" || clientSecret != ""):
		return fmt.Errorf("tailscale: set either api_key or oauth_client_id + oauth_client_secret, not both")
	case apiKey != "":
		if strings.HasPrefix(apiKey, "tskey-auth-") {
			return fmt.Errorf("tailscale: api_key looks like a device auth key; auth keys join devices and cannot call the API, use an API access token (tskey-api-...)")
		}
		if strings.HasPrefix(apiKey, "tskey-client-") {
			return fmt.Errorf("tailscale: api_key looks like an OAuth client secret; put it in oauth_client_secret along with oauth_client_id")
		}
		t.apiKey = apiKey
		t.httpClient = t.client
	case clientID != "" && clientSecret != "":
		cfg := &clientcredentials.Config{
			ClientID:     clientID,
			ClientSecret: clientSecret,
			TokenURL:     t.baseURL + apiPrefix + "/oauth/token",
		}
		tokenCtx := context.WithValue(context.Background(), oauth2.HTTPClient, t.client)
		oc := cfg.Client(tokenCtx)
		oc.Timeout = requestTimeout
		t.httpClient = oc
	case clientID != "" || clientSecret != "":
		return fmt.Errorf("tailscale: oauth_client_id and oauth_client_secret must both be set")
	default:
		return fmt.Errorf("tailscale: api_key or oauth_client_id + oauth_client_secret is required")
	}
	return nil
}

func (t *tailscale) Healthy(ctx context.Context) bool {
	if t.httpClient == nil {
		return false
	}
	_, err := t.doJSON(ctx, http.MethodGet, t.tailnetPath("/devices"), nil)
	return err == nil
}

func (t *tailscale) Tools() []mcp.ToolDefinition { return tools }

func (t *tailscale) CompactSpec(toolName mcp.ToolName) ([]mcp.CompactField, bool) {
	fields, ok := fieldCompactionSpecs[toolName]
	return fields, ok
}

func (t *tailscale) MaxBytes(toolName mcp.ToolName) (int, bool) {
	n, ok := maxBytesByTool[toolName]
	return n, ok
}

func (t *tailscale) Execute(ctx context.Context, toolName mcp.ToolName, args map[string]any) (*mcp.ToolResult, error) {
	fn, ok := dispatch[toolName]
	if !ok {
		return &mcp.ToolResult{Data: fmt.Sprintf("unknown tool: %s", toolName), IsError: true}, nil
	}
	if t.httpClient == nil {
		if t.configureErr != nil {
			return mcp.ErrResult(fmt.Errorf("tailscale: integration is not configured: %w", t.configureErr))
		}
		return mcp.ErrResult(fmt.Errorf("tailscale: integration is not configured"))
	}
	return fn(ctx, t, args)
}

func (t *tailscale) tailnetPath(suffix string) string {
	return "/tailnet/" + url.PathEscape(t.tailnet) + suffix
}

type requestOptions struct {
	contentType string
	accept      string
	ifMatch     string
}

// do performs a single API request and returns the raw body and response
// headers. Callers that need non-JSON content (HuJSON policy files) or
// response headers (policy ETag) use this directly; everything else goes
// through doJSON.
func (t *tailscale) do(ctx context.Context, method, path string, body []byte, opts requestOptions) ([]byte, http.Header, error) {
	var bodyReader io.Reader
	if body != nil {
		bodyReader = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, t.baseURL+apiPrefix+path, bodyReader)
	if err != nil {
		return nil, nil, err
	}
	if t.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+t.apiKey)
	}
	accept := opts.accept
	if accept == "" {
		accept = contentTypeJSON
	}
	req.Header.Set("Accept", accept)
	if body != nil {
		ct := opts.contentType
		if ct == "" {
			ct = contentTypeJSON
		}
		req.Header.Set("Content-Type", ct)
	}
	if opts.ifMatch != "" {
		req.Header.Set("If-Match", opts.ifMatch)
	}

	resp, err := t.httpClient.Do(req)
	if err != nil {
		return nil, nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	data, err := readLimited(resp.Body, maxResponseSize)
	if err != nil {
		return nil, nil, err
	}
	if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
		re := &mcp.RetryableError{StatusCode: resp.StatusCode, Err: fmt.Errorf("tailscale API error (%d): %s", resp.StatusCode, string(data))}
		re.RetryAfter = mcp.ParseRetryAfter(resp.Header.Get("Retry-After"))
		return nil, nil, re
	}
	if resp.StatusCode >= 400 {
		return nil, nil, fmt.Errorf("tailscale API error (%d): %s", resp.StatusCode, string(data))
	}
	return data, resp.Header, nil
}

// readLimited reads up to limit bytes and fails if the body is longer, so a
// truncated payload (device list, policy file) is never mistaken for a
// complete response and, for policy files, written back over the original.
func readLimited(r io.Reader, limit int64) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(r, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, fmt.Errorf("tailscale: response body exceeds %d bytes", limit)
	}
	return data, nil
}

func (t *tailscale) doJSON(ctx context.Context, method, path string, body any) (json.RawMessage, error) {
	var payload []byte
	if body != nil {
		var err error
		payload, err = json.Marshal(body)
		if err != nil {
			return nil, err
		}
	}
	data, _, err := t.do(ctx, method, path, payload, requestOptions{})
	if err != nil {
		return nil, err
	}
	if len(bytes.TrimSpace(data)) == 0 {
		return json.RawMessage(emptySuccessResult), nil
	}
	return json.RawMessage(data), nil
}

type handlerFunc func(ctx context.Context, t *tailscale, args map[string]any) (*mcp.ToolResult, error)

func requireStr(name, value string) error {
	if value == "" {
		return fmt.Errorf("%s is required", name)
	}
	return nil
}

func requirePresent(args map[string]any, name string) error {
	if v, ok := args[name]; !ok || v == nil {
		return fmt.Errorf("%s is required", name)
	}
	return nil
}

func rawResult(data json.RawMessage, err error) (*mcp.ToolResult, error) {
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

var dispatch = map[mcp.ToolName]handlerFunc{
	// Devices
	mcp.ToolName("tailscale_list_devices"):                    listDevices,
	mcp.ToolName("tailscale_get_device"):                      getDevice,
	mcp.ToolName("tailscale_authorize_device"):                authorizeDevice,
	mcp.ToolName("tailscale_delete_device"):                   deleteDevice,
	mcp.ToolName("tailscale_set_device_name"):                 setDeviceName,
	mcp.ToolName("tailscale_set_device_tags"):                 setDeviceTags,
	mcp.ToolName("tailscale_set_device_key_expiry"):           setDeviceKeyExpiry,
	mcp.ToolName("tailscale_expire_device_key"):               expireDeviceKey,
	mcp.ToolName("tailscale_set_device_ip"):                   setDeviceIP,
	mcp.ToolName("tailscale_get_device_routes"):               getDeviceRoutes,
	mcp.ToolName("tailscale_set_device_routes"):               setDeviceRoutes,
	mcp.ToolName("tailscale_get_device_posture_attributes"):   getDevicePostureAttributes,
	mcp.ToolName("tailscale_set_device_posture_attribute"):    setDevicePostureAttribute,
	mcp.ToolName("tailscale_delete_device_posture_attribute"): deleteDevicePostureAttribute,

	// Keys
	mcp.ToolName("tailscale_list_keys"):       listKeys,
	mcp.ToolName("tailscale_get_key"):         getKey,
	mcp.ToolName("tailscale_create_auth_key"): createAuthKey,
	mcp.ToolName("tailscale_delete_key"):      deleteKey,

	// Policy file (ACLs)
	mcp.ToolName("tailscale_get_policy_file"):      getPolicyFile,
	mcp.ToolName("tailscale_validate_policy_file"): validatePolicyFile,
	mcp.ToolName("tailscale_set_policy_file"):      setPolicyFile,
	mcp.ToolName("tailscale_preview_policy_rules"): previewPolicyRules,

	// DNS
	mcp.ToolName("tailscale_get_dns"):              getDNS,
	mcp.ToolName("tailscale_set_dns_nameservers"):  setDNSNameservers,
	mcp.ToolName("tailscale_set_dns_preferences"):  setDNSPreferences,
	mcp.ToolName("tailscale_set_dns_search_paths"): setDNSSearchPaths,
	mcp.ToolName("tailscale_update_split_dns"):     updateSplitDNS,

	// Users
	mcp.ToolName("tailscale_list_users"):         listUsers,
	mcp.ToolName("tailscale_get_user"):           getUser,
	mcp.ToolName("tailscale_update_user_role"):   updateUserRole,
	mcp.ToolName("tailscale_approve_user"):       approveUser,
	mcp.ToolName("tailscale_suspend_user"):       suspendUser,
	mcp.ToolName("tailscale_restore_user"):       restoreUser,
	mcp.ToolName("tailscale_delete_user"):        deleteUser,
	mcp.ToolName("tailscale_list_user_invites"):  listUserInvites,
	mcp.ToolName("tailscale_create_user_invite"): createUserInvite,
	mcp.ToolName("tailscale_delete_user_invite"): deleteUserInvite,

	// Tailnet settings, webhooks, contacts
	mcp.ToolName("tailscale_get_tailnet_settings"):    getTailnetSettings,
	mcp.ToolName("tailscale_update_tailnet_settings"): updateTailnetSettings,
	mcp.ToolName("tailscale_list_webhooks"):           listWebhooks,
	mcp.ToolName("tailscale_create_webhook"):          createWebhook,
	mcp.ToolName("tailscale_delete_webhook"):          deleteWebhook,
	mcp.ToolName("tailscale_test_webhook"):            testWebhook,
	mcp.ToolName("tailscale_get_contacts"):            getContacts,
}
