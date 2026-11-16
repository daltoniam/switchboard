package snipeit

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	_ "embed"
	"encoding/json"
	"fmt"
	"html"
	"io"
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

var fieldCompactionSpecs = compact.MustLoadWithOverlay("snipeit", compactYAML, compact.Options{Strict: false}).Specs

var (
	_ mcp.Integration                = (*snipeit)(nil)
	_ mcp.FieldCompactionIntegration = (*snipeit)(nil)
	_ mcp.PlainTextCredentials       = (*snipeit)(nil)
	_ mcp.OptionalCredentials        = (*snipeit)(nil)
	_ mcp.PlaceholderHints           = (*snipeit)(nil)
	_ io.Closer                      = (*snipeit)(nil)
)

type snipeit struct {
	mu             sync.RWMutex
	client         *http.Client
	baseURL, token string
}

func New() mcp.Integration                 { return &snipeit{} }
func (s *snipeit) Name() string            { return "snipeit" }
func (s *snipeit) PlainTextKeys() []string { return []string{"base_url", "ca_cert"} }
func (s *snipeit) OptionalKeys() []string  { return []string{"ca_cert"} }
func (s *snipeit) Placeholders() map[string]string {
	return map[string]string{
		"base_url": "Trusted HTTPS Snipe-IT v8.8.0 installation URL, optional subpath",
		"token":    "Dedicated company-scoped reader PAT with assets.view only",
		"ca_cert":  `Optional PEM CA certificate, literal \n accepted; TLS verification stays enabled`,
	}
}

func safeText(value string, max int) bool {
	return utf8.ValidString(value) && len(value) <= max && !strings.ContainsFunc(value, unicode.IsControl)
}

func (s *snipeit) clear() {
	if s.client != nil {
		s.client.CloseIdleConnections()
	}
	s.client, s.baseURL, s.token = nil, "", ""
}

func (s *snipeit) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.clear()
	return nil
}

func (s *snipeit) Configure(_ context.Context, creds mcp.Credentials) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.clear()
	raw := creds["base_url"]
	endpoint, err := url.Parse(raw)
	if err != nil || endpoint == nil || endpoint.Scheme != "https" || endpoint.Hostname() == "" || endpoint.User != nil || endpoint.Opaque != "" || strings.ContainsAny(raw, "?#%\\") || strings.ContainsFunc(raw, func(value rune) bool { return unicode.IsSpace(value) || unicode.IsControl(value) }) {
		return fmt.Errorf("snipeit: base_url must be a trusted HTTPS installation URL without userinfo, query, fragment or encoded paths")
	}
	if strings.HasSuffix(endpoint.Host, ":") {
		return fmt.Errorf("snipeit: invalid base_url port")
	}
	if port := endpoint.Port(); port != "" {
		number, err := strconv.Atoi(port)
		if err != nil || number < 1 || number > 65535 {
			return fmt.Errorf("snipeit: invalid base_url port")
		}
	}
	for _, segment := range strings.Split(endpoint.Path, "/") {
		if segment == "." || segment == ".." {
			return fmt.Errorf("snipeit: invalid installation prefix")
		}
	}
	token := creds["token"]
	if token == "" || !safeText(token, 16384) || strings.ContainsFunc(token, unicode.IsSpace) {
		return fmt.Errorf("snipeit: token is required without whitespace or control characters")
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig = &tls.Config{MinVersion: tls.VersionTLS12}
	if cert := creds["ca_cert"]; cert != "" {
		pool, err := x509.SystemCertPool()
		if err != nil {
			pool = x509.NewCertPool()
		}
		if !pool.AppendCertsFromPEM([]byte(strings.ReplaceAll(cert, `\n`, "\n"))) {
			return fmt.Errorf("snipeit: ca_cert must contain a PEM CA certificate")
		}
		transport.TLSClientConfig.RootCAs = pool
	}
	s.client = &http.Client{Transport: transport, Timeout: 15 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	s.baseURL, s.token = strings.TrimRight(endpoint.String(), "/"), token
	return nil
}

var tools = []mcp.ToolDefinition{
	{Name: "snipeit_list_assets", Access: mcp.AccessRead, Description: "List and search Snipe-IT hardware assets by tag, serial or name. Start here for company IT inventory and incident support. One bounded page, default 25 and maximum 100; preserve next_offset with the same search/status. Returns asset status, model, location and assignment references only, never profiles, custom fields or notes.", Parameters: map[string]string{"search": "Optional hardware tag, serial or name search, at most 128 bytes", "status": "Optional status: ready, pending, archived, undeployable, deployed or requestable", "limit": "Page size 1..100, default 25", "offset": "Nonnegative next_offset from the previous page, default 0"}},
	{Name: "snipeit_get_asset", Access: mcp.AccessRead, Description: "Get one Snipe-IT hardware asset by numeric ID from snipeit_list_assets. Inspect inventory status and assignment type/ID without personnel profiles, custom fields, financial information or asset history.", Parameters: map[string]string{"id": "Exact positive numeric asset ID from list_assets"}, Required: []string{"id"}},
	{Name: "snipeit_get_asset_by_tag", Access: mcp.AccessRead, Description: "Resolve an exact Snipe-IT hardware asset tag to bounded candidates. Use after snipeit_list_assets or when an incident provides a tag. Each match is reauthorized through the exact-ID read before any metadata is returned. No checkout, edit, custom fields or personnel profiles.", Parameters: map[string]string{"tag": "Exact literal asset tag, 1..128 bytes, no slash, backslash, percent, query, fragment, controls or traversal"}, Required: []string{"tag"}},
}

func (s *snipeit) Tools() []mcp.ToolDefinition { return tools }
func (s *snipeit) CompactSpec(name mcp.ToolName) ([]mcp.CompactField, bool) {
	fields, ok := fieldCompactionSpecs[name]
	return fields, ok
}

type handler func(context.Context, *snipeit, map[string]any) (*mcp.ToolResult, error)

var dispatch = map[mcp.ToolName]handler{"snipeit_list_assets": listAssets, "snipeit_get_asset": getAsset, "snipeit_get_asset_by_tag": getAssetByTag}

func (s *snipeit) Execute(ctx context.Context, name mcp.ToolName, args map[string]any) (*mcp.ToolResult, error) {
	fn, ok := dispatch[name]
	if !ok {
		return mcp.ErrResult(fmt.Errorf("snipeit: unknown tool: %s", name))
	}
	for _, tool := range tools {
		if tool.Name != name {
			continue
		}
		for key, value := range args {
			if _, ok := tool.Parameters[key]; !ok {
				return mcp.ErrResult(fmt.Errorf("snipeit: unsupported argument"))
			}
			if key == "id" || key == "limit" || key == "offset" {
				text, err := mcp.ArgStr(args, key)
				if err != nil {
					return mcp.ErrResult(fmt.Errorf("snipeit: integer argument required"))
				}
				number, err := strconv.ParseInt(text, 10, 32)
				if err != nil || (key == "limit" && (number < 1 || number > 100)) {
					return mcp.ErrResult(fmt.Errorf("snipeit: integer argument outside supported bounds"))
				}
			} else if _, ok := value.(string); !ok {
				return mcp.ErrResult(fmt.Errorf("snipeit: text argument required"))
			}
		}
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.client == nil {
		return mcp.ErrResult(mcp.ErrNotConfigured)
	}
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	return fn(ctx, s, args)
}

type reference struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}
type assignment struct {
	ID   int64  `json:"id"`
	Type string `json:"type"`
}
type asset struct {
	ID         int64       `json:"id"`
	Name       string      `json:"name"`
	AssetTag   string      `json:"asset_tag"`
	Serial     string      `json:"serial"`
	Model      *reference  `json:"model"`
	Status     *reference  `json:"status_label"`
	Location   *reference  `json:"location"`
	AssignedTo *assignment `json:"assigned_to"`
}
type assetPage struct {
	Total      int64   `json:"total"`
	Rows       []asset `json:"rows"`
	NextOffset *int64  `json:"next_offset"`
}

func validateAsset(value *asset) error {
	if value.ID <= 0 || value.AssetTag == "" {
		return fmt.Errorf("snipeit: invalid asset response")
	}
	texts := []*string{&value.Name, &value.AssetTag, &value.Serial}
	for _, ref := range []*reference{value.Model, value.Status, value.Location} {
		if ref != nil {
			if ref.ID <= 0 {
				return fmt.Errorf("snipeit: invalid asset reference")
			}
			texts = append(texts, &ref.Name)
		}
	}
	for _, text := range texts {
		*text = html.UnescapeString(*text)
		if !safeText(*text, 512) {
			return fmt.Errorf("snipeit: asset metadata exceeds safe text limits")
		}
	}
	if ref := value.AssignedTo; ref != nil && (ref.ID <= 0 || !slices.Contains([]string{"user", "asset", "location"}, ref.Type)) {
		return fmt.Errorf("snipeit: invalid assignment reference")
	}
	return nil
}

func (s *snipeit) request(ctx context.Context, path string, query url.Values) ([]byte, error) {
	target := s.baseURL + "/api/v1/hardware" + path
	if len(query) != 0 {
		target += "?" + query.Encode()
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return nil, fmt.Errorf("snipeit: invalid request")
	}
	request.Header.Set("Authorization", "Bearer "+s.token)
	request.Header.Set("Accept", "application/json")
	response, err := s.client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("snipeit: request failed; check TLS, connectivity and operation deadline")
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode != http.StatusOK {
		if response.StatusCode == http.StatusTooManyRequests {
			if seconds, err := strconv.ParseUint(response.Header.Get("Retry-After"), 10, 32); err == nil && seconds > 0 {
				return nil, fmt.Errorf("snipeit: rate limited (HTTP 429); retry after %d seconds", seconds)
			}
			if retryAt, err := http.ParseTime(response.Header.Get("Retry-After")); err == nil && retryAt.After(time.Now()) {
				return nil, fmt.Errorf("snipeit: rate limited (HTTP 429); retry after %d seconds", int64(time.Until(retryAt).Seconds())+1)
			}
			return nil, fmt.Errorf("snipeit: rate limited (HTTP 429); retry later within the installation quota")
		}
		return nil, fmt.Errorf("snipeit: API returned HTTP %d; verify reader permissions and installation URL", response.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, 2*1024*1024+1))
	if err != nil || len(data) > 2*1024*1024 {
		return nil, fmt.Errorf("snipeit: response exceeds 2 MiB or could not be read")
	}
	var envelope struct {
		Status json.RawMessage `json:"status"`
	}
	if json.Unmarshal(data, &envelope) != nil || len(data) == 0 || data[0] == '[' {
		return nil, fmt.Errorf("snipeit: invalid JSON asset response")
	}
	if len(envelope.Status) != 0 && envelope.Status[0] == '"' {
		return nil, fmt.Errorf("snipeit: API error envelope; asset may be missing or not visible")
	}
	return data, nil
}

func (s *snipeit) page(ctx context.Context, path string, query url.Values, limit, offset int) (*assetPage, error) {
	data, err := s.request(ctx, path, query)
	if err != nil {
		return nil, err
	}
	var value struct {
		Total *int64  `json:"total"`
		Rows  []asset `json:"rows"`
	}
	if json.Unmarshal(data, &value) != nil || value.Total == nil || *value.Total < 0 || value.Rows == nil || len(value.Rows) > limit || int64(len(value.Rows)) > *value.Total {
		return nil, fmt.Errorf("snipeit: invalid or unbounded asset page")
	}
	for index := range value.Rows {
		if err := validateAsset(&value.Rows[index]); err != nil {
			return nil, err
		}
	}
	result := &assetPage{Total: *value.Total, Rows: value.Rows}
	next := int64(offset) + int64(len(value.Rows))
	if len(value.Rows) > 0 && next < result.Total {
		result.NextOffset = &next
	}
	return result, nil
}

func listAssets(ctx context.Context, s *snipeit, args map[string]any) (*mcp.ToolResult, error) {
	reader := mcp.NewArgs(args)
	limit, offset := reader.OptInt("limit", 25), reader.Int("offset")
	search, status := reader.Str("search"), reader.Str("status")
	if reader.Err() != nil {
		return mcp.ErrResult(fmt.Errorf("snipeit: invalid list arguments"))
	}
	if limit < 1 || limit > 100 || offset < 0 || offset > 1000000 || !safeText(search, 128) || !slices.Contains([]string{"", "ready", "pending", "archived", "undeployable", "deployed", "requestable"}, status) {
		return mcp.ErrResult(fmt.Errorf("snipeit: invalid limit, offset, search or status"))
	}
	query := url.Values{"limit": {strconv.Itoa(limit)}, "offset": {strconv.Itoa(offset)}, "sort": {"id"}, "order": {"asc"}}
	if search != "" {
		query.Set("search", search)
	}
	if status != "" {
		query.Set("status", map[string]string{"ready": "RTD", "pending": "Pending", "archived": "Archived", "undeployable": "Undeployable", "deployed": "Deployed", "requestable": "Requestable"}[status])
	}
	value, err := s.page(ctx, "", query, limit, offset)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return boundedResult(value)
}

func (s *snipeit) exact(ctx context.Context, id int64) (*asset, error) {
	data, err := s.request(ctx, "/"+strconv.FormatInt(id, 10), nil)
	if err != nil {
		return nil, err
	}
	var value asset
	if json.Unmarshal(data, &value) != nil || value.ID != id {
		return nil, fmt.Errorf("snipeit: invalid asset identity")
	}
	if err := validateAsset(&value); err != nil {
		return nil, err
	}
	return &value, nil
}

func getAsset(ctx context.Context, s *snipeit, args map[string]any) (*mcp.ToolResult, error) {
	reader := mcp.NewArgs(args)
	id := reader.Int("id")
	if reader.Err() != nil || id <= 0 {
		return mcp.ErrResult(fmt.Errorf("snipeit: positive numeric id is required"))
	}
	value, err := s.exact(ctx, int64(id))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return boundedResult(value)
}

func getAssetByTag(ctx context.Context, s *snipeit, args map[string]any) (*mcp.ToolResult, error) {
	reader := mcp.NewArgs(args)
	tag := reader.Str("tag")
	if reader.Err() != nil || tag == "" || !safeText(tag, 128) || strings.ContainsAny(tag, "/\\%?#") || tag == "." || tag == ".." {
		return mcp.ErrResult(fmt.Errorf("snipeit: literal safe asset tag is required"))
	}
	data, err := s.request(ctx, "/bytag/"+url.PathEscape(tag), url.Values{"limit": {"100"}, "offset": {"0"}})
	if err != nil {
		return mcp.ErrResult(err)
	}
	var candidates struct {
		ID    int64  `json:"id"`
		Total *int64 `json:"total"`
		Rows  []struct {
			ID int64 `json:"id"`
		} `json:"rows"`
	}
	if json.Unmarshal(data, &candidates) != nil {
		return mcp.ErrResult(fmt.Errorf("snipeit: invalid tag response"))
	}
	ids := []int64{}
	if candidates.ID > 0 {
		ids = append(ids, candidates.ID)
	} else {
		if candidates.Total == nil || *candidates.Total != int64(len(candidates.Rows)) || len(candidates.Rows) > 25 {
			return mcp.ErrResult(fmt.Errorf("snipeit: invalid or excessive tag candidates; use list_assets"))
		}
		for _, row := range candidates.Rows {
			ids = append(ids, row.ID)
		}
	}
	result := assetPage{Rows: []asset{}}
	for _, id := range ids {
		if id <= 0 {
			return mcp.ErrResult(fmt.Errorf("snipeit: invalid tag candidate identity"))
		}
		value, err := s.exact(ctx, id)
		if err != nil {
			return mcp.ErrResult(err)
		}
		if value.AssetTag != tag {
			return mcp.ErrResult(fmt.Errorf("snipeit: tag candidate did not match exactly"))
		}
		result.Rows = append(result.Rows, *value)
	}
	result.Total = int64(len(result.Rows))
	return boundedResult(result)
}

func boundedResult(value any) (*mcp.ToolResult, error) {
	result, err := mcp.JSONResult(value)
	if err == nil && len(result.Data) > 256*1024 {
		return mcp.ErrResult(fmt.Errorf("snipeit: output exceeds 256 KiB; request a smaller page"))
	}
	return result, err
}

func (s *snipeit) Healthy(ctx context.Context) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.client == nil {
		return false
	}
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	_, err := s.page(ctx, "", url.Values{"limit": {"1"}, "offset": {"0"}}, 1, 0)
	return err == nil
}
