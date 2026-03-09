package ynab

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	mcp "github.com/daltoniam/switchboard"
)

type ynab struct {
	apiKey  string
	client  *http.Client
	baseURL string
}

var _ mcp.FieldCompactionIntegration = (*ynab)(nil)

const maxResponseSize = 10 * 1024 * 1024 // 10 MB

func New() mcp.Integration {
	return &ynab{
		client:  &http.Client{Timeout: 30 * time.Second},
		baseURL: "https://api.ynab.com/v1",
	}
}

func (y *ynab) Name() string { return "ynab" }

func (y *ynab) Configure(creds mcp.Credentials) error {
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

func (y *ynab) Execute(ctx context.Context, toolName string, args map[string]any) (*mcp.ToolResult, error) {
	fn, ok := dispatch[toolName]
	if !ok {
		return &mcp.ToolResult{Data: fmt.Sprintf("unknown tool: %s", toolName), IsError: true}, nil
	}
	return fn(ctx, y, args)
}

func (y *ynab) CompactSpec(toolName string) ([]mcp.CompactField, bool) {
	fields, ok := fieldCompactionSpecs[toolName]
	return fields, ok
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

func rawResult(data json.RawMessage) (*mcp.ToolResult, error) {
	return &mcp.ToolResult{Data: string(data)}, nil
}

func errResult(err error) (*mcp.ToolResult, error) {
	return &mcp.ToolResult{Data: err.Error(), IsError: true}, nil
}

// --- Argument helpers ---

func argStr(args map[string]any, key string) string {
	v, _ := args[key].(string)
	return v
}

func argInt(args map[string]any, key string) int {
	switch v := args[key].(type) {
	case float64:
		return int(v)
	case int:
		return v
	case string:
		n, _ := strconv.Atoi(v)
		return n
	}
	return 0
}

func argBool(args map[string]any, key string) bool {
	switch v := args[key].(type) {
	case bool:
		return v
	case string:
		return v == "true"
	}
	return false
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

// budget returns the budget_id from args, defaulting to "last-used".
func budget(args map[string]any) string {
	if v := argStr(args, "budget_id"); v != "" {
		return v
	}
	return "last-used"
}

// --- Dispatch map ---

var dispatch = map[string]handlerFunc{
	// User
	"ynab_get_user": getUser,

	// Budgets
	"ynab_list_budgets":       listBudgets,
	"ynab_get_budget":         getBudget,
	"ynab_get_budget_settings": getBudgetSettings,

	// Accounts
	"ynab_list_accounts":  listAccounts,
	"ynab_get_account":    getAccount,
	"ynab_create_account": createAccount,

	// Categories
	"ynab_list_categories":       listCategories,
	"ynab_get_category":          getCategory,
	"ynab_create_category":       createCategory,
	"ynab_update_category":       updateCategory,
	"ynab_get_month_category":    getMonthCategory,
	"ynab_update_month_category": updateMonthCategory,

	// Category Groups
	"ynab_create_category_group": createCategoryGroup,
	"ynab_update_category_group": updateCategoryGroup,

	// Payees
	"ynab_list_payees":  listPayees,
	"ynab_get_payee":    getPayee,
	"ynab_update_payee": updatePayee,

	// Payee Locations
	"ynab_list_payee_locations":    listPayeeLocations,
	"ynab_get_payee_location":      getPayeeLocation,
	"ynab_list_locations_for_payee": listLocationsForPayee,

	// Months
	"ynab_list_months": listMonths,
	"ynab_get_month":   getMonth,

	// Transactions
	"ynab_list_transactions":          listTransactions,
	"ynab_get_transaction":            getTransaction,
	"ynab_list_account_transactions":  listAccountTransactions,
	"ynab_list_category_transactions": listCategoryTransactions,
	"ynab_list_payee_transactions":    listPayeeTransactions,
	"ynab_list_month_transactions":    listMonthTransactions,
	"ynab_create_transaction":         createTransaction,
	"ynab_update_transaction":         updateTransaction,
	"ynab_delete_transaction":         deleteTransaction,

	// Scheduled Transactions
	"ynab_list_scheduled_transactions":   listScheduledTransactions,
	"ynab_get_scheduled_transaction":     getScheduledTransaction,
	"ynab_create_scheduled_transaction":  createScheduledTransaction,
	"ynab_update_scheduled_transaction":  updateScheduledTransaction,
	"ynab_delete_scheduled_transaction":  deleteScheduledTransaction,
}
