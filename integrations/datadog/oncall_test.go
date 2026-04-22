package datadog

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/DataDog/datadog-api-client-go/v2/api/datadog"
	mcp "github.com/daltoniam/switchboard"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupDD returns a *dd pointed at ts for both SDK and raw-HTTP calls.
func setupDD(t *testing.T, ts *httptest.Server) *dd {
	t.Helper()
	cfg := datadog.NewConfiguration()
	cfg.Servers = datadog.ServerConfigurations{{URL: ts.URL}}
	cfg.HTTPClient = ts.Client()
	d := &dd{
		apiKey: "test-api-key",
		appKey: "test-app-key",
		client: datadog.NewAPIClient(cfg),
	}
	return d
}

// jsonHandler returns a simple handler that writes status + body.
func jsonHandler(status int, body string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}
}

// ── ddGet raw HTTP helper ────────────────────────────────────────────

func TestDdGet_Success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, "/api/v2/on-call/schedules", r.URL.Path)
		assert.Equal(t, "teams", r.URL.Query().Get("include"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"id":"s1","type":"schedules"}]}`))
	}))
	defer ts.Close()

	d := setupDD(t, ts)
	ctx := d.ctx(context.Background())

	result, err := listOnCallSchedules(ctx, d, map[string]any{"include": "teams"})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, `"s1"`)
}

func TestDdGet_HTTPError(t *testing.T) {
	ts := httptest.NewServer(jsonHandler(404, `{"errors":["not found"]}`))
	defer ts.Close()

	d := setupDD(t, ts)
	ctx := d.ctx(context.Background())

	result, err := listOnCallSchedules(ctx, d, map[string]any{})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "HTTP 404")
}

// ── List schedules ───────────────────────────────────────────────────

func TestListOnCallSchedules_NoArgs(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Empty(t, r.URL.Query().Get("include"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[]}`))
	}))
	defer ts.Close()

	d := setupDD(t, ts)
	ctx := d.ctx(context.Background())
	result, err := listOnCallSchedules(ctx, d, map[string]any{})
	require.NoError(t, err)
	assert.False(t, result.IsError)
}

// ── List escalation policies ─────────────────────────────────────────

func TestListOnCallEscalationPolicies_Success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v2/on-call/escalation-policies", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"id":"p1","type":"policies"}]}`))
	}))
	defer ts.Close()

	d := setupDD(t, ts)
	ctx := d.ctx(context.Background())
	result, err := listOnCallEscalationPolicies(ctx, d, map[string]any{})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, `"p1"`)
}

// ── Get escalation policy (fixed include) ────────────────────────────

func TestGetOnCallEscalationPolicy_CorrectInclude(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		include := r.URL.Query().Get("include")
		assert.Equal(t, "steps,steps.targets", include, "include param must use valid API values")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{"id":"p1","type":"policies","attributes":{"name":"test"}}}`))
	}))
	defer ts.Close()

	d := setupDD(t, ts)
	ctx := d.ctx(context.Background())
	result, err := getOnCallEscalationPolicy(ctx, d, map[string]any{"policy_id": "p1"})
	require.NoError(t, err)
	assert.False(t, result.IsError)
}

func TestGetOnCallEscalationPolicy_MissingArg(t *testing.T) {
	ts := httptest.NewServer(jsonHandler(400, `{"errors":["bad request"]}`))
	defer ts.Close()
	d := setupDD(t, ts)
	ctx := d.ctx(context.Background())
	result, err := getOnCallEscalationPolicy(ctx, d, map[string]any{})
	require.NoError(t, err)
	assert.True(t, result.IsError)
}

// ── Get team routing rules (fixed include) ───────────────────────────

func TestGetOnCallTeamRoutingRules_CorrectInclude(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		include := r.URL.Query().Get("include")
		assert.Equal(t, "rules,rules.policy", include, "include param must use valid API values")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{"id":"tr1","type":"team_routing_rules"}}`))
	}))
	defer ts.Close()

	d := setupDD(t, ts)
	ctx := d.ctx(context.Background())
	result, err := getOnCallTeamRoutingRules(ctx, d, map[string]any{"team_id": "team1"})
	require.NoError(t, err)
	assert.False(t, result.IsError)
}

func TestGetOnCallTeamRoutingRules_MissingArg(t *testing.T) {
	ts := httptest.NewServer(jsonHandler(400, `{"errors":["bad request"]}`))
	defer ts.Close()
	d := setupDD(t, ts)
	ctx := d.ctx(context.Background())
	result, err := getOnCallTeamRoutingRules(ctx, d, map[string]any{})
	require.NoError(t, err)
	assert.True(t, result.IsError)
}

// ── List/get pages ───────────────────────────────────────────────────

func TestListOnCallPages_WithFilters(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v2/on-call/pages", r.URL.Path)
		assert.Equal(t, "triggered", r.URL.Query().Get("filter[status]"))
		assert.Equal(t, "high", r.URL.Query().Get("filter[urgency]"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[]}`))
	}))
	defer ts.Close()

	d := setupDD(t, ts)
	ctx := d.ctx(context.Background())
	result, err := listOnCallPages(ctx, d, map[string]any{"status": "triggered", "urgency": "high"})
	require.NoError(t, err)
	assert.False(t, result.IsError)
}

func TestGetOnCallPage_Success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.True(t, strings.HasPrefix(r.URL.Path, "/api/v2/on-call/pages/"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{"id":"page1","attributes":{"title":"Alert"}}}`))
	}))
	defer ts.Close()

	d := setupDD(t, ts)
	ctx := d.ctx(context.Background())
	result, err := getOnCallPage(ctx, d, map[string]any{"page_id": "page1"})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "Alert")
}

func TestGetOnCallPage_MissingArg(t *testing.T) {
	ts := httptest.NewServer(jsonHandler(400, `{"errors":["bad request"]}`))
	defer ts.Close()
	d := setupDD(t, ts)
	ctx := d.ctx(context.Background())
	result, err := getOnCallPage(ctx, d, map[string]any{})
	require.NoError(t, err)
	assert.True(t, result.IsError)
}

// ── Notification channels ────────────────────────────────────────────

func TestListUserNotificationChannels_Success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Contains(t, r.URL.Path, "/notification-channels")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"id":"ch1","type":"notification_channels"}]}`))
	}))
	defer ts.Close()

	d := setupDD(t, ts)
	ctx := d.ctx(context.Background())
	result, err := listUserNotificationChannels(ctx, d, map[string]any{"user_id": "u1"})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, `"ch1"`)
}

func TestListUserNotificationChannels_MissingArg(t *testing.T) {
	ts := httptest.NewServer(jsonHandler(400, `{"errors":["bad request"]}`))
	defer ts.Close()
	d := setupDD(t, ts)
	ctx := d.ctx(context.Background())
	result, err := listUserNotificationChannels(ctx, d, map[string]any{})
	require.NoError(t, err)
	assert.True(t, result.IsError)
}

func TestGetUserNotificationChannel_MissingArgs(t *testing.T) {
	ts := httptest.NewServer(jsonHandler(400, `{"errors":["bad request"]}`))
	defer ts.Close()
	d := setupDD(t, ts)
	ctx := d.ctx(context.Background())
	result, err := getUserNotificationChannel(ctx, d, map[string]any{})
	require.NoError(t, err)
	assert.True(t, result.IsError)
}

func TestDeleteUserNotificationChannel_MissingArgs(t *testing.T) {
	ts := httptest.NewServer(jsonHandler(400, `{"errors":["bad request"]}`))
	defer ts.Close()
	d := setupDD(t, ts)
	ctx := d.ctx(context.Background())
	result, err := deleteUserNotificationChannel(ctx, d, map[string]any{})
	require.NoError(t, err)
	assert.True(t, result.IsError)
}

func TestCreateUserNotificationChannel_MissingArgs(t *testing.T) {
	d := &dd{}
	result, err := createUserNotificationChannel(context.Background(), d, map[string]any{})
	require.NoError(t, err)
	assert.True(t, result.IsError)
}

func TestCreateUserNotificationChannel_InvalidJSON(t *testing.T) {
	d := &dd{}
	result, err := createUserNotificationChannel(context.Background(), d, map[string]any{
		"user_id": "u1", "body_json": "not-json",
	})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "invalid body_json")
}

// ── Notification rules ───────────────────────────────────────────────

func TestListUserNotificationRules_Success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Contains(t, r.URL.Path, "/notification-rules")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"id":"r1","type":"notification_rules"}]}`))
	}))
	defer ts.Close()

	d := setupDD(t, ts)
	ctx := d.ctx(context.Background())
	result, err := listUserNotificationRules(ctx, d, map[string]any{"user_id": "u1"})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, `"r1"`)
}

func TestListUserNotificationRules_MissingArg(t *testing.T) {
	ts := httptest.NewServer(jsonHandler(400, `{"errors":["bad request"]}`))
	defer ts.Close()
	d := setupDD(t, ts)
	ctx := d.ctx(context.Background())
	result, err := listUserNotificationRules(ctx, d, map[string]any{})
	require.NoError(t, err)
	assert.True(t, result.IsError)
}

func TestGetUserNotificationRule_MissingArgs(t *testing.T) {
	ts := httptest.NewServer(jsonHandler(400, `{"errors":["bad request"]}`))
	defer ts.Close()
	d := setupDD(t, ts)
	ctx := d.ctx(context.Background())
	result, err := getUserNotificationRule(ctx, d, map[string]any{})
	require.NoError(t, err)
	assert.True(t, result.IsError)
}

func TestCreateUserNotificationRule_InvalidJSON(t *testing.T) {
	d := &dd{}
	result, err := createUserNotificationRule(context.Background(), d, map[string]any{
		"user_id": "u1", "body_json": "{bad",
	})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "invalid body_json")
}

func TestUpdateUserNotificationRule_InvalidJSON(t *testing.T) {
	d := &dd{}
	result, err := updateUserNotificationRule(context.Background(), d, map[string]any{
		"user_id": "u1", "rule_id": "r1", "body_json": "oops",
	})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "invalid body_json")
}

func TestDeleteUserNotificationRule_MissingArgs(t *testing.T) {
	ts := httptest.NewServer(jsonHandler(400, `{"errors":["bad request"]}`))
	defer ts.Close()
	d := setupDD(t, ts)
	ctx := d.ctx(context.Background())
	result, err := deleteUserNotificationRule(ctx, d, map[string]any{})
	require.NoError(t, err)
	assert.True(t, result.IsError)
}

// ── Schedule CRUD arg validation ─────────────────────────────────────

func TestCreateOnCallSchedule_InvalidJSON(t *testing.T) {
	d := &dd{}
	result, err := createOnCallSchedule(context.Background(), d, map[string]any{"body_json": "{"})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "invalid body_json")
}

func TestUpdateOnCallSchedule_InvalidJSON(t *testing.T) {
	d := &dd{}
	result, err := updateOnCallSchedule(context.Background(), d, map[string]any{
		"schedule_id": "s1", "body_json": "[",
	})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "invalid body_json")
}

func TestCreateOnCallEscalationPolicy_InvalidJSON(t *testing.T) {
	d := &dd{}
	result, err := createOnCallEscalationPolicy(context.Background(), d, map[string]any{"body_json": "x"})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "invalid body_json")
}

func TestUpdateOnCallEscalationPolicy_InvalidJSON(t *testing.T) {
	d := &dd{}
	result, err := updateOnCallEscalationPolicy(context.Background(), d, map[string]any{
		"policy_id": "p1", "body_json": "x",
	})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "invalid body_json")
}

func TestSetOnCallTeamRoutingRules_InvalidJSON(t *testing.T) {
	d := &dd{}
	result, err := setOnCallTeamRoutingRules(context.Background(), d, map[string]any{
		"team_id": "t1", "body_json": "x",
	})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "invalid body_json")
}

// ── Paging arg validation ────────────────────────────────────────────

func TestAcknowledgeOnCallPage_InvalidUUID(t *testing.T) {
	d := &dd{}
	result, err := acknowledgeOnCallPage(context.Background(), d, map[string]any{"page_id": "not-uuid"})
	require.NoError(t, err)
	assert.True(t, result.IsError)
}

func TestEscalateOnCallPage_InvalidUUID(t *testing.T) {
	d := &dd{}
	result, err := escalateOnCallPage(context.Background(), d, map[string]any{"page_id": "bad"})
	require.NoError(t, err)
	assert.True(t, result.IsError)
}

func TestResolveOnCallPage_InvalidUUID(t *testing.T) {
	d := &dd{}
	result, err := resolveOnCallPage(context.Background(), d, map[string]any{"page_id": "bad"})
	require.NoError(t, err)
	assert.True(t, result.IsError)
}

// ── Compact specs parity for new tools ───────────────────────────────

func TestOnCallCompactSpecs_NewToolsCovered(t *testing.T) {
	newReadTools := []string{
		"datadog_list_oncall_schedules",
		"datadog_list_oncall_escalation_policies",
		"datadog_list_oncall_pages",
		"datadog_get_oncall_page",
		"datadog_list_user_notification_channels",
		"datadog_get_user_notification_channel",
		"datadog_list_user_notification_rules",
		"datadog_get_user_notification_rule",
	}
	for _, name := range newReadTools {
		fields, ok := fieldCompactionSpecs[mcp.ToolName(name)]
		assert.True(t, ok, "missing compact spec for %s", name)
		assert.NotEmpty(t, fields, "empty compact spec for %s", name)
	}
}
