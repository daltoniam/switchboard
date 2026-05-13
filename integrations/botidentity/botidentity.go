package botidentity

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"

	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime"
	mcp "github.com/daltoniam/switchboard"
	"github.com/daltoniam/switchboard/compact"
)

//go:embed compact.yaml
var compactYAML []byte

var compactResult = compact.MustLoadWithOverlay("botidentity", compactYAML, compact.Options{Strict: false})
var fieldCompactionSpecs = compactResult.Specs
var maxBytesByTool = compactResult.MaxBytes

var (
	_ mcp.Integration                = (*botidentity)(nil)
	_ mcp.FieldCompactionIntegration = (*botidentity)(nil)
	_ mcp.PlainTextCredentials       = (*botidentity)(nil)
	_ mcp.OptionalCredentials        = (*botidentity)(nil)
	_ mcp.ToolMaxBytesIntegration    = (*botidentity)(nil)
)

type botidentity struct {
	mu                sync.Mutex
	githubToken       string
	githubAppPEM      string
	githubAppID       string
	slackConfigToken  string
	slackRefreshToken string
	slackBotToken     string
	githubBaseURL     string
	slackBaseURL      string
	awsRegion         string
	bedrockClient     *bedrockruntime.Client
	inv               *inventory
	client            *http.Client
}

func New() mcp.Integration {
	return &botidentity{
		githubBaseURL: "https://api.github.com",
		slackBaseURL:  "https://slack.com/api/",
		inv:           newInventory(),
		client:        &http.Client{},
	}
}

func (b *botidentity) Name() string { return "botidentity" }

func (b *botidentity) Configure(ctx context.Context, creds mcp.Credentials) error {
	b.githubToken = creds["github_token"]
	b.githubAppPEM = creds["github_app_pem"]
	b.githubAppID = creds["github_app_id"]
	b.slackConfigToken = creds["slack_config_token"]
	b.slackRefreshToken = creds["slack_refresh_token"]
	b.slackBotToken = creds["slack_bot_token"]

	if b.githubToken == "" && b.slackConfigToken == "" {
		return fmt.Errorf("botidentity: at least one of github_token or slack_config_token is required")
	}

	if region := creds["aws_region"]; region != "" {
		b.awsRegion = region
	}
	if b.awsRegion == "" {
		b.awsRegion = "us-west-2"
	}

	opts := []func(*awsconfig.LoadOptions) error{
		awsconfig.WithRegion(b.awsRegion),
	}
	if ak := creds["aws_access_key_id"]; ak != "" {
		if sk := creds["aws_secret_access_key"]; sk != "" {
			opts = append(opts, awsconfig.WithCredentialsProvider(
				credentials.NewStaticCredentialsProvider(ak, sk, creds["aws_session_token"]),
			))
		}
	}
	cfg, err := awsconfig.LoadDefaultConfig(ctx, opts...)
	if err != nil {
		return nil
	}
	b.bedrockClient = bedrockruntime.NewFromConfig(cfg)

	return nil
}

func (b *botidentity) Healthy(ctx context.Context) bool {
	if b.githubToken != "" {
		_, err := b.githubGet(ctx, "/user")
		if err == nil {
			return true
		}
	}
	if b.slackConfigToken != "" {
		_, err := b.slackPost(ctx, "auth.test", nil)
		if err == nil {
			return true
		}
	}
	return false
}

func (b *botidentity) Tools() []mcp.ToolDefinition {
	return tools
}

func (b *botidentity) CompactSpec(toolName mcp.ToolName) ([]mcp.CompactField, bool) {
	fields, ok := fieldCompactionSpecs[toolName]
	return fields, ok
}

func (b *botidentity) MaxBytes(toolName mcp.ToolName) (int, bool) {
	n, ok := maxBytesByTool[toolName]
	return n, ok
}

func (b *botidentity) PlainTextKeys() []string {
	return []string{"github_app_id", "aws_region"}
}

func (b *botidentity) OptionalKeys() []string {
	return []string{"github_token", "github_app_pem", "github_app_id", "slack_config_token", "slack_refresh_token", "slack_bot_token", "aws_access_key_id", "aws_secret_access_key", "aws_session_token", "aws_region"}
}

func (b *botidentity) Execute(ctx context.Context, toolName mcp.ToolName, args map[string]any) (*mcp.ToolResult, error) {
	fn, ok := dispatch[toolName]
	if !ok {
		return &mcp.ToolResult{Data: fmt.Sprintf("unknown tool: %s", toolName), IsError: true}, nil
	}
	return fn(ctx, b, args)
}

type handlerFunc func(ctx context.Context, b *botidentity, args map[string]any) (*mcp.ToolResult, error)

var dispatch = map[mcp.ToolName]handlerFunc{
	// GitHub App Management
	mcp.ToolName("botidentity_gh_get_app"):                 ghGetApp,
	mcp.ToolName("botidentity_gh_list_installations"):      ghListInstallations,
	mcp.ToolName("botidentity_gh_get_installation"):        ghGetInstallation,
	mcp.ToolName("botidentity_gh_create_install_token"):    ghCreateInstallToken,
	mcp.ToolName("botidentity_gh_list_install_repos"):      ghListInstallRepos,
	mcp.ToolName("botidentity_gh_add_install_repo"):        ghAddInstallRepo,
	mcp.ToolName("botidentity_gh_remove_install_repo"):     ghRemoveInstallRepo,
	mcp.ToolName("botidentity_gh_suspend_installation"):    ghSuspendInstallation,
	mcp.ToolName("botidentity_gh_unsuspend_installation"):  ghUnsuspendInstallation,
	mcp.ToolName("botidentity_gh_delete_installation"):     ghDeleteInstallation,
	mcp.ToolName("botidentity_gh_get_webhook_config"):      ghGetWebhookConfig,
	mcp.ToolName("botidentity_gh_update_webhook_config"):   ghUpdateWebhookConfig,
	mcp.ToolName("botidentity_gh_list_webhook_deliveries"): ghListWebhookDeliveries,
	mcp.ToolName("botidentity_gh_redeliver_webhook"):       ghRedeliverWebhook,
	mcp.ToolName("botidentity_gh_create_app"):              ghCreateApp,

	// Slack App Management
	mcp.ToolName("botidentity_slack_create_app"):   slackCreateApp,
	mcp.ToolName("botidentity_slack_update_app"):   slackUpdateApp,
	mcp.ToolName("botidentity_slack_delete_app"):   slackDeleteApp,
	mcp.ToolName("botidentity_slack_validate_app"): slackValidateApp,
	mcp.ToolName("botidentity_slack_export_app"):   slackExportApp,
	mcp.ToolName("botidentity_slack_get_bot_info"): slackGetBotInfo,
	mcp.ToolName("botidentity_slack_rotate_token"): slackRotateToken,
	mcp.ToolName("botidentity_slack_set_bot_icon"): slackSetBotIcon,

	// Logo Generation
	mcp.ToolName("botidentity_generate_logo"): generateLogo,

	// Bot Inventory
	mcp.ToolName("botidentity_create_bot"):      createBot,
	mcp.ToolName("botidentity_delete_bot"):      deleteBot,
	mcp.ToolName("botidentity_inv_list"):        invListBots,
	mcp.ToolName("botidentity_inv_get"):         invGetBot,
	mcp.ToolName("botidentity_inv_update"):      invUpdateBot,
	mcp.ToolName("botidentity_inv_set_cred"):    invSetCred,
	mcp.ToolName("botidentity_inv_delete_cred"): invDeleteCred,
	mcp.ToolName("botidentity_inv_get_creds"):   invGetCreds,
	mcp.ToolName("botidentity_inv_get_cred"):    invGetCredValue,
	mcp.ToolName("botidentity_inv_set_logo"):    invSetLogo,
	mcp.ToolName("botidentity_inv_set_meta"):    invSetMetadata,
}

// --- GitHub REST API helpers ---

func (b *botidentity) githubDo(ctx context.Context, method, path string, body any) (json.RawMessage, error) {
	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		bodyReader = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, b.githubBaseURL+path, bodyReader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+b.githubToken)
	req.Header.Set("Accept", "application/vnd.github+json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := b.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode == 429 || resp.StatusCode >= 500 {
		re := &mcp.RetryableError{StatusCode: resp.StatusCode, Err: fmt.Errorf("github API error (%d): %s", resp.StatusCode, string(data))}
		re.RetryAfter = mcp.ParseRetryAfter(resp.Header.Get("Retry-After"))
		return nil, re
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("github API error (%d): %s", resp.StatusCode, string(data))
	}
	if resp.StatusCode == 204 || len(data) == 0 {
		return json.RawMessage(`{"status":"success"}`), nil
	}
	return json.RawMessage(data), nil
}

func (b *botidentity) githubGet(ctx context.Context, path string) (json.RawMessage, error) {
	return b.githubDo(ctx, "GET", path, nil)
}

func (b *botidentity) githubPost(ctx context.Context, path string, body any) (json.RawMessage, error) {
	return b.githubDo(ctx, "POST", path, body)
}

func (b *botidentity) githubPatch(ctx context.Context, path string, body any) (json.RawMessage, error) {
	return b.githubDo(ctx, "PATCH", path, body)
}

func (b *botidentity) githubDelete(ctx context.Context, path string) (json.RawMessage, error) {
	return b.githubDo(ctx, "DELETE", path, nil)
}

func (b *botidentity) githubPut(ctx context.Context, path string, body any) (json.RawMessage, error) {
	return b.githubDo(ctx, "PUT", path, body)
}

// --- Slack API helpers ---

func (b *botidentity) slackPost(ctx context.Context, method string, body any) (json.RawMessage, error) {
	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		bodyReader = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", b.slackBaseURL+method, bodyReader)
	if err != nil {
		return nil, err
	}
	b.mu.Lock()
	token := b.slackConfigToken
	b.mu.Unlock()
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json; charset=utf-8")

	resp, err := b.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode == 429 || resp.StatusCode >= 500 {
		re := &mcp.RetryableError{StatusCode: resp.StatusCode, Err: fmt.Errorf("slack API error (%d): %s", resp.StatusCode, string(data))}
		re.RetryAfter = mcp.ParseRetryAfter(resp.Header.Get("Retry-After"))
		return nil, re
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("slack API error (%d): %s", resp.StatusCode, string(data))
	}

	var envelope struct {
		OK    bool            `json:"ok"`
		Error string          `json:"error"`
		Data  json.RawMessage `json:"-"`
	}
	if err := json.Unmarshal(data, &envelope); err != nil {
		return data, nil
	}
	if !envelope.OK {
		return nil, fmt.Errorf("slack API error: %s", envelope.Error)
	}
	return json.RawMessage(data), nil
}
