package server

import (
	"encoding/json"
	"flag"
	"os"
	"sort"
	"testing"

	mcp "github.com/daltoniam/switchboard"
	"github.com/daltoniam/switchboard/integrations/acp"
	"github.com/daltoniam/switchboard/integrations/agents"
	"github.com/daltoniam/switchboard/integrations/amazon"
	awsInt "github.com/daltoniam/switchboard/integrations/aws"
	"github.com/daltoniam/switchboard/integrations/botidentity"
	"github.com/daltoniam/switchboard/integrations/clickhouse"
	"github.com/daltoniam/switchboard/integrations/cloudflare"
	"github.com/daltoniam/switchboard/integrations/confluence"
	"github.com/daltoniam/switchboard/integrations/datadog"
	"github.com/daltoniam/switchboard/integrations/digitalocean"
	"github.com/daltoniam/switchboard/integrations/elasticsearch"
	flyInt "github.com/daltoniam/switchboard/integrations/fly"
	"github.com/daltoniam/switchboard/integrations/gcal"
	"github.com/daltoniam/switchboard/integrations/gchat"
	gcpInt "github.com/daltoniam/switchboard/integrations/gcp"
	"github.com/daltoniam/switchboard/integrations/gdocs"
	"github.com/daltoniam/switchboard/integrations/gdrive"
	"github.com/daltoniam/switchboard/integrations/gforms"
	"github.com/daltoniam/switchboard/integrations/github"
	"github.com/daltoniam/switchboard/integrations/gmail"
	"github.com/daltoniam/switchboard/integrations/gmeet"
	"github.com/daltoniam/switchboard/integrations/gpeople"
	"github.com/daltoniam/switchboard/integrations/gsheets"
	"github.com/daltoniam/switchboard/integrations/gslides"
	"github.com/daltoniam/switchboard/integrations/gtasks"
	"github.com/daltoniam/switchboard/integrations/jira"
	"github.com/daltoniam/switchboard/integrations/kubernetes"
	"github.com/daltoniam/switchboard/integrations/linear"
	"github.com/daltoniam/switchboard/integrations/metabase"
	nomadInt "github.com/daltoniam/switchboard/integrations/nomad"
	notionInt "github.com/daltoniam/switchboard/integrations/notion"
	"github.com/daltoniam/switchboard/integrations/ollama"
	"github.com/daltoniam/switchboard/integrations/pganalyze"
	"github.com/daltoniam/switchboard/integrations/postgres"
	"github.com/daltoniam/switchboard/integrations/posthog"
	"github.com/daltoniam/switchboard/integrations/rwx"
	"github.com/daltoniam/switchboard/integrations/salesforce"
	"github.com/daltoniam/switchboard/integrations/sentry"
	signozInt "github.com/daltoniam/switchboard/integrations/signoz"
	slackInt "github.com/daltoniam/switchboard/integrations/slack"
	snowflakeInt "github.com/daltoniam/switchboard/integrations/snowflake"
	"github.com/daltoniam/switchboard/integrations/stripe"
	"github.com/daltoniam/switchboard/integrations/suno"
	switchboardInt "github.com/daltoniam/switchboard/integrations/switchboard"
	"github.com/daltoniam/switchboard/integrations/vercel"
	webfetchInt "github.com/daltoniam/switchboard/integrations/webfetch"
	xInt "github.com/daltoniam/switchboard/integrations/x"
	"github.com/daltoniam/switchboard/integrations/ynab"
	"github.com/daltoniam/switchboard/registry"
	"github.com/stretchr/testify/require"
)

// updateLock regenerates tools_list.lock.json instead of asserting
// against it. Run after an intentional change to a tool's name, description,
// parameters, or required flags:
//
//	go test ./server -run TestToolsList_MatchesWireLock -update
//
// then commit the updated lock alongside the YAML change.
var updateLock = flag.Bool("update", false, "regenerate the tools/list wire lock instead of checking it")

// wireLockPath is the locked snapshot of the full tools/list output the server
// hands to the model. It is a lock file in the same sense as go.sum: a generated
// artifact derived from the tools.yaml source of truth, committed to the repo so
// CI can detect drift. It sits beside the server package (not under testdata)
// because it locks the server's own wire output. You do not hand-edit it; you
// regenerate it with -update.
const wireLockPath = "tools_list.lock.json"

// TestToolsList_MatchesWireLock asserts the live tools/list wire output matches
// the committed lock byte-for-byte. It catches any change — accidental or
// intentional — to what the model sees: a dropped description, a renamed
// parameter, a flipped required flag. An intentional change is expected to fail
// here; rerun with -update to regenerate the lock and commit it with the change.
func TestToolsList_MatchesWireLock(t *testing.T) {
	got := captureToolsListJSON(t)

	if *updateLock {
		require.NoError(t, os.WriteFile(wireLockPath, got, 0o644))
		t.Logf("regenerated %s", wireLockPath)
		return
	}

	want, err := os.ReadFile(wireLockPath)
	require.NoError(t, err)

	require.JSONEq(t, string(want), string(got),
		"tools/list output drifted from %s. If this change is intentional, "+
			"regenerate the lock: go test ./server -run TestToolsList_MatchesWireLock -update",
		wireLockPath)
}

// captureToolsListJSON projects all registered adapter tools into the MCP wire
// shape and returns the JSON. Required arrays are sorted alphabetically — JSON
// Schema treats required as a set semantically, so the sort is a normalization
// that makes the wire output deterministic regardless of how the source types
// order their required entries.
func captureToolsListJSON(t *testing.T) []byte {
	t.Helper()

	srv := buildAllIntegrationsServer(t)

	type wireProp struct {
		Type        string `json:"type"`
		Description string `json:"description"`
	}
	type wireSchema struct {
		Type       string              `json:"type"`
		Properties map[string]wireProp `json:"properties,omitempty"`
		Required   []string            `json:"required,omitempty"`
	}
	type wireTool struct {
		Name        string     `json:"name"`
		Description string     `json:"description"`
		InputSchema wireSchema `json:"inputSchema"`
	}

	tools := make([]wireTool, 0, len(srv.allTools))
	for _, twi := range srv.allTools {
		td := twi.Tool

		props := make(map[string]wireProp, len(td.Parameters))
		var required []string
		for _, p := range td.Parameters {
			props[string(p.Name)] = wireProp{Type: "string", Description: p.Description}
			if p.Required {
				required = append(required, string(p.Name))
			}
		}
		sort.Strings(required)

		schema := wireSchema{Type: "object", Properties: props}
		if len(required) > 0 {
			schema.Required = required
		}

		tools = append(tools, wireTool{
			Name:        string(td.Name),
			Description: td.Description,
			InputSchema: schema,
		})
	}

	data, err := json.MarshalIndent(map[string]any{"tools": tools}, "", "  ")
	require.NoError(t, err)
	return data
}

// buildAllIntegrationsServer registers every production integration and returns
// a Server with discoverAll so the search index covers everything.
func buildAllIntegrationsServer(t *testing.T) *Server {
	t.Helper()

	reg := registry.New()
	for _, i := range []mcp.Integration{
		github.New(),
		datadog.New(),
		linear.New("https://mcp.linear.app"),
		sentry.New(),
		slackInt.New(),
		metabase.New(),
		awsInt.New(),
		posthog.New(),
		postgres.New(),
		clickhouse.New(),
		elasticsearch.New(),
		pganalyze.New(),
		rwx.New(),
		ynab.New(),
		stripe.New(),
		amazon.New(),
		gmail.New(),
		jira.New(),
		confluence.New(),
		notionInt.New(),
		ollama.New(),
		gcpInt.New(),
		suno.New(),
		salesforce.New(),
		cloudflare.New(),
		digitalocean.New(),
		flyInt.New(),
		snowflakeInt.New(),
		acp.New(),
		agents.New(),
		signozInt.New(),
		webfetchInt.New(),
		nomadInt.New(),
		botidentity.New(),
		xInt.New(),
		gcal.New(),
		gchat.New(),
		gdocs.New(),
		gdrive.New(),
		gforms.New(),
		gmeet.New(),
		gpeople.New(),
		gsheets.New(),
		gslides.New(),
		gtasks.New(),
		kubernetes.New(),
		vercel.New(),
	} {
		require.NoError(t, reg.Register(i), "registering %s", i.Name())
	}

	services := &mcp.Services{
		Config:   newMockConfigService(nil),
		Registry: reg,
	}
	switchboardIntegration := switchboardInt.New(services)
	require.NoError(t, reg.Register(switchboardIntegration))

	return New(services, WithDiscoverAll(true))
}
