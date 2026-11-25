package ashby

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"

	mcp "github.com/daltoniam/switchboard"
)

const (
	defaultAPIBaseURL = "https://api.ashbyhq.com"
	// defaultPageLimit keeps list pages small; recruiting records are large
	// and PII-heavy, and Ashby's own default is 100.
	defaultPageLimit = 25
	// maxAPIResponseBytes bounds a single REST response; inline reports are
	// the largest legitimate payloads.
	maxAPIResponseBytes = 16 << 20
	maxErrorBodyChars   = 500
	apiKeysAdminHint    = "check the key in Ashby under Admin > API Keys"
)

// nativeClient calls Ashby's public REST API with an API key. Every endpoint
// is POST /<endpoint> with a JSON body and HTTP Basic auth, the key as the
// username and an empty password.
type nativeClient struct {
	apiKey  string
	baseURL string
	client  *http.Client
}

func newNativeClient(apiKey, baseURL string) *nativeClient {
	return &nativeClient{apiKey: apiKey, baseURL: baseURL, client: &http.Client{Timeout: 60 * time.Second}}
}

func normalizeAPIBaseURL(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return defaultAPIBaseURL, nil
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Hostname() == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return "", fmt.Errorf("ashby: invalid api_base_url: must be an HTTP(S) URL with a host")
	}
	parsed.RawQuery = ""
	parsed.ForceQuery = false
	parsed.Fragment = ""
	parsed.RawFragment = ""
	return strings.TrimRight(parsed.String(), "/"), nil
}

type apiErrorInfo struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	RequestID string `json:"requestId"`
}

func (e apiErrorInfo) String() string {
	parts := make([]string, 0, 2)
	for _, part := range []string{e.Code, e.Message} {
		if part != "" {
			parts = append(parts, part)
		}
	}
	text := strings.Join(parts, ": ")
	if e.RequestID != "" {
		text += " (request " + e.RequestID + ")"
	}
	return text
}

// apiEnvelope is Ashby's response wrapper. Results stay raw so the records
// are parsed once, by the response pipeline, and never re-encoded here.
type apiEnvelope struct {
	Success           bool              `json:"success"`
	Results           json.RawMessage   `json:"results"`
	MoreDataAvailable *bool             `json:"moreDataAvailable"`
	NextCursor        *string           `json:"nextCursor"`
	SyncToken         *string           `json:"syncToken"`
	Warnings          json.RawMessage   `json:"warnings"`
	ErrorInfo         *apiErrorInfo     `json:"errorInfo"`
	Errors            []json.RawMessage `json:"errors"`
}

func (e apiEnvelope) problem() string {
	var parts []string
	if e.ErrorInfo != nil {
		if text := e.ErrorInfo.String(); text != "" {
			parts = append(parts, text)
		}
	}
	for _, raw := range e.Errors {
		var text string
		if json.Unmarshal(raw, &text) == nil {
			parts = append(parts, text)
			continue
		}
		var info apiErrorInfo
		if json.Unmarshal(raw, &info) == nil && info.String() != "" {
			parts = append(parts, info.String())
			continue
		}
		parts = append(parts, string(raw))
	}
	return strings.Join(parts, "; ")
}

// call posts body to endpoint and returns the envelope. HTTP 200 with
// success=false is an error, as are 4xx statuses; 429 and 5xx are retryable.
func (c *nativeClient) call(ctx context.Context, endpoint string, body map[string]any) (*apiEnvelope, error) {
	payload, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/"+endpoint, bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.SetBasicAuth(c.apiKey, "")
	req.Header.Set("Accept", "application/json; version=1")
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("ashby %s: %w", endpoint, err)
	}
	defer func() { _ = resp.Body.Close() }()
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxAPIResponseBytes+1))
	if err != nil {
		return nil, fmt.Errorf("ashby %s: read response: %w", endpoint, err)
	}
	if len(data) > maxAPIResponseBytes {
		return nil, fmt.Errorf("ashby %s: response is larger than %d bytes; narrow the request with filters or a smaller limit", endpoint, maxAPIResponseBytes)
	}
	if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
		return nil, &mcp.RetryableError{
			StatusCode: resp.StatusCode,
			Err:        statusError(endpoint, resp.StatusCode, data),
			RetryAfter: mcp.ParseRetryAfter(resp.Header.Get("Retry-After")),
		}
	}
	if resp.StatusCode >= 400 {
		return nil, statusError(endpoint, resp.StatusCode, data)
	}
	var envelope apiEnvelope
	if err := json.Unmarshal(data, &envelope); err != nil {
		return nil, fmt.Errorf("ashby %s: invalid JSON response: %s", endpoint, truncate(string(data)))
	}
	if !envelope.Success {
		if problem := envelope.problem(); problem != "" {
			return nil, fmt.Errorf("ashby %s failed: %s", endpoint, problem)
		}
		return nil, fmt.Errorf("ashby %s failed: success=false with no error details", endpoint)
	}
	return &envelope, nil
}

func statusError(endpoint string, status int, body []byte) error {
	detail := strings.TrimSpace(string(body))
	var envelope apiEnvelope
	if json.Unmarshal(body, &envelope) == nil {
		if problem := envelope.problem(); problem != "" {
			detail = problem
		}
	}
	detail = truncate(detail)
	switch status {
	case http.StatusUnauthorized:
		return fmt.Errorf("ashby API error (401) on %s: %s; the API key was not sent", endpoint, detail)
	case http.StatusForbidden:
		if strings.Contains(detail, "missing_endpoint_permission") {
			return fmt.Errorf("ashby API error (403) on %s: %s; the API key lacks this module's permission, grant it in Ashby under Admin > API Keys", endpoint, detail)
		}
		return fmt.Errorf("ashby API error (403) on %s: %s; the API key is wrong, deactivated, or lacks permission, %s", endpoint, detail, apiKeysAdminHint)
	}
	return fmt.Errorf("ashby API error (%d) on %s: %s", status, endpoint, detail)
}

func truncate(text string) string {
	if len(text) > maxErrorBodyChars {
		return text[:maxErrorBodyChars] + "... (truncated)"
	}
	return text
}

func (c *nativeClient) healthy(ctx context.Context) bool {
	_, err := c.call(ctx, "apiKey.info", map[string]any{})
	return err == nil
}

// execute runs a native tool. The response drops Ashby's success flag and
// always carries the pagination keys for paginated endpoints, null on the
// last page, so compaction can keep them and callers can follow cursors.
func (c *nativeClient) execute(ctx context.Context, toolName mcp.ToolName, args map[string]any) (*mcp.ToolResult, error) {
	ep, ok := nativeDispatch[toolName]
	if !ok {
		return mcp.ErrResult(fmt.Errorf("unknown tool: %s", toolName))
	}
	body, err := ep.body(args)
	if err != nil {
		return mcp.ErrResult(fmt.Errorf("%s: %w", toolName, err))
	}
	envelope, err := c.call(ctx, ep.path, body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	out := map[string]any{"results": envelope.Results}
	if envelope.MoreDataAvailable != nil || envelope.NextCursor != nil || envelope.SyncToken != nil {
		out["moreDataAvailable"] = envelope.MoreDataAvailable != nil && *envelope.MoreDataAvailable
		out["nextCursor"] = envelope.NextCursor
		out["syncToken"] = envelope.SyncToken
	}
	if len(envelope.Warnings) > 0 && string(envelope.Warnings) != "null" {
		out["warnings"] = envelope.Warnings
	}
	return mcp.JSONResult(out)
}

type paramKind int

const (
	kindString paramKind = iota
	kindInt
	kindBool
	kindStrings
	kindObject
	// kindAny passes a JSON value through unchanged, for union or
	// array-of-object fields such as notes and interview events.
	kindAny
)

// param maps one snake_case tool argument to an Ashby request field.
type param struct {
	name     string
	field    string
	kind     paramKind
	desc     string
	required bool
	// def is sent when the caller omits the argument.
	def any
}

func (p param) apiField() string {
	if p.field != "" {
		return p.field
	}
	parts := strings.Split(p.name, "_")
	for i := 1; i < len(parts); i++ {
		if parts[i] != "" {
			parts[i] = strings.ToUpper(parts[i][:1]) + parts[i][1:]
		}
	}
	return strings.Join(parts, "")
}

func (p param) as(field string) param {
	p.field = field
	return p
}

func (p param) req() param {
	p.required = true
	return p
}

func (p param) or(def any) param {
	p.def = def
	return p
}

func str(name, desc string) param     { return param{name: name, kind: kindString, desc: desc} }
func integer(name, desc string) param { return param{name: name, kind: kindInt, desc: desc} }
func boolean(name, desc string) param { return param{name: name, kind: kindBool, desc: desc} }
func strs(name, desc string) param    { return param{name: name, kind: kindStrings, desc: desc} }
func object(name, desc string) param  { return param{name: name, kind: kindObject, desc: desc} }
func value(name, desc string) param   { return param{name: name, kind: kindAny, desc: desc} }

// page is cursor pagination with a small default page size.
func page() []param {
	return []param{
		str("cursor", "Opaque nextCursor from the previous page; omit for the first page."),
		integer("limit", fmt.Sprintf("Page size, 1 to 100 (default %d).", defaultPageLimit)).or(defaultPageLimit),
	}
}

// synced is page plus Ashby's incremental sync token.
func synced() []param {
	return append(page(), str("sync_token", "syncToken returned on the last page of an earlier full listing; returns only records changed since then."))
}

func params(groups ...[]param) []param { return slices.Concat(groups...) }

func ps(items ...param) []param { return items }

func createdAfter(what string) param {
	return integer("created_after", "Only "+what+" created at or after this time, Unix epoch milliseconds.")
}

// endpoint is one native tool: an Ashby REST endpoint and its arguments.
type endpoint struct {
	tool   mcp.ToolName
	path   string
	access mcp.ToolAccess
	desc   string
	params []param
}

func (e endpoint) definition() mcp.ToolDefinition {
	parameters := make(map[string]string, len(e.params))
	var required []string
	for _, p := range e.params {
		parameters[p.name] = p.desc
		if p.required {
			required = append(required, p.name)
		}
	}
	return mcp.ToolDefinition{Name: e.tool, Description: e.desc, Parameters: parameters, Required: required, Access: e.access}
}

// body builds the request from declared arguments only, because Ashby
// rejects unknown fields; undeclared arguments (such as session defaults
// meant for other integrations) are ignored.
func (e endpoint) body(args map[string]any) (map[string]any, error) {
	r := mcp.NewArgs(args)
	body := make(map[string]any, len(e.params))
	var missing []string
	for _, p := range e.params {
		raw, present := args[p.name]
		if !present || raw == nil {
			if p.required {
				missing = append(missing, p.name)
			} else if p.def != nil {
				body[p.apiField()] = p.def
			}
			continue
		}
		switch p.kind {
		case kindString:
			if text := strings.TrimSpace(r.Str(p.name)); text != "" {
				body[p.apiField()] = text
			} else if p.required {
				missing = append(missing, p.name)
			}
		case kindInt:
			body[p.apiField()] = r.Int(p.name)
		case kindBool:
			body[p.apiField()] = r.Bool(p.name)
		case kindStrings:
			body[p.apiField()] = r.StrSlice(p.name)
		case kindObject:
			body[p.apiField()] = r.Map(p.name)
		case kindAny:
			body[p.apiField()] = raw
		}
	}
	if err := r.Err(); err != nil {
		return nil, err
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf("missing required argument(s): %s", strings.Join(missing, ", "))
	}
	return body, nil
}

var nativeEndpoints = slices.Concat(
	jobEndpoints,
	candidateEndpoints,
	applicationEndpoints,
	interviewEndpoints,
	hiringEndpoints,
	organizationEndpoints,
	offerEndpoints,
)

var nativeDispatch = func() map[mcp.ToolName]endpoint {
	m := make(map[mcp.ToolName]endpoint, len(nativeEndpoints))
	for _, ep := range nativeEndpoints {
		m[ep.tool] = ep
	}
	return m
}()

var nativeTools = func() []mcp.ToolDefinition {
	out := make([]mcp.ToolDefinition, len(nativeEndpoints))
	for i, ep := range nativeEndpoints {
		out[i] = ep.definition()
	}
	return out
}()

func cloneTools(tools []mcp.ToolDefinition) []mcp.ToolDefinition {
	out := slices.Clone(tools)
	for i := range out {
		out[i].Parameters = maps.Clone(out[i].Parameters)
		out[i].Required = slices.Clone(out[i].Required)
	}
	return out
}

var errNoCredential = errors.New("ashby: api_key (API key) or mcp_access_token (sign in with Ashby) is required")
