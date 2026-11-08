package web

import (
	"os"
	"path"
	"strings"
	"testing"

	mcp "github.com/daltoniam/switchboard"
	"github.com/daltoniam/switchboard/integrations/obsidian"
)

// Obsidian is a desktop app with no official container image, so this spec
// reaches a running Obsidian with the Local REST API plugin through stored
// sandbox credentials and runs read tools only:
//
//	OBSIDIAN_LIVE_BASE_URL  plugin URL, e.g. http://127.0.0.1:27123
//	OBSIDIAN_LIVE_API_KEY   plugin API key
//	OBSIDIAN_LIVE_NOTE      vault-relative path of an existing note
//	OBSIDIAN_LIVE_CA_CERT   optional PEM for the plugin's HTTPS certificate
func init() {
	sandboxOnly := "sandbox vaults are read-only for live runs; the request shape is covered by fixture tests in integrations/obsidian"
	registerLiveSpec(liveSpec{
		Integration: "obsidian",
		Tier:        liveSandbox,
		Adapter:     obsidian.New(),
		Provision:   liveObsidianProvision,
		Cases: []liveCase{
			{Tool: "obsidian_list_files", Args: map[string]any{"path": "${note_dir}"}, Expect: []string{"${note}", `"count"`}},
			{Tool: "obsidian_read_note", Args: map[string]any{"path": "${note}"}, Expect: []string{"${note}"}},
			{Tool: "obsidian_get_document_map", Args: map[string]any{"path": "${note}"}, Expect: []string{"${note}", "headings", `"version"`}},
			{Tool: "obsidian_search", Args: map[string]any{"query": "${note_title}", "limit": 5}, Expect: []string{`"total"`, `"returned"`}},
			{Tool: "obsidian_query", Args: map[string]any{"jsonlogic": `{"==":[{"var":"path"},"${note}"]}`}, Expect: []string{"${note}"}},
			{Tool: "obsidian_list_tags", Expect: []string{`"tags"`}},
			{Tool: "obsidian_list_commands", Args: map[string]any{"query": "editor"}, Expect: []string{"editor:"}},
		},
		Skips: map[mcp.ToolName]string{
			"obsidian_get_active_note": "depends on which note the Obsidian editor has open, which a read-only run cannot set",
			"obsidian_append_note":     sandboxOnly,
			"obsidian_patch_note":      sandboxOnly,
			"obsidian_write_note":      sandboxOnly,
			"obsidian_delete_note":     sandboxOnly,
			"obsidian_execute_command": sandboxOnly,
		},
	})
}

func liveObsidianProvision(t *testing.T) liveProvision {
	baseURL := os.Getenv("OBSIDIAN_LIVE_BASE_URL")
	apiKey := os.Getenv("OBSIDIAN_LIVE_API_KEY")
	note := strings.TrimLeft(os.Getenv("OBSIDIAN_LIVE_NOTE"), "/")
	if baseURL == "" || apiKey == "" || note == "" {
		t.Skip("set OBSIDIAN_LIVE_BASE_URL, OBSIDIAN_LIVE_API_KEY, and OBSIDIAN_LIVE_NOTE to reach a running Obsidian; it has no container image")
	}
	creds := map[string]string{"base_url": baseURL, "api_key": apiKey}
	if ca := os.Getenv("OBSIDIAN_LIVE_CA_CERT"); ca != "" {
		creds["ca_cert"] = ca
	}
	dir := path.Dir(note)
	if dir == "." {
		dir = ""
	}
	return liveProvision{
		Credentials: creds,
		Values: map[string]string{
			"note":       note,
			"note_dir":   dir,
			"note_title": strings.TrimSuffix(path.Base(note), path.Ext(note)),
		},
	}
}
