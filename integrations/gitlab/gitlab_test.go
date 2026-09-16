package gitlab

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	mcp "github.com/daltoniam/switchboard"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func configured(t *testing.T, h http.HandlerFunc) (mcp.Integration, *httptest.Server) {
	t.Helper()
	s := httptest.NewServer(h)
	t.Cleanup(s.Close)
	g := New()
	require.NoError(t, g.Configure(context.Background(), mcp.Credentials{"base_url": s.URL, "token": "glpat-test"}))
	return g, s
}

func TestNewAndUnconfigured(t *testing.T) {
	g := New()
	assert.Equal(t, "gitlab", g.Name())
	assert.False(t, g.Healthy(context.Background()))
	res, err := g.Execute(context.Background(), "gitlab_get_current_user", nil)
	require.NoError(t, err)
	assert.True(t, res.IsError)
	res, err = g.Execute(context.Background(), "gitlab_unknown", nil)
	require.NoError(t, err)
	assert.True(t, res.IsError)
	assert.Contains(t, res.Data, "unknown tool")
}

func TestConfigureRequiresToken(t *testing.T) {
	err := New().Configure(context.Background(), mcp.Credentials{"base_url": "https://gitlab.com"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "token")
}

func TestNormalizeInstanceURL(t *testing.T) {
	got, err := normalizeInstanceURL("")
	require.NoError(t, err)
	assert.Equal(t, defaultInstanceURL, got)
	got, err = normalizeInstanceURL("https://gitlab.example.com/api/v4")
	require.NoError(t, err)
	assert.Equal(t, "https://gitlab.example.com", got)
}

func TestDispatchMap_AllToolsCovered(t *testing.T) {
	for _, tool := range tools {
		_, ok := dispatch[tool.Name]
		assert.True(t, ok, "missing handler for %s", tool.Name)
	}
}

func TestDispatchMap_NoOrphanHandlers(t *testing.T) {
	names := map[mcp.ToolName]bool{}
	for _, tool := range tools {
		names[tool.Name] = true
	}
	for name := range dispatch {
		assert.True(t, names[name], "orphan handler %s", name)
	}
}

func TestHealthyAndGetUser(t *testing.T) {
	g, _ := configured(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v4/user", r.URL.Path)
		assert.Equal(t, "glpat-test", r.Header.Get("PRIVATE-TOKEN"))
		fmt.Fprint(w, `{"id":1,"username":"alice"}`)
	})
	require.True(t, g.Healthy(context.Background()))
	res, err := g.Execute(context.Background(), "gitlab_get_current_user", nil)
	require.NoError(t, err)
	require.False(t, res.IsError)
	assert.Contains(t, res.Data, "alice")
}

func TestEncodeProjectID(t *testing.T) {
	assert.Equal(t, "gitlab-org%2Fgitlab", encodeProjectID("gitlab-org/gitlab"))
	assert.Equal(t, "123", encodeProjectID("123"))
}
