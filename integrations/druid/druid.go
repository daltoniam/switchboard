package druid

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

var fieldCompactionSpecs = compact.MustLoadWithOverlay("druid", compactYAML, compact.Options{Strict: false}).Specs

const maxResponseSize = 2 * 1024 * 1024

var (
	_             mcp.Integration                = (*druid)(nil)
	_             mcp.FieldCompactionIntegration = (*druid)(nil)
	_             mcp.PlainTextCredentials       = (*druid)(nil)
	_             mcp.OptionalCredentials        = (*druid)(nil)
	_             mcp.PlaceholderHints           = (*druid)(nil)
	sensitivePath                                = regexp.MustCompile(`(?i)(?:https?://|/|[a-z]:\\)[^\s,;]*`)
)

type druid struct {
	baseURL  string
	username string
	password string
	client   *http.Client
}

func New() mcp.Integration                   { return &druid{} }
func (d *druid) Name() string                { return "druid" }
func (d *druid) Tools() []mcp.ToolDefinition { return tools }
func (d *druid) PlainTextKeys() []string     { return []string{"base_url", "username"} }
func (d *druid) OptionalKeys() []string      { return []string{"ca_cert"} }
func (d *druid) Placeholders() map[string]string {
	return map[string]string{"base_url": "Trusted HTTPS Druid Router URL, including optional proxy prefix", "username": "Dedicated basic-security identity with DATASOURCE READ and STATE READ only", "password": "Password for the dedicated diagnostic identity, never admin credentials", "ca_cert": "Optional PEM CA certificate for private TLS"}
}
func (d *druid) CompactSpec(name mcp.ToolName) ([]mcp.CompactField, bool) {
	fields, ok := fieldCompactionSpecs[name]
	return fields, ok
}

func (d *druid) Configure(_ context.Context, credentials mcp.Credentials) error {
	if d.client != nil {
		d.client.CloseIdleConnections()
	}
	d.client = nil
	parsed, err := url.Parse(strings.TrimRight(strings.TrimSpace(credentials["base_url"]), "/"))
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.ForceQuery || parsed.Fragment != "" || parsed.RawPath != "" || strings.Contains(parsed.Path, "%") {
		return fmt.Errorf("druid: base_url must be a trusted HTTPS Router URL without userinfo, query, fragment or escaped path")
	}
	for _, segment := range strings.Split(parsed.Path, "/") {
		if segment == "." || segment == ".." {
			return fmt.Errorf("druid: base_url must not contain traversal segments")
		}
	}
	username, password := credentials["username"], credentials["password"]
	if strings.TrimSpace(username) == "" || password == "" || strings.ContainsAny(username, ":\r\n") || strings.ContainsAny(password, "\r\n") {
		return fmt.Errorf("druid: username and password are required and must be valid Basic credentials")
	}
	if strings.EqualFold(username, "admin") || strings.EqualFold(username, "druid_system") {
		return fmt.Errorf("druid: use a dedicated least-privilege identity, not a built-in administrator")
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig = &tls.Config{MinVersion: tls.VersionTLS12}
	if certificate := credentials["ca_cert"]; certificate != "" {
		roots, err := x509.SystemCertPool()
		if err != nil {
			roots = x509.NewCertPool()
		}
		if !roots.AppendCertsFromPEM([]byte(strings.ReplaceAll(certificate, `\n`, "\n"))) {
			return fmt.Errorf("druid: ca_cert must contain a valid PEM CA certificate")
		}
		transport.TLSClientConfig.RootCAs = roots
	}
	d.baseURL, d.username, d.password = parsed.String(), username, password
	d.client = &http.Client{Transport: transport, Timeout: 10 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	return nil
}

func (d *druid) Healthy(ctx context.Context) bool {
	if d.client == nil {
		return false
	}
	var names []string
	if err := d.get(ctx, "/druid/coordinator/v1/datasources", nil, &names); err != nil {
		return false
	}
	return names != nil
}

func (d *druid) Execute(ctx context.Context, name mcp.ToolName, args map[string]any) (*mcp.ToolResult, error) {
	handler, ok := dispatch[name]
	if !ok {
		return mcp.ErrResult(fmt.Errorf("unknown tool: %s", name))
	}
	if d.client == nil {
		return mcp.ErrResult(fmt.Errorf("druid: not configured"))
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	return handler(ctx, d, args)
}

func (d *druid) get(ctx context.Context, route string, query url.Values, target any) error {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	for attempt := 0; attempt < 3; attempt++ {
		request, err := http.NewRequestWithContext(ctx, http.MethodGet, d.baseURL+route+"?"+query.Encode(), nil)
		if err != nil {
			return fmt.Errorf("druid: cannot construct request")
		}
		request.SetBasicAuth(d.username, d.password)
		request.Header.Set("Accept", "application/json")
		response, err := d.client.Do(request)
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			return fmt.Errorf("druid: request failed; check Router connectivity and TLS certificate")
		}
		status := response.StatusCode
		if status == http.StatusTooManyRequests {
			retryAfter := mcp.ParseRetryAfter(response.Header.Get("Retry-After"))
			_ = response.Body.Close()
			if attempt == 2 {
				return &mcp.RetryableError{StatusCode: status, RetryAfter: retryAfter, Err: fmt.Errorf("druid: rate limited (429); retry later")}
			}
			delay := retryAfter
			if delay <= 0 {
				delay = 100 * time.Millisecond
			}
			if delay > 2*time.Second {
				return &mcp.RetryableError{StatusCode: status, RetryAfter: retryAfter, Err: fmt.Errorf("druid: rate limited (429); Retry-After exceeds bounded retry window")}
			}
			timer := time.NewTimer(delay)
			select {
			case <-ctx.Done():
				timer.Stop()
				return ctx.Err()
			case <-timer.C:
			}
			continue
		}
		if status < 200 || status >= 300 || status == http.StatusNoContent {
			_ = response.Body.Close()
			switch status {
			case 401, 403:
				return fmt.Errorf("druid: authentication or permission denied (%d); check diagnostic credentials and DATASOURCE READ/STATE READ grants", status)
			case 404:
				return fmt.Errorf("druid: resource not found (404); verify the identifier, Router routing and supported Druid version")
			default:
				return fmt.Errorf("druid: upstream HTTP %d; redirects are not followed and error bodies are withheld to protect credentials and paths", status)
			}
		}
		body, readErr := io.ReadAll(io.LimitReader(response.Body, maxResponseSize+1))
		_ = response.Body.Close()
		if readErr != nil {
			return fmt.Errorf("druid: incomplete response")
		}
		if len(body) > maxResponseSize {
			return fmt.Errorf("druid: response exceeds 2 MiB; narrow filters")
		}
		if strings.TrimSpace(string(body)) == "null" {
			return fmt.Errorf("druid: unexpected null API response")
		}
		if err := json.Unmarshal(body, target); err != nil {
			return fmt.Errorf("druid: invalid or unsupported JSON API response")
		}
		return nil
	}
	return fmt.Errorf("druid: retry limit reached")
}

type listResult[T any] struct {
	Items     []T    `json:"items"`
	Truncated bool   `json:"truncated"`
	Partial   bool   `json:"partial"`
	Notice    string `json:"notice"`
}

func bounded[T any](items []T, limit int, upstreamBound bool) listResult[T] {
	truncated := len(items) > limit
	if truncated {
		items = items[:limit]
	}
	if items == nil {
		items = []T{}
	}
	return listResult[T]{Items: items, Truncated: truncated, Partial: true, Notice: func() string {
		if upstreamBound {
			return "ACL-filtered; complete-task max may omit older results; no cursor pagination"
		}
		return "ACL-filtered visibility, not a complete cluster inventory; local cap does not bound upstream retrieval"
	}()}
}

func validIdentifier(value string) bool {
	if value == "" || len(value) > 512 || !utf8.ValidString(value) || strings.ContainsAny(value, "/\\%?#") || value == "." || value == ".." {
		return false
	}
	return !strings.ContainsFunc(value, unicode.IsControl)
}
func readLimit(args map[string]any, defaultValue int) (int, error) {
	value, err := mcp.ArgInt(args, "limit")
	if err != nil {
		return 0, err
	}
	if _, exists := args["limit"]; !exists {
		value = defaultValue
	}
	if value < 1 || value > 100 {
		return 0, fmt.Errorf("druid: limit must be 1..100")
	}
	return value, nil
}
func (d *druid) safeText(value string) string {
	value = strings.ReplaceAll(value, d.password, "[redacted]")
	value = sensitivePath.ReplaceAllString(value, "[path redacted]")
	value = strings.Map(func(character rune) rune {
		if unicode.IsControl(character) {
			return ' '
		}
		return character
	}, value)
	if len(value) > 512 {
		value = value[:512-len(" [truncated]")]
		for !utf8.ValidString(value) {
			value = value[:len(value)-1]
		}
		value += " [truncated]"
	}
	return value
}

type handlerFunc func(context.Context, *druid, map[string]any) (*mcp.ToolResult, error)

var dispatch = map[mcp.ToolName]handlerFunc{
	"druid_list_datasources": listDatasources,
	"druid_get_datasource":   getDatasource,
	"druid_get_load_status":  getLoadStatus,
	"druid_list_tasks":       listTasks,
	"druid_get_task_status":  getTaskStatus,
}

func listDatasources(ctx context.Context, d *druid, args map[string]any) (*mcp.ToolResult, error) {
	limit, err := readLimit(args, 50)
	if err != nil {
		return mcp.ErrResult(err)
	}
	var names []string
	if err := d.get(ctx, "/druid/coordinator/v1/datasources", nil, &names); err != nil {
		return mcp.ErrResult(err)
	}
	if names == nil {
		return mcp.ErrResult(fmt.Errorf("druid: unsupported datasource list response"))
	}
	sort.Strings(names)
	return mcp.JSONResult(bounded(names, limit, false))
}

type segmentSummary struct {
	Count          *int64  `json:"count,omitempty"`
	Size           *int64  `json:"size,omitempty"`
	ReplicatedSize *int64  `json:"replicatedSize,omitempty"`
	MinTime        *string `json:"minTime,omitempty"`
	MaxTime        *string `json:"maxTime,omitempty"`
}

type datasourceSummary struct {
	Name       string `json:"name"`
	Properties struct {
		Segments *segmentSummary `json:"segments"`
	} `json:"properties"`
}

func getDatasource(ctx context.Context, d *druid, args map[string]any) (*mcp.ToolResult, error) {
	name, err := mcp.ArgStr(args, "datasource")
	if err != nil {
		return mcp.ErrResult(err)
	}
	if !validIdentifier(name) {
		return mcp.ErrResult(fmt.Errorf("druid: datasource must be one nonempty literal identifier, at most 512 bytes"))
	}
	var response struct {
		Segments *segmentSummary `json:"segments"`
	}
	if err := d.get(ctx, "/druid/coordinator/v1/datasources/"+url.PathEscape(name), nil, &response); err != nil {
		return mcp.ErrResult(err)
	}
	if response.Segments == nil {
		return mcp.ErrResult(fmt.Errorf("druid: unsupported datasource summary response"))
	}
	result := datasourceSummary{Name: name}
	result.Properties.Segments = response.Segments
	return mcp.JSONResult(result)
}

type loadSummary struct {
	Name    string  `json:"datasource"`
	Percent float64 `json:"percent_loaded"`
}

func getLoadStatus(ctx context.Context, d *druid, args map[string]any) (*mcp.ToolResult, error) {
	reader := mcp.NewArgs(args)
	name := reader.Str("datasource")
	limit := reader.OptInt("limit", 50)
	if err := reader.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if limit < 1 || limit > 100 || (name != "" && !validIdentifier(name)) {
		return mcp.ErrResult(fmt.Errorf("druid: invalid datasource or limit (1..100)"))
	}
	var loads map[string]float64
	if err := d.get(ctx, "/druid/coordinator/v1/loadstatus", nil, &loads); err != nil {
		return mcp.ErrResult(err)
	}
	if loads == nil {
		return mcp.ErrResult(fmt.Errorf("druid: unsupported load status response"))
	}
	names := make([]string, 0, len(loads))
	for key := range loads {
		if name == "" || key == name {
			names = append(names, key)
		}
	}
	sort.Strings(names)
	items := make([]loadSummary, 0, min(len(names), limit+1))
	for _, key := range names {
		if loads[key] < 0 || loads[key] > 100 {
			return mcp.ErrResult(fmt.Errorf("druid: invalid segment load percentage"))
		}
		items = append(items, loadSummary{Name: key, Percent: loads[key]})
		if len(items) > limit {
			break
		}
	}
	return mcp.JSONResult(bounded(items, limit, false))
}

type taskSummary struct {
	ID               string  `json:"id"`
	Type             string  `json:"type,omitempty"`
	Datasource       string  `json:"dataSource,omitempty"`
	StatusCode       *string `json:"statusCode,omitempty"`
	RunnerStatusCode *string `json:"runnerStatusCode,omitempty"`
	CreatedTime      *string `json:"createdTime,omitempty"`
	Duration         *int64  `json:"duration,omitempty"`
	ErrorMsg         *string `json:"errorMsg,omitempty"`
}

func listTasks(ctx context.Context, d *druid, args map[string]any) (*mcp.ToolResult, error) {
	reader := mcp.NewArgs(args)
	state := reader.Str("state")
	datasource := reader.Str("datasource")
	taskType := reader.Str("type")
	interval := reader.Str("interval")
	limit := reader.OptInt("limit", 50)
	if err := reader.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if state == "" {
		state = "complete"
	}
	if state != "running" && state != "complete" && state != "waiting" && state != "pending" {
		return mcp.ErrResult(fmt.Errorf("druid: state must be running, complete, waiting or pending; FAILED is a returned statusCode"))
	}
	if limit < 1 || limit > 100 || (datasource != "" && !validIdentifier(datasource)) || (taskType != "" && !validIdentifier(taskType)) {
		return mcp.ErrResult(fmt.Errorf("druid: invalid filter or limit (1..100)"))
	}
	if interval == "" {
		end := time.Now().UTC().Truncate(time.Second)
		interval = end.Add(-24*time.Hour).Format(time.RFC3339) + "_" + end.Format(time.RFC3339)
	}
	bounds := strings.Split(interval, "_")
	if len(bounds) != 2 {
		return mcp.ErrResult(fmt.Errorf("druid: interval must be two RFC3339 timestamps separated by underscore"))
	}
	start, startErr := time.Parse(time.RFC3339, bounds[0])
	end, endErr := time.Parse(time.RFC3339, bounds[1])
	if startErr != nil || endErr != nil || !start.Before(end) {
		return mcp.ErrResult(fmt.Errorf("druid: invalid interval, start must precede end"))
	}
	query := url.Values{"state": {state}, "createdTimeInterval": {interval}}
	if state == "complete" {
		query.Set("max", strconv.Itoa(limit))
	}
	if datasource != "" {
		query.Set("datasource", datasource)
	}
	if taskType != "" {
		query.Set("type", taskType)
	}
	var tasks []taskSummary
	if err := d.get(ctx, "/druid/indexer/v1/tasks", query, &tasks); err != nil {
		return mcp.ErrResult(err)
	}
	if tasks == nil {
		return mcp.ErrResult(fmt.Errorf("druid: unsupported task list response"))
	}
	result := bounded(tasks, limit, state == "complete")
	for index := range result.Items {
		task := &result.Items[index]
		if !validIdentifier(task.ID) {
			return mcp.ErrResult(fmt.Errorf("druid: task response has an invalid identifier"))
		}
		if task.ErrorMsg != nil {
			safe := d.safeText(*task.ErrorMsg)
			task.ErrorMsg = &safe
		}
	}
	return mcp.JSONResult(result)
}

type taskStatus struct {
	Task   string `json:"task"`
	Status struct {
		ID       string  `json:"id"`
		Status   *string `json:"status,omitempty"`
		Duration *int64  `json:"duration,omitempty"`
		ErrorMsg *string `json:"errorMsg,omitempty"`
	} `json:"status"`
}

func getTaskStatus(ctx context.Context, d *druid, args map[string]any) (*mcp.ToolResult, error) {
	id, err := mcp.ArgStr(args, "task_id")
	if err != nil {
		return mcp.ErrResult(err)
	}
	if !validIdentifier(id) {
		return mcp.ErrResult(fmt.Errorf("druid: task_id must be one nonempty literal identifier, at most 512 bytes"))
	}
	var result taskStatus
	if err := d.get(ctx, "/druid/indexer/v1/task/"+url.PathEscape(id)+"/status", nil, &result); err != nil {
		return mcp.ErrResult(err)
	}
	if result.Task != id || result.Status.ID != id {
		return mcp.ErrResult(fmt.Errorf("druid: unsupported task status response"))
	}
	if result.Status.ErrorMsg != nil {
		safe := d.safeText(*result.Status.ErrorMsg)
		result.Status.ErrorMsg = &safe
	}
	return mcp.JSONResult(result)
}

var tools = []mcp.ToolDefinition{
	{Name: "druid_list_datasources", Access: mcp.AccessRead, Description: "List Apache Druid datasource names. Start here for ingestion task and segment diagnosis, then get_datasource or list_tasks. ACL-filtered and locally capped, not a complete cluster inventory. No SQL or payload access.", Parameters: map[string]string{"limit": "Local cap 1..100, default 50; upstream response limited to 2 MiB"}},
	{Name: "druid_get_datasource", Access: mcp.AccessRead, Description: "Get Apache Druid datasource segment summary: count, bytes and min/max time. Use after list_datasources. No segment loadSpec, host locations or ingestion configuration.", Parameters: map[string]string{"datasource": "Exact datasource name from list_datasources"}, Required: []string{"datasource"}},
	{Name: "druid_get_load_status", Access: mcp.AccessRead, Description: "Inspect Apache Druid segment load percentages to diagnose unavailable datasources. Optional datasource filter is local; results may be ACL-filtered or truncated. Does not force metadata refresh.", Parameters: map[string]string{"datasource": "Optional exact datasource name, filtered locally", "limit": "Local cap 1..100, default 50; does not bound upstream retrieval"}},
	{Name: "druid_list_tasks", Access: mcp.AccessRead, Description: "List recent Apache Druid ingestion and compaction task summaries to find failed tasks. Default complete over last 24 hours. FAILED is a returned statusCode, not a state filter. max only limits complete tasks upstream; active tasks are locally capped. No task payload or host locations.", Parameters: map[string]string{"datasource": "Optional exact datasource filter", "state": "running, complete, waiting or pending; default complete", "type": "Optional exact task type, e.g. index_parallel", "interval": "Two RFC3339 timestamps separated by underscore (not slash), default last 24 hours", "limit": "1..100, default 50; upstream max only applies to complete tasks"}},
	{Name: "druid_get_task_status", Access: mcp.AccessRead, Description: "Inspect one Apache Druid ingestion task status, duration and sanitized failure context. Use after list_tasks. Paths and URLs are redacted and messages capped at 512 bytes. Never fetches task submission payload, logs or configuration.", Parameters: map[string]string{"task_id": "Exact task id from list_tasks"}, Required: []string{"task_id"}},
}
