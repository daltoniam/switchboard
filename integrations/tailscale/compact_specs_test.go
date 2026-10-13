package tailscale

import (
	"testing"

	mcp "github.com/daltoniam/switchboard"
	"github.com/daltoniam/switchboard/compact"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

var compactedReadTools = []string{
	"tailscale_list_devices",
	"tailscale_get_device",
	"tailscale_list_keys",
	"tailscale_get_key",
	"tailscale_list_users",
	"tailscale_get_user",
	"tailscale_list_user_invites",
	"tailscale_list_webhooks",
}

var mutationTools = []string{
	"tailscale_authorize_device",
	"tailscale_delete_device",
	"tailscale_set_device_name",
	"tailscale_set_device_tags",
	"tailscale_set_device_key_expiry",
	"tailscale_expire_device_key",
	"tailscale_set_device_ip",
	"tailscale_set_device_routes",
	"tailscale_set_device_posture_attribute",
	"tailscale_delete_device_posture_attribute",
	"tailscale_create_auth_key",
	"tailscale_delete_key",
	"tailscale_set_policy_file",
	"tailscale_set_dns_nameservers",
	"tailscale_set_dns_preferences",
	"tailscale_set_dns_search_paths",
	"tailscale_update_split_dns",
	"tailscale_update_user_role",
	"tailscale_approve_user",
	"tailscale_suspend_user",
	"tailscale_restore_user",
	"tailscale_delete_user",
	"tailscale_create_user_invite",
	"tailscale_delete_user_invite",
	"tailscale_update_tailnet_settings",
	"tailscale_create_webhook",
	"tailscale_delete_webhook",
	"tailscale_test_webhook",
}

func TestFieldCompactionSpecs_AllParse(t *testing.T) {
	require.NotEmpty(t, fieldCompactionSpecs)
}

func TestFieldCompactionSpecs_NoDuplicateTools(t *testing.T) {
	var sf compact.SpecFile
	require.NoError(t, yaml.Unmarshal(compactYAML, &sf))
	assert.Equal(t, len(sf.Tools), len(fieldCompactionSpecs))
}

func TestFieldCompactionSpecs_NoOrphanSpecs(t *testing.T) {
	for toolName := range fieldCompactionSpecs {
		_, ok := dispatch[toolName]
		assert.True(t, ok, "field compaction spec for %q has no dispatch handler", toolName)
	}
}

func TestFieldCompactionSpecs_ReadToolsCovered(t *testing.T) {
	for _, name := range compactedReadTools {
		_, ok := fieldCompactionSpecs[mcp.ToolName(name)]
		assert.True(t, ok, "read tool %s should have a compaction spec", name)
	}
}

func TestFieldCompactionSpecs_NoSpecOnMutationTools(t *testing.T) {
	for _, name := range mutationTools {
		_, ok := fieldCompactionSpecs[mcp.ToolName(name)]
		assert.False(t, ok, "mutation tool %s should return full confirmation responses", name)
	}
}

func TestCompactSpec_Accessors(t *testing.T) {
	ts := &tailscale{}
	fields, ok := ts.CompactSpec("tailscale_list_devices")
	require.True(t, ok)
	assert.NotEmpty(t, fields)
	_, ok = ts.CompactSpec("tailscale_nonexistent")
	assert.False(t, ok)
	_, ok = ts.MaxBytes("tailscale_nonexistent")
	assert.False(t, ok)
}

func TestFieldCompactionSpecs_ShapeParity(t *testing.T) {
	device := `{"addresses":["100.64.0.1","fd7a:115c:a1e0::1"],"id":"92960230385","nodeId":"n292kg92CNTRL","user":"ada@example.com","name":"web-1.tail1234.ts.net","hostname":"web-1","clientVersion":"1.80.0","updateAvailable":false,"os":"linux","created":"2025-01-01T00:00:00Z","connectedToControl":true,"lastSeen":"2026-10-13T00:00:00Z","keyExpiryDisabled":false,"expires":"2027-01-01T00:00:00Z","authorized":true,"isExternal":false,"machineKey":"mkey:SECRET","nodeKey":"nodekey:SECRET","blocksIncomingConnections":false,"enabledRoutes":["10.0.0.0/16"],"advertisedRoutes":["10.0.0.0/16"],"clientConnectivity":{"endpoints":["1.2.3.4:41641"],"latency":{"Dallas":{"latencyMs":12}}},"tags":["tag:web"],"tailnetLockKey":"tlpub:SECRET","isEphemeral":false}`
	cases := map[string]struct {
		payload string
		want    []string
		dropped string
	}{
		"tailscale_list_devices": {
			payload: `{"devices":[` + device + `]}`,
			want:    []string{`"nodeId":"n292kg92CNTRL"`, `"hostname":"web-1"`},
			dropped: "SECRET",
		},
		"tailscale_get_device": {
			payload: device,
			want:    []string{`"nodeId":"n292kg92CNTRL"`, `"hostname":"web-1"`},
			dropped: "SECRET",
		},
		"tailscale_list_keys": {
			payload: `{"keys":[{"id":"k1","keyType":"auth","description":"ci","created":"2025-01-01T00:00:00Z","expires":"2025-04-01T00:00:00Z","invalid":false,"capabilities":{"devices":{"create":{"reusable":true,"tags":["tag:ci"]}}},"userId":"u1","noise":"DROPME"}]}`,
			want:    []string{`"id":"k1"`, `"keyType":"auth"`},
			dropped: "DROPME",
		},
		"tailscale_get_key": {
			payload: `{"id":"k1","keyType":"auth","description":"ci","created":"2025-01-01T00:00:00Z","expires":"2025-04-01T00:00:00Z","capabilities":{"devices":{"create":{"reusable":true,"tags":["tag:ci"]}}},"userId":"u1","noise":"DROPME"}`,
			want:    []string{`"id":"k1"`, `"keyType":"auth"`},
			dropped: "DROPME",
		},
		"tailscale_list_users": {
			payload: `{"users":[{"id":"u1","displayName":"Ada","loginName":"ada@example.com","profilePicUrl":"https://x/DROPME.png","tailnetId":"t1","created":"2025-01-01T00:00:00Z","type":"member","role":"admin","status":"active","deviceCount":3,"lastSeen":"2026-10-13T00:00:00Z","currentlyConnected":true}]}`,
			want:    []string{`"id":"u1"`, `"loginName":"ada@example.com"`, `"status":"active"`},
			dropped: "DROPME",
		},
		"tailscale_get_user": {
			payload: `{"id":"u1","displayName":"Ada","loginName":"ada@example.com","profilePicUrl":"https://x/DROPME.png","role":"admin","status":"active","deviceCount":3}`,
			want:    []string{`"id":"u1"`, `"loginName":"ada@example.com"`},
			dropped: "DROPME",
		},
		"tailscale_list_user_invites": {
			payload: `{"invites":[{"id":"29214","role":"admin","tailnetId":12345,"inviterId":34567,"email":"ada@example.com","lastEmailSentAt":"2024-05-09T16:23:26Z","inviteUrl":"https://login.tailscale.com/uinv/code","noise":"DROPME"}]}`,
			want:    []string{`"id":"29214"`, `"email":"ada@example.com"`, "uinv/code"},
			dropped: "DROPME",
		},
		"tailscale_list_webhooks": {
			payload: `{"webhooks":[{"endpointId":"w1","endpointUrl":"https://hooks.example.com","providerType":"slack","creatorLoginName":"ada@example.com","created":"2025-01-01T00:00:00Z","lastModified":"2025-01-02T00:00:00Z","subscriptions":["nodeCreated"],"noise":"DROPME"}]}`,
			want:    []string{`"endpointId":"w1"`, "nodeCreated"},
			dropped: "DROPME",
		},
	}

	covered := make([]string, 0, len(cases))
	for name := range cases {
		covered = append(covered, name)
	}
	assert.ElementsMatch(t, compactedReadTools, covered, "every compacted read tool needs a shape parity fixture")

	for toolName, tc := range cases {
		t.Run(toolName, func(t *testing.T) {
			fields, ok := fieldCompactionSpecs[mcp.ToolName(toolName)]
			require.True(t, ok)
			compacted, err := mcp.CompactJSON([]byte(tc.payload), fields)
			require.NoError(t, err)
			out := string(compacted)
			for _, want := range tc.want {
				assert.Contains(t, out, want, "spec dropped a routing field")
			}
			assert.NotContains(t, out, tc.dropped, "spec did not compact")
			assert.Less(t, len(out), len(tc.payload))
		})
	}
}

func TestFieldCompactionSpecs_DevicesKeepRoutingFieldsDropKeys(t *testing.T) {
	payload := `{"devices":[{"id":"1","nodeId":"n1","hostname":"web-1","name":"web-1.ts.net","addresses":["100.64.0.1"],"tags":["tag:web"],"authorized":true,"connectedToControl":false,"lastSeen":"2026-01-01T00:00:00Z","enabledRoutes":["0.0.0.0/0"],"machineKey":"mkey:SECRET","nodeKey":"nodekey:SECRET","tailnetLockKey":"tlpub:SECRET","clientConnectivity":{"endpoints":["1.2.3.4:41641"]}}]}`
	compacted, err := mcp.CompactJSON([]byte(payload), fieldCompactionSpecs["tailscale_list_devices"])
	require.NoError(t, err)
	out := string(compacted)
	for _, want := range []string{`"nodeId":"n1"`, `"hostname":"web-1"`, "100.64.0.1", "tag:web", `"authorized":true`, `"connectedToControl":false`, "0.0.0.0/0"} {
		assert.Contains(t, out, want)
	}
	assert.NotContains(t, out, "SECRET")
	assert.NotContains(t, out, "1.2.3.4:41641")
}
