package web

import (
	"os"
	"testing"

	mcp "github.com/daltoniam/switchboard"
	"github.com/daltoniam/switchboard/integrations/onepassword"
)

// 1Password is a hosted service with no container image, so this spec signs
// in to a real account through stored sandbox credentials and runs read tools
// only:
//
//	OP_LIVE_SERVICE_ACCOUNT_TOKEN  service account token (ops_...)
//	OP_LIVE_VAULT                  title of a vault the service account can read
//	OP_LIVE_ITEM                   title of a Login item in that vault, made of
//	                               letters, digits, spaces, _, . or - so the
//	                               case can build an op:// reference from names
//
// The harness checks every compaction field against real data, so the item
// must have a website URL, a tag, notes, a password, a field in a titled
// section and a one-time password field.
func init() {
	writes := "changes a real item in a shared vault; request shapes are covered by fixture tests in integrations/onepassword"
	registerLiveSpec(liveSpec{
		Integration: "onepassword",
		Tier:        liveSandbox,
		Adapter:     onepassword.New(),
		Provision:   liveOnePasswordProvision,
		Cases: []liveCase{
			{Tool: "onepassword_list_vaults", Expect: []string{"${vault}"}},
			{Tool: "onepassword_list_items", Args: map[string]any{"vault": "${vault}"}, Expect: []string{"${item}"}},
			{Tool: "onepassword_get_item", Args: map[string]any{"vault": "${vault}", "item": "${item}"}, Expect: []string{"${item}", "[REDACTED]", "op://"}},
			{Tool: "onepassword_get_item", Args: map[string]any{"vault": "${vault}", "item": "${item}", "reveal": true}, Expect: []string{"${item}", `"otp_code"`}},
			{Tool: "onepassword_resolve_secret", Args: map[string]any{"reference": "op://${vault}/${item}/password"}, Expect: []string{`"value"`}},
		},
		Skips: map[mcp.ToolName]string{
			"onepassword_create_item": writes,
			"onepassword_update_item": writes,
		},
	})
}

func liveOnePasswordProvision(t *testing.T) liveProvision {
	token := os.Getenv("OP_LIVE_SERVICE_ACCOUNT_TOKEN")
	vault := os.Getenv("OP_LIVE_VAULT")
	item := os.Getenv("OP_LIVE_ITEM")
	if token == "" || vault == "" || item == "" {
		t.Skip("set OP_LIVE_SERVICE_ACCOUNT_TOKEN, OP_LIVE_VAULT, and OP_LIVE_ITEM to reach a real 1Password account; it has no container image")
	}
	return liveProvision{
		Credentials: map[string]string{"service_account_token": token},
		Values:      map[string]string{"vault": vault, "item": item},
	}
}
