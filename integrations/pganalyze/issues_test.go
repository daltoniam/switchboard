package pganalyze

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestPganalyze(t *testing.T, handler http.HandlerFunc) *pganalyze {
	t.Helper()
	ts := httptest.NewServer(handler)
	t.Cleanup(ts.Close)
	return &pganalyze{
		apiKey:           "test-key",
		graphqlURL:       ts.URL,
		organizationSlug: "test-org",
		client:           ts.Client(),
	}
}

func gqlHandler(t *testing.T, response any) http.HandlerFunc {
	t.Helper()
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"data": response})
	}
}

func TestGetIssues_DefaultExcludesResolved(t *testing.T) {
	issues := []map[string]any{
		{"id": "1", "severity": "warning", "state": "triggered", "description": "active issue"},
		{"id": "2", "severity": "info", "state": "resolved", "description": "resolved issue"},
		{"id": "3", "severity": "critical", "state": "triggered", "description": "critical issue"},
	}
	p := newTestPganalyze(t, gqlHandler(t, map[string]any{"getIssues": issues}))

	result, err := getIssues(context.Background(), p, map[string]any{})
	require.NoError(t, err)
	require.False(t, result.IsError)

	var got []map[string]any
	require.NoError(t, json.Unmarshal([]byte(result.Data), &got))
	assert.Len(t, got, 2, "resolved issues should be excluded by default")
	assert.Equal(t, "1", got[0]["id"])
	assert.Equal(t, "3", got[1]["id"])
}

func TestGetIssues_IncludeResolvedReturnsAll(t *testing.T) {
	issues := []map[string]any{
		{"id": "1", "severity": "warning", "state": "triggered"},
		{"id": "2", "severity": "info", "state": "resolved"},
	}
	p := newTestPganalyze(t, gqlHandler(t, map[string]any{"getIssues": issues}))

	result, err := getIssues(context.Background(), p, map[string]any{"include_resolved": true})
	require.NoError(t, err)

	var got []map[string]any
	require.NoError(t, json.Unmarshal([]byte(result.Data), &got))
	assert.Len(t, got, 2, "include_resolved should return all issues")
}

func TestGetIssues_SeverityFilter(t *testing.T) {
	issues := []map[string]any{
		{"id": "1", "severity": "warning", "state": "triggered"},
		{"id": "2", "severity": "critical", "state": "triggered"},
		{"id": "3", "severity": "warning", "state": "triggered"},
	}
	p := newTestPganalyze(t, gqlHandler(t, map[string]any{"getIssues": issues}))

	result, err := getIssues(context.Background(), p, map[string]any{"severity": "critical"})
	require.NoError(t, err)

	var got []map[string]any
	require.NoError(t, json.Unmarshal([]byte(result.Data), &got))
	assert.Len(t, got, 1)
	assert.Equal(t, "2", got[0]["id"])
}

func TestGetIssues_SeverityFilterCaseInsensitive(t *testing.T) {
	issues := []map[string]any{
		{"id": "1", "severity": "Warning", "state": "triggered"},
	}
	p := newTestPganalyze(t, gqlHandler(t, map[string]any{"getIssues": issues}))

	result, err := getIssues(context.Background(), p, map[string]any{"severity": "warning"})
	require.NoError(t, err)

	var got []map[string]any
	require.NoError(t, json.Unmarshal([]byte(result.Data), &got))
	assert.Len(t, got, 1)
}

func TestGetIssues_SeverityPlusResolvedFilter(t *testing.T) {
	issues := []map[string]any{
		{"id": "1", "severity": "warning", "state": "triggered"},
		{"id": "2", "severity": "warning", "state": "resolved"},
		{"id": "3", "severity": "critical", "state": "triggered"},
	}
	p := newTestPganalyze(t, gqlHandler(t, map[string]any{"getIssues": issues}))

	result, err := getIssues(context.Background(), p, map[string]any{"severity": "warning"})
	require.NoError(t, err)

	var got []map[string]any
	require.NoError(t, json.Unmarshal([]byte(result.Data), &got))
	assert.Len(t, got, 1, "should exclude resolved even when filtering by severity")
	assert.Equal(t, "1", got[0]["id"])
}

func TestGetIssues_NoStateVariableSentToAPI(t *testing.T) {
	var capturedBody map[string]any
	handler := func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&capturedBody)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{"getIssues": []any{}},
		})
	}
	p := newTestPganalyze(t, handler)

	_, err := getIssues(context.Background(), p, map[string]any{})
	require.NoError(t, err)

	vars, _ := capturedBody["variables"].(map[string]any)
	assert.NotContains(t, vars, "state", "state should not be sent as a GraphQL variable")
}

func TestGetIssues_PassesOrgSlugToAPI(t *testing.T) {
	var capturedBody map[string]any
	handler := func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&capturedBody)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{"getIssues": []any{}},
		})
	}
	p := newTestPganalyze(t, handler)

	_, err := getIssues(context.Background(), p, map[string]any{})
	require.NoError(t, err)

	vars, _ := capturedBody["variables"].(map[string]any)
	assert.Equal(t, "test-org", vars["organizationSlug"])
}

func TestGetIssues_EmptyResponse(t *testing.T) {
	p := newTestPganalyze(t, gqlHandler(t, map[string]any{"getIssues": []any{}}))

	result, err := getIssues(context.Background(), p, map[string]any{})
	require.NoError(t, err)
	require.False(t, result.IsError)

	var got []map[string]any
	require.NoError(t, json.Unmarshal([]byte(result.Data), &got))
	assert.Empty(t, got)
}
