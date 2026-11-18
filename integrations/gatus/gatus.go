package gatus

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"mime"
	"net/http"
	"net/url"
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

var fieldCompactionSpecs = compact.MustLoadWithOverlay("gatus", compactYAML, compact.Options{Strict: false}).Specs

var (
	_ mcp.Integration                = (*gatus)(nil)
	_ mcp.FieldCompactionIntegration = (*gatus)(nil)
	_ mcp.PlainTextCredentials       = (*gatus)(nil)
	_ mcp.OptionalCredentials        = (*gatus)(nil)
	_ mcp.PlaceholderHints           = (*gatus)(nil)
	_ io.Closer                      = (*gatus)(nil)
)

type gatus struct {
	mu                          sync.RWMutex
	client                      *http.Client
	baseURL, username, password string
}

func New() mcp.Integration               { return &gatus{} }
func (g *gatus) Name() string            { return "gatus" }
func (g *gatus) PlainTextKeys() []string { return []string{"base_url", "username", "ca_cert"} }
func (g *gatus) OptionalKeys() []string  { return []string{"ca_cert"} }
func (g *gatus) Placeholders() map[string]string {
	return map[string]string{
		"base_url": "Trusted HTTPS Gatus v5.37.0 URL, optional subpath; Basic only, OIDC disabled",
		"username": "Gatus built-in Basic username; installation-wide status visibility",
		"password": "Gatus built-in Basic password, never an external-result bearer token",
		"ca_cert":  `Optional PEM CA certificate, literal \n accepted; TLS verification stays enabled`,
	}
}

func safeText(value string, maximum int) bool {
	return utf8.ValidString(value) && len(value) <= maximum && !strings.ContainsFunc(value, unicode.IsControl)
}

func validKey(value string) bool {
	return value != "" && safeText(value, 256) && !strings.ContainsAny(value, "/\\%?#+") && !strings.Contains(value, "..") && value != "."
}

func (g *gatus) clear() {
	if g.client != nil {
		g.client.CloseIdleConnections()
	}
	g.client, g.baseURL, g.username, g.password = nil, "", "", ""
}

func (g *gatus) Close() error {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.clear()
	return nil
}

func (g *gatus) Configure(_ context.Context, creds mcp.Credentials) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.clear()
	raw := creds["base_url"]
	endpoint, err := url.Parse(raw)
	if err != nil || endpoint == nil || endpoint.Scheme != "https" || endpoint.Hostname() == "" || endpoint.User != nil || endpoint.Opaque != "" || strings.ContainsAny(raw, "?#%\\") || strings.ContainsFunc(raw, func(character rune) bool { return unicode.IsSpace(character) || unicode.IsControl(character) }) {
		return fmt.Errorf("gatus: base_url must be a trusted HTTPS URL without userinfo, query, fragment or encoded paths")
	}
	if strings.HasSuffix(endpoint.Host, ":") {
		return fmt.Errorf("gatus: invalid base_url port")
	}
	if port := endpoint.Port(); port != "" {
		number, err := strconv.Atoi(port)
		if err != nil || number < 1 || number > 65535 {
			return fmt.Errorf("gatus: invalid base_url port")
		}
	}
	for _, segment := range strings.Split(endpoint.Path, "/") {
		if segment == "." || segment == ".." {
			return fmt.Errorf("gatus: invalid installation prefix")
		}
	}
	username, password := creds["username"], creds["password"]
	if username == "" || !safeText(username, 256) || strings.Contains(username, ":") {
		return fmt.Errorf("gatus: username is required without colon or control characters")
	}
	if password == "" || !safeText(password, 4096) {
		return fmt.Errorf("gatus: password is required without control characters")
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig = &tls.Config{MinVersion: tls.VersionTLS12}
	if cert := creds["ca_cert"]; cert != "" {
		pool, err := x509.SystemCertPool()
		if err != nil {
			pool = x509.NewCertPool()
		}
		if !pool.AppendCertsFromPEM([]byte(strings.ReplaceAll(cert, `\n`, "\n"))) {
			return fmt.Errorf("gatus: ca_cert must contain a PEM CA certificate")
		}
		transport.TLSClientConfig.RootCAs = pool
	}
	g.client = &http.Client{Transport: transport, Timeout: 10 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	g.baseURL, g.username, g.password = strings.TrimRight(endpoint.String(), "/"), username, password
	return nil
}

var tools = []mcp.ToolDefinition{
	{Name: "gatus_list_endpoints", Access: mcp.AccessRead, Description: "List Gatus monitored endpoint health and latest observed probe status. Start here to find down or unhealthy endpoints in a self-hosted Gatus installation. Optional local exact group, name substring and only_unhealthy filters; limit 1..50, default 20. Fetches all endpoint inventory once, not endpoint pagination. Unknown state is not healthy or a proven outage; cached/remote results may be incomplete.", Parameters: map[string]string{"group": "Optional exact local group filter, at most 256 bytes", "name": "Optional case-insensitive local endpoint name substring, at most 256 bytes", "only_unhealthy": "Optional boolean, return only endpoints with an observed failed probe (excludes unknown)", "limit": "Local output limit 1..50, default 20; does not reduce upstream inventory"}},
	{Name: "gatus_get_endpoint", Access: mcp.AccessRead, Description: "Get one Gatus endpoint's bounded probe result history by opaque key from gatus_list_endpoints. Use after discovery to inspect success, timestamp, HTTP status and duration_ms, newest timestamp first within the requested history page. page/page_size paginate probe results, not endpoint inventory. Excludes errors, conditions, hostnames and events.", Parameters: map[string]string{"key": "Exact opaque endpoint key from list_endpoints, 1..256 bytes; not a URL", "page": "Probe history page 1..1000000, default 1", "page_size": "Probe results per page 1..20, default 10"}, Required: []string{"key"}},
	{Name: "gatus_get_uptime", Access: mcp.AccessRead, Description: "Get Gatus endpoint uptime ratio for duration 1h, 24h, 7d or 30d (default 24h). Use a key from gatus_list_endpoints. Ratio is not a percentage; zero may mean no observations, not proven outage. Gatus 1h actually starts two hours back due to hourly storage. Raw uptime routes are public upstream even with Basic auth; protect confidential sites at the network boundary.", Parameters: map[string]string{"key": "Exact opaque endpoint key from list_endpoints, 1..256 bytes; not a URL", "duration": "Window enum 1h, 24h, 7d or 30d, default 24h"}, Required: []string{"key"}},
}

func (g *gatus) Tools() []mcp.ToolDefinition { return tools }
func (g *gatus) CompactSpec(name mcp.ToolName) ([]mcp.CompactField, bool) {
	fields, ok := fieldCompactionSpecs[name]
	return fields, ok
}

type handler func(context.Context, *gatus, map[string]any) (*mcp.ToolResult, error)

var dispatch = map[mcp.ToolName]handler{"gatus_list_endpoints": listEndpoints, "gatus_get_endpoint": getEndpoint, "gatus_get_uptime": getUptime}

func (g *gatus) Execute(ctx context.Context, name mcp.ToolName, args map[string]any) (*mcp.ToolResult, error) {
	fn, ok := dispatch[name]
	if !ok {
		return mcp.ErrResult(fmt.Errorf("gatus: unknown tool: %s", name))
	}
	for _, tool := range tools {
		if tool.Name != name {
			continue
		}
		for key, value := range args {
			if _, ok := tool.Parameters[key]; !ok {
				return mcp.ErrResult(fmt.Errorf("gatus: unsupported argument"))
			}
			switch key {
			case "limit", "page", "page_size":
				text, err := mcp.ArgStr(args, key)
				if err != nil {
					return mcp.ErrResult(fmt.Errorf("gatus: integer argument required"))
				}
				number, err := strconv.ParseInt(text, 10, 32)
				maximum := int64(1000000)
				switch key {
				case "limit":
					maximum = 50
				case "page_size":
					maximum = 20
				}
				if err != nil || number < 1 || number > maximum {
					return mcp.ErrResult(fmt.Errorf("gatus: integer argument outside supported bounds"))
				}
			case "only_unhealthy":
				if _, ok := value.(bool); !ok {
					return mcp.ErrResult(fmt.Errorf("gatus: boolean argument required"))
				}
			default:
				if _, ok := value.(string); !ok {
					return mcp.ErrResult(fmt.Errorf("gatus: text argument required"))
				}
			}
		}
	}
	g.mu.RLock()
	defer g.mu.RUnlock()
	if g.client == nil {
		return mcp.ErrResult(mcp.ErrNotConfigured)
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	return fn(ctx, g, args)
}

func (g *gatus) request(ctx context.Context, path string, query url.Values, expectedType string) ([]byte, error) {
	target := g.baseURL + "/api/v1/endpoints/" + path
	if len(query) != 0 {
		target += "?" + query.Encode()
	}
	for attempt := 0; attempt < 3; attempt++ {
		request, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
		if err != nil {
			return nil, fmt.Errorf("gatus: invalid request")
		}
		request.SetBasicAuth(g.username, g.password)
		request.Header.Set("Accept", expectedType)
		response, err := g.client.Do(request)
		if err != nil {
			return nil, fmt.Errorf("gatus: request failed; check TLS, connectivity and deadline")
		}
		if response.StatusCode != http.StatusOK {
			_ = response.Body.Close()
			if attempt < 2 && slices.Contains([]int{429, 500, 502, 503, 504}, response.StatusCode) {
				if err := waitForRetry(ctx, response.Header.Get("Retry-After"), attempt); err != nil {
					return nil, err
				}
				continue
			}
			return nil, fmt.Errorf("gatus: HTTP %d; verify endpoint key, Basic credentials and Basic-only setup with OIDC disabled", response.StatusCode)
		}
		contentType, _, err := mime.ParseMediaType(response.Header.Get("Content-Type"))
		if err != nil || contentType != expectedType {
			_ = response.Body.Close()
			return nil, fmt.Errorf("gatus: unsupported response type; verify Basic-only setup, OIDC disabled and installation URL")
		}
		data, err := io.ReadAll(io.LimitReader(response.Body, (2<<20)+1))
		_ = response.Body.Close()
		if err != nil || len(data) > 2<<20 {
			return nil, fmt.Errorf("gatus: response exceeds 2 MiB decoded limit or could not be read; reduce upstream inventory")
		}
		return data, nil
	}
	return nil, fmt.Errorf("gatus: retry budget exhausted")
}

func waitForRetry(ctx context.Context, retry string, attempt int) error {
	wait := time.Duration(attempt+1) * 100 * time.Millisecond
	if seconds, err := strconv.ParseUint(retry, 10, 32); err == nil {
		wait = time.Duration(seconds) * time.Second
	} else if at, err := http.ParseTime(retry); err == nil {
		wait = max(time.Duration(0), time.Until(at))
	}
	if deadline, ok := ctx.Deadline(); ok && wait >= time.Until(deadline) {
		return fmt.Errorf("gatus: retry exceeds operation deadline; retry later")
	}
	timer := time.NewTimer(wait)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return fmt.Errorf("gatus: operation deadline or cancellation")
	case <-timer.C:
		return nil
	}
}

type probe struct {
	Timestamp time.Time `json:"timestamp"`
	Success   *bool     `json:"success"`
	Status    int       `json:"status"`
	Duration  *int64    `json:"duration"`
}

type probeResults []probe

func (results *probeResults) UnmarshalJSON(data []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	token, err := decoder.Token()
	if err != nil {
		return err
	}
	if token == nil {
		*results = probeResults{}
		return nil
	}
	if token != json.Delim('[') {
		return fmt.Errorf("invalid probe array")
	}
	*results = probeResults{}
	for decoder.More() {
		if len(*results) >= 20 {
			return fmt.Errorf("probe count exceeds 20")
		}
		var value probe
		if err := decoder.Decode(&value); err != nil {
			return err
		}
		*results = append(*results, value)
	}
	_, err = decoder.Token()
	return err
}

type endpointStatus struct {
	Key     string       `json:"key"`
	Name    string       `json:"name"`
	Group   string       `json:"group"`
	Results probeResults `json:"results"`
}

type observation struct {
	Timestamp  time.Time `json:"timestamp"`
	Success    bool      `json:"success"`
	Status     int       `json:"status"`
	DurationMS float64   `json:"duration_ms"`
}

type endpointView struct {
	Key    string       `json:"key"`
	Name   string       `json:"name"`
	Group  string       `json:"group"`
	Latest *observation `json:"latest"`
}

func validateStatus(value *endpointStatus) error {
	if !validKey(value.Key) || !safeText(value.Name, 256) || !safeText(value.Group, 256) {
		return fmt.Errorf("gatus: endpoint metadata exceeds safe text limits or has an invalid key")
	}
	for _, result := range value.Results {
		if result.Timestamp.IsZero() || result.Success == nil || result.Duration == nil || *result.Duration < 0 || result.Status < 0 || result.Status > 599 {
			return fmt.Errorf("gatus: malformed probe result")
		}
	}
	slices.SortStableFunc(value.Results, func(first, second probe) int { return second.Timestamp.Compare(first.Timestamp) })
	return nil
}

func projectProbe(value probe) observation {
	return observation{Timestamp: value.Timestamp, Success: *value.Success, Status: value.Status, DurationMS: float64(*value.Duration) / float64(time.Millisecond)}
}

const inventoryCaveat = "Observed inventory only: list is cached upstream for 10 seconds and remote aggregation may silently omit failed remotes. Filters and limit are local, not inventory pagination. Unknown probe state is not healthy or a proven outage."

func (g *gatus) inventory(ctx context.Context) ([]endpointStatus, error) {
	data, err := g.request(ctx, "statuses", url.Values{"page": {"1"}, "pageSize": {"1"}}, "application/json")
	if err != nil {
		return nil, err
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	token, err := decoder.Token()
	if err != nil || (token != json.Delim('[') && token != nil) {
		return nil, fmt.Errorf("gatus: invalid endpoint inventory JSON")
	}
	values := []endpointStatus{}
	if token != nil {
		for decoder.More() {
			if len(values) >= 2000 {
				return nil, fmt.Errorf("gatus: endpoint inventory exceeds 2000 object limit")
			}
			var value endpointStatus
			if err := decoder.Decode(&value); err != nil {
				return nil, fmt.Errorf("gatus: invalid endpoint inventory or probe count exceeds 20")
			}
			if err := validateStatus(&value); err != nil {
				return nil, err
			}
			values = append(values, value)
		}
		if _, err := decoder.Token(); err != nil {
			return nil, fmt.Errorf("gatus: invalid endpoint inventory JSON")
		}
	}
	if decoder.Decode(new(any)) != io.EOF {
		return nil, fmt.Errorf("gatus: trailing endpoint inventory data")
	}
	return values, nil
}

func listEndpoints(ctx context.Context, g *gatus, args map[string]any) (*mcp.ToolResult, error) {
	reader := mcp.NewArgs(args)
	group, name := reader.Str("group"), reader.Str("name")
	limit, unhealthy := reader.OptInt("limit", 20), reader.Bool("only_unhealthy")
	if err := reader.Err(); err != nil {
		return mcp.ErrResult(fmt.Errorf("gatus: invalid list arguments"))
	}
	if limit < 1 || limit > 50 || !safeText(group, 256) || !safeText(name, 256) {
		return mcp.ErrResult(fmt.Errorf("gatus: limit must be 1..50 and filters at most 256 bytes without controls"))
	}
	values, err := g.inventory(ctx)
	if err != nil {
		return mcp.ErrResult(err)
	}
	slices.SortStableFunc(values, func(first, second endpointStatus) int { return strings.Compare(first.Key, second.Key) })
	views := []endpointView{}
	matching := 0
	for _, value := range values {
		if group != "" && value.Group != group || !strings.Contains(strings.ToLower(value.Name), strings.ToLower(name)) {
			continue
		}
		var latest *observation
		if len(value.Results) > 0 {
			projected := projectProbe(value.Results[0])
			latest = &projected
		}
		if unhealthy && (latest == nil || latest.Success) {
			continue
		}
		matching++
		if len(views) < limit {
			views = append(views, endpointView{Key: value.Key, Name: value.Name, Group: value.Group, Latest: latest})
		}
	}
	return mcp.JSONResult(struct {
		Endpoints     []endpointView `json:"endpoints"`
		TotalFetched  int            `json:"total_fetched"`
		MatchingCount int            `json:"matching_count"`
		ReturnedCount int            `json:"returned_count"`
		Truncated     bool           `json:"truncated"`
		Caveat        string         `json:"caveat"`
	}{views, len(values), matching, len(views), matching > len(views), inventoryCaveat})
}

func getEndpoint(ctx context.Context, g *gatus, args map[string]any) (*mcp.ToolResult, error) {
	reader := mcp.NewArgs(args)
	key, page, size := reader.Str("key"), reader.OptInt("page", 1), reader.OptInt("page_size", 10)
	if err := reader.Err(); err != nil {
		return mcp.ErrResult(fmt.Errorf("gatus: invalid history arguments"))
	}
	if !validKey(key) || page < 1 || page > 1000000 || size < 1 || size > 20 {
		return mcp.ErrResult(fmt.Errorf("gatus: valid opaque key, page 1..1000000 and page_size 1..20 required"))
	}
	data, err := g.request(ctx, url.PathEscape(key)+"/statuses", url.Values{"page": {strconv.Itoa(page)}, "pageSize": {strconv.Itoa(size)}}, "application/json")
	if err != nil {
		return mcp.ErrResult(err)
	}
	var value endpointStatus
	if err := json.Unmarshal(data, &value); err != nil {
		return mcp.ErrResult(fmt.Errorf("gatus: invalid endpoint JSON or probe count exceeds 20"))
	}
	if err := validateStatus(&value); err != nil {
		return mcp.ErrResult(err)
	}
	if value.Key != key || len(value.Results) > size {
		return mcp.ErrResult(fmt.Errorf("gatus: endpoint key mismatch or requested history size exceeded"))
	}
	results := []observation{}
	for _, result := range value.Results {
		results = append(results, projectProbe(result))
	}
	return mcp.JSONResult(struct {
		Key      string        `json:"key"`
		Name     string        `json:"name"`
		Group    string        `json:"group"`
		Results  []observation `json:"results"`
		Page     int           `json:"page"`
		PageSize int           `json:"page_size"`
		Caveat   string        `json:"caveat"`
	}{value.Key, value.Name, value.Group, results, page, size, "Probe history page only, sorted by timestamp within this page; not endpoint pagination or a complete history guarantee. Missing observations do not prove an outage."})
}

func getUptime(ctx context.Context, g *gatus, args map[string]any) (*mcp.ToolResult, error) {
	reader := mcp.NewArgs(args)
	key, duration := reader.Str("key"), reader.Str("duration")
	if err := reader.Err(); err != nil {
		return mcp.ErrResult(fmt.Errorf("gatus: invalid uptime arguments"))
	}
	if duration == "" {
		duration = "24h"
	}
	if !validKey(key) || !slices.Contains([]string{"1h", "24h", "7d", "30d"}, duration) {
		return mcp.ErrResult(fmt.Errorf("gatus: valid opaque key and duration 1h, 24h, 7d or 30d required"))
	}
	data, err := g.request(ctx, url.PathEscape(key)+"/uptimes/"+duration, nil, "text/plain")
	if err != nil {
		return mcp.ErrResult(err)
	}
	ratio, err := strconv.ParseFloat(strings.TrimSpace(string(data)), 64)
	if err != nil || math.IsNaN(ratio) || math.IsInf(ratio, 0) || ratio < 0 || ratio > 1 {
		return mcp.ErrResult(fmt.Errorf("gatus: uptime must be a finite text ratio between 0 and 1"))
	}
	return mcp.JSONResult(struct {
		Key      string  `json:"key"`
		Duration string  `json:"duration"`
		Ratio    float64 `json:"ratio"`
		Caveat   string  `json:"caveat"`
	}{key, duration, ratio, "Ratio, not percentage. Zero may mean no observations, not proven outage. The 1h window starts two hours back because Gatus stores hourly uptime. Raw uptime routes are public upstream even with Basic authentication; protect confidential sites at the network boundary."})
}

func (g *gatus) Healthy(ctx context.Context) bool {
	g.mu.RLock()
	defer g.mu.RUnlock()
	if g.client == nil {
		return false
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	_, err := g.inventory(ctx)
	return err == nil
}
