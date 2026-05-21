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
