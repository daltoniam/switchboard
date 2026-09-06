package okta

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	mcp "github.com/daltoniam/switchboard"
	"github.com/daltoniam/switchboard/compact"
)

//go:embed compact.yaml
var compactYAML []byte

var compactResult = compact.MustLoadWithOverlay("okta", compactYAML, compact.Options{Strict: false})
var fieldCompactionSpecs = compactResult.Specs
var maxBytesByTool = compactResult.MaxBytes

type okta struct {
	apiToken string
	orgURL   string
	client   *http.Client
	baseURL  string
}

const maxResponseSize = 10 * 1024 * 1024 // 10 MB

var (
	_ mcp.Integration                = (*okta)(nil)
	_ mcp.FieldCompactionIntegration = (*okta)(nil)
	_ mcp.ToolMaxBytesIntegration    = (*okta)(nil)
	_ mcp.PlainTextCredentials       = (*okta)(nil)
	_ mcp.PlaceholderHints           = (*okta)(nil)
)

func (o *okta) PlainTextKeys() []string { return []string{"org_url"} }

func (o *okta) Placeholders() map[string]string {
	return map[string]string{
		"api_token": "Okta SSWS API token",
		"org_url":   "https://your-org.okta.com",
	}
}

func New() mcp.Integration {
	return &okta{
		client: &http.Client{Timeout: 30 * time.Second},
	}
}

func (o *okta) Name() string { return "okta" }

func (o *okta) Configure(_ context.Context, creds mcp.Credentials) error {
	o.apiToken = creds["api_token"]
	if o.apiToken == "" {
		return fmt.Errorf("okta: api_token is required")
	}
	orgURL := strings.TrimSpace(creds["org_url"])
	if orgURL == "" {
		return fmt.Errorf("okta: org_url is required")
	}
	o.orgURL = normalizeOrgURL(orgURL)
	o.baseURL = o.orgURL + "/api/v1"
	return nil
}

func normalizeOrgURL(raw string) string {
	raw = strings.TrimSpace(raw)
	raw = strings.TrimRight(raw, "/")
	if !strings.Contains(raw, "://") {
		raw = "https://" + raw
	}
	raw = strings.TrimSuffix(raw, "/api/v1")
	raw = strings.TrimRight(raw, "/")
	return raw
}

func (o *okta) Healthy(ctx context.Context) bool {
	_, err := o.get(ctx, "/org")
	return err == nil
}

func (o *okta) Tools() []mcp.ToolDefinition {
	return tools
}

func (o *okta) CompactSpec(toolName mcp.ToolName) ([]mcp.CompactField, bool) {
	fields, ok := fieldCompactionSpecs[toolName]
	return fields, ok
}

func (o *okta) MaxBytes(toolName mcp.ToolName) (int, bool) {
	n, ok := maxBytesByTool[toolName]
	return n, ok
}

func (o *okta) Execute(ctx context.Context, toolName mcp.ToolName, args map[string]any) (*mcp.ToolResult, error) {
	fn, ok := dispatch[toolName]
	if !ok {
		return &mcp.ToolResult{Data: fmt.Sprintf("unknown tool: %s", toolName), IsError: true}, nil
	}
	return fn(ctx, o, args)
}

func (o *okta) doRequest(ctx context.Context, method, path string, body any) (json.RawMessage, error) {
	data, _, err := o.doRequestWithLink(ctx, method, path, body)
	return data, err
}

func (o *okta) doRequestWithLink(ctx context.Context, method, path string, body any) (json.RawMessage, string, error) {
	var bodyReader io.Reader
	if body != nil {
		payload, err := json.Marshal(body)
		if err != nil {
			return nil, "", err
		}
		bodyReader = bytes.NewReader(payload)
	}

	req, err := http.NewRequestWithContext(ctx, method, o.baseURL+path, bodyReader)
	if err != nil {
		return nil, "", err
	}
	req.Header.Set("Authorization", "SSWS "+o.apiToken)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "switchboard-okta/1.0")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	} else if method == http.MethodPost || method == http.MethodPut {
		req.Header.Set("Content-Length", "0")
	}

	resp, err := o.client.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer func() { _ = resp.Body.Close() }()

	data, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseSize))
	if err != nil {
		return nil, "", err
	}
	if resp.StatusCode == 429 || resp.StatusCode >= 500 {
		re := &mcp.RetryableError{StatusCode: resp.StatusCode, Err: fmt.Errorf("okta API error (%d): %s", resp.StatusCode, string(data))}
		re.RetryAfter = mcp.ParseRetryAfter(resp.Header.Get("Retry-After"))
		return nil, "", re
	}
	if resp.StatusCode >= 400 {
		return nil, "", fmt.Errorf("okta API error (%d): %s", resp.StatusCode, string(data))
	}
	nextAfter := parseNextAfter(resp.Header.Get("Link"))
	if resp.StatusCode == 204 || len(data) == 0 {
		return json.RawMessage(`{"status":"success"}`), nextAfter, nil
	}
	return json.RawMessage(data), nextAfter, nil
}

func (o *okta) get(ctx context.Context, pathFmt string, args ...any) (json.RawMessage, error) {
	return o.doRequest(ctx, http.MethodGet, fmt.Sprintf(pathFmt, args...), nil)
}

func (o *okta) getList(ctx context.Context, pathFmt string, args ...any) (json.RawMessage, error) {
	data, nextAfter, err := o.doRequestWithLink(ctx, http.MethodGet, fmt.Sprintf(pathFmt, args...), nil)
	if err != nil {
		return nil, err
	}
	return wrapList(data, nextAfter)
}

func (o *okta) post(ctx context.Context, path string, body any) (json.RawMessage, error) {
	return o.doRequest(ctx, http.MethodPost, path, body)
}

func (o *okta) put(ctx context.Context, path string, body any) (json.RawMessage, error) {
	return o.doRequest(ctx, http.MethodPut, path, body)
}

func (o *okta) del(ctx context.Context, pathFmt string, args ...any) (json.RawMessage, error) {
	return o.doRequest(ctx, http.MethodDelete, fmt.Sprintf(pathFmt, args...), nil)
}

func wrapList(data json.RawMessage, nextAfter string) (json.RawMessage, error) {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 || trimmed[0] != '[' {
		return data, nil
	}
	out := map[string]any{
		"items": json.RawMessage(data),
	}
	if nextAfter != "" {
		out["next_after"] = nextAfter
	}
	return json.Marshal(out)
}

func parseNextAfter(linkHeader string) string {
	for _, part := range strings.Split(linkHeader, ",") {
		part = strings.TrimSpace(part)
		if !strings.Contains(part, `rel="next"`) {
			continue
		}
		start := strings.Index(part, "<")
		end := strings.Index(part, ">")
		if start < 0 || end <= start {
			continue
		}
		href := part[start+1 : end]
		u, err := url.Parse(href)
		if err != nil {
			continue
		}
		return u.Query().Get("after")
	}
	return ""
}

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

func limitParam(r *mcp.Args, def int) string {
	n := r.OptInt("limit", def)
	if n > 200 {
		n = 200
	}
	return fmt.Sprintf("%d", n)
}

func parseJSONObject(raw string) (map[string]any, error) {
	if raw == "" {
		return map[string]any{}, nil
	}
	var out map[string]any
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return nil, fmt.Errorf("invalid JSON object: %w", err)
	}
	return out, nil
}

type handlerFunc func(ctx context.Context, o *okta, args map[string]any) (*mcp.ToolResult, error)

var dispatch = map[mcp.ToolName]handlerFunc{
	mcp.ToolName("okta_list_users"):          listUsers,
	mcp.ToolName("okta_get_user"):            getUser,
	mcp.ToolName("okta_create_user"):         createUser,
	mcp.ToolName("okta_update_user"):         updateUser,
	mcp.ToolName("okta_activate_user"):       activateUser,
	mcp.ToolName("okta_deactivate_user"):     deactivateUser,
	mcp.ToolName("okta_suspend_user"):        suspendUser,
	mcp.ToolName("okta_unsuspend_user"):      unsuspendUser,
	mcp.ToolName("okta_unlock_user"):         unlockUser,
	mcp.ToolName("okta_reset_password"):      resetPassword,
	mcp.ToolName("okta_expire_password"):     expirePassword,
	mcp.ToolName("okta_list_user_groups"):    listUserGroups,
	mcp.ToolName("okta_list_user_factors"):   listUserFactors,
	mcp.ToolName("okta_list_groups"):         listGroups,
	mcp.ToolName("okta_get_group"):           getGroup,
	mcp.ToolName("okta_create_group"):        createGroup,
	mcp.ToolName("okta_update_group"):        updateGroup,
	mcp.ToolName("okta_delete_group"):        deleteGroup,
	mcp.ToolName("okta_list_group_members"):  listGroupMembers,
	mcp.ToolName("okta_add_group_member"):    addGroupMember,
	mcp.ToolName("okta_remove_group_member"): removeGroupMember,
	mcp.ToolName("okta_list_apps"):           listApps,
	mcp.ToolName("okta_get_app"):             getApp,
	mcp.ToolName("okta_list_app_users"):      listAppUsers,
	mcp.ToolName("okta_assign_app_user"):     assignAppUser,
	mcp.ToolName("okta_unassign_app_user"):   unassignAppUser,
	mcp.ToolName("okta_list_app_groups"):     listAppGroups,
	mcp.ToolName("okta_assign_app_group"):    assignAppGroup,
	mcp.ToolName("okta_unassign_app_group"):  unassignAppGroup,
	mcp.ToolName("okta_list_policies"):       listPolicies,
	mcp.ToolName("okta_get_policy"):          getPolicy,
	mcp.ToolName("okta_list_policy_rules"):   listPolicyRules,
	mcp.ToolName("okta_list_logs"):           listLogs,
	mcp.ToolName("okta_get_org"):             getOrg,
}
