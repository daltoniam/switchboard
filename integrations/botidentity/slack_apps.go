package botidentity

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"

	mcp "github.com/daltoniam/switchboard"
)

func slackCreateApp(ctx context.Context, b *botidentity, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	manifestStr := r.Str("manifest")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}

	var manifest json.RawMessage
	if err := json.Unmarshal([]byte(manifestStr), &manifest); err != nil {
		return mcp.ErrResult(fmt.Errorf("manifest must be valid JSON: %w", err))
	}

	body := map[string]any{
		"manifest": string(manifest),
	}

	data, err := b.slackPost(ctx, "apps.manifest.create", body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func slackUpdateApp(ctx context.Context, b *botidentity, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	appID := r.Str("app_id")
	manifestStr := r.Str("manifest")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}

	var manifest json.RawMessage
	if err := json.Unmarshal([]byte(manifestStr), &manifest); err != nil {
		return mcp.ErrResult(fmt.Errorf("manifest must be valid JSON: %w", err))
	}

	body := map[string]any{
		"app_id":   appID,
		"manifest": string(manifest),
	}

	data, err := b.slackPost(ctx, "apps.manifest.update", body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func slackDeleteApp(ctx context.Context, b *botidentity, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	appID := r.Str("app_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}

	body := map[string]any{
		"app_id": appID,
	}

	data, err := b.slackPost(ctx, "apps.manifest.delete", body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func slackValidateApp(ctx context.Context, b *botidentity, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	manifestStr := r.Str("manifest")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}

	var manifest json.RawMessage
	if err := json.Unmarshal([]byte(manifestStr), &manifest); err != nil {
		return mcp.ErrResult(fmt.Errorf("manifest must be valid JSON: %w", err))
	}

	body := map[string]any{
		"manifest": string(manifest),
	}

	if appID, _ := mcp.ArgStr(args, "app_id"); appID != "" {
		body["app_id"] = appID
	}

	data, err := b.slackPost(ctx, "apps.manifest.validate", body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func slackExportApp(ctx context.Context, b *botidentity, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	appID := r.Str("app_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}

	body := map[string]any{
		"app_id": appID,
	}

	data, err := b.slackPost(ctx, "apps.manifest.export", body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func slackGetBotInfo(ctx context.Context, b *botidentity, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	botID := r.Str("bot")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}

	body := map[string]any{
		"bot": botID,
	}

	token, _ := mcp.ArgStr(args, "token")
	if token != "" {
		body["token"] = token
	}

	data, err := b.slackPost(ctx, "bots.info", body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func slackRotateToken(ctx context.Context, b *botidentity, args map[string]any) (*mcp.ToolResult, error) {
	refreshToken, _ := mcp.ArgStr(args, "refresh_token")
	if refreshToken == "" {
		b.mu.Lock()
		refreshToken = b.slackRefreshToken
		b.mu.Unlock()
	}
	if refreshToken == "" {
		return mcp.ErrResult(fmt.Errorf("refresh_token is required (provide as argument or configure slack_refresh_token)"))
	}

	body := map[string]any{
		"refresh_token": refreshToken,
	}

	data, err := b.slackPost(ctx, "tooling.tokens.rotate", body)
	if err != nil {
		return mcp.ErrResult(err)
	}

	var resp struct {
		OK           bool   `json:"ok"`
		Token        string `json:"token"`
		RefreshToken string `json:"refresh_token"`
	}
	if err := json.Unmarshal(data, &resp); err == nil && resp.OK {
		b.mu.Lock()
		b.slackConfigToken = resp.Token
		b.slackRefreshToken = resp.RefreshToken
		b.mu.Unlock()
	}

	return mcp.RawResult(data)
}

func slackSetBotIcon(ctx context.Context, b *botidentity, args map[string]any) (*mcp.ToolResult, error) {
	token, _ := mcp.ArgStr(args, "token")
	if token == "" {
		token = b.slackBotToken
	}
	if token == "" {
		return mcp.ErrResult(fmt.Errorf("token is required — provide as argument or configure slack_bot_token (needs users.profile:write scope)"))
	}

	var imageData []byte

	if p, _ := mcp.ArgStr(args, "path"); p != "" {
		data, err := os.ReadFile(p)
		if err != nil {
			return mcp.ErrResult(fmt.Errorf("failed to read image file: %w", err))
		}
		imageData = data
	} else {
		r := mcp.NewArgs(args)
		imageBase64 := r.Str("image_base64")
		if err := r.Err(); err != nil {
			return mcp.ErrResult(err)
		}
		data, err := base64.StdEncoding.DecodeString(imageBase64)
		if err != nil {
			return mcp.ErrResult(fmt.Errorf("invalid base64 image data: %w", err))
		}
		imageData = data
	}

	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	part, err := w.CreateFormFile("image", "logo.png")
	if err != nil {
		return mcp.ErrResult(err)
	}
	if _, err := io.Copy(part, bytes.NewReader(imageData)); err != nil {
		return mcp.ErrResult(err)
	}
	if err := w.Close(); err != nil {
		return mcp.ErrResult(err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", b.slackBaseURL+"users.setPhoto", &buf)
	if err != nil {
		return mcp.ErrResult(err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", w.FormDataContentType())

	resp, err := b.client.Do(req)
	if err != nil {
		return mcp.ErrResult(err)
	}
	defer func() { _ = resp.Body.Close() }()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return mcp.ErrResult(err)
	}

	var envelope struct {
		OK    bool   `json:"ok"`
		Error string `json:"error"`
	}
	if err := json.Unmarshal(data, &envelope); err != nil {
		return mcp.RawResult(data)
	}
	if !envelope.OK {
		return mcp.ErrResult(fmt.Errorf("slack API error: %s", envelope.Error))
	}
	return mcp.RawResult(data)
}
