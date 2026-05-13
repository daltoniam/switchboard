package ynab

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
	"github.com/daltoniam/switchboard/compactyaml"
)

//go:embed compact.yaml
var compactYAML []byte

var compactResult = compactyaml.MustLoadWithOverlay("ynab", compactYAML, compactyaml.Options{Strict: false})
var fieldCompactionSpecs = compactResult.Specs
var maxBytesByTool = compactResult.MaxBytes

type ynab struct {
	apiKey  string
	client  *http.Client
	baseURL string
}

var (
	_ mcp.FieldCompactionIntegration = (*ynab)(nil)
	_ mcp.ToolMaxBytesIntegration    = (*ynab)(nil)
)

const maxResponseSize = 10 * 1024 * 1024 // 10 MB

func New() mcp.Integration {
	return &ynab{
		client:  &http.Client{Timeout: 30 * time.Second},
		baseURL: "https://api.ynab.com/v1",
	}
}

func (y *ynab) Name() string { return "ynab" }

func (y *ynab) Configure(_ context.Context, creds mcp.Credentials) error {
	y.apiKey = creds["api_key"]
	if y.apiKey == "" {
		return fmt.Errorf("ynab: api_key is required")
	}
	if v := creds["base_url"]; v != "" {
		y.baseURL = strings.TrimRight(v, "/")
	}
	return nil
}

func (y *ynab) Healthy(ctx context.Context) bool {
	_, err := y.get(ctx, "/user")
	return err == nil
}

func (y *ynab) Tools() []mcp.ToolDefinition {
	return tools
}

func (y *ynab) Execute(ctx context.Context, toolName mcp.ToolName, args map[string]any) (*mcp.ToolResult, error) {
	fn, ok := dispatch[toolName]
	if !ok {
		return &mcp.ToolResult{Data: fmt.Sprintf("unknown tool: %s", toolName), IsError: true}, nil
	}
	return fn(ctx, y, args)
}

func (y *ynab) CompactSpec(toolName mcp.ToolName) ([]mcp.CompactField, bool) {
	fields, ok := fieldCompactionSpecs[toolName]
	return fields, ok
}

func (y *ynab) MaxBytes(toolName mcp.ToolName) (int, bool) {
	n, ok := maxBytesByTool[toolName]
	return n, ok
}

// --- HTTP helpers ---

func (y *ynab) doRequest(ctx context.Context, method, path string, body any) (json.RawMessage, error) {
	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		bodyReader = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, y.baseURL+path, bodyReader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+y.apiKey)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := y.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	data, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseSize))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode == 429 || resp.StatusCode >= 500 {
		re := &mcp.RetryableError{StatusCode: resp.StatusCode, Err: fmt.Errorf("ynab API error (%d): %s", resp.StatusCode, string(data))}
		re.RetryAfter = mcp.ParseRetryAfter(resp.Header.Get("Retry-After"))
		return nil, re
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("ynab API error (%d): %s", resp.StatusCode, string(data))
	}
	if resp.StatusCode == 204 || len(data) == 0 {
		return json.RawMessage(`{"status":"success"}`), nil
	}
	return json.RawMessage(data), nil
}

func (y *ynab) get(ctx context.Context, pathFmt string, args ...any) (json.RawMessage, error) {
	return y.doRequest(ctx, "GET", fmt.Sprintf(pathFmt, args...), nil)
}

func (y *ynab) post(ctx context.Context, path string, body any) (json.RawMessage, error) {
	return y.doRequest(ctx, "POST", path, body)
}

func (y *ynab) put(ctx context.Context, path string, body any) (json.RawMessage, error) {
	return y.doRequest(ctx, "PUT", path, body)
}

func (y *ynab) patch(ctx context.Context, path string, body any) (json.RawMessage, error) {
	return y.doRequest(ctx, "PATCH", path, body)
}

func (y *ynab) del(ctx context.Context, pathFmt string, args ...any) (json.RawMessage, error) {
	return y.doRequest(ctx, "DELETE", fmt.Sprintf(pathFmt, args...), nil)
}

// --- Result helpers ---

type handlerFunc func(ctx context.Context, y *ynab, args map[string]any) (*mcp.ToolResult, error)

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

// budget returns the budget_id from args, defaulting to "last-used".
func budget(args map[string]any) string {
	v, _ := mcp.ArgStr(args, "budget_id")
	if v != "" {
		return v
	}
	return "last-used"
}

// --- Dispatch map ---

var dispatch = map[mcp.ToolName]handlerFunc{
	// User
	mcp.ToolName("ynab_get_user"): getUser,

	// Budgets
	mcp.ToolName("ynab_list_budgets"):        listBudgets,
	mcp.ToolName("ynab_get_budget"):          getBudget,
	mcp.ToolName("ynab_get_budget_settings"): getBudgetSettings,

	// Accounts
	mcp.ToolName("ynab_list_accounts"):  listAccounts,
	mcp.ToolName("ynab_get_account"):    getAccount,
	mcp.ToolName("ynab_create_account"): createAccount,

	// Categories
	mcp.ToolName("ynab_list_categories"):       listCategories,
	mcp.ToolName("ynab_get_category"):          getCategory,
	mcp.ToolName("ynab_create_category"):       createCategory,
	mcp.ToolName("ynab_update_category"):       updateCategory,
	mcp.ToolName("ynab_get_month_category"):    getMonthCategory,
	mcp.ToolName("ynab_update_month_category"): updateMonthCategory,

	// Category Groups
	mcp.ToolName("ynab_create_category_group"): createCategoryGroup,
	mcp.ToolName("ynab_update_category_group"): updateCategoryGroup,

	// Payees
	mcp.ToolName("ynab_list_payees"):  listPayees,
	mcp.ToolName("ynab_get_payee"):    getPayee,
	mcp.ToolName("ynab_update_payee"): updatePayee,

	// Payee Locations
	mcp.ToolName("ynab_list_payee_locations"):     listPayeeLocations,
	mcp.ToolName("ynab_get_payee_location"):       getPayeeLocation,
	mcp.ToolName("ynab_list_locations_for_payee"): listLocationsForPayee,

	// Months
	mcp.ToolName("ynab_list_months"): listMonths,
	mcp.ToolName("ynab_get_month"):   getMonth,

	// Transactions
	mcp.ToolName("ynab_list_transactions"):          listTransactions,
	mcp.ToolName("ynab_get_transaction"):            getTransaction,
	mcp.ToolName("ynab_list_account_transactions"):  listAccountTransactions,
	mcp.ToolName("ynab_list_category_transactions"): listCategoryTransactions,
	mcp.ToolName("ynab_list_payee_transactions"):    listPayeeTransactions,
	mcp.ToolName("ynab_list_month_transactions"):    listMonthTransactions,
	mcp.ToolName("ynab_create_transaction"):         createTransaction,
	mcp.ToolName("ynab_update_transaction"):         updateTransaction,
	mcp.ToolName("ynab_delete_transaction"):         deleteTransaction,

	// Scheduled Transactions
	mcp.ToolName("ynab_list_scheduled_transactions"):  listScheduledTransactions,
	mcp.ToolName("ynab_get_scheduled_transaction"):    getScheduledTransaction,
	mcp.ToolName("ynab_create_scheduled_transaction"): createScheduledTransaction,
	mcp.ToolName("ynab_update_scheduled_transaction"): updateScheduledTransaction,
	mcp.ToolName("ynab_delete_scheduled_transaction"): deleteScheduledTransaction,
}
