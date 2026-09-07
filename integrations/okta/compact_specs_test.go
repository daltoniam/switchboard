package okta

import (
	"testing"

	mcp "github.com/daltoniam/switchboard"
	"github.com/daltoniam/switchboard/compact"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestFieldCompactionSpecs_AllParse(t *testing.T) {
	require.NotEmpty(t, fieldCompactionSpecs, "fieldCompactionSpecs should not be empty")
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

func TestFieldCompactionSpec_ReturnsFieldsForListTool(t *testing.T) {
	o := &okta{}
	fields, ok := o.CompactSpec("okta_list_users")
	require.True(t, ok, "okta_list_users should have field compaction spec")
	assert.NotEmpty(t, fields)
}

func TestFieldCompactionSpec_ReturnsFalseForUnknownTool(t *testing.T) {
	o := &okta{}
	_, ok := o.CompactSpec("okta_nonexistent")
	assert.False(t, ok, "unknown tools should return false")
}

func TestFieldCompactionSpec_NoSpecOnMutation(t *testing.T) {
	o := &okta{}
	_, ok := o.CompactSpec("okta_create_user")
	assert.False(t, ok, "mutation tools should not have compaction specs")
}

func TestFieldCompactionSpecs_ShapeParity(t *testing.T) {
	handlerOutputs := map[string]string{
		"okta_list_users":         `{"items":[{"id":"00u1","status":"ACTIVE","created":"2024-01-01T00:00:00.000Z","activated":"2024-01-01T00:00:00.000Z","lastLogin":"2024-01-02T00:00:00.000Z","lastUpdated":"2024-01-02T00:00:00.000Z","profile":{"login":"jane@acme.com","email":"jane@acme.com","firstName":"Jane","lastName":"Doe","displayName":"Jane Doe","department":"Eng","title":"Engineer"}}],"next_after":"00unext"}`,
		"okta_get_user":           `{"id":"00u1","status":"ACTIVE","created":"2024-01-01T00:00:00.000Z","activated":"2024-01-01T00:00:00.000Z","statusChanged":"2024-01-01T00:00:00.000Z","lastLogin":"2024-01-02T00:00:00.000Z","lastUpdated":"2024-01-02T00:00:00.000Z","profile":{"login":"jane@acme.com","email":"jane@acme.com","firstName":"Jane","lastName":"Doe","displayName":"Jane Doe","secondEmail":"j@ex.com","mobilePhone":"+15551212","department":"Eng","title":"Engineer"},"credentials":{"provider":{"type":"OKTA","name":"OKTA"}}}`,
		"okta_list_user_groups":   `{"items":[{"id":"00g1","type":"OKTA_GROUP","profile":{"name":"Engineering","description":"eng"}}],"next_after":"n1"}`,
		"okta_list_user_factors":  `{"items":[{"id":"f1","factorType":"token:software:totp","provider":"OKTA","status":"ACTIVE","created":"2024-01-01T00:00:00.000Z","lastUpdated":"2024-01-01T00:00:00.000Z","profile":{"phoneNumber":"+1555","credentialId":"jane@acme.com"}}]}`,
		"okta_list_groups":        `{"items":[{"id":"00g1","type":"OKTA_GROUP","created":"2024-01-01T00:00:00.000Z","lastUpdated":"2024-01-01T00:00:00.000Z","lastMembershipUpdated":"2024-01-02T00:00:00.000Z","profile":{"name":"Engineering","description":"eng"}}],"next_after":"n1"}`,
		"okta_get_group":          `{"id":"00g1","type":"OKTA_GROUP","created":"2024-01-01T00:00:00.000Z","lastUpdated":"2024-01-01T00:00:00.000Z","lastMembershipUpdated":"2024-01-02T00:00:00.000Z","profile":{"name":"Engineering","description":"eng"}}`,
		"okta_list_group_members": `{"items":[{"id":"00u1","status":"ACTIVE","profile":{"login":"jane@acme.com","email":"jane@acme.com","firstName":"Jane","lastName":"Doe"}}],"next_after":"n1"}`,
		"okta_list_apps":          `{"items":[{"id":"0oa1","name":"oidc_client","label":"SSO App","status":"ACTIVE","signOnMode":"OPENID_CONNECT","created":"2024-01-01T00:00:00.000Z","lastUpdated":"2024-01-01T00:00:00.000Z"}],"next_after":"n1"}`,
		"okta_get_app":            `{"id":"0oa1","name":"oidc_client","label":"SSO App","status":"ACTIVE","signOnMode":"OPENID_CONNECT","created":"2024-01-01T00:00:00.000Z","lastUpdated":"2024-01-01T00:00:00.000Z","features":["PUSH"],"visibility":{"hide":{"iOS":false,"web":false}}}`,
		"okta_list_app_users":     `{"items":[{"id":"00u1","status":"ACTIVE","created":"2024-01-01T00:00:00.000Z","lastUpdated":"2024-01-01T00:00:00.000Z","scope":"USER","credentials":{"userName":"jane@acme.com"}}],"next_after":"n1"}`,
		"okta_list_app_groups":    `{"items":[{"id":"00g1","priority":0,"lastUpdated":"2024-01-01T00:00:00.000Z"}],"next_after":"n1"}`,
		"okta_list_policies":      `{"items":[{"id":"00p1","name":"Default","type":"OKTA_SIGN_ON","status":"ACTIVE","created":"2024-01-01T00:00:00.000Z","lastUpdated":"2024-01-01T00:00:00.000Z","description":"sign on","priority":1,"system":true}]}`,
		"okta_get_policy":         `{"id":"00p1","name":"Default","type":"OKTA_SIGN_ON","status":"ACTIVE","created":"2024-01-01T00:00:00.000Z","lastUpdated":"2024-01-01T00:00:00.000Z","description":"sign on","priority":1,"system":true}`,
		"okta_list_policy_rules":  `{"items":[{"id":"00r1","name":"Catch-all","type":"SIGN_ON","status":"ACTIVE","created":"2024-01-01T00:00:00.000Z","lastUpdated":"2024-01-01T00:00:00.000Z","priority":1,"system":true}]}`,
		"okta_list_logs":          `{"items":[{"uuid":"e1","published":"2024-01-01T00:00:00.000Z","eventType":"user.session.start","displayMessage":"User login","severity":"INFO","outcome":{"result":"SUCCESS","reason":""},"actor":{"id":"00u1","type":"User","alternateId":"jane@acme.com","displayName":"Jane Doe"},"client":{"ipAddress":"1.2.3.4"},"target":[{"id":"00u1","type":"User"}]}],"next_after":"n1"}`,
		"okta_get_org":            `{"id":"org1","subdomain":"acme","companyName":"Acme","status":"ACTIVE","created":"2020-01-01T00:00:00.000Z","website":"https://acme.com","endUserSupportHelpURL":"https://help.acme.com"}`,
	}

	for toolName, payload := range handlerOutputs {
		t.Run(toolName, func(t *testing.T) {
			fields, ok := fieldCompactionSpecs[mcp.ToolName(toolName)]
			require.True(t, ok, "missing compaction spec for %s", toolName)
			compacted, err := mcp.CompactJSON([]byte(payload), fields)
			require.NoError(t, err)
			assert.NotEqual(t, "{}", string(compacted), "compaction returned empty object for %s: %s", toolName, compacted)
			assert.NotEqual(t, "[]", string(compacted), "compaction returned empty array for %s", toolName)
			assert.NotEqual(t, "[{}]", string(compacted), "compaction returned array of empty objects for %s: %s", toolName, compacted)
		})
	}
}
