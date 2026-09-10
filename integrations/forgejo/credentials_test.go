package forgejo

import (
	"testing"

	mcp "github.com/daltoniam/switchboard"
	"github.com/stretchr/testify/require"
)

func TestCredentialUIHints(t *testing.T) {
	integration := New()
	plain, ok := integration.(mcp.PlainTextCredentials)
	require.True(t, ok, "instance URL should be visible while the token remains masked")
	require.Equal(t, []string{"base_url"}, plain.PlainTextKeys())
	hints, ok := integration.(mcp.PlaceholderHints)
	require.True(t, ok)
	require.Contains(t, hints.Placeholders()["base_url"], "instance")
	require.Contains(t, hints.Placeholders()["token"], "Personal access token")
	require.Len(t, hints.Placeholders(), 2)
}
