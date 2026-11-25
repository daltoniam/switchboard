package config

import (
	"log"
	"slices"
	"strings"

	mcp "github.com/daltoniam/switchboard"
)

// logRetired is swapped in tests to observe migration warnings.
var logRetired = log.Printf

// retiredIntegrations maps config keys of integrations that were removed
// because they duplicated another product's integration. Each is folded into
// the surviving integration when its credentials fit, otherwise dropped.
var retiredIntegrations = []struct {
	names   []string
	migrate func(cfg *mcp.Config, name string, old *mcp.IntegrationConfig) []string
}{
	{names: []string{"slackmcp", "slack-mcp"}, migrate: migrateSlackMCP},
	{names: []string{"notion-mcp", "notionmcp"}, migrate: migrateNotionMCP},
}

// migrateRetiredIntegrations removes retired integration entries from cfg in
// place and returns one warning per decision. Warnings never include secrets.
// It is idempotent so it can run on every read of the config file.
func migrateRetiredIntegrations(cfg *mcp.Config) []string {
	if cfg == nil || cfg.Integrations == nil {
		return nil
	}
	var warnings []string
	for _, retired := range retiredIntegrations {
		for _, name := range retired.names {
			old, ok := cfg.Integrations[name]
			if !ok {
				continue
			}
			delete(cfg.Integrations, name)
			if old == nil {
				old = &mcp.IntegrationConfig{}
			}
			warnings = append(warnings, retired.migrate(cfg, name, old)...)
		}
	}
	return warnings
}

func survivingIntegration(cfg *mcp.Config, name string) *mcp.IntegrationConfig {
	ic := cfg.Integrations[name]
	if ic == nil {
		ic = &mcp.IntegrationConfig{}
		cfg.Integrations[name] = ic
	}
	if ic.Credentials == nil {
		ic.Credentials = mcp.Credentials{}
	}
	return ic
}

// Slack's hosted MCP only accepted user OAuth tokens, which the Slack Web API
// also accepts, so the first identity can become the slack token. Slack
// holds one config token, so further identities cannot be carried over.
func migrateSlackMCP(cfg *mcp.Config, name string, old *mcp.IntegrationConfig) []string {
	ids := make([]string, 0, len(old.Identities))
	for id, identity := range old.Identities {
		if strings.TrimSpace(identity.Credentials["access_token"]) != "" {
			ids = append(ids, id)
		}
	}
	slices.Sort(ids)
	if len(ids) == 0 {
		return []string{"config: removed retired integration " + name + "; it had no identities to migrate. Use the slack integration instead."}
	}
	if existing := cfg.Integrations["slack"]; existing != nil && existing.Credentials["token"] != "" {
		return []string{"config: removed retired integration " + name + "; slack already has a token, so its identities were not migrated. Use the slack integration instead."}
	}
	slack := survivingIntegration(cfg, "slack")
	first := old.Identities[ids[0]]
	slack.Credentials["token"] = strings.TrimSpace(first.Credentials["access_token"])
	if teamID := first.Metadata["team_id"]; teamID != "" && slack.Credentials["team_id"] == "" {
		slack.Credentials["team_id"] = teamID
	}
	slack.Enabled = slack.Enabled || old.Enabled
	warnings := []string{"config: migrated identity " + ids[0] + " from retired integration " + name + " into slack."}
	if len(ids) > 1 {
		warnings = append(warnings, "config: dropped identities "+strings.Join(ids[1:], ", ")+" from retired integration "+name+"; add those workspaces from the Slack setup page.")
	}
	return warnings
}

// The hosted Notion MCP sign-in is now the notion integration's
// mcp_access_token mode, so the token carries over unchanged unless notion
// already has its own credentials, which must not be silently replaced.
func migrateNotionMCP(cfg *mcp.Config, name string, old *mcp.IntegrationConfig) []string {
	token := strings.TrimSpace(old.Credentials["mcp_access_token"])
	if token == "" {
		return []string{"config: removed retired integration " + name + "; it was not signed in. Use Sign in with Notion on the notion setup page instead."}
	}
	if existing := cfg.Integrations["notion"]; existing != nil {
		for _, key := range []string{"mcp_access_token", "access_token", "token_v2"} {
			if existing.Credentials[key] != "" {
				return []string{"config: removed retired integration " + name + "; notion already has credentials, so its sign-in was not migrated. Use Sign in with Notion on the notion setup page to switch."}
			}
		}
	}
	notion := survivingIntegration(cfg, "notion")
	notion.Credentials["mcp_access_token"] = token
	if baseURL := old.Credentials["base_url"]; baseURL != "" {
		notion.Credentials["mcp_base_url"] = baseURL
	}
	if source := old.Credentials[mcp.CredKeyTokenSource]; source != "" {
		notion.Credentials[mcp.CredKeyTokenSource] = source
	}
	notion.Enabled = notion.Enabled || old.Enabled
	return []string{"config: migrated the hosted MCP sign-in from retired integration " + name + " into notion."}
}
