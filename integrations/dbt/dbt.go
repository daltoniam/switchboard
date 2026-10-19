package dbt

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	mcp "github.com/daltoniam/switchboard"
	"github.com/daltoniam/switchboard/compact"
)

//go:embed compact.yaml
var compactYAML []byte

var compactResult = compact.MustLoadWithOverlay("dbt", compactYAML, compact.Options{Strict: false})
var fieldCompactionSpecs = compactResult.Specs
var maxBytesByTool = compactResult.MaxBytes

// dbt talks to the dbt Cloud (dbt platform) Admin API v2. Every tool is
// read-only: the integration never triggers, retries, or cancels jobs.
type dbt struct {
	token             string
	accountID         string
	prodJobID         string
	baseURL           string
	client            *http.Client
	manifests         *manifestCache
	inlineArtifactMax int
}

const (
	defaultBaseURL     = "https://cloud.getdbt.com"
	maxResponseSize    = 10 * 1024 * 1024  // 10 MB for API envelopes
	maxArtifactSize    = 256 * 1024 * 1024 // 256 MB for manifest.json and friends
	defaultInlineMax   = 512 * 1024        // artifacts larger than this need a jq filter
	defaultLimit       = 10
	maxLimit           = 100
	defaultLineageMax  = 500
	manifestCacheSize  = 2
	compiledCodeMaxLen = 4000
	logTailLines       = 60
)

var (
	_ mcp.Integration                = (*dbt)(nil)
	_ mcp.FieldCompactionIntegration = (*dbt)(nil)
	_ mcp.ToolMaxBytesIntegration    = (*dbt)(nil)
	_ mcp.PlainTextCredentials       = (*dbt)(nil)
	_ mcp.PlaceholderHints           = (*dbt)(nil)
	_ mcp.OptionalCredentials        = (*dbt)(nil)
)

func New() mcp.Integration {
	return &dbt{
		client:            &http.Client{Timeout: 120 * time.Second},
		baseURL:           defaultBaseURL,
		manifests:         newManifestCache(manifestCacheSize),
		inlineArtifactMax: defaultInlineMax,
	}
}

func (d *dbt) Name() string { return "dbt" }

func (d *dbt) PlainTextKeys() []string { return []string{"account_id", "base_url", "prod_job_id"} }

func (d *dbt) OptionalKeys() []string { return []string{"base_url", "prod_job_id"} }

func (d *dbt) Placeholders() map[string]string {
	return map[string]string{
		"token":       "dbt Cloud service token (Job Viewer permission is enough) or personal access token",
		"account_id":  "Numeric dbt Cloud account ID (from the account settings URL)",
		"base_url":    "https://cloud.getdbt.com (default); cell or regional access URL such as https://ab123.us1.dbt.com, https://emea.dbt.com, https://au.dbt.com",
		"prod_job_id": "Optional production job ID whose latest successful run supplies manifest.json for lineage tools",
	}
}

func (d *dbt) Configure(_ context.Context, creds mcp.Credentials) error {
	token := strings.TrimSpace(creds["token"])
	for _, prefix := range []string{"Token ", "Bearer "} {
		token = strings.TrimSpace(strings.TrimPrefix(token, prefix))
	}
	if token == "" {
		return fmt.Errorf("dbt: token is required")
	}
	accountID := strings.TrimSpace(creds["account_id"])
	if accountID == "" {
		return fmt.Errorf("dbt: account_id is required")
	}
	if _, err := strconv.ParseInt(accountID, 10, 64); err != nil {
		return fmt.Errorf("dbt: account_id must be numeric, got %q", accountID)
	}
	prodJobID := strings.TrimSpace(creds["prod_job_id"])
	if prodJobID != "" {
		if _, err := strconv.ParseInt(prodJobID, 10, 64); err != nil {
			return fmt.Errorf("dbt: prod_job_id must be numeric, got %q", prodJobID)
		}
	}
	d.token = token
	d.accountID = accountID
	d.prodJobID = prodJobID
	if v := strings.TrimSpace(creds["base_url"]); v != "" {
		d.baseURL = strings.TrimRight(v, "/")
	}
	return nil
}

func (d *dbt) Healthy(ctx context.Context) bool {
	if d.client == nil || d.token == "" || d.accountID == "" {
		return false
	}
	_, err := d.getJSON(ctx, "/", nil)
	return err == nil
}

func (d *dbt) Tools() []mcp.ToolDefinition { return tools }

func (d *dbt) CompactSpec(toolName mcp.ToolName) ([]mcp.CompactField, bool) {
	fields, ok := fieldCompactionSpecs[toolName]
	return fields, ok
}

func (d *dbt) MaxBytes(toolName mcp.ToolName) (int, bool) {
	n, ok := maxBytesByTool[toolName]
	return n, ok
}

func (d *dbt) Execute(ctx context.Context, toolName mcp.ToolName, args map[string]any) (*mcp.ToolResult, error) {
	fn, ok := dispatch[toolName]
	if !ok {
		return &mcp.ToolResult{Data: fmt.Sprintf("unknown tool: %s", toolName), IsError: true}, nil
	}
	return fn(ctx, d, args)
}

// errNotFound marks a 404, which artifact callers treat as "not produced yet"
// rather than a failure.
type errNotFound struct{ msg string }

func (e *errNotFound) Error() string { return e.msg }

// open issues an authenticated GET against /api/v2/accounts/{account_id}{path}
// and returns the body for the caller to stream. dbt documents
// "Authorization: Token <token>" (Bearer is accepted as an alias).
func (d *dbt) open(ctx context.Context, path string, query url.Values) (io.ReadCloser, error) {
	u := d.baseURL + "/api/v2/accounts/" + d.accountID + path
	if len(query) > 0 {
		u += "?" + query.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Token "+d.token)
	req.Header.Set("Accept", "application/json")

	resp, err := d.client.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 400 {
		return resp.Body, nil
	}
	defer func() { _ = resp.Body.Close() }()
	data, _ := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
	apiErr := fmt.Errorf("dbt API error (%d) for %s: %s", resp.StatusCode, path, strings.TrimSpace(string(data)))
	switch {
	case resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500:
		return nil, &mcp.RetryableError{
			StatusCode: resp.StatusCode,
			Err:        apiErr,
			RetryAfter: mcp.ParseRetryAfter(resp.Header.Get("Retry-After")),
		}
	case resp.StatusCode == http.StatusNotFound:
		return nil, &errNotFound{msg: apiErr.Error()}
	case resp.StatusCode == http.StatusForbidden:
		return nil, fmt.Errorf("%w (check the token's permission set and project scope; a valid token still gets 403 when the caller's IP is not on the account's IP allowlist)", apiErr)
	case resp.StatusCode == http.StatusUnauthorized:
		return nil, fmt.Errorf("%w (token rejected; use a service token or personal access token for this account)", apiErr)
	default:
		return nil, apiErr
	}
}

// envelope is the v2 response wrapper: {"status": {...}, "data": ..., "extra": {...}}.
type envelope struct {
	Data  json.RawMessage `json:"data"`
	Extra struct {
		Pagination struct {
			Count      int `json:"count"`
			TotalCount int `json:"total_count"`
		} `json:"pagination"`
	} `json:"extra"`
}

func (d *dbt) getEnvelope(ctx context.Context, path string, query url.Values) (*envelope, error) {
	body, err := d.open(ctx, path, query)
	if err != nil {
		return nil, err
	}
	defer func() { _ = body.Close() }()
	var env envelope
	if err := json.NewDecoder(io.LimitReader(body, maxResponseSize)).Decode(&env); err != nil {
		return nil, fmt.Errorf("dbt: decode response for %s: %w", path, err)
	}
	return &env, nil
}

// getJSON returns the decoded "data" member of a v2 response.
func (d *dbt) getJSON(ctx context.Context, path string, query url.Values) (any, error) {
	env, err := d.getEnvelope(ctx, path, query)
	if err != nil {
		return nil, err
	}
	var out any
	if len(env.Data) > 0 {
		if err := json.Unmarshal(env.Data, &out); err != nil {
			return nil, fmt.Errorf("dbt: decode data for %s: %w", path, err)
		}
	}
	return out, nil
}

// getInto decodes the "data" member of a v2 response into v.
func (d *dbt) getInto(ctx context.Context, path string, query url.Values, v any) error {
	env, err := d.getEnvelope(ctx, path, query)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(env.Data, v); err != nil {
		return fmt.Errorf("dbt: decode data for %s: %w", path, err)
	}
	return nil
}

func artifactPath(runID int64, path string) string {
	return fmt.Sprintf("/runs/%d/artifacts/%s", runID, path)
}

func stepQuery(step int) url.Values {
	if step <= 0 {
		return nil
	}
	return url.Values{"step": {strconv.Itoa(step)}}
}

// loadManifest returns the decoded manifest.json for a run step, streaming
// the download straight into the partial struct and caching the result.
func (d *dbt) loadManifest(ctx context.Context, runID int64, step int) (*manifest, error) {
	key := fmt.Sprintf("%d:%d", runID, step)
	if m, ok := d.manifests.get(key); ok {
		return m, nil
	}
	body, err := d.open(ctx, artifactPath(runID, "manifest.json"), stepQuery(step))
	if err != nil {
		return nil, err
	}
	defer func() { _ = body.Close() }()
	m, err := decodeManifest(io.LimitReader(body, maxArtifactSize))
	if err != nil {
		return nil, err
	}
	d.manifests.put(key, m)
	return m, nil
}

type handlerFunc func(ctx context.Context, d *dbt, args map[string]any) (*mcp.ToolResult, error)

func clampLimit(n int) int {
	if n > maxLimit {
		return maxLimit
	}
	if n < 1 {
		return defaultLimit
	}
	return n
}

var dispatch = map[mcp.ToolName]handlerFunc{
	mcp.ToolName("dbt_list_jobs_runs"):               listJobsRuns,
	mcp.ToolName("dbt_get_job_run_error"):            getJobRunError,
	mcp.ToolName("dbt_get_job_run_regression_range"): getJobRunRegressionRange,
	mcp.ToolName("dbt_get_job_run_artifact"):         getJobRunArtifact,
	mcp.ToolName("dbt_get_manifest_nodes"):           getManifestNodes,
	mcp.ToolName("dbt_get_model_lineage"):            getModelLineage,
}
