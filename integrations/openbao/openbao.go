package openbao

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	mcp "github.com/daltoniam/switchboard"
	"github.com/daltoniam/switchboard/compact"
)

//go:embed compact.yaml
var compactYAML []byte

var fieldCompactionSpecs = compact.MustLoadWithOverlay("openbao", compactYAML, compact.Options{Strict: false}).Specs

type openbao struct {
	baseURL string
	token   string
	mount   string
	prefix  string
	client  *http.Client
}

var (
	_ mcp.Integration                = (*openbao)(nil)
	_ mcp.FieldCompactionIntegration = (*openbao)(nil)
	_ mcp.PlainTextCredentials       = (*openbao)(nil)
	_ mcp.PlaceholderHints           = (*openbao)(nil)
	_ mcp.OptionalCredentials        = (*openbao)(nil)
)

func New() mcp.Integration      { return &openbao{} }
func (o *openbao) Name() string { return "openbao" }
func (o *openbao) PlainTextKeys() []string {
	return []string{"base_url", "mount", "path_prefix", "ca_cert"}
}
func (o *openbao) Close() error {
	if o.client != nil {
		o.client.CloseIdleConnections()
		o.client = nil
	}
	o.token = ""
	return nil
}

func (o *openbao) OptionalKeys() []string { return []string{"ca_cert"} }
func (o *openbao) Placeholders() map[string]string {
	return map[string]string{
		"base_url":    "https://bao.example.com (HTTPS OpenBao 2.7.2 server)",
		"token":       "Short-lived metadata-only token, never root or a secret-value reader",
		"mount":       "secret (one KV v2 mount)",
		"path_prefix": "approved/app (required narrow prefix; key names are visible to the model)",
		"ca_cert":     "Optional PEM CA certificate for a private HTTPS server",
	}
}

func validPath(value string, allowEmpty bool) bool {
	if value == "" {
		return allowEmpty
	}
	if !utf8.ValidString(value) || len(value) > 512 || strings.ContainsAny(value, "%\\?#*") || strings.IndexFunc(value, func(char rune) bool { return unicode.IsControl(char) || unicode.IsSpace(char) }) >= 0 {
		return false
	}
	for _, segment := range strings.Split(value, "/") {
		if segment == "" || segment == "." || segment == ".." {
			return false
		}
	}
	return true
}

func (o *openbao) Configure(_ context.Context, creds mcp.Credentials) error {
	if o.client != nil {
		o.client.CloseIdleConnections()
	}
	*o = openbao{}
	base, err := url.Parse(creds["base_url"])
	if err != nil || base.Scheme != "https" || base.Host == "" || base.User != nil || base.RawQuery != "" || base.ForceQuery || base.Fragment != "" || base.RawPath != "" || !validPath(strings.Trim(base.Path, "/"), true) {
		return errors.New("openbao: base_url must be a trusted HTTPS URL without credentials, query, fragment, or encoded/traversal paths")
	}
	token := creds["token"]
	if token == "" || strings.IndexFunc(token, func(char rune) bool { return unicode.IsControl(char) || unicode.IsSpace(char) }) >= 0 {
		return errors.New("openbao: token is required and must not contain whitespace")
	}
	mount, prefix := creds["mount"], creds["path_prefix"]
	if !validPath(mount, false) || strings.Contains(mount, "/") {
		return errors.New("openbao: mount must be one literal KV v2 mount name")
	}
	if !validPath(prefix, false) {
		return errors.New("openbao: path_prefix must be a nonempty narrow relative path without traversal")
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig = &tls.Config{MinVersion: tls.VersionTLS12}
	if certificate := creds["ca_cert"]; certificate != "" {
		roots, rootErr := x509.SystemCertPool()
		if rootErr != nil {
			roots = x509.NewCertPool()
		}
		if !roots.AppendCertsFromPEM([]byte(strings.ReplaceAll(certificate, `\n`, "\n"))) {
			return errors.New("openbao: ca_cert must contain a valid PEM CA certificate")
		}
		transport.TLSClientConfig.RootCAs = roots
	}
	o.baseURL = strings.TrimRight(base.String(), "/")
	o.token, o.mount, o.prefix = token, mount, prefix
	o.client = &http.Client{Transport: transport, Timeout: 10 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	return nil
}

func (o *openbao) Healthy(ctx context.Context) bool {
	if o.client == nil {
		return false
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	var mount struct {
		Data *struct {
			Type    string            `json:"type"`
			Path    string            `json:"path"`
			Options map[string]string `json:"options"`
		} `json:"data"`
	}
	if o.request(ctx, "GET", "/v1/sys/internal/ui/mounts/"+o.mount+"/metadata/"+o.prefix+"/", nil, false, &mount) != nil || mount.Data == nil || mount.Data.Type != "kv" || mount.Data.Path != o.mount+"/" || mount.Data.Options["version"] != "2" {
		return false
	}
	var listing struct {
		Data *struct {
			Keys []string `json:"keys"`
		} `json:"data"`
	}
	return o.request(ctx, "LIST", "/v1/"+o.mount+"/metadata/"+o.prefix+"/", url.Values{"limit": {"1"}}, false, &listing) == nil && listing.Data != nil
}

var tools = []mcp.ToolDefinition{
	{Name: "openbao_list_secret_keys", Description: "List secret key names and folders in the configured OpenBao KV v2 prefix, never secret values. Start here for deployment secret existence and metadata diagnostics. Names themselves may be sensitive. One bounded page; use next_after with the same path to continue.", Parameters: map[string]string{"path": "Optional folder relative to the configured narrow prefix; no recursion", "limit": "Keys per page (integer 1-100), default 20", "after": "Continuation key from next_after, for the same folder"}, Access: mcp.AccessRead},
	{Name: "openbao_get_secret_metadata", Description: "Get OpenBao KV v2 secret version numbers, timestamps, deletion and destroyed state for a key. Use after list_secret_keys. Never returns secret values, custom_metadata or arbitrary annotations.", Parameters: map[string]string{"path": "Required key path relative to the configured narrow prefix"}, Required: []string{"path"}, Access: mcp.AccessRead},
	{Name: "openbao_get_health", Description: "Get OpenBao sealed, initialized and standby health state and server version. Diagnostic non-2xx health statuses are returned as states; this unauthenticated endpoint does not verify token permissions.", Access: mcp.AccessRead},
}
var dispatch = map[mcp.ToolName]func(context.Context, *openbao, map[string]any) (*mcp.ToolResult, error){
	"openbao_list_secret_keys":    listKeys,
	"openbao_get_secret_metadata": getMetadata,
	"openbao_get_health":          getHealth,
}

func (o *openbao) Tools() []mcp.ToolDefinition { return tools }
func (o *openbao) CompactSpec(name mcp.ToolName) ([]mcp.CompactField, bool) {
	fields, ok := fieldCompactionSpecs[name]
	return fields, ok
}
func (o *openbao) Execute(ctx context.Context, name mcp.ToolName, args map[string]any) (*mcp.ToolResult, error) {
	handler, ok := dispatch[name]
	if !ok {
		return mcp.ErrResult(errors.New("openbao: unknown tool"))
	}
	if o.client == nil {
		return mcp.ErrResult(errors.New("openbao: integration is not configured"))
	}
	for key := range args {
		allowed := false
		for _, tool := range tools {
			if tool.Name == name {
				_, allowed = tool.Parameters[key]
				break
			}
		}
		if !allowed {
			return mcp.ErrResult(errors.New("openbao: unsupported argument"))
		}
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	return handler(ctx, o, args)
}

func (o *openbao) request(ctx context.Context, method, route string, query url.Values, health bool, out any) error {
	endpoint := o.baseURL + route
	if len(query) > 0 {
		endpoint += "?" + query.Encode()
	}
	request, err := http.NewRequestWithContext(ctx, method, endpoint, nil)
	if err != nil {
		return errors.New("openbao: cannot construct request")
	}
	request.Header.Set("Accept", "application/json")
	if !health {
		request.Header.Set("X-Vault-Token", o.token)
	}
	response, err := o.client.Do(request)
	if err != nil {
		return errors.New("openbao: HTTPS request failed; verify connectivity, CA certificate, hostname and deadline")
	}
	defer func() { _ = response.Body.Close() }()
	data, err := io.ReadAll(io.LimitReader(response.Body, 1024*1024+1))
	if err != nil {
		return errors.New("openbao: cannot read response")
	}
	if len(data) > 1024*1024 {
		return errors.New("openbao: response exceeds 1 MiB; narrow the request")
	}
	status := response.StatusCode
	healthState := health && (status == 429 || status == 501 || status == 503)
	if (status < 200 || status >= 300) && !healthState {
		apiErr := fmt.Errorf("openbao: HTTP %d; verify token permissions, prefix, mount and server state", status)
		if status == 429 || status >= 500 {
			return &mcp.RetryableError{StatusCode: status, RetryAfter: mcp.ParseRetryAfter(response.Header.Get("Retry-After")), Err: apiErr}
		}
		return apiErr
	}
	if err = json.Unmarshal(data, out); err != nil {
		return errors.New("openbao: malformed JSON response")
	}
	return nil
}

func listKeys(ctx context.Context, o *openbao, args map[string]any) (*mcp.ToolResult, error) {
	reader := mcp.NewArgs(args)
	path, after, limit := reader.Str("path"), reader.Str("after"), reader.OptInt("limit", 20)
	if _, present := args["limit"]; present {
		limit = reader.Int("limit")
	}
	if err := reader.Err(); err != nil {
		return mcp.ErrResult(errors.New("openbao: invalid list arguments"))
	}
	if number, ok := args["limit"].(float64); ok && (math.IsNaN(number) || math.IsInf(number, 0) || number != math.Trunc(number)) {
		return mcp.ErrResult(errors.New("openbao: limit must be an integer"))
	}
	if !validPath(path, true) || !validPath(strings.TrimSuffix(after, "/"), true) || strings.Contains(strings.TrimSuffix(after, "/"), "/") || limit < 1 || limit > 100 {
		return mcp.ErrResult(errors.New("openbao: invalid folder, continuation key, or limit (1-100)"))
	}
	folder := o.prefix + "/"
	if path != "" {
		folder += path + "/"
	}
	var response struct {
		Data *struct {
			Keys []string `json:"keys"`
		} `json:"data"`
	}
	if err := o.request(ctx, "LIST", "/v1/"+o.mount+"/metadata/"+folder, url.Values{"limit": {strconv.Itoa(limit + 1)}, "after": {after}}, false, &response); err != nil {
		return mcp.ErrResult(err)
	}
	if response.Data == nil {
		return mcp.ErrResult(errors.New("openbao: missing list data"))
	}
	keys := response.Data.Keys
	if keys == nil {
		keys = []string{}
	}
	for index, key := range keys {
		literal := strings.TrimSuffix(key, "/")
		if !validPath(literal, false) || strings.Contains(literal, "/") || key <= after || (index > 0 && keys[index-1] >= key) {
			return mcp.ErrResult(errors.New("openbao: invalid or unordered key listing"))
		}
	}
	if len(keys) > limit+1 {
		return mcp.ErrResult(errors.New("openbao: server ignored bounded list pagination"))
	}
	more := len(keys) > limit
	result := struct {
		Path      string   `json:"path"`
		Keys      []string `json:"keys"`
		HasMore   bool     `json:"has_more"`
		NextAfter string   `json:"next_after"`
	}{Path: folder, Keys: keys, HasMore: more}
	if more {
		result.Keys = keys[:limit]
		result.NextAfter = result.Keys[limit-1]
	}
	return mcp.JSONResult(result)
}

type version struct {
	Version      uint64  `json:"version"`
	CreatedTime  *string `json:"created_time,omitempty"`
	DeletionTime *string `json:"deletion_time,omitempty"`
	Destroyed    *bool   `json:"destroyed,omitempty"`
}
type metadata struct {
	CurrentVersion *uint64            `json:"current_version,omitempty"`
	OldestVersion  *uint64            `json:"oldest_version,omitempty"`
	CreatedTime    *string            `json:"created_time,omitempty"`
	UpdatedTime    *string            `json:"updated_time,omitempty"`
	Versions       map[string]version `json:"versions"`
}

func getMetadata(ctx context.Context, o *openbao, args map[string]any) (*mcp.ToolResult, error) {
	reader := mcp.NewArgs(args)
	path := reader.Str("path")
	if err := reader.Err(); err != nil {
		return mcp.ErrResult(errors.New("openbao: invalid key path"))
	}
	if !validPath(path, false) {
		return mcp.ErrResult(errors.New("openbao: path must be a literal relative key path"))
	}
	var response struct {
		Data *metadata `json:"data"`
	}
	if err := o.request(ctx, "GET", "/v1/"+o.mount+"/metadata/"+o.prefix+"/"+path, nil, false, &response); err != nil {
		return mcp.ErrResult(err)
	}
	if response.Data == nil || response.Data.CurrentVersion == nil || response.Data.Versions == nil {
		return mcp.ErrResult(errors.New("openbao: missing KV v2 metadata"))
	}
	if len(response.Data.Versions) > 1000 {
		return mcp.ErrResult(errors.New("openbao: metadata exceeds 1000 versions"))
	}
	versions := make([]version, 0, len(response.Data.Versions))
	for key, item := range response.Data.Versions {
		number, err := strconv.ParseUint(key, 10, 64)
		if err != nil || number == 0 {
			return mcp.ErrResult(errors.New("openbao: invalid metadata version number"))
		}
		item.Version = number
		versions = append(versions, item)
	}
	sort.Slice(versions, func(left, right int) bool { return versions[left].Version < versions[right].Version })
	return mcp.JSONResult(struct {
		Path           string    `json:"path"`
		CurrentVersion *uint64   `json:"current_version,omitempty"`
		OldestVersion  *uint64   `json:"oldest_version,omitempty"`
		CreatedTime    *string   `json:"created_time,omitempty"`
		UpdatedTime    *string   `json:"updated_time,omitempty"`
		Versions       []version `json:"versions"`
	}{o.prefix + "/" + path, response.Data.CurrentVersion, response.Data.OldestVersion, response.Data.CreatedTime, response.Data.UpdatedTime, versions})
}
func getHealth(ctx context.Context, o *openbao, _ map[string]any) (*mcp.ToolResult, error) {
	var response struct {
		Initialized *bool   `json:"initialized"`
		Sealed      *bool   `json:"sealed"`
		Standby     *bool   `json:"standby"`
		Version     *string `json:"version,omitempty"`
	}
	if err := o.request(ctx, "GET", "/v1/sys/health", nil, true, &response); err != nil {
		return mcp.ErrResult(err)
	}
	if response.Initialized == nil || response.Sealed == nil || response.Standby == nil {
		return mcp.ErrResult(errors.New("openbao: missing health state"))
	}
	return mcp.JSONResult(response)
}
