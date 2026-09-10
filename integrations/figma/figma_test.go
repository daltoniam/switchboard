package figma

import (
	"context"
	"testing"

	mcp "github.com/daltoniam/switchboard"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeRemote struct {
	configured mcp.Credentials
	executed   mcp.ToolName
	args       map[string]any
}

func (f *fakeRemote) Name() string { return "figma" }

func (f *fakeRemote) Configure(_ context.Context, creds mcp.Credentials) error {
	f.configured = creds
	return nil
}

func (f *fakeRemote) Healthy(context.Context) bool { return true }

func (f *fakeRemote) Tools() []mcp.ToolDefinition {
	return []mcp.ToolDefinition{
		{Name: "figma_get_figjam", Description: "upstream read", Parameters: map[string]string{"fileKey": "file key"}, Required: []string{"fileKey"}},
		{Name: "figma_use_figma", Description: "upstream write", Parameters: map[string]string{"fileKey": "file key", "skillNames": "skills"}, Required: []string{"fileKey"}},
		{Name: "figma_generate_diagram", Description: "upstream diagram"},
		{Name: "figma_create_new_file", Description: "upstream create"},
		{Name: "figma_upload_assets", Description: "upstream upload"},
		{Name: "figma_get_screenshot", Description: "upstream screenshot"},
		{Name: "figma_whoami", Description: "upstream account"},
		{Name: "figma_get_design_context", Description: "design-only tool"},
	}
}

func (f *fakeRemote) Execute(_ context.Context, name mcp.ToolName, args map[string]any) (*mcp.ToolResult, error) {
	f.executed = name
	f.args = args
	return mcp.RawResult([]byte("ok"))
}

func TestNew(t *testing.T) {
	integration := New()
	require.NotNil(t, integration)
	assert.Equal(t, "figma", integration.Name())
	assert.Equal(t, defaultBaseURL, MCPServerURL(integration))
}

func TestConfigure(t *testing.T) {
	remote := &fakeRemote{}
	integration := &figma{baseURL: defaultBaseURL, newRemote: func(string) mcp.Integration { return remote }}

	err := integration.Configure(context.Background(), mcp.Credentials{"mcp_access_token": "token"})
	require.NoError(t, err)
	assert.Equal(t, "token", remote.configured["access_token"])

	err = integration.Configure(context.Background(), mcp.Credentials{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "mcp_access_token is required")
}

func TestCredentialMetadata(t *testing.T) {
	integration := New()
	plainText := integration.(mcp.PlainTextCredentials)
	optional := integration.(mcp.OptionalCredentials)
	assert.Equal(t, []string{"base_url"}, plainText.PlainTextKeys())
	assert.Equal(t, []string{"base_url"}, optional.OptionalKeys())
}

func TestToolsExposeFigJamWorkflow(t *testing.T) {
	remote := &fakeRemote{}
	integration := &figma{baseURL: defaultBaseURL, remote: remote}

	tools := integration.Tools()
	names := make(map[mcp.ToolName]mcp.ToolDefinition, len(tools))
	for _, tool := range tools {
		names[tool.Name] = tool
	}

	assert.Len(t, tools, 7)
	for _, name := range []mcp.ToolName{
		"figma_get_figjam",
		"figma_use_figma",
		"figma_generate_diagram",
		"figma_create_new_file",
		"figma_upload_assets",
		"figma_get_screenshot",
		"figma_whoami",
	} {
		assert.Contains(t, names, name)
	}
	assert.NotContains(t, names, mcp.ToolName("figma_get_design_context"))
	assert.Contains(t, names["figma_get_figjam"].Description, "Start here")
	assert.Contains(t, names["figma_use_figma"].Description, "FigJam")
	assert.Equal(t, []string{"fileKey"}, names["figma_get_figjam"].Required)
}

func TestExecuteInjectsFigJamSkill(t *testing.T) {
	remote := &fakeRemote{}
	integration := &figma{baseURL: defaultBaseURL, remote: remote}

	result, err := integration.Execute(context.Background(), "figma_use_figma", map[string]any{"fileKey": "abc"})
	require.NoError(t, err)
	require.False(t, result.IsError)
	assert.Equal(t, mcp.ToolName("figma_use_figma"), remote.executed)
	assert.Equal(t, []any{"figma-use-figjam"}, remote.args["skillNames"])
}

func TestExecutePreservesExplicitSkills(t *testing.T) {
	remote := &fakeRemote{}
	integration := &figma{baseURL: defaultBaseURL, remote: remote}
	skills := []any{"resource:figma-use-figjam"}

	_, err := integration.Execute(context.Background(), "figma_use_figma", map[string]any{"skillNames": skills})
	require.NoError(t, err)
	assert.Equal(t, skills, remote.args["skillNames"])
}

func TestExecuteRejectsNonFigJamTool(t *testing.T) {
	integration := &figma{baseURL: defaultBaseURL, remote: &fakeRemote{}}

	result, err := integration.Execute(context.Background(), "figma_get_design_context", nil)
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "unknown tool")
}

func TestExecuteRequiresConfiguration(t *testing.T) {
	integration := New()

	result, err := integration.Execute(context.Background(), "figma_get_figjam", nil)
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, mcp.ErrNotConfigured.Error())
	assert.False(t, integration.Healthy(context.Background()))
}
