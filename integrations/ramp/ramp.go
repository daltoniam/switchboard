package ramp

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

var compactResult = compact.MustLoadWithOverlay("ramp", compactYAML, compact.Options{Strict: false})
var fieldCompactionSpecs = compactResult.Specs
var maxBytesByTool = compactResult.MaxBytes

type ramp struct {
	accessToken string
	client      *http.Client
	baseURL     string
}

const maxResponseSize = 10 * 1024 * 1024 // 10 MB

var (
	_ mcp.Integration                = (*ramp)(nil)
	_ mcp.FieldCompactionIntegration = (*ramp)(nil)
	_ mcp.ToolMaxBytesIntegration    = (*ramp)(nil)
	_ mcp.PlainTextCredentials       = (*ramp)(nil)
	_ mcp.PlaceholderHints           = (*ramp)(nil)
	_ mcp.OptionalCredentials        = (*ramp)(nil)
)

func (r *ramp) PlainTextKeys() []string { return []string{"base_url"} }

func (r *ramp) Placeholders() map[string]string {
	return map[string]string{
		"access_token": "Ramp OAuth access token or API bearer token",
		"base_url":     "https://api.ramp.com (default) or https://demo-api.ramp.com",
	}
}

func (r *ramp) OptionalKeys() []string { return []string{"base_url"} }

func New() mcp.Integration {
	return &ramp{
		client:  &http.Client{Timeout: 30 * time.Second},
		baseURL: "https://api.ramp.com",
	}
}

func (r *ramp) Name() string { return "ramp" }

func (r *ramp) Configure(_ context.Context, creds mcp.Credentials) error {
	r.accessToken = creds["access_token"]
	if r.accessToken == "" {
		return fmt.Errorf("ramp: access_token is required")
	}
	if v := creds["base_url"]; v != "" {
		r.baseURL = strings.TrimRight(v, "/")
	}
	return nil
}

func (r *ramp) Healthy(ctx context.Context) bool {
	_, err := r.get(ctx, "/developer/v1/users?page_size=2")
	return err == nil
}

func (r *ramp) Tools() []mcp.ToolDefinition {
	return tools
}

func (r *ramp) CompactSpec(toolName mcp.ToolName) ([]mcp.CompactField, bool) {
	fields, ok := fieldCompactionSpecs[toolName]
	return fields, ok
}

func (r *ramp) MaxBytes(toolName mcp.ToolName) (int, bool) {
	n, ok := maxBytesByTool[toolName]
	return n, ok
}

func (r *ramp) Execute(ctx context.Context, toolName mcp.ToolName, args map[string]any) (*mcp.ToolResult, error) {
	fn, ok := dispatch[toolName]
	if !ok {
		return &mcp.ToolResult{Data: fmt.Sprintf("unknown tool: %s", toolName), IsError: true}, nil
	}
	return fn(ctx, r, args)
}

func (r *ramp) doRequest(ctx context.Context, method, path string, body any) (json.RawMessage, error) {
	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		bodyReader = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, r.baseURL+path, bodyReader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+r.accessToken)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := r.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	data, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseSize))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode == 429 || resp.StatusCode >= 500 {
		re := &mcp.RetryableError{StatusCode: resp.StatusCode, Err: fmt.Errorf("ramp API error (%d): %s", resp.StatusCode, string(data))}
		re.RetryAfter = mcp.ParseRetryAfter(resp.Header.Get("Retry-After"))
		return nil, re
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("ramp API error (%d): %s", resp.StatusCode, string(data))
	}
	if resp.StatusCode == 204 || len(data) == 0 {
		return json.RawMessage(`{"status":"success"}`), nil
	}
	return json.RawMessage(data), nil
}

func (r *ramp) get(ctx context.Context, pathFmt string, args ...any) (json.RawMessage, error) {
	return r.doRequest(ctx, http.MethodGet, fmt.Sprintf(pathFmt, args...), nil)
}

func (r *ramp) patch(ctx context.Context, path string, body any) (json.RawMessage, error) {
	return r.doRequest(ctx, http.MethodPatch, path, body)
}

type handlerFunc func(ctx context.Context, r *ramp, args map[string]any) (*mcp.ToolResult, error)

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

func pageParams(args map[string]any) map[string]string {
	r := mcp.NewArgs(args)
	start := r.Str("start")
	pageSize := r.Str("page_size")
	_ = r.Err()
	return map[string]string{
		"start":     start,
		"page_size": pageSize,
	}
}

var dispatch = map[mcp.ToolName]handlerFunc{
	mcp.ToolName("ramp_list_transactions"):   listTransactions,
	mcp.ToolName("ramp_get_transaction"):     getTransaction,
	mcp.ToolName("ramp_update_transaction"):  updateTransaction,
	mcp.ToolName("ramp_list_reimbursements"): listReimbursements,
	mcp.ToolName("ramp_get_reimbursement"):   getReimbursement,
	mcp.ToolName("ramp_list_bills"):          listBills,
	mcp.ToolName("ramp_get_bill"):            getBill,
	mcp.ToolName("ramp_list_users"):          listUsers,
	mcp.ToolName("ramp_get_user"):            getUser,
	mcp.ToolName("ramp_list_virtual_cards"):  listVirtualCards,
	mcp.ToolName("ramp_get_virtual_card"):    getVirtualCard,
	mcp.ToolName("ramp_list_physical_cards"): listPhysicalCards,
	mcp.ToolName("ramp_get_physical_card"):   getPhysicalCard,
	mcp.ToolName("ramp_list_receipts"):       listReceipts,
	mcp.ToolName("ramp_get_receipt"):         getReceipt,
	mcp.ToolName("ramp_list_departments"):    listDepartments,
	mcp.ToolName("ramp_get_department"):      getDepartment,
	mcp.ToolName("ramp_list_locations"):      listLocations,
	mcp.ToolName("ramp_get_location"):        getLocation,
	mcp.ToolName("ramp_list_merchants"):      listMerchants,
	mcp.ToolName("ramp_list_vendors"):        listVendors,
	mcp.ToolName("ramp_get_vendor"):          getVendor,
	mcp.ToolName("ramp_list_entities"):       listEntities,
	mcp.ToolName("ramp_get_entity"):          getEntity,
	mcp.ToolName("ramp_list_bank_accounts"):  listBankAccounts,
	mcp.ToolName("ramp_get_bank_account"):    getBankAccount,
}
