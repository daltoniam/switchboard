package stripe

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	mcp "github.com/daltoniam/switchboard"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	i := New()
	require.NotNil(t, i)
	assert.Equal(t, "stripe", i.Name())
}

func TestConfigure_Success(t *testing.T) {
	i := New()
	err := i.Configure(context.Background(), mcp.Credentials{"api_key": "sk_test_123"})
	assert.NoError(t, err)
}

func TestConfigure_MissingAPIKey(t *testing.T) {
	i := New()
	err := i.Configure(context.Background(), mcp.Credentials{"api_key": ""})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "api_key is required")
}

func TestConfigure_CustomBaseURL(t *testing.T) {
	s := &stripe{client: &http.Client{}, baseURL: "https://api.stripe.com/v1"}
	err := s.Configure(context.Background(), mcp.Credentials{
		"api_key":  "sk_test",
		"base_url": "https://custom.stripe.example/v1/",
	})
	assert.NoError(t, err)
	assert.Equal(t, "https://custom.stripe.example/v1", s.baseURL)
}

func TestConfigure_Account(t *testing.T) {
	s := &stripe{client: &http.Client{}, baseURL: "https://api.stripe.com/v1"}
	err := s.Configure(context.Background(), mcp.Credentials{
		"api_key": "sk_test",
		"account": "acct_123",
	})
	require.NoError(t, err)
	assert.Equal(t, "acct_123", s.account)
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

func TestTools_AllHaveStripePrefix(t *testing.T) {
	i := New()
	for _, tool := range i.Tools() {
		assert.True(t, strings.HasPrefix(string(tool.Name), "stripe_"), "tool %s missing stripe_ prefix", tool.Name)
	}
}

func TestTools_NoDuplicateNames(t *testing.T) {
	i := New()
	seen := make(map[mcp.ToolName]bool)
	for _, tool := range i.Tools() {
		assert.False(t, seen[tool.Name], "duplicate tool name: %s", tool.Name)
		seen[tool.Name] = true
	}
}

func TestExecute_UnknownTool(t *testing.T) {
	s := &stripe{apiKey: "sk_test", client: &http.Client{}, baseURL: "http://localhost"}
	result, err := s.Execute(context.Background(), "stripe_nonexistent", nil)
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
	toolNames := make(map[mcp.ToolName]bool)
	for _, tool := range i.Tools() {
		toolNames[tool.Name] = true
	}
	for name := range dispatch {
		assert.True(t, toolNames[name], "dispatch handler %s has no tool definition", name)
	}
}

// --- HTTP helper tests ---

func TestDoRequest_GETSuccess(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "Bearer sk_test_123", r.Header.Get("Authorization"))
		assert.Equal(t, http.MethodGet, r.Method)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"cus_abc"}`))
	}))
	defer ts.Close()

	s := &stripe{apiKey: "sk_test_123", client: ts.Client(), baseURL: ts.URL}
	data, err := s.get(context.Background(), "/customers/cus_abc", nil)
	require.NoError(t, err)
	assert.Contains(t, string(data), "cus_abc")
}

func TestDoRequest_StripeAccountHeader(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "acct_999", r.Header.Get("Stripe-Account"))
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer ts.Close()

	s := &stripe{apiKey: "sk_test", account: "acct_999", client: ts.Client(), baseURL: ts.URL}
	_, err := s.get(context.Background(), "/balance", nil)
	require.NoError(t, err)
}

func TestDoRequest_APIError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(400)
		_, _ = w.Write([]byte(`{"error":{"type":"invalid_request_error","message":"missing param"}}`))
	}))
	defer ts.Close()

	s := &stripe{apiKey: "sk_test", client: ts.Client(), baseURL: ts.URL}
	_, err := s.get(context.Background(), "/customers", nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "stripe API error (400)")
}

func TestDoRequest_RetryableOn429(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Retry-After", "2")
		w.WriteHeader(429)
		_, _ = w.Write([]byte(`{"error":"rate limit"}`))
	}))
	defer ts.Close()

	s := &stripe{apiKey: "sk_test", client: ts.Client(), baseURL: ts.URL}
	_, err := s.get(context.Background(), "/balance", nil)
	require.Error(t, err)
	re, ok := err.(*mcp.RetryableError)
	require.True(t, ok, "expected *mcp.RetryableError, got %T", err)
	assert.Equal(t, 429, re.StatusCode)
}

func TestDoRequest_RetryableOn500(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(503)
	}))
	defer ts.Close()

	s := &stripe{apiKey: "sk_test", client: ts.Client(), baseURL: ts.URL}
	_, err := s.get(context.Background(), "/balance", nil)
	require.Error(t, err)
	_, ok := err.(*mcp.RetryableError)
	assert.True(t, ok)
}

func TestDoRequest_204NoContent(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(204)
	}))
	defer ts.Close()

	s := &stripe{apiKey: "sk_test", client: ts.Client(), baseURL: ts.URL}
	data, err := s.del(context.Background(), "/customers/cus_x", nil)
	require.NoError(t, err)
	assert.Contains(t, string(data), "success")
}

func TestPost_FormEncoded(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "application/x-www-form-urlencoded", r.Header.Get("Content-Type"))
		_ = r.ParseForm()
		assert.Equal(t, "alice@example.com", r.PostForm.Get("email"))
		assert.Equal(t, "v1", r.PostForm.Get("metadata[k1]"))
		_, _ = w.Write([]byte(`{"id":"cus_new","email":"alice@example.com"}`))
	}))
	defer ts.Close()

	s := &stripe{apiKey: "sk_test", client: ts.Client(), baseURL: ts.URL}
	data, err := s.post(context.Background(), "/customers", map[string]any{
		"email":    "alice@example.com",
		"metadata": map[string]any{"k1": "v1"},
	})
	require.NoError(t, err)
	assert.Contains(t, string(data), "cus_new")
}

func TestDoRequest_GETWithQuery(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "10", r.URL.Query().Get("limit"))
		assert.Equal(t, "alice@example.com", r.URL.Query().Get("email"))
		_, _ = w.Write([]byte(`{"object":"list","data":[]}`))
	}))
	defer ts.Close()

	s := &stripe{apiKey: "sk_test", client: ts.Client(), baseURL: ts.URL}
	_, err := s.get(context.Background(), "/customers", map[string]any{
		"limit": 10,
		"email": "alice@example.com",
	})
	require.NoError(t, err)
}

// --- Form encoder tests ---

func TestEncodeForm_Empty(t *testing.T) {
	assert.Empty(t, encodeForm(nil))
	assert.Empty(t, encodeForm(map[string]any{}))
}

func TestEncodeForm_Scalars(t *testing.T) {
	out := encodeForm(map[string]any{
		"a": "x",
		"b": 42,
		"c": true,
	})
	// keys sorted
	assert.Equal(t, "a=x&b=42&c=true", out)
}

func TestEncodeForm_SkipsEmptyString(t *testing.T) {
	out := encodeForm(map[string]any{"a": "", "b": "x"})
	assert.Equal(t, "b=x", out)
}

func TestEncodeForm_NestedMap(t *testing.T) {
	out := encodeForm(map[string]any{
		"metadata": map[string]any{"plan": "pro", "tier": "gold"},
	})
	// keys are sorted; bracket-encoded keys URL-escaped
	assert.Contains(t, out, "metadata%5Bplan%5D=pro")
	assert.Contains(t, out, "metadata%5Btier%5D=gold")
}

func TestEncodeForm_ArrayOfObjects(t *testing.T) {
	out := encodeForm(map[string]any{
		"items": []any{
			map[string]any{"price": "price_1"},
			map[string]any{"price": "price_2"},
		},
	})
	assert.Contains(t, out, "items%5B0%5D%5Bprice%5D=price_1")
	assert.Contains(t, out, "items%5B1%5D%5Bprice%5D=price_2")
}

func TestEncodeForm_FloatAsInt(t *testing.T) {
	// JSON numbers come in as float64; integral values should render as ints.
	out := encodeForm(map[string]any{"amount": float64(2000)})
	assert.Equal(t, "amount=2000", out)
}

func TestEncodeForm_StringSlice(t *testing.T) {
	out := encodeForm(map[string]any{"expand": []string{"customer", "default_payment_method"}})
	assert.Contains(t, out, "expand%5B0%5D=customer")
	assert.Contains(t, out, "expand%5B1%5D=default_payment_method")
}

// --- Param helper tests ---

func TestListParamsFrom(t *testing.T) {
	args := map[string]any{
		"limit":          50,
		"starting_after": "cus_abc",
		"email":          "alice@example.com",
		"unused":         "ignored",
	}
	out := listParamsFrom(args, "email")
	assert.Equal(t, 50, out["limit"])
	assert.Equal(t, "cus_abc", out["starting_after"])
	assert.Equal(t, "alice@example.com", out["email"])
	_, hasUnused := out["unused"]
	assert.False(t, hasUnused)
}

func TestSearchParamsFrom(t *testing.T) {
	args := map[string]any{"query": "email:'a@b.com'", "limit": 25, "page": "next_x", "extra": "drop"}
	out := searchParamsFrom(args)
	assert.Equal(t, "email:'a@b.com'", out["query"])
	assert.Equal(t, 25, out["limit"])
	assert.Equal(t, "next_x", out["page"])
	_, hasExtra := out["extra"]
	assert.False(t, hasExtra)
}

// --- Handler integration tests ---

func TestGetBalance(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, "/balance", r.URL.Path)
		_, _ = w.Write([]byte(`{"object":"balance","available":[{"amount":1000,"currency":"usd"}]}`))
	}))
	defer ts.Close()

	s := &stripe{apiKey: "sk_test", client: ts.Client(), baseURL: ts.URL}
	result, err := s.Execute(context.Background(), "stripe_get_balance", map[string]any{})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "available")
}

func TestListCustomers(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, "/customers", r.URL.Path)
		assert.Equal(t, "alice@example.com", r.URL.Query().Get("email"))
		_, _ = w.Write([]byte(`{"object":"list","has_more":false,"data":[{"id":"cus_1","email":"alice@example.com"}]}`))
	}))
	defer ts.Close()

	s := &stripe{apiKey: "sk_test", client: ts.Client(), baseURL: ts.URL}
	result, err := s.Execute(context.Background(), "stripe_list_customers", map[string]any{
		"email": "alice@example.com",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "cus_1")
}

func TestRetrieveCustomer(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/customers/cus_42", r.URL.Path)
		_, _ = w.Write([]byte(`{"id":"cus_42","email":"x@example.com"}`))
	}))
	defer ts.Close()

	s := &stripe{apiKey: "sk_test", client: ts.Client(), baseURL: ts.URL}
	result, err := s.Execute(context.Background(), "stripe_retrieve_customer", map[string]any{"id": "cus_42"})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "cus_42")
}

func TestCreateCustomer(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/customers", r.URL.Path)
		assert.Equal(t, "application/x-www-form-urlencoded", r.Header.Get("Content-Type"))
		_ = r.ParseForm()
		assert.Equal(t, "Alice", r.PostForm.Get("name"))
		assert.Equal(t, "pro", r.PostForm.Get("metadata[plan]"))
		_, _ = w.Write([]byte(`{"id":"cus_new","name":"Alice"}`))
	}))
	defer ts.Close()

	s := &stripe{apiKey: "sk_test", client: ts.Client(), baseURL: ts.URL}
	result, err := s.Execute(context.Background(), "stripe_create_customer", map[string]any{
		"name":     "Alice",
		"metadata": map[string]any{"plan": "pro"},
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "cus_new")
}

func TestUpdateCustomer(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/customers/cus_42", r.URL.Path)
		_ = r.ParseForm()
		assert.Equal(t, "new@example.com", r.PostForm.Get("email"))
		_, _ = w.Write([]byte(`{"id":"cus_42","email":"new@example.com"}`))
	}))
	defer ts.Close()

	s := &stripe{apiKey: "sk_test", client: ts.Client(), baseURL: ts.URL}
	result, err := s.Execute(context.Background(), "stripe_update_customer", map[string]any{
		"id":    "cus_42",
		"email": "new@example.com",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "new@example.com")
}

func TestDeleteCustomer(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodDelete, r.Method)
		assert.Equal(t, "/customers/cus_42", r.URL.Path)
		_, _ = w.Write([]byte(`{"id":"cus_42","deleted":true}`))
	}))
	defer ts.Close()

	s := &stripe{apiKey: "sk_test", client: ts.Client(), baseURL: ts.URL}
	result, err := s.Execute(context.Background(), "stripe_delete_customer", map[string]any{"id": "cus_42"})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "deleted")
}

func TestSearchCustomers(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/customers/search", r.URL.Path)
		assert.Equal(t, `email:"a@b.com"`, r.URL.Query().Get("query"))
		_, _ = w.Write([]byte(`{"object":"search_result","data":[],"has_more":false}`))
	}))
	defer ts.Close()

	s := &stripe{apiKey: "sk_test", client: ts.Client(), baseURL: ts.URL}
	result, err := s.Execute(context.Background(), "stripe_search_customers", map[string]any{
		"query": `email:"a@b.com"`,
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
}

func TestCreatePaymentIntent(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/payment_intents", r.URL.Path)
		_ = r.ParseForm()
		assert.Equal(t, "2000", r.PostForm.Get("amount"))
		assert.Equal(t, "usd", r.PostForm.Get("currency"))
		_, _ = w.Write([]byte(`{"id":"pi_new","amount":2000,"currency":"usd"}`))
	}))
	defer ts.Close()

	s := &stripe{apiKey: "sk_test", client: ts.Client(), baseURL: ts.URL}
	result, err := s.Execute(context.Background(), "stripe_create_payment_intent", map[string]any{
		"amount":   float64(2000),
		"currency": "usd",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "pi_new")
}

func TestCancelSubscription_UsesDELETE(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodDelete, r.Method)
		assert.Equal(t, "/subscriptions/sub_42", r.URL.Path)
		_, _ = w.Write([]byte(`{"id":"sub_42","status":"canceled"}`))
	}))
	defer ts.Close()

	s := &stripe{apiKey: "sk_test", client: ts.Client(), baseURL: ts.URL}
	result, err := s.Execute(context.Background(), "stripe_cancel_subscription", map[string]any{"id": "sub_42"})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "canceled")
}

func TestListInvoices(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/invoices", r.URL.Path)
		assert.Equal(t, "open", r.URL.Query().Get("status"))
		_, _ = w.Write([]byte(`{"object":"list","data":[{"id":"in_1","status":"open"}],"has_more":false}`))
	}))
	defer ts.Close()

	s := &stripe{apiKey: "sk_test", client: ts.Client(), baseURL: ts.URL}
	result, err := s.Execute(context.Background(), "stripe_list_invoices", map[string]any{"status": "open"})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "in_1")
}

func TestRetrieveBalanceTransaction_MissingID(t *testing.T) {
	s := &stripe{apiKey: "sk_test", client: &http.Client{}, baseURL: "http://localhost"}
	result, err := s.Execute(context.Background(), "stripe_retrieve_balance_transaction", map[string]any{})
	require.NoError(t, err)
	assert.True(t, result.IsError)
}
