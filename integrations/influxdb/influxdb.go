package influxdb

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	mcp "github.com/daltoniam/switchboard"
	"github.com/daltoniam/switchboard/compact"
)

//go:embed compact.yaml
var compactYAML []byte

var compactResult = compact.MustLoadWithOverlay("influxdb", compactYAML, compact.Options{Strict: false})
var fieldCompactionSpecs = compactResult.Specs
var idPattern = regexp.MustCompile(`^[0-9a-f]{16}$`)
var versionPattern = regexp.MustCompile(`^v?2\.8\.[0-9]+(?:[-+].*)?$`)

const maxResponseBytes = 1024 * 1024

type influxdb struct {
	baseURL, token, orgID string
	client                *http.Client
}

var (
	_ mcp.Integration                = (*influxdb)(nil)
	_ mcp.FieldCompactionIntegration = (*influxdb)(nil)
	_ mcp.PlainTextCredentials       = (*influxdb)(nil)
	_ mcp.OptionalCredentials        = (*influxdb)(nil)
	_ mcp.PlaceholderHints           = (*influxdb)(nil)
)

func New() mcp.Integration                { return &influxdb{} }
func (*influxdb) Name() string            { return "influxdb" }
func (*influxdb) PlainTextKeys() []string { return []string{"base_url", "org_id", "ca_cert"} }
func (*influxdb) OptionalKeys() []string  { return []string{"ca_cert"} }
func (*influxdb) Placeholders() map[string]string {
	return map[string]string{
		"base_url":  "https://influx.example.com (InfluxDB OSS 2.8 only)",
		"org_id":    "16-character hexadecimal organization ID",
		"api_token": "Custom token: read tasks for this org and read selected buckets only",
		"ca_cert":   "Optional PEM CA certificate; escape newlines as \\n",
	}
}

func (i *influxdb) Configure(_ context.Context, creds mcp.Credentials) error {
	if i.client != nil {
		i.client.CloseIdleConnections()
	}
	i.client = nil
	base := strings.TrimRight(strings.TrimSpace(creds["base_url"]), "/")
	parsed, err := url.Parse(base)
	if err != nil || parsed.Scheme != "https" || parsed.Hostname() == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.ForceQuery || parsed.Fragment != "" || parsed.RawFragment != "" || strings.Contains(parsed.Path, "..") || parsed.RawPath != "" {
		return fmt.Errorf("influxdb: base_url must be a trusted HTTPS URL without credentials, query or fragment")
	}
	token := strings.TrimSpace(creds["api_token"])
	if token == "" || strings.ContainsAny(token, "\r\n") {
		return fmt.Errorf("influxdb: api_token is required and must not contain newlines")
	}
	orgID := strings.TrimSpace(creds["org_id"])
	if !validID(orgID) {
		return fmt.Errorf("influxdb: org_id must be a nonzero 16-character hexadecimal ID")
	}
	tlsConfig := &tls.Config{MinVersion: tls.VersionTLS12}
	if certificate := strings.TrimSpace(strings.ReplaceAll(creds["ca_cert"], `\n`, "\n")); certificate != "" {
		pool, err := x509.SystemCertPool()
		if err != nil {
			pool = x509.NewCertPool()
		}
		if !pool.AppendCertsFromPEM([]byte(certificate)) {
			return fmt.Errorf("influxdb: ca_cert must contain a PEM certificate")
		}
		tlsConfig.RootCAs = pool
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig = tlsConfig
	i.client = &http.Client{Transport: transport, Timeout: 10 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	i.baseURL = base
	i.token = token
	i.orgID = orgID
	return nil
}

func validID(value string) bool { return idPattern.MatchString(value) && value != "0000000000000000" }

func (i *influxdb) Healthy(ctx context.Context) bool {
	if i.client == nil {
		return false
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	var health struct {
		Version string `json:"version"`
		Status  string `json:"status"`
	}
	if err := i.get(ctx, "/health", nil, &health); err != nil || health.Status != "pass" || !versionPattern.MatchString(health.Version) {
		return false
	}
	var response struct {
		Tasks *[]task `json:"tasks"`
	}
	if err := i.get(ctx, "/api/v2/tasks", url.Values{"orgID": {i.orgID}, "type": {"basic"}, "limit": {"1"}}, &response); err != nil || response.Tasks == nil {
		return false
	}
	for _, item := range *response.Tasks {
		if item.OrgID != i.orgID {
			return false
		}
	}
	return true
}

func (*influxdb) Tools() []mcp.ToolDefinition { return tools }
func (*influxdb) CompactSpec(name mcp.ToolName) ([]mcp.CompactField, bool) {
	fields, ok := fieldCompactionSpecs[name]
	return fields, ok
}

type handler func(context.Context, *influxdb, map[string]any) (*mcp.ToolResult, error)

var dispatch = map[mcp.ToolName]handler{
	"influxdb_list_buckets":   listBuckets,
	"influxdb_list_tasks":     listTasks,
	"influxdb_get_task":       getTask,
	"influxdb_list_task_runs": listTaskRuns,
	"influxdb_get_task_run":   getTaskRun,
}

var tools = []mcp.ToolDefinition{
	{Name: "influxdb_list_buckets", Description: "List InfluxDB OSS 2.8 bucket metadata and retention durations for the configured organization. Start here for task, downsampling and retention diagnostics. No time-series queries or point data. Returns one bounded offset page, not a complete inventory.", Parameters: map[string]string{"limit": "Page size, integer 1..100 (default 50)", "offset": "Nonnegative bucket offset (default 0)"}, Access: mcp.AccessRead},
	{Name: "influxdb_list_tasks", Description: "List InfluxDB OSS 2.8 Flux task summaries, active state, schedule and last run status. Use after influxdb_list_buckets for downsampling task diagnosis. Excludes Flux scripts, logs and error text. One cursor page only.", Parameters: map[string]string{"limit": "Page size, integer 1..100 (default 50)", "after": "Task ID cursor from next_after, not an offset", "name": "Exact task name filter", "status": "Optional active or inactive"}, Access: mcp.AccessRead},
	{Name: "influxdb_get_task", Description: "Get one InfluxDB OSS 2.8 task summary and schedule in the configured organization. Use a task_id from influxdb_list_tasks. Does not return executable Flux or lastRunError.", Parameters: map[string]string{"task_id": "16-character hexadecimal task ID from influxdb_list_tasks"}, Required: []string{"task_id"}, Access: mcp.AccessRead},
	{Name: "influxdb_list_task_runs", Description: "List recent InfluxDB OSS 2.8 task run states and timestamps to diagnose failed downsampling. Use a task_id from influxdb_list_tasks. Default last 24 hours, maximum 7 days. Status filtering applies locally to this page; next_after is preserved even when no runs match. No logs, retry or cancel.", Parameters: map[string]string{"task_id": "16-character hexadecimal task ID", "limit": "Page size, integer 1..100 (default 50)", "after": "Run ID cursor from next_after", "after_time": "Inclusive lower RFC3339 time bound (default 24 hours before before_time)", "before_time": "Upper RFC3339 time bound (default now)", "status": "Optional local page filter: scheduled, started, success, failed or canceled"}, Required: []string{"task_id"}, Access: mcp.AccessRead},
	{Name: "influxdb_get_task_run", Description: "Get one InfluxDB OSS 2.8 task run state and timestamps, without logs or script error bodies. Use task_id and run_id from influxdb_list_task_runs. Validates the parent task organization first.", Parameters: map[string]string{"task_id": "16-character hexadecimal task ID", "run_id": "16-character hexadecimal run ID"}, Required: []string{"task_id", "run_id"}, Access: mcp.AccessRead},
}

func (i *influxdb) Execute(ctx context.Context, name mcp.ToolName, args map[string]any) (*mcp.ToolResult, error) {
	fn, ok := dispatch[name]
	if !ok {
		return mcp.ErrResult(fmt.Errorf("influxdb: unknown tool: %s", name))
	}
	if i.client == nil {
		return mcp.ErrResult(fmt.Errorf("influxdb: integration is not configured"))
	}
	for key := range args {
		if key == "limit" || key == "offset" {
			if number, ok := args[key].(float64); ok && (math.IsNaN(number) || math.IsInf(number, 0) || math.Trunc(number) != number) {
				return mcp.ErrResult(fmt.Errorf("influxdb: pagination must use finite integers"))
			}
		}
		var allowed bool
		for _, tool := range tools {
			if tool.Name == name {
				_, allowed = tool.Parameters[key]
				break
			}
		}
		if !allowed {
			return mcp.ErrResult(fmt.Errorf("influxdb: unknown argument %q", key))
		}
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	return fn(ctx, i, args)
}

func (i *influxdb) get(ctx context.Context, path string, query url.Values, out any) error {
	target := i.baseURL + path
	if len(query) > 0 {
		target += "?" + query.Encode()
	}
	for attempt := 0; attempt < 3; attempt++ {
		request, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
		if err != nil {
			return fmt.Errorf("influxdb: cannot construct request")
		}
		request.Header.Set("Authorization", "Token "+i.token)
		request.Header.Set("Accept", "application/json")
		response, err := i.client.Do(request)
		if err != nil {
			return fmt.Errorf("influxdb: request failed (network, TLS or deadline); verify base_url and ca_cert")
		}
		if response.StatusCode == http.StatusTooManyRequests && attempt < 2 {
			delay := mcp.ParseRetryAfter(response.Header.Get("Retry-After"))
			_ = response.Body.Close()
			if delay == 0 {
				delay = 100 * time.Millisecond
			}
			if delay > 2*time.Second {
				return fmt.Errorf("influxdb: HTTP 429; retry later")
			}
			timer := time.NewTimer(delay)
			select {
			case <-ctx.Done():
				timer.Stop()
				return fmt.Errorf("influxdb: deadline while rate limited")
			case <-timer.C:
			}
			continue
		}
		if response.StatusCode != http.StatusOK {
			_ = response.Body.Close()
			return fmt.Errorf("influxdb: HTTP %d; verify OSS 2.8 URL, scoped read token and resource access", response.StatusCode)
		}
		data, err := io.ReadAll(io.LimitReader(response.Body, maxResponseBytes+1))
		_ = response.Body.Close()
		if err != nil {
			return fmt.Errorf("influxdb: response read failed")
		}
		if len(data) > maxResponseBytes {
			return fmt.Errorf("influxdb: response exceeds 1 MiB limit")
		}
		if len(data) == 0 || strings.TrimSpace(string(data)) == "null" || json.Unmarshal(data, out) != nil {
			return fmt.Errorf("influxdb: malformed JSON response")
		}
		return nil
	}
	return fmt.Errorf("influxdb: HTTP 429; retry later")
}

type task struct {
	ID              string  `json:"id"`
	OrgID           string  `json:"orgID"`
	Name            string  `json:"name"`
	Status          *string `json:"status"`
	Every           *string `json:"every"`
	Cron            *string `json:"cron"`
	Offset          *string `json:"offset"`
	LatestCompleted *string `json:"latestCompleted"`
	LastRunStatus   *string `json:"lastRunStatus"`
}
type run struct {
	ID           string  `json:"id"`
	TaskID       string  `json:"taskID"`
	Status       *string `json:"status"`
	ScheduledFor *string `json:"scheduledFor"`
	StartedAt    *string `json:"startedAt"`
	FinishedAt   *string `json:"finishedAt"`
	RequestedAt  *string `json:"requestedAt"`
}
type retentionRule struct {
	Type         string `json:"type"`
	EverySeconds int64  `json:"everySeconds"`
}
type bucket struct {
	ID             string          `json:"id"`
	OrgID          string          `json:"orgID"`
	Name           string          `json:"name"`
	RetentionRules []retentionRule `json:"retentionRules"`
}

func pagination(args map[string]any) (int, string, error) {
	limit := 50
	if _, ok := args["limit"]; ok {
		value, err := mcp.ArgInt(args, "limit")
		if err != nil || value < 1 || value > 100 {
			return 0, "", fmt.Errorf("influxdb: limit must be an integer 1..100")
		}
		limit = value
	}
	after, err := mcp.ArgStr(args, "after")
	if err != nil {
		return 0, "", err
	}
	if after != "" && !validID(after) {
		return 0, "", fmt.Errorf("influxdb: after must be a hexadecimal ID")
	}
	return limit, after, nil
}

func listBuckets(ctx context.Context, i *influxdb, args map[string]any) (*mcp.ToolResult, error) {
	limit, _, err := pagination(args)
	if err != nil {
		return mcp.ErrResult(err)
	}
	offset, err := mcp.ArgInt(args, "offset")
	if err != nil || offset < 0 || offset > 2147483547 {
		return mcp.ErrResult(fmt.Errorf("influxdb: offset must be a nonnegative bounded integer"))
	}
	var response struct {
		Buckets *[]bucket `json:"buckets"`
	}
	err = i.get(ctx, "/api/v2/buckets", url.Values{"orgID": {i.orgID}, "limit": {strconv.Itoa(limit)}, "offset": {strconv.Itoa(offset)}}, &response)
	if err != nil {
		return mcp.ErrResult(err)
	}
	if response.Buckets == nil {
		return mcp.ErrResult(fmt.Errorf("influxdb: missing buckets array"))
	}
	items := *response.Buckets
	for _, item := range items {
		if item.OrgID != i.orgID || !validID(item.ID) {
			return mcp.ErrResult(fmt.Errorf("influxdb: bucket org or ID mismatch"))
		}
	}
	if len(items) > limit {
		return mcp.ErrResult(fmt.Errorf("influxdb: upstream exceeded requested limit"))
	}
	page := map[string]any{"buckets": items, "limit": limit, "offset": offset, "next_offset": nil, "has_more": len(items) == limit, "page_only": true}
	if len(items) == limit {
		page["next_offset"] = offset + len(items)
	}
	return mcp.JSONResult(page)
}

func listTasks(ctx context.Context, i *influxdb, args map[string]any) (*mcp.ToolResult, error) {
	limit, after, err := pagination(args)
	if err != nil {
		return mcp.ErrResult(err)
	}
	reader := mcp.NewArgs(args)
	name := reader.Str("name")
	status := reader.Str("status")
	if err := reader.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if status != "" && status != "active" && status != "inactive" {
		return mcp.ErrResult(fmt.Errorf("influxdb: task status must be active or inactive"))
	}
	if len(name) > 256 {
		return mcp.ErrResult(fmt.Errorf("influxdb: name exceeds 256 bytes"))
	}
	query := url.Values{"orgID": {i.orgID}, "type": {"basic"}, "limit": {strconv.Itoa(limit)}}
	if after != "" {
		query.Set("after", after)
	}
	if name != "" {
		query.Set("name", name)
	}
	if status != "" {
		query.Set("status", status)
	}
	var response struct {
		Tasks *[]task `json:"tasks"`
	}
	if err := i.get(ctx, "/api/v2/tasks", query, &response); err != nil {
		return mcp.ErrResult(err)
	}
	if response.Tasks == nil {
		return mcp.ErrResult(fmt.Errorf("influxdb: missing tasks array"))
	}
	items := *response.Tasks
	for _, item := range items {
		if item.OrgID != i.orgID || !validID(item.ID) {
			return mcp.ErrResult(fmt.Errorf("influxdb: task org or ID mismatch"))
		}
	}
	if len(items) > limit {
		return mcp.ErrResult(fmt.Errorf("influxdb: upstream exceeded requested limit"))
	}
	page := map[string]any{"tasks": items, "limit": limit, "next_after": nil, "has_more": len(items) == limit, "page_only": true}
	if len(items) == limit {
		page["next_after"] = items[len(items)-1].ID
	}
	return mcp.JSONResult(page)
}

func taskID(args map[string]any) (string, error) {
	value, err := mcp.ArgStr(args, "task_id")
	if err != nil || !validID(value) {
		return "", fmt.Errorf("influxdb: task_id must be a nonzero hexadecimal ID")
	}
	return value, nil
}
func (i *influxdb) readTask(ctx context.Context, id string) (task, error) {
	var result task
	if err := i.get(ctx, "/api/v2/tasks/"+id, nil, &result); err != nil {
		return result, err
	}
	if result.OrgID != i.orgID || result.ID != id {
		return result, fmt.Errorf("influxdb: task org or ID mismatch")
	}
	return result, nil
}
func getTask(ctx context.Context, i *influxdb, args map[string]any) (*mcp.ToolResult, error) {
	id, err := taskID(args)
	if err != nil {
		return mcp.ErrResult(err)
	}
	result, err := i.readTask(ctx, id)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.JSONResult(result)
}

func listTaskRuns(ctx context.Context, i *influxdb, args map[string]any) (*mcp.ToolResult, error) {
	id, err := taskID(args)
	if err != nil {
		return mcp.ErrResult(err)
	}
	limit, after, err := pagination(args)
	if err != nil {
		return mcp.ErrResult(err)
	}
	reader := mcp.NewArgs(args)
	lower := reader.Str("after_time")
	upper := reader.Str("before_time")
	status := reader.Str("status")
	if err := reader.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	switch status {
	case "", "scheduled", "started", "success", "failed", "canceled":
	default:
		return mcp.ErrResult(fmt.Errorf("influxdb: invalid run status"))
	}
	before := time.Now().UTC().Truncate(time.Second)
	if upper != "" {
		before, err = time.Parse(time.RFC3339, upper)
		if err != nil {
			return mcp.ErrResult(fmt.Errorf("influxdb: before_time must be RFC3339"))
		}
	}
	since := before.Add(-24 * time.Hour)
	if lower != "" {
		since, err = time.Parse(time.RFC3339, lower)
		if err != nil {
			return mcp.ErrResult(fmt.Errorf("influxdb: after_time must be RFC3339"))
		}
	}
	if !before.After(since) || before.Sub(since) > 7*24*time.Hour {
		return mcp.ErrResult(fmt.Errorf("influxdb: run time window must be positive and at most 7 days"))
	}
	if _, err := i.readTask(ctx, id); err != nil {
		return mcp.ErrResult(err)
	}
	query := url.Values{"limit": {strconv.Itoa(limit)}, "afterTime": {since.UTC().Format(time.RFC3339)}, "beforeTime": {before.UTC().Format(time.RFC3339)}}
	if after != "" {
		query.Set("after", after)
	}
	var response struct {
		Runs *[]run `json:"runs"`
	}
	if err := i.get(ctx, "/api/v2/tasks/"+id+"/runs", query, &response); err != nil {
		return mcp.ErrResult(err)
	}
	if response.Runs == nil {
		return mcp.ErrResult(fmt.Errorf("influxdb: missing runs array"))
	}
	items := *response.Runs
	if len(items) > limit {
		return mcp.ErrResult(fmt.Errorf("influxdb: upstream exceeded requested limit"))
	}
	filtered := make([]run, 0, len(items))
	for _, item := range items {
		if item.TaskID != id || !validID(item.ID) {
			return mcp.ErrResult(fmt.Errorf("influxdb: run task or ID mismatch"))
		}
		if status == "" || (item.Status != nil && *item.Status == status) {
			filtered = append(filtered, item)
		}
	}
	page := map[string]any{"runs": filtered, "limit": limit, "next_after": nil, "has_more": len(items) == limit, "page_only": true, "after_time": since.UTC().Format(time.RFC3339), "before_time": before.UTC().Format(time.RFC3339)}
	if len(items) == limit {
		page["next_after"] = items[len(items)-1].ID
	}
	return mcp.JSONResult(page)
}
func getTaskRun(ctx context.Context, i *influxdb, args map[string]any) (*mcp.ToolResult, error) {
	id, err := taskID(args)
	if err != nil {
		return mcp.ErrResult(err)
	}
	runID, err := mcp.ArgStr(args, "run_id")
	if err != nil || !validID(runID) {
		return mcp.ErrResult(fmt.Errorf("influxdb: run_id must be a nonzero hexadecimal ID"))
	}
	if _, err := i.readTask(ctx, id); err != nil {
		return mcp.ErrResult(err)
	}
	var result run
	if err := i.get(ctx, "/api/v2/tasks/"+id+"/runs/"+runID, nil, &result); err != nil {
		return mcp.ErrResult(err)
	}
	if result.TaskID != id || result.ID != runID {
		return mcp.ErrResult(fmt.Errorf("influxdb: run task or ID mismatch"))
	}
	return mcp.JSONResult(result)
}
