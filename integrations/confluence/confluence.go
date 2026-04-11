package confluence

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	mcp "github.com/daltoniam/switchboard"
)

// Compile-time interface assertions.
var (
	_ mcp.Integration                 = (*confluence)(nil)
	_ mcp.FieldCompactionIntegration  = (*confluence)(nil)
	_ mcp.PlainTextCredentials        = (*confluence)(nil)
	_ mcp.MaxResponseBytesIntegration = (*confluence)(nil)
)

// confluenceMaxResponseBytes raises the response cap for Confluence above the
// server default. Confluence page bodies (Atlassian Document Format or Storage
// Format XHTML) routinely exceed 50KB for richly authored pages, and the whole
// point of tools like confluence_get_page is to return that content to the LLM.
const confluenceMaxResponseBytes = 256 * 1024 // 256KB

type confluence struct {
	email    string
	apiToken string
	domain   string
	client   *http.Client
	baseURL  string // https://{domain}.atlassian.net/wiki/api/v2
	v1URL    string // https://{domain}.atlassian.net/wiki/rest/api
}

func New() mcp.Integration {
	return &confluence{
		client: &http.Client{Timeout: 30 * time.Second},
	}
}

func (c *confluence) Name() string { return "confluence" }

func (c *confluence) PlainTextKeys() []string { return []string{"email", "domain"} }

func (c *confluence) Configure(_ context.Context, creds mcp.Credentials) error {
	c.email = creds["email"]
	c.apiToken = creds["api_token"]
	c.domain = creds["domain"]
	if c.email == "" {
		return fmt.Errorf("confluence: email is required")
	}
	if c.apiToken == "" {
		return fmt.Errorf("confluence: api_token is required")
	}
	if c.domain == "" {
		return fmt.Errorf("confluence: domain is required")
	}
	c.baseURL = fmt.Sprintf("https://%s.atlassian.net/wiki/api/v2", c.domain)
	c.v1URL = fmt.Sprintf("https://%s.atlassian.net/wiki/rest/api", c.domain)
	return nil
}

func (c *confluence) Healthy(ctx context.Context) bool {
	q := queryEncode(map[string]string{"limit": "1"})
	_, err := c.get(ctx, "/spaces%s", q)
	return err == nil
}

func (c *confluence) Tools() []mcp.ToolDefinition {
	return tools
}

func (c *confluence) CompactSpec(toolName string) ([]mcp.CompactField, bool) {
	fields, ok := fieldCompactionSpecs[toolName]
	return fields, ok
}

func (c *confluence) MaxResponseBytes() int { return confluenceMaxResponseBytes }

func (c *confluence) Execute(ctx context.Context, toolName string, args map[string]any) (*mcp.ToolResult, error) {
	fn, ok := dispatch[toolName]
	if !ok {
		return mcp.ErrResult(fmt.Errorf("unknown tool: %s", toolName))
	}
	return fn(ctx, c, args)
}

// --- HTTP helpers ---

func (c *confluence) authHeader() string {
	creds := c.email + ":" + c.apiToken
	return "Basic " + base64.StdEncoding.EncodeToString([]byte(creds))
}

func (c *confluence) doRequest(ctx context.Context, method, fullURL string, body any) (json.RawMessage, error) {
	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		bodyReader = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, fullURL, bodyReader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", c.authHeader())
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	const maxResponseSize = 10 * 1024 * 1024 // 10 MB
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseSize))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode == 429 || resp.StatusCode >= 500 {
		re := &mcp.RetryableError{StatusCode: resp.StatusCode, Err: fmt.Errorf("confluence API error (%d): %s", resp.StatusCode, string(data))}
		re.RetryAfter = mcp.ParseRetryAfter(resp.Header.Get("Retry-After"))
		return nil, re
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("confluence API error (%d): %s", resp.StatusCode, string(data))
	}
	if resp.StatusCode == 204 || len(data) == 0 {
		return json.RawMessage(`{"status":"success"}`), nil
	}
	return json.RawMessage(data), nil
}

// v2 API helpers
func (c *confluence) get(ctx context.Context, pathFmt string, args ...any) (json.RawMessage, error) {
	return c.doRequest(ctx, "GET", c.baseURL+fmt.Sprintf(pathFmt, args...), nil)
}

func (c *confluence) post(ctx context.Context, path string, body any) (json.RawMessage, error) {
	return c.doRequest(ctx, "POST", c.baseURL+path, body)
}

func (c *confluence) put(ctx context.Context, path string, body any) (json.RawMessage, error) {
	return c.doRequest(ctx, "PUT", c.baseURL+path, body)
}

func (c *confluence) del(ctx context.Context, pathFmt string, args ...any) (json.RawMessage, error) {
	return c.doRequest(ctx, "DELETE", c.baseURL+fmt.Sprintf(pathFmt, args...), nil)
}

// v1 API helper (for CQL search)
func (c *confluence) v1Get(ctx context.Context, pathFmt string, args ...any) (json.RawMessage, error) {
	return c.doRequest(ctx, "GET", c.v1URL+fmt.Sprintf(pathFmt, args...), nil)
}

type handlerFunc func(ctx context.Context, c *confluence, args map[string]any) (*mcp.ToolResult, error)

// queryEncode builds a query string from non-empty key/value pairs.
func queryEncode(params map[string]string) string {
	vals := url.Values{}
	for k, v := range params {
		if v != "" {
			vals.Set(k, v)
		}
	}
	if len(vals) == 0 {
		return ""
	}
	return "?" + vals.Encode()
}

// --- Dispatch map ---

var dispatch = map[string]handlerFunc{
	// Spaces
	"confluence_list_spaces": listSpaces,
	"confluence_get_space":   getSpace,
	"confluence_search":      search,

	// Pages
	"confluence_list_pages":        listPages,
	"confluence_get_page":          getPage,
	"confluence_create_page":       createPage,
	"confluence_update_page":       updatePage,
	"confluence_delete_page":       deletePage,
	"confluence_get_page_children": getPageChildren,

	// Blog Posts
	"confluence_list_blog_posts":  listBlogPosts,
	"confluence_get_blog_post":    getBlogPost,
	"confluence_create_blog_post": createBlogPost,
	"confluence_update_blog_post": updateBlogPost,
	"confluence_delete_blog_post": deleteBlogPost,

	// Comments
	"confluence_list_comments":  listComments,
	"confluence_create_comment": createComment,
	"confluence_delete_comment": deleteComment,
}
