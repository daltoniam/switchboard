package web

import (
	"testing"

	mcp "github.com/daltoniam/switchboard"
	"github.com/daltoniam/switchboard/integrations/higgsfield"
)

// Higgsfield is a hosted SaaS with no container image, so the spec is a
// read-only sandbox run. Generation tools spend real credits and are never
// run live; mocked coverage lives in integrations/higgsfield.
func init() {
	registerLiveSpec(liveSpec{
		Integration: "higgsfield", Tier: liveSandbox, Adapter: higgsfield.New(), Provision: liveHiggsfieldProvision,
		Cases: []liveCase{
			{Tool: "higgsfield_balance", Expect: []string{"credits", "subscription_plan_type"}},
			{Tool: "higgsfield_transactions", Args: map[string]any{"size": 1}, Expect: []string{"items", "next_cursor"}},
			{Tool: "higgsfield_show_plans_and_credits", Expect: []string{"plans", "purchase_links"}},
			{Tool: "higgsfield_list_workspaces", Expect: []string{"workspaces", "is_selected"}},
			{Tool: "higgsfield_get_preferences", Expect: []string{"auto_create_project"}},
			{Tool: "higgsfield_models_explore", Args: map[string]any{"action": "list", "type": "image", "limit": 3}, Expect: []string{"items", "aspect_ratios"}},
			{Tool: "higgsfield_get_presets", Args: map[string]any{"limit": 3}, Expect: []string{"items", "categories"}},
			{Tool: "higgsfield_list_voices", Args: map[string]any{"size": 3}, Expect: []string{"voices"}},
			{Tool: "higgsfield_show_generations", Args: map[string]any{"size": 3}, Expect: []string{"items"}},
		},
		Skips: map[mcp.ToolName]string{
			"higgsfield_jobs_wait":           "needs job IDs from a paid generation",
			"higgsfield_job_display":         "needs a job ID that exists only after a paid generation",
			"higgsfield_list_projects":       "needs the account's workspace UUID, which differs per sandbox account",
			"higgsfield_get_project":         "needs a project UUID that differs per sandbox account",
			"higgsfield_list_project_assets": "needs a project UUID that differs per sandbox account",
			"higgsfield_create_project":      "write tool; sandbox specs run read tools only",
			"higgsfield_media_import_url":    "write tool; sandbox specs run read tools only",
			"higgsfield_media_upload":        "write tool; sandbox specs run read tools only",
			"higgsfield_media_confirm":       "write tool; sandbox specs run read tools only",
			"higgsfield_generate_image":      "spends credits",
			"higgsfield_generate_video":      "spends credits",
			"higgsfield_generate_audio":      "spends credits",
			"higgsfield_upscale_image":       "spends credits",
			"higgsfield_remove_background":   "spends credits",
		},
	})
}

// liveHiggsfieldProvision skips: Higgsfield authenticates only through hosted
// OAuth, and the live harness configures integrations by filling credential
// fields, which an OAuth setup page does not render.
func liveHiggsfieldProvision(t *testing.T) liveProvision {
	t.Skip("higgsfield needs an interactive OAuth sign-in; the live harness only fills credential forms")
	return liveProvision{}
}
