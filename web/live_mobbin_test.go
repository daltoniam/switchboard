package web

import (
	"testing"

	mcp "github.com/daltoniam/switchboard"
	"github.com/daltoniam/switchboard/integrations/mobbin"
)

// Mobbin has no container image and signs in only through browser OAuth,
// while the live harness enters credentials through a setup form. Its read
// cases document the expected live shapes; run them with the isolated smoke
// test in the add-integration skill after signing in to a dedicated account.
func init() {
	task := map[string]any{"task_intent": "Verify the Switchboard Mobbin integration.", "output_destination": "chat"}
	with := func(args map[string]any) map[string]any {
		for key, value := range task {
			args[key] = value
		}
		return args
	}
	registerLiveSpec(liveSpec{
		Integration: "mobbin",
		Tier:        liveSandbox,
		Adapter:     mobbin.New(),
		Provision: func(t *testing.T) liveProvision {
			t.Skip("Mobbin signs in only through browser OAuth, which the live harness cannot complete; use the isolated smoke test")
			return liveProvision{}
		},
		Cases: []liveCase{
			{Tool: "mobbin_search_screens", Args: with(map[string]any{"query": "meditation app paywall", "platform": "ios", "mode": "standard", "limit": 3}), Expect: []string{"screens", "mobbin_url", "image_url"}},
			{Tool: "mobbin_search_flows", Args: with(map[string]any{"query": "fitness app onboarding", "platform": "ios", "limit": 1}), Expect: []string{"flows", "screen_count"}},
			{Tool: "mobbin_search_sections", Args: with(map[string]any{"query": "pricing section", "limit": 2}), Expect: []string{"sections", "site_name"}},
			{Tool: "mobbin_list_collections", Args: with(map[string]any{}), Expect: []string{"all-saved", "counts"}},
			{Tool: "mobbin_get_collection_contents", Args: with(map[string]any{"id": "all-saved", "content_type": "screens", "platform": "ios", "limit": 2}), Expect: []string{"all-saved", "has_next_page"}},
		},
		Skips: map[mcp.ToolName]string{
			"mobbin_create_collection": "creates a collection on the shared sandbox account",
			"mobbin_save_content":      "saves items to the shared sandbox account",
			"mobbin_add_comment":       "posts a comment the account's team can see",
		},
	})
}
