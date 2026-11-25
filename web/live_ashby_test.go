package web

import (
	"testing"

	mcp "github.com/daltoniam/switchboard"
	"github.com/daltoniam/switchboard/integrations/ashby"
)

// Ashby is hosted SaaS with no container image, and its MCP server accepts
// only per-user browser OAuth (no API key), which the live harness cannot
// complete because it configures integrations by filling credential fields.
// The read cases document what a dedicated sandbox account must show; run
// them with the isolated smoke test in the add-integration skill after
// signing in. Values name records seeded in that account.
func init() {
	const noSchema = "Ashby does not publish input schemas; argument names are only known after sign-in, so a case would guess them"
	registerLiveSpec(liveSpec{
		Integration: "ashby",
		Tier:        liveSandbox,
		Adapter:     ashby.New(),
		Provision: func(t *testing.T) liveProvision {
			t.Skip("ashby signs in only through per-user browser OAuth (MCP Server opt-in, Elevated Access user); no Ashby sandbox credentials are available and the live harness only fills credential forms")
			return liveProvision{}
		},
		Cases: []liveCase{
			{Tool: "ashby_get_pending_tasks", Args: map[string]any{}, Expect: []string{"${pending_task_candidate}"}},
			{Tool: "ashby_get_upcoming_interviews", Args: map[string]any{}, Expect: []string{"${interview_candidate}"}},
		},
		Skips: map[mcp.ToolName]string{
			"ashby_search_records_by_name":     noSchema,
			"ashby_filter_records":             noSchema,
			"ashby_describe_object_fields":     noSchema,
			"ashby_get_record_details":         "needs record IDs that differ per Ashby account",
			"ashby_get_candidate":              "needs a candidate ID that differs per Ashby account",
			"ashby_get_interview_details":      "needs an interview ID that differs per Ashby account",
			"ashby_get_interview_plan":         "needs a job or application ID that differs per Ashby account",
			"ashby_get_submitted_feedback":     "needs feedback the signed-in user submitted in the sandbox account",
			"ashby_get_job_pipeline":           noSchema,
			"ashby_search_product_docs":        noSchema,
			"ashby_create_candidate":           "write tool; sandbox specs run read tools only",
			"ashby_add_note_to_candidate":      "write tool; sandbox specs run read tools only",
			"ashby_change_application_stage":   "destructive tool (archives applications, cancels interviews); sandbox specs run read tools only",
			"ashby_consider_candidate_for_job": "write tool; sandbox specs run read tools only",
		},
	})
}
