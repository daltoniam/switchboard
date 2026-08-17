package botidentity

import mcp "github.com/daltoniam/switchboard"

var tools = []mcp.ToolDefinition{
	// --- GitHub App Management ---
	{
		Name:        mcp.ToolName("botidentity_gh_get_app"),
		Description: "Get the authenticated GitHub App's metadata including name, permissions, and events. Start here for GitHub bot and app identity management.",
		Parameters:  map[string]string{},
	},
	{
		Name:        mcp.ToolName("botidentity_gh_list_installations"),
		Description: "List all installations of a GitHub App across organizations and users. Shows where the bot is installed and its access scope.",
		Parameters: map[string]string{
			"per_page": "Results per page (default: 30, max: 100)",
			"page":     "Page number (default: 1)",
		},
	},
	{
		Name:        mcp.ToolName("botidentity_gh_get_installation"),
		Description: "Get details of a specific GitHub App installation including target account, permissions, and repository access. Use after botidentity_gh_list_installations.",
		Parameters: map[string]string{
			"installation_id": "Installation ID (from botidentity_gh_list_installations)",
		},
		Required: []string{"installation_id"},
	},
	{
		Name:        mcp.ToolName("botidentity_gh_create_install_token"),
		Description: "Create a short-lived installation access token for a GitHub App bot. Token grants the bot's permissions and expires in 1 hour. Use to authenticate bot API calls.",
		Parameters: map[string]string{
			"installation_id": "Installation ID to create token for",
			"repositories":    "JSON array of repository names to scope the token to (optional, defaults to all)",
			"permissions":     "JSON object of permission overrides e.g. {\"contents\":\"read\",\"issues\":\"write\"} (optional, defaults to app permissions)",
		},
		Required: []string{"installation_id"},
	},
	{
		Name:        mcp.ToolName("botidentity_gh_list_install_repos"),
		Description: "List repositories accessible to a GitHub App installation. Shows which repos the bot identity can access.",
		Parameters: map[string]string{
			"installation_id": "Installation ID",
			"per_page":        "Results per page (default: 30, max: 100)",
			"page":            "Page number (default: 1)",
		},
		Required: []string{"installation_id"},
	},
	{
		Name:        mcp.ToolName("botidentity_gh_add_install_repo"),
		Description: "Add a repository to a GitHub App installation. Grants the bot identity access to an additional repository.",
		Parameters: map[string]string{
			"installation_id": "Installation ID",
			"repository_id":   "Repository ID to add (numeric ID, not name)",
		},
		Required: []string{"installation_id", "repository_id"},
	},
	{
		Name:        mcp.ToolName("botidentity_gh_remove_install_repo"),
		Description: "Remove a repository from a GitHub App installation. Revokes the bot identity's access to a repository.",
		Parameters: map[string]string{
			"installation_id": "Installation ID",
			"repository_id":   "Repository ID to remove (numeric ID, not name)",
		},
		Required: []string{"installation_id", "repository_id"},
	},
	{
		Name:        mcp.ToolName("botidentity_gh_suspend_installation"),
		Description: "Suspend a GitHub App installation. Disables the bot identity without uninstalling. All API access and webhooks are paused.",
		Parameters: map[string]string{
			"installation_id": "Installation ID to suspend",
		},
		Required: []string{"installation_id"},
	},
	{
		Name:        mcp.ToolName("botidentity_gh_unsuspend_installation"),
		Description: "Unsuspend a GitHub App installation. Re-enables a previously suspended bot identity, restoring API access and webhooks.",
		Parameters: map[string]string{
			"installation_id": "Installation ID to unsuspend",
		},
		Required: []string{"installation_id"},
	},
	{
		Name:        mcp.ToolName("botidentity_gh_delete_installation"),
		Description: "Delete a GitHub App installation. Permanently removes the bot identity from an organization or user account. Irreversible.",
		Parameters: map[string]string{
			"installation_id": "Installation ID to delete",
		},
		Required: []string{"installation_id"},
	},
	{
		Name:        mcp.ToolName("botidentity_gh_get_webhook_config"),
		Description: "Get the webhook configuration for a GitHub App. Shows delivery URL, content type, and SSL verification settings.",
		Parameters:  map[string]string{},
	},
	{
		Name:        mcp.ToolName("botidentity_gh_update_webhook_config"),
		Description: "Update the webhook configuration for a GitHub App. Change the delivery URL, content type, secret, or SSL settings.",
		Parameters: map[string]string{
			"url":          "Webhook payload delivery URL",
			"content_type": "Content type: 'json' or 'form' (default: 'form')",
			"secret":       "Secret for HMAC signature verification",
			"insecure_ssl": "SSL verification: '0' (verify) or '1' (skip)",
		},
	},
	{
		Name:        mcp.ToolName("botidentity_gh_list_webhook_deliveries"),
		Description: "List recent webhook deliveries for a GitHub App. Shows delivery status, response codes, and timing for debugging bot event handling.",
		Parameters: map[string]string{
			"per_page": "Results per page (default: 30, max: 100)",
			"cursor":   "Pagination cursor from previous response",
		},
	},
	{
		Name:        mcp.ToolName("botidentity_gh_redeliver_webhook"),
		Description: "Redeliver a failed webhook for a GitHub App. Retry a specific delivery that the bot failed to process.",
		Parameters: map[string]string{
			"delivery_id": "Webhook delivery ID to redeliver",
		},
		Required: []string{"delivery_id"},
	},
	{
		Name:        mcp.ToolName("botidentity_gh_create_app"),
		Description: "Create a GitHub App via the manifest flow. Starts a temporary local web server, opens a browser form that auto-submits to GitHub, waits for the user to approve, captures the redirect code, exchanges it for credentials, and stores everything in the bot inventory. All-in-one.",
		Parameters: map[string]string{
			"name":        "GitHub App name",
			"bot_id":      "Bot inventory ID to store credentials on (from botidentity_create_bot)",
			"url":         "App homepage URL (default: https://github.com)",
			"webhook_url": "Webhook delivery URL (optional)",
			"org":         "Organization slug to create the app under (optional, defaults to personal account)",
			"permissions": "JSON object of permissions e.g. {\"contents\":\"read\",\"issues\":\"write\"} (optional, sensible defaults provided)",
			"events":      "JSON array of webhook events e.g. [\"push\",\"pull_request\"] (optional, sensible defaults provided)",
		},
		Required: []string{"name"},
	},

	// --- Slack App Management ---
	{
		Name:        mcp.ToolName("botidentity_slack_create_app"),
		Description: "Create a new Slack bot app from a manifest. Defines the bot's name, scopes, event subscriptions, slash commands, and OAuth settings. Start here for Slack bot identity creation.",
		Parameters: map[string]string{
			"manifest": "JSON string of the Slack app manifest defining bot name, scopes, features, and settings",
		},
		Required: []string{"manifest"},
	},
	{
		Name:        mcp.ToolName("botidentity_slack_update_app"),
		Description: "Update an existing Slack bot app's manifest. Modify the bot's name, scopes, event subscriptions, slash commands, or OAuth settings.",
		Parameters: map[string]string{
			"app_id":   "Slack app ID to update",
			"manifest": "JSON string of the full updated manifest (must include all fields, not just changes)",
		},
		Required: []string{"app_id", "manifest"},
	},
	{
		Name:        mcp.ToolName("botidentity_slack_delete_app"),
		Description: "Permanently delete a Slack bot app. Removes the bot identity and revokes all tokens. Irreversible.",
		Parameters: map[string]string{
			"app_id": "Slack app ID to delete",
		},
		Required: []string{"app_id"},
	},
	{
		Name:        mcp.ToolName("botidentity_slack_validate_app"),
		Description: "Validate a Slack bot app manifest without creating or updating. Check for errors before applying changes to bot configuration.",
		Parameters: map[string]string{
			"manifest": "JSON string of the Slack app manifest to validate",
			"app_id":   "Optional existing app ID to validate against",
		},
		Required: []string{"manifest"},
	},
	{
		Name:        mcp.ToolName("botidentity_slack_export_app"),
		Description: "Export the current manifest of an existing Slack bot app. Use to inspect a bot's configuration or as a template for creating new bots.",
		Parameters: map[string]string{
			"app_id": "Slack app ID to export manifest from",
		},
		Required: []string{"app_id"},
	},
	{
		Name:        mcp.ToolName("botidentity_slack_get_bot_info"),
		Description: "Get information about a Slack bot user including name, icons, and associated app. Use to verify bot identity details.",
		Parameters: map[string]string{
			"bot":   "Bot user ID (e.g. B12345678)",
			"token": "Bot or user token with users:read scope (uses configured slack_config_token if omitted)",
		},
		Required: []string{"bot"},
	},
	{
		Name:        mcp.ToolName("botidentity_slack_rotate_token"),
		Description: "Rotate the Slack app configuration token using the refresh token. Returns a new access token and refresh token. The old tokens are invalidated. Config tokens expire after 12 hours.",
		Parameters: map[string]string{
			"refresh_token": "The xoxe refresh token (uses configured slack_refresh_token if omitted)",
		},
	},
	{
		Name:        mcp.ToolName("botidentity_slack_set_bot_icon"),
		Description: "Set a Slack bot's profile photo from a file path or base64-encoded image. Requires a bot token with users.profile:write scope. Use after botidentity_generate_logo to apply the generated image.",
		Parameters: map[string]string{
			"path":         "File path to a PNG image (e.g. from botidentity_generate_logo output). Preferred over image_base64.",
			"image_base64": "Base64-encoded PNG image data (alternative to path)",
			"token":        "Bot token with users.profile:write scope (uses configured slack_bot_token if omitted)",
		},
	},

	// --- Logo Generation ---
	{
		Name:        mcp.ToolName("botidentity_generate_logo"),
		Description: "Open an interactive logo generator in the browser. Preview logos, tweak the prompt, regenerate until satisfied, then confirm. Logo is compressed under 900KB and saved to ~/.config/switchboard/logos/<bot-id> when bot_id is provided. Start here for bot branding and avatar creation.",
		Parameters: map[string]string{
			"prompt":          "Initial text prompt for logo generation",
			"negative_prompt": "Initial negative prompt (things to avoid)",
			"bot_id":          "Bot inventory ID — saves logo to config dir and updates inventory automatically",
			"model_id":        "Bedrock model ID (default: stability.stable-image-core-v1:1). Alternatives: stability.stable-image-ultra-v1:1, stability.sd3-5-large-v1:0",
		},
	},

	// --- Bot Inventory ---
	{
		Name:        mcp.ToolName("botidentity_create_bot"),
		Description: "Create a new bot identity across GitHub and Slack. Attempts to create on both platforms, registers in local inventory, and returns next steps for setup (install URLs, tokens to provide). Start here for new bot creation.",
		Parameters: map[string]string{
			"id":             "Unique bot identifier (e.g. 'deploy-bot', 'ci-notifier')",
			"name":           "Human-readable display name for the bot",
			"slack":          "Enable Slack identity: 'true' (default) or 'false'",
			"github":         "Enable GitHub identity: 'true' (default) or 'false'",
			"slack_manifest": "Custom Slack app manifest JSON (optional — a sensible default is generated from the bot name)",
		},
		Required: []string{"id", "name"},
	},
	{
		Name:        mcp.ToolName("botidentity_delete_bot"),
		Description: "Delete a bot identity. Removes the Slack app (if created), provides instructions for GitHub App deletion, and removes the local inventory entry.",
		Parameters: map[string]string{
			"id": "Bot identifier to delete",
		},
		Required: []string{"id"},
	},
	{
		Name:        mcp.ToolName("botidentity_inv_list"),
		Description: "List all bots in the local inventory. Shows each bot's enabled platforms, app IDs, and logo. Filter by platform.",
		Parameters: map[string]string{
			"platform": "Filter by platform: 'github' or 'slack' (optional, lists all if omitted)",
		},
	},
	{
		Name:        mcp.ToolName("botidentity_inv_get"),
		Description: "Get full details of a bot from the inventory including credentials (masked), platform status, and next steps for incomplete setup. Use after botidentity_inv_list.",
		Parameters: map[string]string{
			"id": "Bot identifier",
		},
		Required: []string{"id"},
	},
	{
		Name:        mcp.ToolName("botidentity_inv_update"),
		Description: "Update a bot's profile in the inventory. Change name, logo, or platform-specific IDs.",
		Parameters: map[string]string{
			"id":                "Bot identifier",
			"name":              "New display name",
			"logo_path":         "New logo image path",
			"slack_app_id":      "Slack App ID (also enables Slack if not already)",
			"slack_bot_user_id": "Slack bot user ID",
			"github_app_id":     "GitHub App ID (also enables GitHub if not already)",
			"github_app_slug":   "GitHub App slug (for URL generation)",
		},
		Required: []string{"id"},
	},
	{
		Name:        mcp.ToolName("botidentity_inv_set_cred"),
		Description: "Store a credential for a bot. Well-known keys (slack_bot_token, github_private_key, github_webhook_secret, github_client_id, github_client_secret, slack_webhook_url) are stored in platform-specific fields. Other keys go to the generic credentials map.",
		Parameters: map[string]string{
			"id":    "Bot identifier",
			"key":   "Credential key (e.g. 'slack_bot_token', 'github_private_key', 'github_webhook_secret')",
			"value": "Credential value",
		},
		Required: []string{"id", "key", "value"},
	},
	{
		Name:        mcp.ToolName("botidentity_inv_delete_cred"),
		Description: "Remove a stored credential from a bot in the inventory.",
		Parameters: map[string]string{
			"id":  "Bot identifier",
			"key": "Credential key to remove",
		},
		Required: []string{"id", "key"},
	},
	{
		Name:        mcp.ToolName("botidentity_inv_get_creds"),
		Description: "List all credentials stored for a bot (values masked). Use botidentity_inv_get_cred to reveal a specific value.",
		Parameters: map[string]string{
			"id": "Bot identifier",
		},
		Required: []string{"id"},
	},
	{
		Name:        mcp.ToolName("botidentity_inv_get_cred"),
		Description: "Get the full unmasked value of a specific credential for a bot. Use after botidentity_inv_get_creds to identify the key.",
		Parameters: map[string]string{
			"id":  "Bot identifier",
			"key": "Credential key to retrieve",
		},
		Required: []string{"id", "key"},
	},
	{
		Name:        mcp.ToolName("botidentity_inv_set_logo"),
		Description: "Set or update a bot's logo path in the inventory. Use after botidentity_generate_logo to associate the generated image.",
		Parameters: map[string]string{
			"id":        "Bot identifier",
			"logo_path": "Path to logo image file",
		},
		Required: []string{"id", "logo_path"},
	},
	{
		Name:        mcp.ToolName("botidentity_inv_set_meta"),
		Description: "Set a metadata key-value pair on a bot in the inventory. Use for descriptions, environment, team ownership, or any custom labels.",
		Parameters: map[string]string{
			"id":    "Bot identifier",
			"key":   "Metadata key (e.g. 'environment', 'team', 'description')",
			"value": "Metadata value",
		},
		Required: []string{"id", "key", "value"},
	},
}
