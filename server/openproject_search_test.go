package server

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/require"
)

func TestOpenProjectSearchDiscoverability(t *testing.T) {
	buildContext, cancelBuild := context.WithTimeout(t.Context(), 2*time.Minute)
	defer cancelBuild()
	binary := filepath.Join(t.TempDir(), "switchboard")
	build := exec.CommandContext(buildContext, "go", "build", "-o", binary, "../cmd/server")
	output, err := build.CombinedOutput()
	require.NoError(t, err, "build production discovery catalog: %s", output)

	for _, discoverAll := range []bool{false, true} {
		name := "disabled_by_default"
		arguments := []string{"--stdio"}
		if discoverAll {
			name = "all_registered_integrations"
			arguments = append(arguments, "--discover-all")
		}
		t.Run(name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
			defer cancel()
			home := t.TempDir()
			command := exec.CommandContext(ctx, binary, arguments...)
			command.Dir = home
			command.Env = []string{"HOME=" + home, "XDG_CONFIG_HOME=" + filepath.Join(home, ".config"), "PATH=" + os.Getenv("PATH")}
			stderr, err := os.Create(filepath.Join(home, "server.log"))
			require.NoError(t, err)
			t.Cleanup(func() {
				require.NoError(t, stderr.Close())
				if t.Failed() {
					output, err := os.ReadFile(stderr.Name())
					require.NoError(t, err)
					t.Logf("isolated server stderr: %s", output)
				}
			})
			command.Stderr = stderr
			client := mcpsdk.NewClient(&mcpsdk.Implementation{Name: "openproject-discovery-test", Version: "1"}, nil)
			session, err := client.Connect(ctx, &mcpsdk.CommandTransport{Command: command}, nil)
			require.NoError(t, err)
			defer func() { require.NoError(t, session.Close()) }()
			search := func(t *testing.T, arguments map[string]any) searchResponse {
				t.Helper()
				result, err := session.CallTool(ctx, &mcpsdk.CallToolParams{Name: "search", Arguments: arguments})
				require.NoError(t, err)
				require.False(t, result.IsError)
				return parseSearchResponse(t, result)
			}
			catalog := search(t, map[string]any{"integration": "openproject", "limit": 3})
			if !discoverAll {
				require.Zero(t, catalog.Total, "disabled OpenProject must not appear in default discovery")
				require.Empty(t, searchToolNames(t, catalog))
				return
			}
			require.Equal(t, 2, catalog.Total)
			require.ElementsMatch(t, []string{"openproject_list_work_packages", "openproject_get_work_package"}, searchToolNames(t, catalog))
			all := search(t, map[string]any{"limit": 0})
			require.Greater(t, all.Total, catalog.Total, "rank against other registered integrations, not only OpenProject")
			t.Logf("production discovery corpus contains %d tools", all.Total)
			for _, scenario := range []struct {
				query string
				tool  string
			}{
				{"find OpenProject work packages", "openproject_list_work_packages"},
				{"search OpenProject Community tasks", "openproject_list_work_packages"},
				{"get exact OpenProject work package description", "openproject_get_work_package"},
				{"OpenProject project issue status", "openproject_list_work_packages"},
			} {
				t.Run(scenario.query, func(t *testing.T) {
					results := search(t, map[string]any{"query": scenario.query, "limit": 3})
					names := searchToolNames(t, results)
					require.LessOrEqual(t, len(names), 3)
					require.Contains(t, names, scenario.tool)
					t.Logf("%q top three in production catalog: %v", scenario.query, names)
				})
			}
		})
	}
}
