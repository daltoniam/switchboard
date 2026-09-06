package pages

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/daltoniam/switchboard/web/templates/layouts"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func renderIntegrationDetail(t *testing.T, data IntegrationDetailData) string {
	t.Helper()
	var buf bytes.Buffer
	page := layouts.PageData{Title: data.Name, CurrentPath: "/integrations"}
	require.NoError(t, IntegrationDetail(page, data).Render(context.Background(), &buf))
	return buf.String()
}

func TestIntegrationDetail_RendersToolDescriptions(t *testing.T) {
	data := IntegrationDetailData{
		Name:    "github",
		Enabled: true,
		Healthy: true,
		Tools: []ToolInfo{
			{Name: "github_search_repos", Description: "Search across GitHub repositories."},
			{Name: "github_create_issue", Description: ""},
		},
	}

	out := renderIntegrationDetail(t, data)

	assert.Contains(t, out, "Available Tools (2)")
	assert.Contains(t, out, "github_search_repos")
	assert.Contains(t, out, "Search across GitHub repositories.")
	assert.Contains(t, out, "github_create_issue")
	assert.Contains(t, out, "No description available")
	assert.True(t, strings.Contains(out, "tool-row"), "expected tool-row layout class")
}

func TestIntegrationDetail_OmitsToolsSectionWhenEmpty(t *testing.T) {
	data := IntegrationDetailData{Name: "github"}
	out := renderIntegrationDetail(t, data)
	assert.NotContains(t, out, "Available Tools")
}

func TestIntegrationDetail_RendersIdentityEditorWithoutSecrets(t *testing.T) {
	data := IntegrationDetailData{
		Name:               "slackmcp",
		SupportsIdentities: true,
		IdentityCredentialKeys: []string{
			"access_token",
		},
		IdentityMetadataKeys: []string{"label", "app_id"},
		Identities: []IdentityView{
			{
				ID:    "work",
				Label: "Work Slack",
				Credentials: []IdentityCredentialField{
					{Key: "access_token", Configured: true},
				},
				Metadata: []CredentialField{
					{Key: "label", Value: "Work Slack"},
					{Key: "app_id", Value: "A123"},
				},
			},
		},
	}

	out := renderIntegrationDetail(t, data)
	assert.Contains(t, out, "Named identities")
	assert.Contains(t, out, "Work Slack")
	assert.Contains(t, out, "work")
	assert.Contains(t, out, "Configured — leave blank to keep")
	assert.Contains(t, out, "identity_cred_access_token")
	assert.Contains(t, out, "identity_meta_label")
	assert.Contains(t, out, `id="identity-work-meta-label"`)
	assert.Contains(t, out, `id="identity-new-meta-label"`)
	assert.NotContains(t, out, `id="identity_meta_label"`)
	assert.Contains(t, out, "Add identity")
	assert.NotContains(t, out, "secret-token-value")
}

func TestIntegrationDetail_OmitsIdentityEditorWhenUnsupported(t *testing.T) {
	out := renderIntegrationDetail(t, IntegrationDetailData{Name: "github"})
	assert.NotContains(t, out, "Named identities")
	assert.NotContains(t, out, "Add identity")
}
