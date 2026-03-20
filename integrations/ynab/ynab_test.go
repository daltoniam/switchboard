package ynab

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	mcp "github.com/daltoniam/switchboard"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	i := New()
	require.NotNil(t, i)
	assert.Equal(t, "ynab", i.Name())
}

func TestConfigure_Success(t *testing.T) {
	i := New()
	err := i.Configure(context.Background(), mcp.Credentials{"api_key": "test-token"})
	assert.NoError(t, err)
}

func TestConfigure_MissingAPIKey(t *testing.T) {
	i := New()
	err := i.Configure(context.Background(), mcp.Credentials{"api_key": ""})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "api_key is required")
}

func TestConfigure_CustomBaseURL(t *testing.T) {
	y := &ynab{client: &http.Client{}, baseURL: "https://api.ynab.com/v1"}
	err := y.Configure(context.Background(), mcp.Credentials{
		"api_key":  "test",
		"base_url": "https://custom.ynab.com/v1/",
	})
	assert.NoError(t, err)
	assert.Equal(t, "https://custom.ynab.com/v1", y.baseURL)
}

func TestTools(t *testing.T) {
	i := New()
	tools := i.Tools()
	assert.NotEmpty(t, tools)

	for _, tool := range tools {
		assert.NotEmpty(t, tool.Name, "tool has empty name")
		assert.NotEmpty(t, tool.Description, "tool %s has empty description", tool.Name)
	}
}

func TestTools_AllHaveYnabPrefix(t *testing.T) {
	i := New()
	for _, tool := range i.Tools() {
		assert.Contains(t, tool.Name, "ynab_", "tool %s missing ynab_ prefix", tool.Name)
	}
}

func TestTools_NoDuplicateNames(t *testing.T) {
	i := New()
	seen := make(map[string]bool)
	for _, tool := range i.Tools() {
		assert.False(t, seen[tool.Name], "duplicate tool name: %s", tool.Name)
		seen[tool.Name] = true
	}
}

func TestExecute_UnknownTool(t *testing.T) {
	y := &ynab{apiKey: "test", client: &http.Client{}, baseURL: "http://localhost"}
	result, err := y.Execute(context.Background(), "ynab_nonexistent", nil)
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "unknown tool")
}

func TestDispatchMap_AllToolsCovered(t *testing.T) {
	i := New()
	for _, tool := range i.Tools() {
		_, ok := dispatch[tool.Name]
		assert.True(t, ok, "tool %s has no dispatch handler", tool.Name)
	}
}

func TestDispatchMap_NoOrphanHandlers(t *testing.T) {
	i := New()
	toolNames := make(map[string]bool)
	for _, tool := range i.Tools() {
		toolNames[tool.Name] = true
	}
	for name := range dispatch {
		assert.True(t, toolNames[name], "dispatch handler %s has no tool definition", name)
	}
}

// --- HTTP helper tests ---

func TestDoRequest_Success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{"user":{"id":"abc"}}}`))
	}))
	defer ts.Close()

	y := &ynab{apiKey: "test-token", client: ts.Client(), baseURL: ts.URL}
	data, err := y.get(context.Background(), "/user")
	require.NoError(t, err)
	assert.Contains(t, string(data), "abc")
}

func TestDoRequest_APIError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(401)
		_, _ = w.Write([]byte(`{"error":{"id":"401","name":"unauthorized","detail":"Invalid token"}}`))
	}))
	defer ts.Close()

	y := &ynab{apiKey: "bad-token", client: ts.Client(), baseURL: ts.URL}
	_, err := y.get(context.Background(), "/user")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "ynab API error (401)")
}

func TestDoRequest_204NoContent(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(204)
	}))
	defer ts.Close()

	y := &ynab{apiKey: "token", client: ts.Client(), baseURL: ts.URL}
	data, err := y.doRequest(context.Background(), "DELETE", "/test", nil)
	require.NoError(t, err)
	assert.Contains(t, string(data), "success")
}

func TestPost(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		assert.NotEmpty(t, body)
		_, _ = w.Write([]byte(`{"data":{"created":true}}`))
	}))
	defer ts.Close()

	y := &ynab{apiKey: "token", client: ts.Client(), baseURL: ts.URL}
	data, err := y.post(context.Background(), "/test", map[string]string{"key": "val"})
	require.NoError(t, err)
	assert.Contains(t, string(data), "created")
}

func TestPut(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "PUT", r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		_, _ = w.Write([]byte(`{"data":{"updated":true}}`))
	}))
	defer ts.Close()

	y := &ynab{apiKey: "token", client: ts.Client(), baseURL: ts.URL}
	data, err := y.put(context.Background(), "/test", map[string]string{"key": "val"})
	require.NoError(t, err)
	assert.Contains(t, string(data), "updated")
}

func TestPatch(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "PATCH", r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		_, _ = w.Write([]byte(`{"data":{"updated":true}}`))
	}))
	defer ts.Close()

	y := &ynab{apiKey: "token", client: ts.Client(), baseURL: ts.URL}
	data, err := y.patch(context.Background(), "/test", map[string]string{"key": "val"})
	require.NoError(t, err)
	assert.Contains(t, string(data), "updated")
}

// --- Result helper tests ---

func TestRawResult(t *testing.T) {
	data := json.RawMessage(`{"key":"value"}`)
	result, err := mcp.RawResult(data)
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Equal(t, `{"key":"value"}`, result.Data)
}

func TestErrResult(t *testing.T) {
	result, err := mcp.ErrResult(fmt.Errorf("test error"))
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Equal(t, "test error", result.Data)
}

// --- Argument helper tests ---

func TestArgStr(t *testing.T) {
	v, err := mcp.ArgStr(map[string]any{"k": "val"}, "k")
	assert.NoError(t, err)
	assert.Equal(t, "val", v)
	v, err = mcp.ArgStr(map[string]any{}, "k")
	assert.NoError(t, err)
	assert.Empty(t, v)
}

func TestArgInt(t *testing.T) {
	v, err := mcp.ArgInt(map[string]any{"n": float64(42)}, "n")
	assert.NoError(t, err)
	assert.Equal(t, 42, v)
	v, err = mcp.ArgInt(map[string]any{"n": 42}, "n")
	assert.NoError(t, err)
	assert.Equal(t, 42, v)
	v, err = mcp.ArgInt(map[string]any{"n": "42"}, "n")
	assert.NoError(t, err)
	assert.Equal(t, 42, v)
	v, err = mcp.ArgInt(map[string]any{}, "n")
	assert.NoError(t, err)
	assert.Equal(t, 0, v)
}

func TestArgBool(t *testing.T) {
	v, err := mcp.ArgBool(map[string]any{"b": true}, "b")
	assert.NoError(t, err)
	assert.True(t, v)
	v, err = mcp.ArgBool(map[string]any{"b": false}, "b")
	assert.NoError(t, err)
	assert.False(t, v)
	v, err = mcp.ArgBool(map[string]any{"b": "true"}, "b")
	assert.NoError(t, err)
	assert.True(t, v)
	v, err = mcp.ArgBool(map[string]any{}, "b")
	assert.NoError(t, err)
	assert.False(t, v)
}

func TestQueryEncode(t *testing.T) {
	t.Run("with values", func(t *testing.T) {
		result := queryEncode(map[string]string{"key": "val", "empty": ""})
		assert.Contains(t, result, "key=val")
		assert.NotContains(t, result, "empty")
		assert.True(t, result[0] == '?')
	})

	t.Run("all empty", func(t *testing.T) {
		result := queryEncode(map[string]string{"empty": ""})
		assert.Empty(t, result)
	})
}

func TestBudget(t *testing.T) {
	assert.Equal(t, "last-used", budget(map[string]any{}))
	assert.Equal(t, "my-budget-id", budget(map[string]any{"budget_id": "my-budget-id"}))
}

// --- Handler integration tests ---

func TestGetUser(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Equal(t, "/user", r.URL.Path)
		_, _ = w.Write([]byte(`{"data":{"user":{"id":"abc-123"}}}`))
	}))
	defer ts.Close()

	y := &ynab{apiKey: "token", client: ts.Client(), baseURL: ts.URL}
	result, err := y.Execute(context.Background(), "ynab_get_user", map[string]any{})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "abc-123")
}

func TestListBudgets(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Equal(t, "/budgets", r.URL.Path)
		_, _ = w.Write([]byte(`{"data":{"budgets":[{"id":"budget-1","name":"My Budget"}]}}`))
	}))
	defer ts.Close()

	y := &ynab{apiKey: "token", client: ts.Client(), baseURL: ts.URL}
	result, err := y.Execute(context.Background(), "ynab_list_budgets", map[string]any{})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "My Budget")
}

func TestListAccounts(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Contains(t, r.URL.Path, "/budgets/last-used/accounts")
		_, _ = w.Write([]byte(`{"data":{"accounts":[{"id":"acc-1","name":"Checking","balance":150000}]}}`))
	}))
	defer ts.Close()

	y := &ynab{apiKey: "token", client: ts.Client(), baseURL: ts.URL}
	result, err := y.Execute(context.Background(), "ynab_list_accounts", map[string]any{})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "Checking")
}

func TestCreateAccount(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		acct := body["account"].(map[string]any)
		assert.Equal(t, "Savings", acct["name"])
		assert.Equal(t, "savings", acct["type"])
		_, _ = w.Write([]byte(`{"data":{"account":{"id":"acc-2","name":"Savings"}}}`))
	}))
	defer ts.Close()

	y := &ynab{apiKey: "token", client: ts.Client(), baseURL: ts.URL}
	result, err := y.Execute(context.Background(), "ynab_create_account", map[string]any{
		"name":    "Savings",
		"type":    "savings",
		"balance": float64(100000),
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "Savings")
}

func TestListCategories(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Contains(t, r.URL.Path, "/budgets/last-used/categories")
		_, _ = w.Write([]byte(`{"data":{"category_groups":[{"id":"grp-1","name":"Bills","categories":[{"id":"cat-1","name":"Rent"}]}]}}`))
	}))
	defer ts.Close()

	y := &ynab{apiKey: "token", client: ts.Client(), baseURL: ts.URL}
	result, err := y.Execute(context.Background(), "ynab_list_categories", map[string]any{})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "Rent")
}

func TestListTransactions(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Contains(t, r.URL.Path, "/budgets/last-used/transactions")
		_, _ = w.Write([]byte(`{"data":{"transactions":[{"id":"txn-1","amount":-50000,"payee_name":"Grocery Store"}]}}`))
	}))
	defer ts.Close()

	y := &ynab{apiKey: "token", client: ts.Client(), baseURL: ts.URL}
	result, err := y.Execute(context.Background(), "ynab_list_transactions", map[string]any{})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "Grocery Store")
}

func TestListTransactions_WithFilters(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "2024-01-01", r.URL.Query().Get("since_date"))
		assert.Equal(t, "unapproved", r.URL.Query().Get("type"))
		_, _ = w.Write([]byte(`{"data":{"transactions":[]}}`))
	}))
	defer ts.Close()

	y := &ynab{apiKey: "token", client: ts.Client(), baseURL: ts.URL}
	result, err := y.Execute(context.Background(), "ynab_list_transactions", map[string]any{
		"since_date": "2024-01-01",
		"type":       "unapproved",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
}

func TestCreateTransaction(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		txn := body["transaction"].(map[string]any)
		assert.Equal(t, "acc-1", txn["account_id"])
		assert.Equal(t, "2024-03-15", txn["date"])
		assert.Equal(t, float64(-25000), txn["amount"])
		assert.Equal(t, "Coffee Shop", txn["payee_name"])
		_, _ = w.Write([]byte(`{"data":{"transaction":{"id":"txn-new"}}}`))
	}))
	defer ts.Close()

	y := &ynab{apiKey: "token", client: ts.Client(), baseURL: ts.URL}
	result, err := y.Execute(context.Background(), "ynab_create_transaction", map[string]any{
		"account_id": "acc-1",
		"date":       "2024-03-15",
		"amount":     float64(-25000),
		"payee_name": "Coffee Shop",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "txn-new")
}

func TestUpdateTransaction(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "PUT", r.Method)
		assert.Contains(t, r.URL.Path, "/transactions/txn-1")
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		txn := body["transaction"].(map[string]any)
		assert.Equal(t, "Updated memo", txn["memo"])
		_, _ = w.Write([]byte(`{"data":{"transaction":{"id":"txn-1","memo":"Updated memo"}}}`))
	}))
	defer ts.Close()

	y := &ynab{apiKey: "token", client: ts.Client(), baseURL: ts.URL}
	result, err := y.Execute(context.Background(), "ynab_update_transaction", map[string]any{
		"transaction_id": "txn-1",
		"memo":           "Updated memo",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "Updated memo")
}

func TestDeleteTransaction(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "DELETE", r.Method)
		assert.Contains(t, r.URL.Path, "/transactions/txn-1")
		_, _ = w.Write([]byte(`{"data":{"transaction":{"id":"txn-1","deleted":true}}}`))
	}))
	defer ts.Close()

	y := &ynab{apiKey: "token", client: ts.Client(), baseURL: ts.URL}
	result, err := y.Execute(context.Background(), "ynab_delete_transaction", map[string]any{
		"transaction_id": "txn-1",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "deleted")
}

func TestListScheduledTransactions(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Contains(t, r.URL.Path, "/budgets/last-used/scheduled_transactions")
		_, _ = w.Write([]byte(`{"data":{"scheduled_transactions":[{"id":"st-1","frequency":"monthly"}]}}`))
	}))
	defer ts.Close()

	y := &ynab{apiKey: "token", client: ts.Client(), baseURL: ts.URL}
	result, err := y.Execute(context.Background(), "ynab_list_scheduled_transactions", map[string]any{})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "monthly")
}

func TestBudgetOverride(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Contains(t, r.URL.Path, "/budgets/custom-budget/")
		_, _ = w.Write([]byte(`{"data":{"settings":{}}}`))
	}))
	defer ts.Close()

	y := &ynab{apiKey: "token", client: ts.Client(), baseURL: ts.URL}
	result, err := y.Execute(context.Background(), "ynab_get_budget_settings", map[string]any{
		"budget_id": "custom-budget",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
}

func TestUpdateMonthCategory(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "PATCH", r.Method)
		assert.Contains(t, r.URL.Path, "/months/2024-03-01/categories/cat-1")
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		cat := body["category"].(map[string]any)
		assert.Equal(t, float64(150000), cat["budgeted"])
		_, _ = w.Write([]byte(`{"data":{"category":{"id":"cat-1","budgeted":150000}}}`))
	}))
	defer ts.Close()

	y := &ynab{apiKey: "token", client: ts.Client(), baseURL: ts.URL}
	result, err := y.Execute(context.Background(), "ynab_update_month_category", map[string]any{
		"month":       "2024-03-01",
		"category_id": "cat-1",
		"budgeted":    float64(150000),
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "150000")
}

func TestListPayees(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Contains(t, r.URL.Path, "/budgets/last-used/payees")
		_, _ = w.Write([]byte(`{"data":{"payees":[{"id":"p-1","name":"Landlord"}]}}`))
	}))
	defer ts.Close()

	y := &ynab{apiKey: "token", client: ts.Client(), baseURL: ts.URL}
	result, err := y.Execute(context.Background(), "ynab_list_payees", map[string]any{})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "Landlord")
}

func TestListMonths(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Contains(t, r.URL.Path, "/budgets/last-used/months")
		_, _ = w.Write([]byte(`{"data":{"months":[{"month":"2024-03-01","to_be_budgeted":500000}]}}`))
	}))
	defer ts.Close()

	y := &ynab{apiKey: "token", client: ts.Client(), baseURL: ts.URL}
	result, err := y.Execute(context.Background(), "ynab_list_months", map[string]any{})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "2024-03-01")
}

func TestCreateCategoryGroup(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Contains(t, r.URL.Path, "/budgets/last-used/category_groups")
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		grp := body["category_group"].(map[string]any)
		assert.Equal(t, "Subscriptions", grp["name"])
		_, _ = w.Write([]byte(`{"data":{"category_group":{"id":"grp-new","name":"Subscriptions"}}}`))
	}))
	defer ts.Close()

	y := &ynab{apiKey: "token", client: ts.Client(), baseURL: ts.URL}
	result, err := y.Execute(context.Background(), "ynab_create_category_group", map[string]any{
		"name": "Subscriptions",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "Subscriptions")
}

func TestUpdatePayee(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "PATCH", r.Method)
		assert.Contains(t, r.URL.Path, "/payees/p-1")
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		payee := body["payee"].(map[string]any)
		assert.Equal(t, "New Landlord", payee["name"])
		_, _ = w.Write([]byte(`{"data":{"payee":{"id":"p-1","name":"New Landlord"}}}`))
	}))
	defer ts.Close()

	y := &ynab{apiKey: "token", client: ts.Client(), baseURL: ts.URL}
	result, err := y.Execute(context.Background(), "ynab_update_payee", map[string]any{
		"payee_id": "p-1",
		"name":     "New Landlord",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "New Landlord")
}

func TestListPayeeLocations(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Contains(t, r.URL.Path, "/budgets/last-used/payee_locations")
		_, _ = w.Write([]byte(`{"data":{"payee_locations":[{"id":"loc-1","payee_id":"p-1","latitude":"40.7128","longitude":"-74.0060"}]}}`))
	}))
	defer ts.Close()

	y := &ynab{apiKey: "token", client: ts.Client(), baseURL: ts.URL}
	result, err := y.Execute(context.Background(), "ynab_list_payee_locations", map[string]any{})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "40.7128")
}

func TestListMonthTransactions(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Contains(t, r.URL.Path, "/budgets/last-used/months/2024-03-01/transactions")
		_, _ = w.Write([]byte(`{"data":{"transactions":[{"id":"txn-m1","amount":-30000}]}}`))
	}))
	defer ts.Close()

	y := &ynab{apiKey: "token", client: ts.Client(), baseURL: ts.URL}
	result, err := y.Execute(context.Background(), "ynab_list_month_transactions", map[string]any{
		"month": "2024-03-01",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "txn-m1")
}

func TestCreateScheduledTransaction(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Contains(t, r.URL.Path, "/budgets/last-used/scheduled_transactions")
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		st := body["scheduled_transaction"].(map[string]any)
		assert.Equal(t, "acc-1", st["account_id"])
		assert.Equal(t, "2024-04-01", st["date"])
		assert.Equal(t, float64(-100000), st["amount"])
		assert.Equal(t, "monthly", st["frequency"])
		assert.Equal(t, "Rent payment", st["memo"])
		_, _ = w.Write([]byte(`{"data":{"scheduled_transaction":{"id":"st-new","frequency":"monthly"}}}`))
	}))
	defer ts.Close()

	y := &ynab{apiKey: "token", client: ts.Client(), baseURL: ts.URL}
	result, err := y.Execute(context.Background(), "ynab_create_scheduled_transaction", map[string]any{
		"account_id": "acc-1",
		"date":       "2024-04-01",
		"amount":     float64(-100000),
		"frequency":  "monthly",
		"memo":       "Rent payment",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "st-new")
}

func TestUpdateScheduledTransaction(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "PUT", r.Method)
		assert.Contains(t, r.URL.Path, "/scheduled_transactions/st-1")
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		st := body["scheduled_transaction"].(map[string]any)
		assert.Equal(t, "Updated memo", st["memo"])
		_, _ = w.Write([]byte(`{"data":{"scheduled_transaction":{"id":"st-1","memo":"Updated memo"}}}`))
	}))
	defer ts.Close()

	y := &ynab{apiKey: "token", client: ts.Client(), baseURL: ts.URL}
	result, err := y.Execute(context.Background(), "ynab_update_scheduled_transaction", map[string]any{
		"scheduled_transaction_id": "st-1",
		"memo":                     "Updated memo",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "Updated memo")
}

func TestDeleteScheduledTransaction(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "DELETE", r.Method)
		assert.Contains(t, r.URL.Path, "/scheduled_transactions/st-1")
		_, _ = w.Write([]byte(`{"data":{"scheduled_transaction":{"id":"st-1","deleted":true}}}`))
	}))
	defer ts.Close()

	y := &ynab{apiKey: "token", client: ts.Client(), baseURL: ts.URL}
	result, err := y.Execute(context.Background(), "ynab_delete_scheduled_transaction", map[string]any{
		"scheduled_transaction_id": "st-1",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "deleted")
}
