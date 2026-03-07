package homeassistant

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
	assert.Equal(t, "homeassistant", i.Name())
}

func TestConfigure_Success(t *testing.T) {
	i := New()
	err := i.Configure(mcp.Credentials{"token": "test-token", "base_url": "http://localhost:8123"})
	assert.NoError(t, err)
}

func TestConfigure_MissingToken(t *testing.T) {
	i := New()
	err := i.Configure(mcp.Credentials{"base_url": "http://localhost:8123"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "token is required")
}

func TestConfigure_MissingBaseURL(t *testing.T) {
	i := New()
	err := i.Configure(mcp.Credentials{"token": "test-token"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "base_url is required")
}

func TestConfigure_TrimsTrailingSlash(t *testing.T) {
	ha := &homeassistant{client: &http.Client{}}
	err := ha.Configure(mcp.Credentials{
		"token":    "test",
		"base_url": "http://localhost:8123/",
	})
	assert.NoError(t, err)
	assert.Equal(t, "http://localhost:8123", ha.baseURL)
}

func TestPlainTextKeys(t *testing.T) {
	ha := &homeassistant{}
	keys := ha.PlainTextKeys()
	assert.Contains(t, keys, "base_url")
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

func TestTools_AllHavePrefix(t *testing.T) {
	i := New()
	for _, tool := range i.Tools() {
		assert.Contains(t, tool.Name, "homeassistant_", "tool %s missing homeassistant_ prefix", tool.Name)
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
	ha := &homeassistant{token: "test", baseURL: "http://localhost", client: &http.Client{}}
	result, err := ha.Execute(context.Background(), "homeassistant_nonexistent", nil)
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

func TestFieldCompactionSpecs_NoOrphanSpecs(t *testing.T) {
	for name := range fieldCompactionSpecs {
		_, ok := dispatch[name]
		assert.True(t, ok, "field compaction spec %s has no dispatch handler", name)
	}
}

// --- HTTP helper tests ---

func TestDoRequest_Success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"message":"API running."}`))
	}))
	defer ts.Close()

	ha := &homeassistant{token: "test-token", client: ts.Client(), baseURL: ts.URL}
	data, err := ha.get(context.Background(), "/api/")
	require.NoError(t, err)
	assert.Contains(t, string(data), "API running")
}

func TestDoRequest_APIError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(401)
		_, _ = w.Write([]byte(`{"message":"unauthorized"}`))
	}))
	defer ts.Close()

	ha := &homeassistant{token: "bad-token", client: ts.Client(), baseURL: ts.URL}
	_, err := ha.get(context.Background(), "/api/")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "homeassistant API error (401)")
}

func TestDoRequest_204NoContent(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(204)
	}))
	defer ts.Close()

	ha := &homeassistant{token: "token", client: ts.Client(), baseURL: ts.URL}
	data, err := ha.doRequest(context.Background(), "DELETE", "/test", nil)
	require.NoError(t, err)
	assert.Contains(t, string(data), "success")
}

func TestDoRequestRaw_Success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("plain text response"))
	}))
	defer ts.Close()

	ha := &homeassistant{token: "token", client: ts.Client(), baseURL: ts.URL}
	data, err := ha.doRequestRaw(context.Background(), "GET", "/api/error_log", nil)
	require.NoError(t, err)
	assert.Equal(t, "plain text response", string(data))
}

func TestPost(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		var body map[string]string
		_ = json.NewDecoder(r.Body).Decode(&body)
		assert.Equal(t, "val", body["key"])
		_, _ = w.Write([]byte(`{"created":true}`))
	}))
	defer ts.Close()

	ha := &homeassistant{token: "token", client: ts.Client(), baseURL: ts.URL}
	data, err := ha.post(context.Background(), "/test", map[string]string{"key": "val"})
	require.NoError(t, err)
	assert.Contains(t, string(data), "created")
}

// --- Result helper tests ---

func TestRawResult(t *testing.T) {
	data := json.RawMessage(`{"key":"value"}`)
	result, err := rawResult(data)
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Equal(t, `{"key":"value"}`, result.Data)
}

func TestErrResult(t *testing.T) {
	result, err := errResult(fmt.Errorf("test error"))
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Equal(t, "test error", result.Data)
}

// --- Argument helper tests ---

func TestArgStr(t *testing.T) {
	assert.Equal(t, "val", argStr(map[string]any{"k": "val"}, "k"))
	assert.Empty(t, argStr(map[string]any{}, "k"))
}

func TestArgInt(t *testing.T) {
	assert.Equal(t, 42, argInt(map[string]any{"n": float64(42)}, "n"))
	assert.Equal(t, 42, argInt(map[string]any{"n": 42}, "n"))
	assert.Equal(t, 42, argInt(map[string]any{"n": "42"}, "n"))
	assert.Equal(t, 0, argInt(map[string]any{}, "n"))
}

func TestArgBool(t *testing.T) {
	assert.True(t, argBool(map[string]any{"b": true}, "b"))
	assert.False(t, argBool(map[string]any{"b": false}, "b"))
	assert.True(t, argBool(map[string]any{"b": "true"}, "b"))
	assert.False(t, argBool(map[string]any{}, "b"))
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

// --- Handler integration tests ---

func TestListStates(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Equal(t, "/api/states", r.URL.Path)
		_, _ = w.Write([]byte(`[{"entity_id":"light.living_room","state":"on"}]`))
	}))
	defer ts.Close()

	ha := &homeassistant{token: "token", client: ts.Client(), baseURL: ts.URL}
	result, err := ha.Execute(context.Background(), "homeassistant_list_states", map[string]any{})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "light.living_room")
}

func TestGetState(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Equal(t, "/api/states/sensor.temperature", r.URL.Path)
		_, _ = w.Write([]byte(`{"entity_id":"sensor.temperature","state":"22.5","attributes":{"unit_of_measurement":"°C"}}`))
	}))
	defer ts.Close()

	ha := &homeassistant{token: "token", client: ts.Client(), baseURL: ts.URL}
	result, err := ha.Execute(context.Background(), "homeassistant_get_state", map[string]any{
		"entity_id": "sensor.temperature",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "22.5")
}

func TestSetState(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Contains(t, r.URL.Path, "/api/states/sensor.test")
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		assert.Equal(t, "25", body["state"])
		_, _ = w.Write([]byte(`{"entity_id":"sensor.test","state":"25"}`))
	}))
	defer ts.Close()

	ha := &homeassistant{token: "token", client: ts.Client(), baseURL: ts.URL}
	result, err := ha.Execute(context.Background(), "homeassistant_set_state", map[string]any{
		"entity_id": "sensor.test",
		"state":     "25",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "sensor.test")
}

func TestSetState_InvalidAttributes(t *testing.T) {
	ha := &homeassistant{token: "token", baseURL: "http://localhost", client: &http.Client{}}
	result, err := ha.Execute(context.Background(), "homeassistant_set_state", map[string]any{
		"entity_id":  "sensor.test",
		"state":      "25",
		"attributes": "{bad json}",
	})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "invalid JSON for attributes")
}

func TestCallService(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/api/services/light/turn_on", r.URL.Path)
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		assert.Equal(t, "light.living_room", body["entity_id"])
		_, _ = w.Write([]byte(`[{"entity_id":"light.living_room","state":"on"}]`))
	}))
	defer ts.Close()

	ha := &homeassistant{token: "token", client: ts.Client(), baseURL: ts.URL}
	result, err := ha.Execute(context.Background(), "homeassistant_call_service", map[string]any{
		"domain":       "light",
		"service":      "turn_on",
		"service_data": `{"entity_id":"light.living_room"}`,
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "light.living_room")
}

func TestCallService_InvalidServiceData(t *testing.T) {
	ha := &homeassistant{token: "token", baseURL: "http://localhost", client: &http.Client{}}
	result, err := ha.Execute(context.Background(), "homeassistant_call_service", map[string]any{
		"domain":       "light",
		"service":      "turn_on",
		"service_data": "{bad}",
	})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "invalid JSON for service_data")
}

func TestCallService_WithReturnResponse(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Contains(t, r.URL.RawQuery, "return_response")
		_, _ = w.Write([]byte(`{"changed_states":[],"service_response":{}}`))
	}))
	defer ts.Close()

	ha := &homeassistant{token: "token", client: ts.Client(), baseURL: ts.URL}
	result, err := ha.Execute(context.Background(), "homeassistant_call_service", map[string]any{
		"domain":          "light",
		"service":         "turn_on",
		"return_response": "true",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
}

func TestListEvents(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Equal(t, "/api/events", r.URL.Path)
		_, _ = w.Write([]byte(`[{"event":"state_changed","listener_count":5}]`))
	}))
	defer ts.Close()

	ha := &homeassistant{token: "token", client: ts.Client(), baseURL: ts.URL}
	result, err := ha.Execute(context.Background(), "homeassistant_list_events", map[string]any{})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "state_changed")
}

func TestFireEvent(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/api/events/my_custom_event", r.URL.Path)
		_, _ = w.Write([]byte(`{"message":"Event my_custom_event fired."}`))
	}))
	defer ts.Close()

	ha := &homeassistant{token: "token", client: ts.Client(), baseURL: ts.URL}
	result, err := ha.Execute(context.Background(), "homeassistant_fire_event", map[string]any{
		"event_type": "my_custom_event",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "my_custom_event")
}

func TestFireEvent_InvalidEventData(t *testing.T) {
	ha := &homeassistant{token: "token", baseURL: "http://localhost", client: &http.Client{}}
	result, err := ha.Execute(context.Background(), "homeassistant_fire_event", map[string]any{
		"event_type": "test",
		"event_data": "{bad}",
	})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "invalid JSON for event_data")
}

func TestGetHistory(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Contains(t, r.URL.Path, "/api/history/period")
		assert.Contains(t, r.URL.RawQuery, "filter_entity_id=sensor.temp")
		_, _ = w.Write([]byte(`[[{"entity_id":"sensor.temp","state":"20","last_changed":"2024-01-01T00:00:00Z"}]]`))
	}))
	defer ts.Close()

	ha := &homeassistant{token: "token", client: ts.Client(), baseURL: ts.URL}
	result, err := ha.Execute(context.Background(), "homeassistant_get_history", map[string]any{
		"entity_id": "sensor.temp",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "sensor.temp")
}

func TestGetHistory_WithStartTime(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Contains(t, r.URL.Path, "/api/history/period/2024-01-01T00:00:00Z")
		_, _ = w.Write([]byte(`[[]]`))
	}))
	defer ts.Close()

	ha := &homeassistant{token: "token", client: ts.Client(), baseURL: ts.URL}
	result, err := ha.Execute(context.Background(), "homeassistant_get_history", map[string]any{
		"entity_id":  "sensor.temp",
		"start_time": "2024-01-01T00:00:00Z",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
}

func TestGetLogbook(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Contains(t, r.URL.Path, "/api/logbook")
		_, _ = w.Write([]byte(`[{"entity_id":"light.living_room","name":"Living Room","message":"turned on"}]`))
	}))
	defer ts.Close()

	ha := &homeassistant{token: "token", client: ts.Client(), baseURL: ts.URL}
	result, err := ha.Execute(context.Background(), "homeassistant_get_logbook", map[string]any{})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "Living Room")
}

func TestGetConfig(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Equal(t, "/api/config", r.URL.Path)
		_, _ = w.Write([]byte(`{"location_name":"Home","version":"2024.1.0"}`))
	}))
	defer ts.Close()

	ha := &homeassistant{token: "token", client: ts.Client(), baseURL: ts.URL}
	result, err := ha.Execute(context.Background(), "homeassistant_get_config", map[string]any{})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "2024.1.0")
}

func TestCheckConfig(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/api/config/core/check_config", r.URL.Path)
		_, _ = w.Write([]byte(`{"result":"valid","errors":null}`))
	}))
	defer ts.Close()

	ha := &homeassistant{token: "token", client: ts.Client(), baseURL: ts.URL}
	result, err := ha.Execute(context.Background(), "homeassistant_check_config", map[string]any{})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "valid")
}

func TestRenderTemplate(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/api/template", r.URL.Path)
		var body map[string]string
		_ = json.NewDecoder(r.Body).Decode(&body)
		assert.NotEmpty(t, body["template"])
		_, _ = w.Write([]byte("Paulus is at work!"))
	}))
	defer ts.Close()

	ha := &homeassistant{token: "token", client: ts.Client(), baseURL: ts.URL}
	result, err := ha.Execute(context.Background(), "homeassistant_render_template", map[string]any{
		"template": `Paulus is at {{ states("device_tracker.paulus") }}!`,
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Equal(t, "Paulus is at work!", result.Data)
}

func TestGetErrorLog(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Equal(t, "/api/error_log", r.URL.Path)
		_, _ = w.Write([]byte("15-12-20 11:02:50 homeassistant.components.recorder: Found unfinished sessions"))
	}))
	defer ts.Close()

	ha := &homeassistant{token: "token", client: ts.Client(), baseURL: ts.URL}
	result, err := ha.Execute(context.Background(), "homeassistant_get_error_log", map[string]any{})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "unfinished sessions")
}

func TestListCalendars(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Equal(t, "/api/calendars", r.URL.Path)
		_, _ = w.Write([]byte(`[{"entity_id":"calendar.personal","name":"Personal"}]`))
	}))
	defer ts.Close()

	ha := &homeassistant{token: "token", client: ts.Client(), baseURL: ts.URL}
	result, err := ha.Execute(context.Background(), "homeassistant_list_calendars", map[string]any{})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "calendar.personal")
}

func TestGetCalendarEvents(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Contains(t, r.URL.Path, "/api/calendars/calendar.personal")
		assert.Contains(t, r.URL.RawQuery, "start=")
		assert.Contains(t, r.URL.RawQuery, "end=")
		_, _ = w.Write([]byte(`[{"summary":"Meeting","start":"2024-01-01T09:00:00Z"}]`))
	}))
	defer ts.Close()

	ha := &homeassistant{token: "token", client: ts.Client(), baseURL: ts.URL}
	result, err := ha.Execute(context.Background(), "homeassistant_get_calendar_events", map[string]any{
		"entity_id": "calendar.personal",
		"start":     "2024-01-01T00:00:00Z",
		"end":       "2024-01-02T00:00:00Z",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "Meeting")
}

func TestHandleIntent(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/api/intent/handle", r.URL.Path)
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		assert.Equal(t, "HassTurnOn", body["name"])
		_, _ = w.Write([]byte(`{"speech":{"plain":{"speech":"Turned on the lights"}}}`))
	}))
	defer ts.Close()

	ha := &homeassistant{token: "token", client: ts.Client(), baseURL: ts.URL}
	result, err := ha.Execute(context.Background(), "homeassistant_handle_intent", map[string]any{
		"name": "HassTurnOn",
		"data": `{"name":{"value":"lights"}}`,
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "Turned on the lights")
}

func TestHandleIntent_InvalidData(t *testing.T) {
	ha := &homeassistant{token: "token", baseURL: "http://localhost", client: &http.Client{}}
	result, err := ha.Execute(context.Background(), "homeassistant_handle_intent", map[string]any{
		"name": "HassTurnOn",
		"data": "{bad}",
	})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "invalid JSON for data")
}

func TestDeleteState(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "DELETE", r.Method)
		assert.Equal(t, "/api/states/sensor.test", r.URL.Path)
		w.WriteHeader(204)
	}))
	defer ts.Close()

	ha := &homeassistant{token: "token", client: ts.Client(), baseURL: ts.URL}
	result, err := ha.Execute(context.Background(), "homeassistant_delete_state", map[string]any{
		"entity_id": "sensor.test",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
}

func TestHealthy_NotConfigured(t *testing.T) {
	ha := &homeassistant{client: &http.Client{}}
	assert.False(t, ha.Healthy(context.Background()))
}

func TestHealthy_Success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"message":"API running."}`))
	}))
	defer ts.Close()

	ha := &homeassistant{token: "token", client: ts.Client(), baseURL: ts.URL}
	assert.True(t, ha.Healthy(context.Background()))
}

func TestCompactSpec(t *testing.T) {
	ha := &homeassistant{}
	fields, ok := ha.CompactSpec("homeassistant_list_states")
	assert.True(t, ok)
	assert.NotEmpty(t, fields)

	_, ok = ha.CompactSpec("homeassistant_nonexistent")
	assert.False(t, ok)
}

func TestListServices(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Equal(t, "/api/services", r.URL.Path)
		_, _ = w.Write([]byte(`[{"domain":"light","services":["turn_on","turn_off"]}]`))
	}))
	defer ts.Close()

	ha := &homeassistant{token: "token", client: ts.Client(), baseURL: ts.URL}
	result, err := ha.Execute(context.Background(), "homeassistant_list_services", map[string]any{})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "light")
}
