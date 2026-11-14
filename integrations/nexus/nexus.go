package nexus

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	_ "embed"
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

var fieldCompactionSpecs = compact.MustLoadWithOverlay("nexus", compactYAML, compact.Options{Strict: false}).Specs

const maxResponseBytes = 2 * 1024 * 1024

var repositoryPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$`)

var (
	_ mcp.Integration                = (*nexus)(nil)
	_ mcp.FieldCompactionIntegration = (*nexus)(nil)
	_ mcp.PlainTextCredentials       = (*nexus)(nil)
	_ mcp.OptionalCredentials        = (*nexus)(nil)
	_ mcp.PlaceholderHints           = (*nexus)(nil)
	_ io.Closer                      = (*nexus)(nil)
)

type nexus struct {
	mu                          sync.RWMutex
	client                      *http.Client
	baseURL, username, password string
	repositories                []string
}

func New() mcp.Integration    { return &nexus{} }
func (n *nexus) Name() string { return "nexus" }
func (n *nexus) PlainTextKeys() []string {
	return []string{"base_url", "username", "repositories", "ca_cert"}
}
func (n *nexus) OptionalKeys() []string { return []string{"ca_cert"} }
func (n *nexus) Placeholders() map[string]string {
	return map[string]string{
		"base_url":     "Trusted HTTPS Nexus Repository 3.96.x installation URL",
		"username":     "Dedicated non-admin reader with nx-search-read and repository browse/read",
		"password":     "Reader password or authorized Pro user-token Basic pair secret",
		"repositories": "Comma-separated approved repository names, no wildcards",
		"ca_cert":      `Optional PEM CA certificate, literal \n accepted; TLS verification remains enabled`,
	}
}

func (n *nexus) Close() error {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.clear()
	return nil
}

func (n *nexus) clear() {
	if n.client != nil {
		n.client.CloseIdleConnections()
	}
	n.client, n.baseURL, n.username, n.password, n.repositories = nil, "", "", "", nil
}

func safeText(value string, limit int) bool {
	return utf8.ValidString(value) && len(value) <= limit && !strings.ContainsFunc(value, unicode.IsControl)
}

func safeID(value string) bool {
	return value != "" && safeText(value, 1024) && !strings.ContainsAny(value, "/\\%?#") && value != "." && value != ".."
}

func (n *nexus) Configure(_ context.Context, creds mcp.Credentials) error {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.clear()
	raw := creds["base_url"]
	endpoint, err := url.Parse(raw)
	if err != nil || endpoint == nil || endpoint.Scheme != "https" || endpoint.Hostname() == "" || endpoint.User != nil || endpoint.Opaque != "" || strings.ContainsAny(raw, "?#%\\") || strings.ContainsFunc(raw, func(value rune) bool { return unicode.IsSpace(value) || unicode.IsControl(value) }) {
		return fmt.Errorf("nexus: base_url must be a trusted HTTPS installation URL without credentials, query, fragment or encoded paths")
	}
	if strings.HasSuffix(endpoint.Host, ":") {
		return fmt.Errorf("nexus: invalid base_url port")
	}
	if port := endpoint.Port(); port != "" {
		parsed, err := strconv.Atoi(port)
		if err != nil || parsed < 1 || parsed > 65535 {
			return fmt.Errorf("nexus: invalid base_url port")
		}
	}
	for _, segment := range strings.Split(endpoint.Path, "/") {
		if segment == "." || segment == ".." {
			return fmt.Errorf("nexus: invalid installation prefix")
		}
	}
	username, password := creds["username"], creds["password"]
	if username == "" || !safeText(username, 256) || strings.Contains(username, ":") || strings.TrimSpace(username) != username || strings.EqualFold(username, "admin") || strings.EqualFold(username, "anonymous") {
		return fmt.Errorf("nexus: username must identify a dedicated non-admin, non-anonymous reader")
	}
	if password == "" || !safeText(password, 4096) {
		return fmt.Errorf("nexus: password is required without control characters")
	}
	repositories := strings.Split(creds["repositories"], ",")
	if len(repositories) > 20 {
		return fmt.Errorf("nexus: configure at most 20 approved repositories")
	}
	for index, repository := range repositories {
		repository = strings.TrimSpace(repository)
		if !repositoryPattern.MatchString(repository) || strings.Contains(repository, "..") || slices.Contains(repositories[:index], repository) {
			return fmt.Errorf("nexus: repositories must be distinct literal names without wildcards or paths")
		}
		repositories[index] = repository
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig = &tls.Config{MinVersion: tls.VersionTLS12}
	if cert := creds["ca_cert"]; cert != "" {
		pool, err := x509.SystemCertPool()
		if err != nil {
			pool = x509.NewCertPool()
		}
		if !pool.AppendCertsFromPEM([]byte(strings.ReplaceAll(cert, `\n`, "\n"))) {
			return fmt.Errorf("nexus: ca_cert must contain a PEM CA certificate")
		}
		transport.TLSClientConfig.RootCAs = pool
	}
	n.client = &http.Client{Transport: transport, Timeout: 15 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	n.baseURL, n.username, n.password, n.repositories = strings.TrimRight(endpoint.String(), "/"), username, password, repositories
	return nil
}

func (n *nexus) Healthy(ctx context.Context) bool {
	n.mu.RLock()
	defer n.mu.RUnlock()
	if n.client == nil {
		return false
	}
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	_, err := n.page(ctx, "/components", url.Values{"repository": {n.repositories[0]}})
	return err == nil
}

var tools = []mcp.ToolDefinition{
	{Name: "nexus_list_components", Access: mcp.AccessRead, Description: "List private package component coordinates in one configured Nexus Repository. Start here for missing Maven, npm and PyPI package versions. One server page with an opaque continuationToken; no downloads or repository administration. Select an operator-approved repository name.", Parameters: map[string]string{"repository": "Exact configured repository name", "continuation_token": "Opaque continuationToken from the previous page in the same repository, at most 2048 bytes"}, Required: []string{"repository"}},
	{Name: "nexus_search_components", Access: mcp.AccessRead, Description: "Search private Nexus Repository package components by name, group and version in one approved repository. Use after nexus_list_components. One server page, preserve continuationToken for the same repository and filters. Metadata only, no vulnerability findings or artifact download.", Parameters: map[string]string{"repository": "Exact configured repository name", "name": "Optional common package name filter, at most 512 bytes", "group": "Optional common package group filter, at most 512 bytes", "version": "Optional exact package version filter, at most 512 bytes", "continuation_token": "Opaque continuationToken for the same repository and filters, at most 2048 bytes"}, Required: []string{"repository"}},
	{Name: "nexus_get_component", Access: mcp.AccessRead, Description: "Get one Nexus Repository component by opaque ID from nexus_list_components or nexus_search_components. Returns package coordinates and at most 20 asset metadata records, with asset_count and truncated. No artifact content, download URLs, uploader identities or security configuration.", Parameters: map[string]string{"id": "Opaque component id copied from a list/search result, at most 1024 bytes"}, Required: []string{"id"}},
}

func (n *nexus) Tools() []mcp.ToolDefinition {
	n.mu.RLock()
	defer n.mu.RUnlock()
	definitions := make([]mcp.ToolDefinition, len(tools))
	copy(definitions, tools)
	if len(n.repositories) == 0 {
		return definitions
	}
	scope := strings.Join(n.repositories, ", ")
	definitions[0].Description += " Configured repositories: " + scope + "."
	for index := range definitions {
		if _, ok := definitions[index].Parameters["repository"]; !ok {
			continue
		}
		parameters := make(map[string]string, len(definitions[index].Parameters))
		for key, description := range definitions[index].Parameters {
			parameters[key] = description
		}
		parameters["repository"] = "Exact configured repository name, one of: " + scope
		definitions[index].Parameters = parameters
	}
	return definitions
}

func (n *nexus) CompactSpec(name mcp.ToolName) ([]mcp.CompactField, bool) {
	fields, ok := fieldCompactionSpecs[name]
	return fields, ok
}

type handler func(context.Context, *nexus, map[string]any) (*mcp.ToolResult, error)

var dispatch = map[mcp.ToolName]handler{"nexus_list_components": listComponents, "nexus_search_components": searchComponents, "nexus_get_component": getComponent}

func (n *nexus) Execute(ctx context.Context, name mcp.ToolName, args map[string]any) (*mcp.ToolResult, error) {
	fn, ok := dispatch[name]
	if !ok {
		return mcp.ErrResult(fmt.Errorf("nexus: unknown tool: %s", name))
	}
	for _, tool := range tools {
		if tool.Name != name {
			continue
		}
		for key := range args {
			if _, ok := tool.Parameters[key]; !ok {
				return mcp.ErrResult(fmt.Errorf("nexus: unsupported argument"))
			}
		}
	}
	n.mu.RLock()
	defer n.mu.RUnlock()
	if n.client == nil {
		return mcp.ErrResult(mcp.ErrNotConfigured)
	}
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	return fn(ctx, n, args)
}

type component struct {
	ID         string  `json:"id"`
	Repository string  `json:"repository"`
	Format     string  `json:"format"`
	Group      *string `json:"group"`
	Name       string  `json:"name"`
	Version    *string `json:"version"`
	Assets     []asset `json:"assets,omitempty"`
	AssetCount *int    `json:"asset_count,omitempty"`
	Truncated  *bool   `json:"truncated,omitempty"`
}

type asset struct {
	Repository   string            `json:"repository,omitempty"`
	ID           string            `json:"id"`
	Path         string            `json:"path"`
	ContentType  *string           `json:"contentType,omitempty"`
	FileSize     *int64            `json:"fileSize,omitempty"`
	Checksum     map[string]string `json:"checksum,omitempty"`
	LastModified *string           `json:"lastModified,omitempty"`
}

type componentPage struct {
	Items             []component `json:"items"`
	ContinuationToken *string     `json:"continuationToken"`
}

func (n *nexus) validateComponent(value component) error {
	if !safeID(value.ID) || !slices.Contains(n.repositories, value.Repository) || value.Format == "" || !safeText(value.Format, 64) || value.Name == "" || !safeText(value.Name, 512) {
		return fmt.Errorf("nexus: invalid component identity or repository outside configured scope")
	}
	for _, text := range []*string{value.Group, value.Version} {
		if text != nil && !safeText(*text, 512) {
			return fmt.Errorf("nexus: component metadata exceeds safe text limits")
		}
	}
	return nil
}

func listComponents(ctx context.Context, n *nexus, args map[string]any) (*mcp.ToolResult, error) {
	return componentList(ctx, n, args, "/components")
}
func searchComponents(ctx context.Context, n *nexus, args map[string]any) (*mcp.ToolResult, error) {
	return componentList(ctx, n, args, "/search")
}

func componentList(ctx context.Context, n *nexus, args map[string]any, path string) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	repository, token := r.Str("repository"), r.Str("continuation_token")
	name, group, version := r.Str("name"), r.Str("group"), r.Str("version")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(fmt.Errorf("nexus: invalid argument type"))
	}
	if !slices.Contains(n.repositories, repository) {
		return mcp.ErrResult(fmt.Errorf("nexus: repository must be a configured approved name"))
	}
	if !safeText(token, 2048) {
		return mcp.ErrResult(fmt.Errorf("nexus: invalid continuation_token"))
	}
	query := url.Values{"repository": {repository}}
	for key, value := range map[string]string{"continuationToken": token, "name": name, "group": group, "version": version} {
		if key != "continuationToken" && !safeText(value, 512) {
			return mcp.ErrResult(fmt.Errorf("nexus: filter exceeds safe text limits"))
		}
		if value != "" {
			query.Set(key, value)
		}
	}
	page, err := n.page(ctx, path, query)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return boundedResult(page)
}

func (n *nexus) page(ctx context.Context, path string, query url.Values) (*componentPage, error) {
	var page componentPage
	if err := n.get(ctx, path, query, &page); err != nil {
		return nil, err
	}
	if page.Items == nil || len(page.Items) > 100 {
		return nil, fmt.Errorf("nexus: missing items or server page exceeds 100 components; no incomplete page returned")
	}
	if page.ContinuationToken != nil && (!safeText(*page.ContinuationToken, 2048) || *page.ContinuationToken == "" || *page.ContinuationToken == query.Get("continuationToken")) {
		return nil, fmt.Errorf("nexus: invalid or repeated continuation token")
	}
	for index := range page.Items {
		item := &page.Items[index]
		if err := n.validateComponent(*item); err != nil {
			return nil, err
		}
		if item.Repository != query.Get("repository") {
			return nil, fmt.Errorf("nexus: returned component does not belong to requested repository")
		}
		item.Assets, item.AssetCount, item.Truncated = nil, nil, nil
	}
	return &page, nil
}

func getComponent(ctx context.Context, n *nexus, args map[string]any) (*mcp.ToolResult, error) {
	id, err := mcp.ArgStr(args, "id")
	if err != nil || !safeID(id) {
		return mcp.ErrResult(fmt.Errorf("nexus: id must be one bounded opaque component ID without path separators"))
	}
	var value component
	if err := n.get(ctx, "/components/"+url.PathEscape(id), nil, &value); err != nil {
		return mcp.ErrResult(err)
	}
	if err := n.validateComponent(value); err != nil {
		return mcp.ErrResult(err)
	}
	if value.ID != id {
		return mcp.ErrResult(fmt.Errorf("nexus: returned component ID does not match request"))
	}
	count := len(value.Assets)
	truncated := count > 20
	if truncated {
		value.Assets = value.Assets[:20]
	}
	for index := range value.Assets {
		if value.Assets[index].Repository != "" && value.Assets[index].Repository != value.Repository {
			return mcp.ErrResult(fmt.Errorf("nexus: asset does not belong to requested component repository"))
		}
		value.Assets[index].Repository = ""
		if err := sanitizeAsset(&value.Assets[index]); err != nil {
			return mcp.ErrResult(err)
		}
	}
	value.AssetCount, value.Truncated = &count, &truncated
	return boundedResult(value)
}

func sanitizeAsset(value *asset) error {
	if !safeID(value.ID) || !safeText(value.Path, 2048) || value.Path == "" {
		return fmt.Errorf("nexus: asset identity or path exceeds safe text limits")
	}
	for _, text := range []*string{value.ContentType, value.LastModified} {
		if text != nil && !safeText(*text, 256) {
			return fmt.Errorf("nexus: asset metadata exceeds safe text limits")
		}
	}
	if value.FileSize != nil && *value.FileSize < 0 {
		return fmt.Errorf("nexus: invalid asset file size")
	}
	for algorithm, checksum := range value.Checksum {
		if !slices.Contains([]string{"md5", "sha1", "sha256", "sha512"}, algorithm) {
			delete(value.Checksum, algorithm)
			continue
		}
		if checksum == "" || len(checksum) > 128 || strings.Trim(checksum, "0123456789abcdefABCDEF") != "" {
			return fmt.Errorf("nexus: invalid asset checksum")
		}
	}
	return nil
}

func boundedResult(value any) (*mcp.ToolResult, error) {
	result, err := mcp.JSONResult(value)
	if err != nil {
		return mcp.ErrResult(fmt.Errorf("nexus: cannot encode response"))
	}
	if len(result.Data) > 128*1024 {
		return mcp.ErrResult(fmt.Errorf("nexus: metadata output exceeds 128 KiB; narrow the search, no incomplete page returned"))
	}
	return result, nil
}

func (n *nexus) get(ctx context.Context, path string, query url.Values, target any) error {
	endpoint := n.baseURL + "/service/rest/v1" + path
	if len(query) > 0 {
		endpoint += "?" + query.Encode()
	}
	for attempt := 0; attempt < 3; attempt++ {
		request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
		if err != nil {
			return fmt.Errorf("nexus: cannot construct request")
		}
		request.SetBasicAuth(n.username, n.password)
		request.Header.Set("Accept", "application/json")
		response, err := n.client.Do(request)
		if err != nil {
			return fmt.Errorf("nexus: request failed; check connectivity, TLS trust and deadline")
		}
		if response.StatusCode == http.StatusTooManyRequests && attempt < 2 {
			_ = response.Body.Close()
			if err := retryWait(ctx, response.Header.Get("Retry-After")); err != nil {
				return err
			}
			continue
		}
		if response.StatusCode != http.StatusOK {
			_ = response.Body.Close()
			return fmt.Errorf("nexus: HTTP %d; verify repository access and dedicated reader credentials", response.StatusCode)
		}
		payload, err := io.ReadAll(io.LimitReader(response.Body, maxResponseBytes+1))
		_ = response.Body.Close()
		if err != nil {
			return fmt.Errorf("nexus: cannot read response within deadline")
		}
		if len(payload) > maxResponseBytes {
			return fmt.Errorf("nexus: response exceeds 2 MiB limit")
		}
		if err := json.Unmarshal(payload, target); err != nil {
			return fmt.Errorf("nexus: malformed JSON response")
		}
		return nil
	}
	return fmt.Errorf("nexus: rate limited; retry later")
}

func retryWait(ctx context.Context, header string) error {
	wait := time.Second
	if seconds, err := strconv.ParseInt(header, 10, 32); err == nil && seconds >= 0 {
		wait = time.Duration(seconds) * time.Second
	} else if parsed, err := http.ParseTime(header); err == nil {
		wait = max(time.Until(parsed), 0)
	}
	if wait > 10*time.Second {
		return fmt.Errorf("nexus: rate limited; retry later")
	}
	if wait < 50*time.Millisecond {
		wait = 50 * time.Millisecond
	}
	timer := time.NewTimer(wait)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return fmt.Errorf("nexus: request deadline reached while rate limited")
	case <-timer.C:
		return nil
	}
}
