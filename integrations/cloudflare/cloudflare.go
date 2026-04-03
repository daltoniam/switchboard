package cloudflare

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
)

type cloudflare struct {
	apiToken  string
	accountID string
	client    *http.Client
	baseURL   string
}

var _ mcp.FieldCompactionIntegration = (*cloudflare)(nil)

const maxResponseSize = 10 * 1024 * 1024 // 10 MB

func New() mcp.Integration {
	return &cloudflare{
		client:  &http.Client{Timeout: 30 * time.Second},
		baseURL: "https://api.cloudflare.com/client/v4",
	}
}

func (c *cloudflare) Name() string { return "cloudflare" }

func (c *cloudflare) Configure(_ context.Context, creds mcp.Credentials) error {
	c.apiToken = creds["api_token"]
	if c.apiToken == "" {
		return fmt.Errorf("cloudflare: api_token is required")
	}
	c.accountID = creds["account_id"]
	if v := creds["base_url"]; v != "" {
		c.baseURL = strings.TrimRight(v, "/")
	}
	return nil
}

func (c *cloudflare) Healthy(ctx context.Context) bool {
	if c.client == nil || c.apiToken == "" {
		return false
	}
	_, err := c.get(ctx, "/user/tokens/verify")
	return err == nil
}

func (c *cloudflare) Tools() []mcp.ToolDefinition {
	return tools
}

func (c *cloudflare) Execute(ctx context.Context, toolName string, args map[string]any) (*mcp.ToolResult, error) {
	fn, ok := dispatch[toolName]
	if !ok {
		return &mcp.ToolResult{Data: fmt.Sprintf("unknown tool: %s", toolName), IsError: true}, nil
	}
	return fn(ctx, c, args)
}

func (c *cloudflare) CompactSpec(toolName string) ([]mcp.CompactField, bool) {
	fields, ok := fieldCompactionSpecs[toolName]
	return fields, ok
}

func (c *cloudflare) PlainTextKeys() []string {
	return []string{"account_id"}
}

func (c *cloudflare) OptionalKeys() []string {
	return []string{"account_id", "base_url"}
}

// --- HTTP helpers ---

func (c *cloudflare) doRequest(ctx context.Context, method, path string, body any) (json.RawMessage, error) {
	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		bodyReader = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, bodyReader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.apiToken)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	data, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseSize))
	if err != nil {
		return nil, err
	}
	return c.handleResponse(resp, data)
}

func (c *cloudflare) get(ctx context.Context, pathFmt string, args ...any) (json.RawMessage, error) {
	return c.doRequest(ctx, "GET", fmt.Sprintf(pathFmt, args...), nil)
}

func (c *cloudflare) post(ctx context.Context, path string, body any) (json.RawMessage, error) {
	return c.doRequest(ctx, "POST", path, body)
}

func (c *cloudflare) put(ctx context.Context, path string, body any) (json.RawMessage, error) {
	return c.doRequest(ctx, "PUT", path, body)
}

func (c *cloudflare) patch(ctx context.Context, path string, body any) (json.RawMessage, error) {
	return c.doRequest(ctx, "PATCH", path, body)
}

func (c *cloudflare) del(ctx context.Context, pathFmt string, args ...any) (json.RawMessage, error) {
	return c.doRequest(ctx, "DELETE", fmt.Sprintf(pathFmt, args...), nil)
}

// doRawRequest sends a request with a raw string body (not JSON-marshaled).
// Used for KV value writes where the API expects raw bytes with text/plain content type.
func (c *cloudflare) doRawRequest(ctx context.Context, method, path, body string) (json.RawMessage, error) {
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, strings.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.apiToken)
	req.Header.Set("Content-Type", "text/plain")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	data, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseSize))
	if err != nil {
		return nil, err
	}
	return c.handleResponse(resp, data)
}

// getRaw performs a GET and returns the response body wrapped in a {"value":"..."} JSON
// envelope. Used for KV value reads where the API returns raw bytes, not a JSON envelope.
func (c *cloudflare) getRaw(ctx context.Context, pathFmt string, args ...any) (json.RawMessage, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", c.baseURL+fmt.Sprintf(pathFmt, args...), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.apiToken)

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	data, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseSize))
	if err != nil {
		return nil, err
	}
	if _, err := c.handleResponse(resp, data); err != nil {
		return nil, err
	}
	wrapped, err := json.Marshal(map[string]string{"value": string(data)})
	if err != nil {
		return nil, err
	}
	return json.RawMessage(wrapped), nil
}

// handleResponse processes an HTTP response, returning retryable errors for 429/5xx,
// plain errors for 4xx, and the raw body for success responses.
func (c *cloudflare) handleResponse(resp *http.Response, data []byte) (json.RawMessage, error) {
	if resp.StatusCode == 429 || resp.StatusCode >= 500 {
		re := &mcp.RetryableError{StatusCode: resp.StatusCode, Err: fmt.Errorf("cloudflare API error (%d): %s", resp.StatusCode, string(data))}
		re.RetryAfter = mcp.ParseRetryAfter(resp.Header.Get("Retry-After"))
		return nil, re
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("cloudflare API error (%d): %s", resp.StatusCode, string(data))
	}
	if resp.StatusCode == 204 || len(data) == 0 {
		return json.RawMessage(`{"status":"success"}`), nil
	}
	return json.RawMessage(data), nil
}

// --- Helpers ---

type handlerFunc func(ctx context.Context, c *cloudflare, args map[string]any) (*mcp.ToolResult, error)

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

// acctID returns the account_id from args, falling back to the configured default.
// Returns an error if neither is set — prevents malformed URLs like /accounts//workers/scripts.
func (c *cloudflare) acctID(args map[string]any) (string, error) {
	v, _ := mcp.ArgStr(args, "account_id")
	if v != "" {
		return v, nil
	}
	if c.accountID != "" {
		return c.accountID, nil
	}
	return "", fmt.Errorf("account_id is required — pass it as an argument or set CLOUDFLARE_ACCOUNT_ID")
}

// --- Dispatch map ---

var dispatch = map[string]handlerFunc{
	// Zones
	"cloudflare_list_zones":  listZones,
	"cloudflare_get_zone":    getZone,
	"cloudflare_create_zone": createZone,
	"cloudflare_edit_zone":   editZone,
	"cloudflare_delete_zone": deleteZone,
	"cloudflare_purge_cache": purgeCache,

	// DNS Records
	"cloudflare_list_dns_records":  listDNSRecords,
	"cloudflare_get_dns_record":    getDNSRecord,
	"cloudflare_create_dns_record": createDNSRecord,
	"cloudflare_update_dns_record": updateDNSRecord,
	"cloudflare_delete_dns_record": deleteDNSRecord,

	// Workers
	"cloudflare_list_workers":       listWorkers,
	"cloudflare_get_worker":         getWorker,
	"cloudflare_delete_worker":      deleteWorker,
	"cloudflare_list_worker_routes": listWorkerRoutes,

	// Pages
	"cloudflare_list_pages_projects":       listPagesProjects,
	"cloudflare_get_pages_project":         getPagesProject,
	"cloudflare_list_pages_deployments":    listPagesDeployments,
	"cloudflare_get_pages_deployment":      getPagesDeployment,
	"cloudflare_delete_pages_deployment":   deletePagesDeployment,
	"cloudflare_rollback_pages_deployment": rollbackPagesDeployment,

	// R2
	"cloudflare_list_r2_buckets":  listR2Buckets,
	"cloudflare_create_r2_bucket": createR2Bucket,
	"cloudflare_delete_r2_bucket": deleteR2Bucket,

	// KV
	"cloudflare_list_kv_namespaces":  listKVNamespaces,
	"cloudflare_create_kv_namespace": createKVNamespace,
	"cloudflare_delete_kv_namespace": deleteKVNamespace,
	"cloudflare_list_kv_keys":        listKVKeys,
	"cloudflare_get_kv_value":        getKVValue,
	"cloudflare_put_kv_value":        putKVValue,
	"cloudflare_delete_kv_value":     deleteKVValue,

	// D1
	"cloudflare_list_d1_databases":  listD1Databases,
	"cloudflare_get_d1_database":    getD1Database,
	"cloudflare_create_d1_database": createD1Database,
	"cloudflare_delete_d1_database": deleteD1Database,
	"cloudflare_query_d1_database":  queryD1Database,

	// Firewall / WAF
	"cloudflare_list_waf_rulesets": listWAFRulesets,
	"cloudflare_get_waf_ruleset":   getWAFRuleset,

	// Load Balancers
	"cloudflare_list_load_balancers": listLoadBalancers,
	"cloudflare_get_load_balancer":   getLoadBalancer,
	"cloudflare_list_lb_pools":       listLBPools,
	"cloudflare_get_lb_pool":         getLBPool,
	"cloudflare_list_lb_monitors":    listLBMonitors,

	// Analytics
	"cloudflare_get_zone_analytics": getZoneAnalytics,

	// Accounts
	"cloudflare_list_accounts":        listAccounts,
	"cloudflare_get_account":          getAccount,
	"cloudflare_list_account_members": listAccountMembers,
}
