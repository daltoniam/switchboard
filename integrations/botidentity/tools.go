package botidentity

import mcp "github.com/daltoniam/switchboard"

var tools = []mcp.ToolDefinition{
	// --- GitHub App Management ---
	{
		Name:        mcp.ToolName("botidentity_gh_get_app"),
		Description: "Get the authenticated GitHub App's metadata including name, permissions, and events. Start here for GitHub bot and app identity management.",
		Parameters:  []mcp.Parameter{},
	},
	{
		Name:        mcp.ToolName("botidentity_gh_list_installations"),
		Description: "List all installations of a GitHub App across organizations and users. Shows where the bot is installed and its access scope.",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("per_page"), Description: "Results per page (default: 30, max: 100)"}, {Name: mcp.ParamName("page"), Description: "Page number (default: 1)"}},
	},
	{
		Name:        mcp.ToolName("botidentity_gh_get_installation"),
		Description: "Get details of a specific GitHub App installation including target account, permissions, and repository access. Use after botidentity_gh_list_installations.",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("installation_id"), Description: "Installation ID (from botidentity_gh_list_installations)", Required: true}},
	},
	{
		Name:        mcp.ToolName("botidentity_gh_create_install_token"),
		Description: "Create a short-lived installation access token for a GitHub App bot. Token grants the bot's permissions and expires in 1 hour. Use to authenticate bot API calls.",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("installation_id"), Description: "Installation ID to create token for", Required: true}, {Name: mcp.ParamName("repositories"), Description: "JSON array of repository names to scope the token to (optional, defaults to all)"}, {Name: mcp.ParamName("permissions"), Description: `JSON object of permission overrides e.g. {"contents":"read","issues":"write"} (optional, defaults to app permissions)`}},
	},
	{
		Name:        mcp.ToolName("botidentity_gh_list_install_repos"),
		Description: "List repositories accessible to a GitHub App installation. Shows which repos the bot identity can access.",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("installation_id"), Description: "Installation ID", Required: true}, {Name: mcp.ParamName("per_page"), Description: "Results per page (default: 30, max: 100)"}, {Name: mcp.ParamName("page"), Description: "Page number (default: 1)"}},
	},
	{
		Name:        mcp.ToolName("botidentity_gh_add_install_repo"),
		Description: "Add a repository to a GitHub App installation. Grants the bot identity access to an additional repository.",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("installation_id"), Description: "Installation ID", Required: true}, {Name: mcp.ParamName("repository_id"), Description: "Repository ID to add (numeric ID, not name)", Required: true}},
	},
	{
		Name:        mcp.ToolName("botidentity_gh_remove_install_repo"),
		Description: "Remove a repository from a GitHub App installation. Revokes the bot identity's access to a repository.",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("installation_id"), Description: "Installation ID", Required: true}, {Name: mcp.ParamName("repository_id"), Description: "Repository ID to remove (numeric ID, not name)", Required: true}},
	},
	{
		Name:        mcp.ToolName("botidentity_gh_suspend_installation"),
		Description: "Suspend a GitHub App installation. Disables the bot identity without uninstalling. All API access and webhooks are paused.",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("installation_id"), Description: "Installation ID to suspend", Required: true}},
	},
	{
		Name:        mcp.ToolName("botidentity_gh_unsuspend_installation"),
		Description: "Unsuspend a GitHub App installation. Re-enables a previously suspended bot identity, restoring API access and webhooks.",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("installation_id"), Description: "Installation ID to unsuspend", Required: true}},
	},
	{
		Name:        mcp.ToolName("botidentity_gh_delete_installation"),
		Description: "Delete a GitHub App installation. Permanently removes the bot identity from an organization or user account. Irreversible.",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("installation_id"), Description: "Installation ID to delete", Required: true}},
	},
	{
		Name:        mcp.ToolName("botidentity_gh_get_webhook_config"),
		Description: "Get the webhook configuration for a GitHub App. Shows delivery URL, content type, and SSL verification settings.",
		Parameters:  []mcp.Parameter{},
	},
	{
		Name:        mcp.ToolName("botidentity_gh_update_webhook_config"),
		Description: "Update the webhook configuration for a GitHub App. Change the delivery URL, content type, secret, or SSL settings.",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("url"), Description: "Webhook payload delivery URL"}, {Name: mcp.ParamName("content_type"), Description: "Content type: 'json' or 'form' (default: 'form')"}, {Name: mcp.ParamName("secret"), Description: "Secret for HMAC signature verification"}, {Name: mcp.ParamName("insecure_ssl"), Description: "SSL verification: '0' (verify) or '1' (skip)"}},
	},
	{
		Name:        mcp.ToolName("botidentity_gh_list_webhook_deliveries"),
		Description: "List recent webhook deliveries for a GitHub App. Shows delivery status, response codes, and timing for debugging bot event handling.",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("per_page"), Description: "Results per page (default: 30, max: 100)"}, {Name: mcp.ParamName("cursor"), Description: "Pagination cursor from previous response"}},
	},
	{
		Name:        mcp.ToolName("botidentity_gh_redeliver_webhook"),
		Description: "Redeliver a failed webhook for a GitHub App. Retry a specific delivery that the bot failed to process.",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("delivery_id"), Description: "Webhook delivery ID to redeliver", Required: true}},
	},
	{
		Name:        mcp.ToolName("botidentity_gh_create_app"),
		Description: "Create a GitHub App via the manifest flow. Starts a temporary local web server, opens a browser form that auto-submits to GitHub, waits for the user to approve, captures the redirect code, exchanges it for credentials, and stores everything in the bot inventory. All-in-one.",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("name"), Description: "GitHub App name", Required: true}, {Name: mcp.ParamName("bot_id"), Description: "Bot inventory ID to store credentials on (from botidentity_create_bot)"}, {Name: mcp.ParamName("url"), Description: "App homepage URL (default: https://github.com)"}, {Name: mcp.ParamName("webhook_url"), Description: "Webhook delivery URL (optional)"}, {Name: mcp.ParamName("org"), Description: "Organization slug to create the app under (optional, defaults to personal account)"}, {Name: mcp.ParamName("permissions"), Description: `JSON object of permissions e.g. {"contents":"read","issues":"write"} (optional, sensible defaults provided)`}, {Name: mcp.ParamName(

		// --- Slack App Management ---
		"events"), Description: `JSON array of webhook events e.g. ["push","pull_request"] (optional, sensible defaults provided)`}},
	},

	{
		Name:        mcp.ToolName("botidentity_slack_create_app"),
		Description: "Create a new Slack bot app from a manifest. Defines the bot's name, scopes, event subscriptions, slash commands, and OAuth settings. Start here for Slack bot identity creation.",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("manifest"), Description: "JSON string of the Slack app manifest defining bot name, scopes, features, and settings", Required: true}},
	},
	{
		Name:        mcp.ToolName("botidentity_slack_update_app"),
		Description: "Update an existing Slack bot app's manifest. Modify the bot's name, scopes, event subscriptions, slash commands, or OAuth settings.",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("app_id"), Description: "Slack app ID to update", Required: true}, {Name: mcp.ParamName("manifest"), Description: "JSON string of the full updated manifest (must include all fields, not just changes)", Required: true}},
	},
	{
		Name:        mcp.ToolName("botidentity_slack_delete_app"),
		Description: "Permanently delete a Slack bot app. Removes the bot identity and revokes all tokens. Irreversible.",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("app_id"), Description: "Slack app ID to delete", Required: true}},
	},
	{
		Name:        mcp.ToolName("botidentity_slack_validate_app"),
		Description: "Validate a Slack bot app manifest without creating or updating. Check for errors before applying changes to bot configuration.",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("manifest"), Description: "JSON string of the Slack app manifest to validate", Required: true}, {Name: mcp.ParamName("app_id"), Description: "Optional existing app ID to validate against"}},
	},
	{
		Name:        mcp.ToolName("botidentity_slack_export_app"),
		Description: "Export the current manifest of an existing Slack bot app. Use to inspect a bot's configuration or as a template for creating new bots.",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("app_id"), Description: "Slack app ID to export manifest from", Required: true}},
	},
	{
		Name:        mcp.ToolName("botidentity_slack_get_bot_info"),
		Description: "Get information about a Slack bot user including name, icons, and associated app. Use to verify bot identity details.",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("bot"), Description: "Bot user ID (e.g. B12345678)", Required: true}, {Name: mcp.ParamName("token"), Description: "Bot or user token with users:read scope (uses configured slack_config_token if omitted)"}},
	},
	{
		Name:        mcp.ToolName("botidentity_slack_rotate_token"),
		Description: "Rotate the Slack app configuration token using the refresh token. Returns a new access token and refresh token. The old tokens are invalidated. Config tokens expire after 12 hours.",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("refresh_token"), Description: "The xoxe refresh token (uses configured slack_refresh_token if omitted)"}},
	},
	{
		Name:        mcp.ToolName("botidentity_slack_set_bot_icon"),
		Description: "Set a Slack bot's profile photo from a file path or base64-encoded image. Requires a bot token with users.profile:write scope. Use after botidentity_generate_logo to apply the generated image.",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("path"), Description: "File path to a PNG image (e.g. from botidentity_generate_logo output). Preferred over image_base64."}, {Name: mcp.ParamName("image_base64"), Description: "Base64-encoded PNG image data (alternative to path)"}, {Name: mcp.ParamName("token"), Description: "Bot token with users.profile:write scope (uses configured slack_bot_token if omitted)"}},
	},

	// --- Logo Generation ---
	{
		Name:        mcp.ToolName("botidentity_generate_logo"),
		Description: "Open an interactive logo generator in the browser. Preview logos, tweak the prompt, regenerate until satisfied, then confirm. Logo is compressed under 900KB and saved to ~/.config/switchboard/logos/<bot-id> when bot_id is provided. Start here for bot branding and avatar creation.",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("prompt"), Description: "Initial text prompt for logo generation"}, {Name: mcp.ParamName("negative_prompt"), Description: "Initial negative prompt (things to avoid)"}, {Name: mcp.ParamName("bot_id"), Description: "Bot inventory ID — saves logo to config dir and updates inventory automatically"}, {Name: mcp.ParamName("model_id"), Description: "Bedrock model ID (default: stability.stable-image-core-v1:1). Alternatives: stability.stable-image-ultra-v1:1, stability.sd3-5-large-v1:0"}},
	},

	// --- Bot Inventory ---
	{
		Name:        mcp.ToolName("botidentity_create_bot"),
		Description: "Create a new bot identity across GitHub and Slack. Attempts to create on both platforms, registers in local inventory, and returns next steps for setup (install URLs, tokens to provide). Start here for new bot creation.",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("id"), Description: "Unique bot identifier (e.g. 'deploy-bot', 'ci-notifier')", Required: true}, {Name: mcp.ParamName("name"), Description: "Human-readable display name for the bot", Required: true}, {Name: mcp.ParamName("slack"), Description: "Enable Slack identity: 'true' (default) or 'false'"}, {Name: mcp.ParamName("github"), Description: "Enable GitHub identity: 'true' (default) or 'false'"}, {Name: mcp.ParamName("slack_manifest"), Description: "Custom Slack app manifest JSON (optional — a sensible default is generated from the bot name)"}},
	},
	{
		Name:        mcp.ToolName("botidentity_delete_bot"),
		Description: "Delete a bot identity. Removes the Slack app (if created), provides instructions for GitHub App deletion, and removes the local inventory entry.",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("id"), Description: "Bot identifier to delete", Required: true}},
	},
	{
		Name:        mcp.ToolName("botidentity_inv_list"),
		Description: "List all bots in the local inventory. Shows each bot's enabled platforms, app IDs, and logo. Filter by platform.",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("platform"), Description: "Filter by platform: 'github' or 'slack' (optional, lists all if omitted)"}},
	},
	{
		Name:        mcp.ToolName("botidentity_inv_get"),
		Description: "Get full details of a bot from the inventory including credentials (masked), platform status, and next steps for incomplete setup. Use after botidentity_inv_list.",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("id"), Description: "Bot identifier", Required: true}},
	},
	{
		Name:        mcp.ToolName("botidentity_inv_update"),
		Description: "Update a bot's profile in the inventory. Change name, logo, or platform-specific IDs.",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("id"), Description: "Bot identifier", Required: true}, {Name: mcp.ParamName("name"), Description: "New display name"}, {Name: mcp.ParamName("logo_path"), Description: "New logo image path"}, {Name: mcp.ParamName("slack_app_id"), Description: "Slack App ID (also enables Slack if not already)"}, {Name: mcp.ParamName("slack_bot_user_id"), Description: "Slack bot user ID"}, {Name: mcp.ParamName("github_app_id"), Description: "GitHub App ID (also enables GitHub if not already)"}, {Name: mcp.ParamName("github_app_slug"), Description: "GitHub App slug (for URL generation)"}},
	},
	{
		Name:        mcp.ToolName("botidentity_inv_set_cred"),
		Description: "Store a credential for a bot. Well-known keys (slack_bot_token, github_private_key, github_webhook_secret, github_client_id, github_client_secret, slack_webhook_url) are stored in platform-specific fields. Other keys go to the generic credentials map.",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("id"), Description: "Bot identifier", Required: true}, {Name: mcp.ParamName("key"), Description: "Credential key (e.g. 'slack_bot_token', 'github_private_key', 'github_webhook_secret')", Required: true}, {Name: mcp.ParamName("value"), Description: "Credential value", Required: true}},
	},
	{
		Name:        mcp.ToolName("botidentity_inv_delete_cred"),
		Description: "Remove a stored credential from a bot in the inventory.",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("id"), Description: "Bot identifier", Required: true}, {Name: mcp.ParamName("key"), Description: "Credential key to remove", Required: true}},
	},
	{
		Name:        mcp.ToolName("botidentity_inv_get_creds"),
		Description: "List all credentials stored for a bot (values masked). Use botidentity_inv_get_cred to reveal a specific value.",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("id"), Description: "Bot identifier", Required: true}},
	},
	{
		Name:        mcp.ToolName("botidentity_inv_get_cred"),
		Description: "Get the full unmasked value of a specific credential for a bot. Use after botidentity_inv_get_creds to identify the key.",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("id"), Description: "Bot identifier", Required: true}, {Name: mcp.ParamName("key"), Description: "Credential key to retrieve", Required: true}},
	},
	{
		Name:        mcp.ToolName("botidentity_inv_set_logo"),
		Description: "Set or update a bot's logo path in the inventory. Use after botidentity_generate_logo to associate the generated image.",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("id"), Description: "Bot identifier", Required: true}, {Name: mcp.ParamName("logo_path"), Description: "Path to logo image file", Required: true}},
	},
	{
		Name:        mcp.ToolName("botidentity_inv_set_meta"),
		Description: "Set a metadata key-value pair on a bot in the inventory. Use for descriptions, environment, team ownership, or any custom labels.",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("id"), Description: "Bot identifier", Required: true}, {Name: mcp.ParamName("key"), Description: "Metadata key (e.g. 'environment', 'team', 'description')", Required: true}, {Name: mcp.ParamName("value"), Description: "Metadata value", Required: true}},
	},
}
