package wazuh

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	_ "embed"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"

	mcp "github.com/daltoniam/switchboard"
	"github.com/daltoniam/switchboard/compact"
)

//go:embed compact.yaml
var compactYAML []byte

var fieldCompactionSpecs = compact.MustLoadWithOverlay("wazuh", compactYAML, compact.Options{Strict: false}).Specs

var (
	_              mcp.Integration                = (*wazuh)(nil)
	_              mcp.FieldCompactionIntegration = (*wazuh)(nil)
	_              mcp.PlainTextCredentials       = (*wazuh)(nil)
	_              mcp.OptionalCredentials        = (*wazuh)(nil)
	_              mcp.PlaceholderHints           = (*wazuh)(nil)
	_              io.Closer                      = (*wazuh)(nil)
	agentIDPattern                                = regexp.MustCompile(`^[0-9]{3,8}$`)
)

const maxResponseBytes = 2 * 1024 * 1024
const selectedFields = "id,name,status,version,os.name,os.version,lastKeepAlive,group_config_status"

type wazuh struct {
	mu                                 sync.Mutex
	client                             *http.Client
	baseURL, username, password, token string
	expires                            time.Time
}

func New() mcp.Integration                     { return &wazuh{} }
func (adapter *wazuh) Name() string            { return "wazuh" }
func (adapter *wazuh) PlainTextKeys() []string { return []string{"base_url", "username", "ca_cert"} }
func (adapter *wazuh) OptionalKeys() []string  { return []string{"ca_cert"} }
func (adapter *wazuh) Placeholders() map[string]string {
	return map[string]string{
		"base_url": "Trusted HTTPS Wazuh manager URL, normally port 55000",
		"username": "Dedicated non-admin reader with scoped agent:read, RBAC white mode, allow_run_as=false",
		"password": "Dedicated reader password, never a default administrator credential",
		"ca_cert":  `Optional PEM CA certificate, literal \n accepted; TLS verification remains enabled`,
	}
}

func safeText(value string, limit int) bool {
	return utf8.ValidString(value) && len(value) <= limit && !strings.ContainsFunc(value, unicode.IsControl)
}

func (adapter *wazuh) clear() {
	if adapter.client != nil {
		adapter.client.CloseIdleConnections()
	}
	adapter.client, adapter.baseURL, adapter.username, adapter.password, adapter.token = nil, "", "", "", ""
	adapter.expires = time.Time{}
}

func (adapter *wazuh) Close() error {
	adapter.mu.Lock()
	defer adapter.mu.Unlock()
	adapter.clear()
	return nil
}

func (adapter *wazuh) Configure(_ context.Context, creds mcp.Credentials) error {
	adapter.mu.Lock()
	defer adapter.mu.Unlock()
	adapter.clear()
	raw := creds["base_url"]
	endpoint, err := url.Parse(raw)
	if err != nil || endpoint == nil || endpoint.Scheme != "https" || endpoint.Hostname() == "" || endpoint.User != nil || endpoint.Opaque != "" || strings.ContainsAny(raw, "?#%\\") || strings.ContainsFunc(raw, unicode.IsSpace) || !safeText(raw, 2048) {
		return fmt.Errorf("wazuh: base_url must be a trusted HTTPS manager URL without credentials, query, fragment or encoded paths")
	}
	if strings.HasSuffix(endpoint.Host, ":") {
		return fmt.Errorf("wazuh: invalid base_url port")
	}
	if port := endpoint.Port(); port != "" {
		parsed, err := strconv.Atoi(port)
		if err != nil || parsed < 1 || parsed > 65535 {
			return fmt.Errorf("wazuh: invalid base_url port")
		}
	}
	for _, segment := range strings.Split(endpoint.Path, "/") {
		if segment == "." || segment == ".." {
			return fmt.Errorf("wazuh: invalid installation prefix")
		}
	}
	username, password := creds["username"], creds["password"]
	if username == "" || !safeText(username, 256) || strings.TrimSpace(username) != username || strings.Contains(username, ":") || slices.Contains([]string{"wazuh", "wazuh-wui", "admin", "administrator"}, strings.ToLower(username)) {
		return fmt.Errorf("wazuh: username must identify a dedicated non-admin reader, not a default administrator")
	}
	if password == "" || !safeText(password, 4096) {
		return fmt.Errorf("wazuh: password is required without control characters")
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig = &tls.Config{MinVersion: tls.VersionTLS12}
	if cert := creds["ca_cert"]; cert != "" {
		pool, err := x509.SystemCertPool()
		if err != nil {
			pool = x509.NewCertPool()
		}
		if !pool.AppendCertsFromPEM([]byte(strings.ReplaceAll(cert, `\n`, "\n"))) {
			return fmt.Errorf("wazuh: ca_cert must contain a PEM CA certificate")
		}
		transport.TLSClientConfig.RootCAs = pool
	}
	adapter.client = &http.Client{Transport: transport, Timeout: 15 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	adapter.baseURL, adapter.username, adapter.password = strings.TrimRight(endpoint.String(), "/"), username, password
	return nil
}

var tools = []mcp.ToolDefinition{
	{Name: "wazuh_list_agents", Access: mcp.AccessRead, Description: "List Wazuh manager endpoint security agents and connection health. Start here to find disconnected agents, missing endpoint coverage and configuration synchronization failures. One RBAC-scoped page, default 25 and maximum 100, with next_offset. Agent inventory only, not indexer alerts or vulnerability search.", Parameters: map[string]string{"search": "Optional agent search text, at most 256 bytes", "status": "Optional connection status: active, disconnected, never_connected or pending", "limit": "Page size, integer 1..100, default 25", "offset": "Zero-based offset from next_offset, default 0; retain the same filters"}},
	{Name: "wazuh_get_agent", Access: mcp.AccessRead, Description: "Get one Wazuh manager endpoint agent by exact agent_id from wazuh_list_agents. Inspect connection status, lastKeepAlive, OS version and group_config_status to diagnose missing check-ins or configuration sync. Missing and forbidden agents return an error. No IPs, raw logs, keys, alerts or active response.", Parameters: map[string]string{"agent_id": "Exact decimal agent id copied from wazuh_list_agents, including leading zeros"}, Required: []string{"agent_id"}},
	{Name: "wazuh_get_agent_status_summary", Access: mcp.AccessRead, Description: "Get Wazuh manager connection and configuration synchronization totals for agents visible to the reader's RBAC role. Use after wazuh_list_agents for active, disconnected, never_connected, pending, synced and not_synced counts. Excludes the manager agent 000; not global fleet totals or indexer alert counts."},
}

type handler func(context.Context, *wazuh, map[string]any) (*mcp.ToolResult, error)

var dispatch = map[mcp.ToolName]handler{"wazuh_list_agents": listAgents, "wazuh_get_agent": getAgent, "wazuh_get_agent_status_summary": statusSummary}

func (adapter *wazuh) Tools() []mcp.ToolDefinition { return slices.Clone(tools) }
func (adapter *wazuh) CompactSpec(name mcp.ToolName) ([]mcp.CompactField, bool) {
	fields, ok := fieldCompactionSpecs[name]
	return fields, ok
}

func (adapter *wazuh) Execute(ctx context.Context, name mcp.ToolName, args map[string]any) (*mcp.ToolResult, error) {
	fn, ok := dispatch[name]
	if !ok {
		return mcp.ErrResult(fmt.Errorf("wazuh: unknown tool: %s", name))
	}
	for _, tool := range tools {
		if tool.Name != name {
			continue
		}
		for key := range args {
			if _, ok := tool.Parameters[key]; !ok {
				return mcp.ErrResult(fmt.Errorf("wazuh: unsupported argument"))
			}
		}
	}
	adapter.mu.Lock()
	defer adapter.mu.Unlock()
	if adapter.client == nil {
		return mcp.ErrResult(mcp.ErrNotConfigured)
	}
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	return fn(ctx, adapter, args)
}

func (adapter *wazuh) Healthy(ctx context.Context) bool {
	result, err := adapter.Execute(ctx, "wazuh_list_agents", map[string]any{"limit": 1})
	return err == nil && result != nil && !result.IsError
}

type envelope struct {
	Error *int            `json:"error"`
	Data  json.RawMessage `json:"data"`
}

func (adapter *wazuh) exchange(ctx context.Context, method, path string, query url.Values, login bool) ([]byte, int, error) {
	for attempt := range 3 {
		request, err := http.NewRequestWithContext(ctx, method, adapter.baseURL+path+"?"+query.Encode(), nil)
		if err != nil {
			return nil, 0, fmt.Errorf("wazuh: cannot construct request")
		}
		request.Header.Set("Accept", "application/json")
		if login {
			request.SetBasicAuth(adapter.username, adapter.password)
		} else {
			request.Header.Set("Authorization", "Bearer "+adapter.token)
		}
		response, err := adapter.client.Do(request)
		if err != nil {
			return nil, 0, fmt.Errorf("wazuh: request failed; check connectivity, TLS trust and deadline")
		}
		if response.StatusCode == http.StatusTooManyRequests {
			retryAfter := response.Header.Get("Retry-After")
			_ = response.Body.Close()
			delay, allowed := retryDelay(retryAfter, attempt)
			if !allowed {
				return nil, 429, fmt.Errorf("wazuh: rate limited; retry later")
			}
			timer := time.NewTimer(delay)
			select {
			case <-ctx.Done():
				timer.Stop()
				return nil, 429, fmt.Errorf("wazuh: request deadline exceeded")
			case <-timer.C:
			}
			continue
		}
		if response.StatusCode != http.StatusOK {
			_ = response.Body.Close()
			return nil, response.StatusCode, fmt.Errorf("wazuh: HTTP %d; verify dedicated reader credentials and scoped agent:read permissions", response.StatusCode)
		}
		data, err := io.ReadAll(io.LimitReader(response.Body, maxResponseBytes+1))
		_ = response.Body.Close()
		if err != nil {
			return nil, 200, fmt.Errorf("wazuh: cannot read response within deadline")
		}
		if len(data) > maxResponseBytes {
			return nil, 200, fmt.Errorf("wazuh: response exceeds 2 MiB limit")
		}
		return data, 200, nil
	}
	return nil, 429, fmt.Errorf("wazuh: rate limited; retry later")
}

func retryDelay(retryAfter string, attempt int) (time.Duration, bool) {
	if attempt == 2 {
		return 0, false
	}
	if retryAfter == "" {
		return time.Duration(attempt+1) * 200 * time.Millisecond, true
	}
	if seconds, err := strconv.Atoi(retryAfter); err == nil && seconds >= 0 {
		if seconds > 2 {
			return 0, false
		}
		return time.Duration(seconds) * time.Second, true
	}
	when, err := http.ParseTime(retryAfter)
	if err != nil {
		return 0, false
	}
	delay := max(0, time.Until(when))
	return delay, delay <= 2*time.Second
}

func decodeEnvelope(data []byte) (*envelope, error) {
	var response envelope
	if json.Unmarshal(data, &response) != nil || response.Error == nil || len(response.Data) == 0 || string(response.Data) == "null" {
		return nil, fmt.Errorf("wazuh: malformed API response")
	}
	return &response, nil
}

func (adapter *wazuh) authenticate(ctx context.Context) error {
	data, _, err := adapter.exchange(ctx, http.MethodPost, "/security/user/authenticate", nil, true)
	if err != nil {
		return err
	}
	response, err := decodeEnvelope(data)
	if err != nil {
		return err
	}
	var auth struct {
		Token string `json:"token"`
	}
	if *response.Error != 0 || json.Unmarshal(response.Data, &auth) != nil || !safeText(auth.Token, 16384) {
		return fmt.Errorf("wazuh: authentication failed; verify the dedicated reader credentials")
	}
	parts := strings.Split(auth.Token, ".")
	if len(parts) != 3 {
		return fmt.Errorf("wazuh: invalid authentication token")
	}
	claims, err := base64.RawURLEncoding.DecodeString(parts[1])
	var payload struct {
		Exp int64 `json:"exp"`
	}
	if err != nil || json.Unmarshal(claims, &payload) != nil || payload.Exp <= time.Now().Unix() {
		return fmt.Errorf("wazuh: invalid or expired authentication token")
	}
	adapter.token, adapter.expires = auth.Token, time.Unix(payload.Exp, 0)
	return nil
}

func (adapter *wazuh) read(ctx context.Context, path string, query url.Values) (*envelope, error) {
	if adapter.token == "" || !time.Now().Add(10*time.Second).Before(adapter.expires) {
		if err := adapter.authenticate(ctx); err != nil {
			return nil, err
		}
	}
	data, status, err := adapter.exchange(ctx, http.MethodGet, path, query, false)
	if status == http.StatusUnauthorized {
		adapter.token, adapter.expires = "", time.Time{}
		if err := adapter.authenticate(ctx); err != nil {
			return nil, err
		}
		data, _, err = adapter.exchange(ctx, http.MethodGet, path, query, false)
	}
	if err != nil {
		return nil, err
	}
	return decodeEnvelope(data)
}

type agent struct {
	ID      string  `json:"id"`
	Name    *string `json:"name,omitempty"`
	Status  *string `json:"status,omitempty"`
	Version *string `json:"version,omitempty"`
	OS      *struct {
		Name    *string `json:"name,omitempty"`
		Version *string `json:"version,omitempty"`
	} `json:"os,omitempty"`
	LastKeepAlive     *string `json:"lastKeepAlive,omitempty"`
	GroupConfigStatus *string `json:"group_config_status,omitempty"`
}

type itemFailure struct {
	ID    []string `json:"id"`
	Error struct {
		Code int `json:"code"`
	} `json:"error"`
}

type agentPage struct {
	Items       []agent       `json:"affected_items"`
	Total       *int          `json:"total_affected_items"`
	TotalFailed *int          `json:"total_failed_items"`
	Failures    []itemFailure `json:"failed_items"`
	Partial     bool          `json:"partial"`
	NextOffset  *int          `json:"next_offset"`
}

func listAgents(ctx context.Context, adapter *wazuh, args map[string]any) (*mcp.ToolResult, error) {
	reader := mcp.NewArgs(args)
	search, status := reader.Str("search"), reader.Str("status")
	limitText, offsetText := reader.Str("limit"), reader.Str("offset")
	if reader.Err() != nil {
		return mcp.ErrResult(fmt.Errorf("wazuh: invalid argument type"))
	}
	limit, offset := 25, 0
	var err error
	if limitText != "" {
		limit, err = strconv.Atoi(limitText)
		if err != nil {
			return mcp.ErrResult(fmt.Errorf("wazuh: limit must be an integer"))
		}
	}
	if offsetText != "" {
		offset, err = strconv.Atoi(offsetText)
		if err != nil {
			return mcp.ErrResult(fmt.Errorf("wazuh: offset must be an integer"))
		}
	}
	if limit < 1 || limit > 100 || offset < 0 || offset > 2147483647 || !safeText(search, 256) || !slices.Contains([]string{"", "active", "disconnected", "never_connected", "pending"}, status) {
		return mcp.ErrResult(fmt.Errorf("wazuh: invalid search, status or pagination bounds"))
	}
	query := url.Values{"limit": {strconv.Itoa(limit)}, "offset": {strconv.Itoa(offset)}, "select": {selectedFields}, "sort": {"id"}}
	if search != "" {
		query.Set("search", search)
	}
	if status != "" {
		query.Set("status", status)
	}
	page, err := adapter.page(ctx, query, limit, offset)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.JSONResult(page)
}

func getAgent(ctx context.Context, adapter *wazuh, args map[string]any) (*mcp.ToolResult, error) {
	reader := mcp.NewArgs(args)
	id := reader.Str("agent_id")
	if reader.Err() != nil || !agentIDPattern.MatchString(id) {
		return mcp.ErrResult(fmt.Errorf("wazuh: agent_id must be one exact decimal agent ID including leading zeros"))
	}
	page, err := adapter.page(ctx, url.Values{"agents_list": {id}, "limit": {"1"}, "select": {selectedFields}, "sort": {"id"}}, 1, 0)
	if err != nil {
		return mcp.ErrResult(err)
	}
	if len(page.Items) != 1 || page.Items[0].ID != id || *page.TotalFailed != 0 {
		return mcp.ErrResult(fmt.Errorf("wazuh: agent missing or forbidden for this reader"))
	}
	return mcp.JSONResult(page)
}

func (adapter *wazuh) page(ctx context.Context, query url.Values, limit, offset int) (*agentPage, error) {
	response, err := adapter.read(ctx, "/agents", query)
	if err != nil {
		return nil, err
	}
	var page agentPage
	if json.Unmarshal(response.Data, &page) != nil || page.Items == nil || page.Total == nil || page.TotalFailed == nil || page.Failures == nil || *page.Total < 0 || *page.TotalFailed < 0 || len(page.Items) > limit || *page.Total < len(page.Items) || len(page.Failures) > 100 {
		return nil, fmt.Errorf("wazuh: malformed or oversized agent page")
	}
	if *page.Total > 2147483647 || *page.TotalFailed > 2147483647 || (*page.TotalFailed == 0) != (len(page.Failures) == 0) {
		return nil, fmt.Errorf("wazuh: inconsistent agent page counts")
	}
	expectedError := 0
	if *page.TotalFailed > 0 {
		expectedError = 1
		if *page.Total > 0 {
			expectedError = 2
		}
	}
	if *response.Error != expectedError {
		return nil, fmt.Errorf("wazuh: API agent read failed")
	}
	if *page.TotalFailed > 0 && len(page.Items) == 0 {
		return nil, fmt.Errorf("wazuh: agents missing or forbidden for this reader")
	}
	for _, item := range page.Items {
		if !agentIDPattern.MatchString(item.ID) {
			return nil, fmt.Errorf("wazuh: invalid agent identity in response")
		}
		values := []*string{item.Name, item.Status, item.Version, item.LastKeepAlive, item.GroupConfigStatus}
		if item.OS != nil {
			values = append(values, item.OS.Name, item.OS.Version)
		}
		for _, value := range values {
			if value != nil && !safeText(*value, 256) {
				return nil, fmt.Errorf("wazuh: agent metadata exceeds safe text limits")
			}
		}
	}
	for _, failure := range page.Failures {
		if len(failure.ID) > 100 {
			return nil, fmt.Errorf("wazuh: excessive failure identifiers")
		}
		for _, id := range failure.ID {
			if !agentIDPattern.MatchString(id) {
				return nil, fmt.Errorf("wazuh: invalid failure identifier")
			}
		}
	}
	page.Partial = *page.TotalFailed > 0
	page.NextOffset = nil
	if len(page.Items) > 0 && offset+len(page.Items) < *page.Total {
		next := offset + len(page.Items)
		page.NextOffset = &next
	}
	return &page, nil
}

type statusCounts struct {
	Active         *int `json:"active"`
	Disconnected   *int `json:"disconnected"`
	NeverConnected *int `json:"never_connected"`
	Pending        *int `json:"pending"`
	Total          *int `json:"total"`
}

type syncCounts struct {
	Synced    *int `json:"synced"`
	NotSynced *int `json:"not_synced"`
	Total     *int `json:"total"`
}

func statusSummary(ctx context.Context, adapter *wazuh, _ map[string]any) (*mcp.ToolResult, error) {
	response, err := adapter.read(ctx, "/agents/summary/status", nil)
	if err != nil {
		return mcp.ErrResult(err)
	}
	var summary struct {
		Connection    statusCounts `json:"connection"`
		Configuration syncCounts   `json:"configuration"`
	}
	if *response.Error != 0 || json.Unmarshal(response.Data, &summary) != nil {
		return mcp.ErrResult(fmt.Errorf("wazuh: summary read failed"))
	}
	for _, value := range []*int{summary.Connection.Active, summary.Connection.Disconnected, summary.Connection.NeverConnected, summary.Connection.Pending, summary.Connection.Total, summary.Configuration.Synced, summary.Configuration.NotSynced, summary.Configuration.Total} {
		if value == nil || *value < 0 || *value > 2147483647 {
			return mcp.ErrResult(fmt.Errorf("wazuh: malformed summary counts"))
		}
	}
	if *summary.Connection.Active+*summary.Connection.Disconnected+*summary.Connection.NeverConnected+*summary.Connection.Pending != *summary.Connection.Total || *summary.Configuration.Synced+*summary.Configuration.NotSynced != *summary.Configuration.Total || *summary.Configuration.Total != *summary.Connection.Total {
		return mcp.ErrResult(fmt.Errorf("wazuh: inconsistent summary counts"))
	}
	return mcp.JSONResult(summary)
}
