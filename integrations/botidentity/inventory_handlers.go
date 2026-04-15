package botidentity

import (
	"context"
	"encoding/json"
	"fmt"

	mcp "github.com/daltoniam/switchboard"
)

type platformResult struct {
	Platform string `json:"platform"`
	Status   string `json:"status"`
	AppID    string `json:"app_id,omitempty"`
	Error    string `json:"error,omitempty"`
}

func createBot(ctx context.Context, b *botidentity, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	id := r.Str("id")
	name := r.Str("name")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}

	enableSlack := true
	enableGitHub := true
	if v, _ := mcp.ArgStr(args, "slack"); v == "false" {
		enableSlack = false
	}
	if v, _ := mcp.ArgStr(args, "github"); v == "false" {
		enableGitHub = false
	}

	entry := &BotEntry{
		ID:       id,
		Name:     name,
		Slack:    SlackIdentity{Enabled: enableSlack},
		GitHub:   GitHubIdentity{Enabled: enableGitHub},
		Creds:    make(map[string]string),
		Metadata: make(map[string]string),
	}

	var results []platformResult
	var nextSteps []string

	if enableSlack && b.slackConfigToken != "" {
		res, steps := tryCreateSlackApp(ctx, b, id, name, args, entry)
		results = append(results, res...)
		nextSteps = append(nextSteps, steps...)
	} else if enableSlack {
		results = append(results, platformResult{Platform: "slack", Status: "skipped", Error: "slack_config_token not configured"})
		nextSteps = append(nextSteps, "Configure slack_config_token to enable Slack bot creation")
	}

	if enableGitHub && b.githubToken != "" {
		results = append(results, platformResult{Platform: "github", Status: "pending"})
		nextSteps = append(nextSteps,
			fmt.Sprintf("Create the GitHub App: botidentity_gh_create_app name=%q bot_id=%q", name, id),
			"This starts a local server — open the provided URL in your browser and approve the app. Credentials are stored automatically.",
		)
	} else if enableGitHub {
		results = append(results, platformResult{Platform: "github", Status: "skipped", Error: "github_token not configured"})
		nextSteps = append(nextSteps, "Configure github_token to enable GitHub bot management")
	}

	if err := b.inv.add(entry); err != nil {
		return mcp.ErrResult(err)
	}

	return mcp.JSONResult(map[string]any{
		"bot":        entry,
		"results":    results,
		"next_steps": nextSteps,
	})
}

func deleteBot(ctx context.Context, b *botidentity, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	id := r.Str("id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}

	entry, err := b.inv.get(id)
	if err != nil {
		return mcp.ErrResult(err)
	}

	var results []platformResult

	if entry.Slack.Enabled && entry.Slack.AppID != "" && b.slackConfigToken != "" {
		_, err := b.slackPost(ctx, "apps.manifest.delete", map[string]any{"app_id": entry.Slack.AppID})
		if err != nil {
			results = append(results, platformResult{Platform: "slack", Status: "failed", Error: err.Error()})
		} else {
			results = append(results, platformResult{Platform: "slack", Status: "deleted"})
		}
	}

	if entry.GitHub.Enabled && entry.GitHub.AppID != "" {
		results = append(results, platformResult{Platform: "github", Status: "manual",
			Error: fmt.Sprintf("Delete manually at https://github.com/settings/apps/%s/advanced", entry.GitHub.AppSlug)})
	}

	if err := b.inv.remove(id); err != nil {
		return mcp.ErrResult(err)
	}

	return mcp.JSONResult(map[string]any{
		"status":  "deleted",
		"id":      id,
		"results": results,
	})
}

func tryCreateSlackApp(ctx context.Context, b *botidentity, id, name string, args map[string]any, entry *BotEntry) ([]platformResult, []string) {
	manifest, _ := mcp.ArgStr(args, "slack_manifest")
	if manifest == "" {
		manifest = defaultSlackManifest(name)
	}

	var m json.RawMessage
	if err := json.Unmarshal([]byte(manifest), &m); err != nil {
		return []platformResult{{Platform: "slack", Status: "failed", Error: "invalid slack_manifest JSON"}}, nil
	}

	data, err := b.slackPost(ctx, "apps.manifest.create", map[string]any{"manifest": string(m)})
	if err != nil {
		return []platformResult{{Platform: "slack", Status: "failed", Error: err.Error()}},
			[]string{"Slack app creation failed — check slack_config_token is valid and retry with botidentity_slack_create_app"}
	}

	var resp struct {
		AppID string `json:"app_id"`
		Creds struct {
			ClientID     string `json:"client_id"`
			ClientSecret string `json:"client_secret"`
		} `json:"credentials"`
	}
	if err := json.Unmarshal(data, &resp); err != nil || resp.AppID == "" {
		return []platformResult{{Platform: "slack", Status: "failed", Error: "unexpected response"}},
			[]string{"Slack app creation failed — check response and retry"}
	}

	entry.Slack.AppID = resp.AppID
	entry.Slack.ClientID = resp.Creds.ClientID
	entry.Slack.ClientSecret = resp.Creds.ClientSecret

	installURL := fmt.Sprintf("https://api.slack.com/apps/%s/install-on-team", resp.AppID)
	_ = openBrowser(installURL)

	return []platformResult{{Platform: "slack", Status: "created", AppID: resp.AppID}},
		[]string{
			fmt.Sprintf("Slack install page opened in browser (or go to: %s)", installURL),
			fmt.Sprintf("After installing, provide the Bot User OAuth Token (starts with xoxb-). Use: botidentity_inv_set_cred id=%q key=slack_bot_token value=<token>", id),
		}
}

func defaultSlackManifest(name string) string {
	m := map[string]any{
		"display_information": map[string]any{
			"name": name,
		},
		"features": map[string]any{
			"bot_user": map[string]any{
				"display_name":  name,
				"always_online": true,
			},
		},
		"oauth_config": map[string]any{
			"scopes": map[string]any{
				"bot": []string{"chat:write", "users:read"},
			},
		},
	}
	data, _ := json.Marshal(m)
	return string(data)
}

func invListBots(_ context.Context, b *botidentity, args map[string]any) (*mcp.ToolResult, error) {
	bots, err := b.inv.list()
	if err != nil {
		return mcp.ErrResult(err)
	}

	platform, _ := mcp.ArgStr(args, "platform")
	if platform != "" {
		var filtered []*BotEntry
		for _, bot := range bots {
			if platform == "slack" && bot.Slack.Enabled {
				filtered = append(filtered, bot)
			} else if platform == "github" && bot.GitHub.Enabled {
				filtered = append(filtered, bot)
			}
		}
		bots = filtered
	}

	type summary struct {
		ID        string   `json:"id"`
		Name      string   `json:"name"`
		Platforms []string `json:"platforms"`
		SlackApp  string   `json:"slack_app_id,omitempty"`
		GitHubApp string   `json:"github_app_id,omitempty"`
		LogoPath  string   `json:"logo_path,omitempty"`
		CreatedAt string   `json:"created_at"`
	}
	out := make([]summary, len(bots))
	for i, bot := range bots {
		out[i] = summary{
			ID:        bot.ID,
			Name:      bot.Name,
			Platforms: bot.platforms(),
			SlackApp:  bot.Slack.AppID,
			GitHubApp: bot.GitHub.AppID,
			LogoPath:  bot.LogoPath,
			CreatedAt: bot.CreatedAt.Format("2006-01-02T15:04:05Z"),
		}
	}
	return mcp.JSONResult(out)
}

func invGetBot(_ context.Context, b *botidentity, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	id := r.Str("id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}

	entry, err := b.inv.get(id)
	if err != nil {
		return mcp.ErrResult(err)
	}

	var nextSteps []string
	if entry.Slack.Enabled {
		if entry.Slack.AppID == "" {
			nextSteps = append(nextSteps, "Slack app not yet created — use botidentity_slack_create_app with a manifest")
		} else if entry.Slack.BotToken == "" {
			nextSteps = append(nextSteps,
				fmt.Sprintf("Install the Slack app: https://api.slack.com/apps/%s/install-on-team", entry.Slack.AppID),
				fmt.Sprintf("Then store the Bot User OAuth Token: botidentity_inv_set_cred id=%q key=slack_bot_token value=<xoxb-token>", id),
			)
		}
	}
	if entry.GitHub.Enabled && entry.GitHub.AppID == "" {
		nextSteps = append(nextSteps,
			"GitHub App not yet created — create at https://github.com/settings/apps/new",
			fmt.Sprintf("Then: botidentity_inv_update id=%q github_app_id=<id>", id),
		)
	}

	safe := *entry
	safe.Slack.ClientSecret = maskValue(safe.Slack.ClientSecret)
	safe.Slack.BotToken = maskValue(safe.Slack.BotToken)
	safe.GitHub.PrivateKey = maskValue(safe.GitHub.PrivateKey)
	safe.GitHub.WebhookSecret = maskValue(safe.GitHub.WebhookSecret)
	safe.GitHub.ClientSecret = maskValue(safe.GitHub.ClientSecret)
	safeCreds := make(map[string]string, len(entry.Creds))
	for k, v := range entry.Creds {
		safeCreds[k] = maskValue(v)
	}
	safe.Creds = safeCreds

	out := map[string]any{"bot": safe}
	if len(nextSteps) > 0 {
		out["next_steps"] = nextSteps
	}
	return mcp.JSONResult(out)
}

func invUpdateBot(_ context.Context, b *botidentity, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	id := r.Str("id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}

	err := b.inv.update(id, func(e *BotEntry) {
		if v, _ := mcp.ArgStr(args, "name"); v != "" {
			e.Name = v
		}
		if v, _ := mcp.ArgStr(args, "logo_path"); v != "" {
			e.LogoPath = v
		}
		if v, _ := mcp.ArgStr(args, "slack_app_id"); v != "" {
			e.Slack.AppID = v
			e.Slack.Enabled = true
		}
		if v, _ := mcp.ArgStr(args, "slack_bot_user_id"); v != "" {
			e.Slack.BotUserID = v
		}
		if v, _ := mcp.ArgStr(args, "github_app_id"); v != "" {
			e.GitHub.AppID = v
			e.GitHub.Enabled = true
		}
		if v, _ := mcp.ArgStr(args, "github_app_slug"); v != "" {
			e.GitHub.AppSlug = v
		}
	})
	if err != nil {
		return mcp.ErrResult(err)
	}

	return invGetBot(context.Background(), b, args)
}

func invSetCred(_ context.Context, b *botidentity, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	id := r.Str("id")
	key := r.Str("key")
	value := r.Str("value")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}

	err := b.inv.update(id, func(e *BotEntry) {
		switch key {
		case "slack_bot_token":
			e.Slack.BotToken = value
		case "slack_webhook_url":
			e.Slack.WebhookURL = value
		case "github_private_key":
			e.GitHub.PrivateKey = value
		case "github_webhook_secret":
			e.GitHub.WebhookSecret = value
		case "github_client_id":
			e.GitHub.ClientID = value
		case "github_client_secret":
			e.GitHub.ClientSecret = value
		default:
			if e.Creds == nil {
				e.Creds = make(map[string]string)
			}
			e.Creds[key] = value
		}
	})
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.JSONResult(map[string]string{"status": "ok", "id": id, "key": key})
}

func invDeleteCred(_ context.Context, b *botidentity, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	id := r.Str("id")
	key := r.Str("key")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}

	err := b.inv.update(id, func(e *BotEntry) {
		switch key {
		case "slack_bot_token":
			e.Slack.BotToken = ""
		case "github_private_key":
			e.GitHub.PrivateKey = ""
		case "github_webhook_secret":
			e.GitHub.WebhookSecret = ""
		default:
			delete(e.Creds, key)
		}
	})
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.JSONResult(map[string]string{"status": "deleted", "id": id, "key": key})
}

func invSetLogo(_ context.Context, b *botidentity, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	id := r.Str("id")
	logoPath := r.Str("logo_path")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}

	err := b.inv.update(id, func(e *BotEntry) {
		e.LogoPath = logoPath
	})
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.JSONResult(map[string]string{"status": "ok", "id": id, "logo_path": logoPath})
}

func invSetMetadata(_ context.Context, b *botidentity, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	id := r.Str("id")
	key := r.Str("key")
	value := r.Str("value")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}

	err := b.inv.update(id, func(e *BotEntry) {
		if e.Metadata == nil {
			e.Metadata = make(map[string]string)
		}
		e.Metadata[key] = value
	})
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.JSONResult(map[string]string{"status": "ok", "id": id, "key": key})
}

func invGetCreds(_ context.Context, b *botidentity, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	id := r.Str("id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}

	entry, err := b.inv.get(id)
	if err != nil {
		return mcp.ErrResult(err)
	}

	all := make(map[string]string)
	if entry.Slack.BotToken != "" {
		all["slack_bot_token"] = maskValue(entry.Slack.BotToken)
	}
	if entry.Slack.ClientSecret != "" {
		all["slack_client_secret"] = maskValue(entry.Slack.ClientSecret)
	}
	if entry.GitHub.PrivateKey != "" {
		all["github_private_key"] = maskValue(entry.GitHub.PrivateKey)
	}
	if entry.GitHub.WebhookSecret != "" {
		all["github_webhook_secret"] = maskValue(entry.GitHub.WebhookSecret)
	}
	if entry.GitHub.ClientSecret != "" {
		all["github_client_secret"] = maskValue(entry.GitHub.ClientSecret)
	}
	for k, v := range entry.Creds {
		all[k] = maskValue(v)
	}

	return mcp.JSONResult(map[string]any{
		"id":          id,
		"name":        entry.Name,
		"credentials": all,
	})
}

func invGetCredValue(_ context.Context, b *botidentity, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	id := r.Str("id")
	key := r.Str("key")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}

	entry, err := b.inv.get(id)
	if err != nil {
		return mcp.ErrResult(err)
	}

	var val string
	switch key {
	case "slack_bot_token":
		val = entry.Slack.BotToken
	case "slack_client_secret":
		val = entry.Slack.ClientSecret
	case "slack_client_id":
		val = entry.Slack.ClientID
	case "github_private_key":
		val = entry.GitHub.PrivateKey
	case "github_webhook_secret":
		val = entry.GitHub.WebhookSecret
	case "github_client_id":
		val = entry.GitHub.ClientID
	case "github_client_secret":
		val = entry.GitHub.ClientSecret
	default:
		val = entry.Creds[key]
	}

	if val == "" {
		return mcp.ErrResult(fmt.Errorf("credential %q not found on bot %q", key, id))
	}

	return mcp.JSONResult(map[string]string{"id": id, "key": key, "value": val})
}

func maskValue(v string) string {
	if v == "" {
		return ""
	}
	if len(v) > 8 {
		return v[:4] + "..." + v[len(v)-4:]
	}
	return "***"
}
