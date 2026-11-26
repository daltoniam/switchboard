package web

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"testing"
	"time"

	mcp "github.com/daltoniam/switchboard"
	"github.com/daltoniam/switchboard/integrations/ashby"
	"github.com/stretchr/testify/require"
)

// Ashby is hosted SaaS with no container image. The live spec drives API key
// mode, because the harness configures integrations by filling credential
// fields and Ashby's hosted MCP server accepts only per-user browser OAuth.
// Run it with ASHBY_LIVE_API_KEY set to a dedicated key that has only the
// read permissions the setup page documents. Provision reads (never writes)
// the account to learn record ids, so the cases assert real data.
func init() {
	adapter := ashby.New()
	if err := adapter.Configure(context.Background(), mcp.Credentials{"api_key": "catalog-only"}); err != nil {
		panic(err)
	}
	cases := []liveCase{
		{Tool: "ashby_get_api_key_info", Args: map[string]any{}, Expect: []string{"${api_key_created_at}"}},
		{Tool: "ashby_list_jobs", Args: map[string]any{}, Expect: []string{"${job_id}"}},
		{Tool: "ashby_search_jobs", Args: map[string]any{"title": "${job_title}"}, Expect: []string{"${job_id}"}},
		{Tool: "ashby_list_candidates", Args: map[string]any{}, Expect: []string{"${candidate_id}"}},
		{Tool: "ashby_search_candidates", Args: map[string]any{"name": "${candidate_name}"}, Expect: []string{"${candidate_id}"}},
		{Tool: "ashby_list_applications", Args: map[string]any{}, Expect: []string{"${application_id}"}},
		{Tool: "ashby_list_users", Args: map[string]any{}, Expect: []string{"${user_id}"}},
		{Tool: "ashby_list_departments", Args: map[string]any{}, Expect: []string{"${department_id}"}},
		{Tool: "ashby_list_locations", Args: map[string]any{}, Expect: []string{"${location_id}"}},
		{Tool: "ashby_list_sources", Args: map[string]any{}, Expect: []string{"${source_id}"}},
		{Tool: "ashby_list_archive_reasons", Args: map[string]any{}, Expect: []string{"${archive_reason_id}"}},
		{Tool: "ashby_list_interview_plans", Args: map[string]any{}, Expect: []string{"${interview_plan_id}"}},
		{Tool: "ashby_list_interview_stage_groups", Args: map[string]any{}, Expect: []string{"${stage_group_id}"}},
	}
	covered := map[mcp.ToolName]bool{}
	for _, c := range cases {
		covered[c.Tool] = true
	}
	const (
		writeReason  = "write tool; sandbox specs run read tools only"
		recordReason = "single-record spec keeps optional fields (dates, archive reasons, nested objects) that are empty on many real records, so the live compaction check would flag data rather than code; unit fixtures cover the shape and a list case covers the endpoint family"
		dataReason   = "depends on records (offers, openings, postings, projects, referrals, surveys, take-homes, sequences, schedules, feedback, emails) that a given Ashby account may not have, or on a module permission beyond the documented read-only set"
	)
	skips := map[mcp.ToolName]string{}
	for _, tool := range adapter.Tools() {
		switch {
		case covered[tool.Name]:
		case tool.Access != mcp.AccessRead:
			skips[tool.Name] = writeReason
		case len(tool.Required) > 0:
			skips[tool.Name] = recordReason
		default:
			skips[tool.Name] = dataReason
		}
	}
	registerLiveSpec(liveSpec{
		Integration: "ashby",
		Tier:        liveSandbox,
		Adapter:     adapter,
		Provision:   provisionAshbyLive,
		Cases:       cases,
		Skips:       skips,
	})
}

func provisionAshbyLive(t *testing.T) liveProvision {
	key := os.Getenv("ASHBY_LIVE_API_KEY")
	if key == "" {
		t.Skip("ASHBY_LIVE_API_KEY is not set: create an Ashby API key under Admin > API Keys with read on Candidates, Jobs, Interviews, Hiring Process, Organization, and API Keys, and export it to run the Ashby live read cases")
	}
	first := func(endpoint string, body map[string]any, fields ...string) map[string]string {
		results := ashbyLiveCall(t, key, endpoint, body)
		var records []map[string]any
		require.NoError(t, json.Unmarshal(results, &records), "%s results", endpoint)
		require.NotEmpty(t, records, "the Ashby account needs at least one record from %s for the live cases", endpoint)
		values := map[string]string{}
		for _, field := range fields {
			text, _ := records[0][field].(string)
			require.NotEmpty(t, text, "%s: first record has no %s", endpoint, field)
			values[field] = text
		}
		return values
	}
	page := map[string]any{"limit": 25}
	var info map[string]any
	require.NoError(t, json.Unmarshal(ashbyLiveCall(t, key, "apiKey.info", map[string]any{}), &info))
	createdAt, _ := info["createdAt"].(string)
	require.NotEmpty(t, createdAt, "apiKey.info returned no createdAt")
	job := first("job.list", page, "id", "title")
	candidate := first("candidate.list", page, "id", "name")
	return liveProvision{
		Credentials: map[string]string{"api_key": key},
		Values: map[string]string{
			"api_key_created_at": createdAt,
			"job_id":             job["id"],
			"job_title":          job["title"],
			"candidate_id":       candidate["id"],
			"candidate_name":     candidate["name"],
			"application_id":     first("application.list", page, "id")["id"],
			"user_id":            first("user.list", page, "id")["id"],
			"department_id":      first("department.list", page, "id")["id"],
			"location_id":        first("location.list", page, "id")["id"],
			"source_id":          first("source.list", map[string]any{}, "id")["id"],
			"archive_reason_id":  first("archiveReason.list", map[string]any{}, "id")["id"],
			"interview_plan_id":  first("interviewPlan.list", page, "id")["id"],
			"stage_group_id":     first("interviewStageGroup.list", map[string]any{}, "id")["id"],
		},
	}
}

// ashbyLiveCall reads one endpoint with the live key and returns its results.
// It never prints the response, which holds candidate data.
func ashbyLiveCall(t *testing.T, key, endpoint string, body map[string]any) json.RawMessage {
	t.Helper()
	payload, err := json.Marshal(body)
	require.NoError(t, err)
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.ashbyhq.com/"+endpoint, bytes.NewReader(payload))
	require.NoError(t, err)
	req.SetBasicAuth(key, "")
	req.Header.Set("Accept", "application/json; version=1")
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	data, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode, "%s returned HTTP %d; check the key and its read permissions", endpoint, resp.StatusCode)
	var envelope struct {
		Success   bool            `json:"success"`
		Results   json.RawMessage `json:"results"`
		ErrorInfo *struct {
			Code string `json:"code"`
		} `json:"errorInfo"`
	}
	require.NoError(t, json.Unmarshal(data, &envelope), "%s returned invalid JSON", endpoint)
	if !envelope.Success {
		code := ""
		if envelope.ErrorInfo != nil {
			code = envelope.ErrorInfo.Code
		}
		require.Fail(t, endpoint+" failed", "error code %q; check the key's read permissions", code)
	}
	return envelope.Results
}
