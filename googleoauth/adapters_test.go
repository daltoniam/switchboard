package googleoauth_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	mcp "github.com/daltoniam/switchboard"
	"github.com/daltoniam/switchboard/googleoauth"
	"github.com/daltoniam/switchboard/integrations/gcal"
	"github.com/daltoniam/switchboard/integrations/gchat"
	"github.com/daltoniam/switchboard/integrations/gdocs"
	"github.com/daltoniam/switchboard/integrations/gdrive"
	"github.com/daltoniam/switchboard/integrations/gforms"
	"github.com/daltoniam/switchboard/integrations/gmail"
	"github.com/daltoniam/switchboard/integrations/gmeet"
	"github.com/daltoniam/switchboard/integrations/gpeople"
	"github.com/daltoniam/switchboard/integrations/gsheets"
	"github.com/daltoniam/switchboard/integrations/gslides"
	"github.com/daltoniam/switchboard/integrations/gtasks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// googleAdapters lists every adapter in the Google Workspace family.
var googleAdapters = map[string]func() mcp.Integration{
	"gmail":   gmail.New,
	"gcal":    gcal.New,
	"gdrive":  gdrive.New,
	"gdocs":   gdocs.New,
	"gsheets": gsheets.New,
	"gslides": gslides.New,
	"gforms":  gforms.New,
	"gtasks":  gtasks.New,
	"gchat":   gchat.New,
	"gpeople": gpeople.New,
	"gmeet":   gmeet.New,
}

func TestGoogleAdapters_CoverEveryWorkspaceService(t *testing.T) {
	for _, svc := range googleoauth.Services() {
		_, ok := googleAdapters[svc.Name]
		assert.True(t, ok, "service %s missing from multi-account adapter test", svc.Name)
	}
}

// recordingServer answers every request with 200 {} except that the first
// request carrying stale401 gets a 401. It records the bearer tokens seen.
type recordingServer struct {
	mu       sync.Mutex
	tokens   []string
	stale401 string
	failed   bool
}

func (rs *recordingServer) handler(w http.ResponseWriter, r *http.Request) {
	tok := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	rs.mu.Lock()
	rs.tokens = append(rs.tokens, tok)
	fail := rs.stale401 != "" && tok == rs.stale401 && !rs.failed
	if fail {
		rs.failed = true
	}
	rs.mu.Unlock()
	if fail {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":"expired"}`))
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`{}`))
}

func (rs *recordingServer) seen() []string {
	rs.mu.Lock()
	defer rs.mu.Unlock()
	return append([]string(nil), rs.tokens...)
}

// readTool returns a cheap tool and its arguments: an argument-free list
// tool when one exists, otherwise a get tool with a single required ID.
func readTool(t *testing.T, tools []mcp.ToolDefinition, skip mcp.ToolName) (mcp.ToolName, map[string]any) {
	t.Helper()
	for _, tool := range tools {
		if len(tool.Required) == 0 && tool.Name != skip && strings.Contains(string(tool.Name), "list") {
			return tool.Name, map[string]any{}
		}
	}
	for _, tool := range tools {
		if len(tool.Required) == 1 && strings.Contains(string(tool.Name), "_get_") {
			return tool.Name, map[string]any{tool.Required[0]: "id1"}
		}
	}
	t.Fatalf("no simple read tool found")
	return "", nil
}

func withAccount(args map[string]any, account string) map[string]any {
	out := map[string]any{googleoauth.AccountParam: account}
	for k, v := range args {
		out[k] = v
	}
	return out
}

func configured(t *testing.T, name string, baseURL string) mcp.Integration {
	t.Helper()
	i := googleAdapters[name]()
	_, ok := i.(mcp.MultiIdentityIntegration)
	require.True(t, ok, "%s must implement MultiIdentityIntegration", name)
	hints, ok := i.(mcp.IdentityConfigHints)
	require.True(t, ok, "%s must implement IdentityConfigHints", name)
	assert.Equal(t, []string{"access_token", "refresh_token"}, hints.IdentityCredentialKeys())

	require.NoError(t, mcp.ConfigureIntegration(context.Background(), i, &mcp.IntegrationConfig{
		Enabled: true,
		Credentials: mcp.Credentials{
			"access_token":  "default-access",
			"client_id":     "cid",
			"client_secret": "csec",
			"base_url":      baseURL,
		},
		Identities: map[string]mcp.IntegrationIdentity{
			"work": {
				Credentials: mcp.Credentials{"access_token": "work-access", "refresh_token": "work-refresh"},
				Metadata:    map[string]string{"label": "Work"},
			},
		},
	}))
	return i
}

func TestGoogleAdapters_RouteCallsToSelectedAccount(t *testing.T) {
	for name := range googleAdapters {
		t.Run(name, func(t *testing.T) {
			rs := &recordingServer{}
			ts := httptest.NewServer(http.HandlerFunc(rs.handler))
			defer ts.Close()

			i := configured(t, name, ts.URL)
			listName := mcp.ToolName(name + "_list_accounts")
			tools := i.Tools()
			names := map[mcp.ToolName]bool{}
			for _, tool := range tools {
				names[tool.Name] = true
				if tool.Name != listName {
					assert.Contains(t, tool.Parameters, googleoauth.AccountParam, "%s lacks account param", tool.Name)
				}
			}
			require.True(t, names[listName], "missing %s", listName)

			tool, args := readTool(t, tools, listName)
			res, err := i.Execute(context.Background(), tool, withAccount(args, "work"))
			require.NoError(t, err)
			require.False(t, res.IsError, res.Data)
			res, err = i.Execute(context.Background(), tool, args)
			require.NoError(t, err)
			require.False(t, res.IsError, res.Data)

			seen := rs.seen()
			require.NotEmpty(t, seen)
			assert.Equal(t, "work-access", seen[0])
			assert.Equal(t, "default-access", seen[len(seen)-1])

			res, err = i.Execute(context.Background(), listName, map[string]any{})
			require.NoError(t, err)
			assert.False(t, res.IsError)
			assert.Contains(t, res.Data, `"work"`)
			for _, secret := range []string{"work-access", "work-refresh", "default-access", "csec"} {
				assert.NotContains(t, res.Data, secret)
			}
		})
	}
}

func TestGoogleAdapters_RefreshNamedAccountOn401(t *testing.T) {
	tokenSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		if r.FormValue("refresh_token") != "work-refresh" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		_, _ = w.Write([]byte(`{"access_token":"work-new"}`))
	}))
	defer tokenSrv.Close()
	googleoauth.SetTokenURLForTest(t, tokenSrv.URL)

	for name := range googleAdapters {
		t.Run(name, func(t *testing.T) {
			rs := &recordingServer{stale401: "work-access"}
			ts := httptest.NewServer(http.HandlerFunc(rs.handler))
			defer ts.Close()

			i := configured(t, name, ts.URL)
			tool, args := readTool(t, i.Tools(), mcp.ToolName(name+"_list_accounts"))
			res, err := i.Execute(context.Background(), tool, withAccount(args, "work"))
			require.NoError(t, err)
			require.False(t, res.IsError, res.Data)
			assert.Equal(t, []string{"work-access", "work-new"}, rs.seen())
		})
	}
}

func TestGoogleAdapters_SingleAccountCatalogUnchanged(t *testing.T) {
	for name, newFn := range googleAdapters {
		t.Run(name, func(t *testing.T) {
			before := newFn().Tools()
			i := newFn()
			require.NoError(t, mcp.ConfigureIntegration(context.Background(), i, &mcp.IntegrationConfig{
				Credentials: mcp.Credentials{"access_token": "only"},
			}))
			assert.Equal(t, before, i.Tools())
			for _, tool := range i.Tools() {
				assert.NotContains(t, tool.Parameters, googleoauth.AccountParam)
			}
		})
	}
}
