package botidentity

import (
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"net"
	"net/http"
	"strings"
	"time"

	mcp "github.com/daltoniam/switchboard"
)

func ghGetApp(ctx context.Context, b *botidentity, _ map[string]any) (*mcp.ToolResult, error) {
	data, err := b.githubGet(ctx, "/app")
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func ghListInstallations(ctx context.Context, b *botidentity, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	perPage := r.OptInt("per_page", 30)
	page := r.OptInt("page", 1)
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}

	data, err := b.githubGet(ctx, fmt.Sprintf("/app/installations?per_page=%d&page=%d", perPage, page))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func ghGetInstallation(ctx context.Context, b *botidentity, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	installID := r.Str("installation_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}

	data, err := b.githubGet(ctx, fmt.Sprintf("/app/installations/%s", installID))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func ghCreateInstallToken(ctx context.Context, b *botidentity, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	installID := r.Str("installation_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}

	body := make(map[string]any)

	if reposRaw, _ := mcp.ArgStr(args, "repositories"); reposRaw != "" {
		var repos []string
		if err := json.Unmarshal([]byte(reposRaw), &repos); err != nil {
			return mcp.ErrResult(fmt.Errorf("repositories must be a JSON array of strings: %w", err))
		}
		body["repositories"] = repos
	}

	if permsRaw, _ := mcp.ArgStr(args, "permissions"); permsRaw != "" {
		var perms map[string]string
		if err := json.Unmarshal([]byte(permsRaw), &perms); err != nil {
			return mcp.ErrResult(fmt.Errorf("permissions must be a JSON object: %w", err))
		}
		body["permissions"] = perms
	}

	var reqBody any
	if len(body) > 0 {
		reqBody = body
	}

	data, err := b.githubPost(ctx, fmt.Sprintf("/app/installations/%s/access_tokens", installID), reqBody)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func ghListInstallRepos(ctx context.Context, b *botidentity, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	installID := r.Str("installation_id")
	perPage := r.OptInt("per_page", 30)
	page := r.OptInt("page", 1)
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}

	data, err := b.githubGet(ctx, fmt.Sprintf("/user/installations/%s/repositories?per_page=%d&page=%d", installID, perPage, page))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func ghAddInstallRepo(ctx context.Context, b *botidentity, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	installID := r.Str("installation_id")
	repoID := r.Str("repository_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}

	data, err := b.githubPut(ctx, fmt.Sprintf("/user/installations/%s/repositories/%s", installID, repoID), nil)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func ghRemoveInstallRepo(ctx context.Context, b *botidentity, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	installID := r.Str("installation_id")
	repoID := r.Str("repository_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}

	data, err := b.githubDelete(ctx, fmt.Sprintf("/user/installations/%s/repositories/%s", installID, repoID))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func ghSuspendInstallation(ctx context.Context, b *botidentity, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	installID := r.Str("installation_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}

	data, err := b.githubPut(ctx, fmt.Sprintf("/app/installations/%s/suspended", installID), nil)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func ghUnsuspendInstallation(ctx context.Context, b *botidentity, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	installID := r.Str("installation_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}

	data, err := b.githubDelete(ctx, fmt.Sprintf("/app/installations/%s/suspended", installID))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func ghDeleteInstallation(ctx context.Context, b *botidentity, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	installID := r.Str("installation_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}

	data, err := b.githubDelete(ctx, fmt.Sprintf("/app/installations/%s", installID))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func ghGetWebhookConfig(ctx context.Context, b *botidentity, _ map[string]any) (*mcp.ToolResult, error) {
	data, err := b.githubGet(ctx, "/app/hook/config")
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func ghUpdateWebhookConfig(ctx context.Context, b *botidentity, args map[string]any) (*mcp.ToolResult, error) {
	body := make(map[string]any)
	if v, _ := mcp.ArgStr(args, "url"); v != "" {
		body["url"] = v
	}
	if v, _ := mcp.ArgStr(args, "content_type"); v != "" {
		body["content_type"] = v
	}
	if v, _ := mcp.ArgStr(args, "secret"); v != "" {
		body["secret"] = v
	}
	if v, _ := mcp.ArgStr(args, "insecure_ssl"); v != "" {
		body["insecure_ssl"] = v
	}

	data, err := b.githubPatch(ctx, "/app/hook/config", body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func ghListWebhookDeliveries(ctx context.Context, b *botidentity, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	perPage := r.OptInt("per_page", 30)
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}

	path := fmt.Sprintf("/app/hook/deliveries?per_page=%d", perPage)
	if cursor, _ := mcp.ArgStr(args, "cursor"); cursor != "" {
		path += "&cursor=" + cursor
	}

	data, err := b.githubGet(ctx, path)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func ghRedeliverWebhook(ctx context.Context, b *botidentity, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	deliveryID := r.Str("delivery_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}

	data, err := b.githubPost(ctx, fmt.Sprintf("/app/hook/deliveries/%s/attempts", deliveryID), nil)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func ghCreateApp(ctx context.Context, b *botidentity, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	name := r.Str("name")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return mcp.ErrResult(fmt.Errorf("failed to start listener: %w", err))
	}
	port := listener.Addr().(*net.TCPAddr).Port
	callbackURL := fmt.Sprintf("http://localhost:%d/callback", port)

	appURL, _ := mcp.ArgStr(args, "url")
	if appURL == "" {
		appURL = "https://github.com"
	}
	webhookURL, _ := mcp.ArgStr(args, "webhook_url")

	manifest := map[string]any{
		"name":         name,
		"url":          appURL,
		"redirect_url": callbackURL,
		"public":       false,
		"hook_attributes": map[string]any{
			"url":    appURL,
			"active": false,
		},
		"default_permissions": map[string]string{
			"contents":      "read",
			"metadata":      "read",
			"issues":        "write",
			"pull_requests": "write",
		},
		"default_events": []string{"issues", "pull_request", "push"},
	}

	if webhookURL != "" {
		manifest["hook_attributes"] = map[string]any{"url": webhookURL, "active": true}
	}
	if permsRaw, _ := mcp.ArgStr(args, "permissions"); permsRaw != "" {
		var perms map[string]string
		if err := json.Unmarshal([]byte(permsRaw), &perms); err == nil {
			manifest["default_permissions"] = perms
		}
	}
	if eventsRaw, _ := mcp.ArgStr(args, "events"); eventsRaw != "" {
		var events []string
		if err := json.Unmarshal([]byte(eventsRaw), &events); err == nil {
			manifest["default_events"] = events
		}
	}

	manifestJSON, err := json.Marshal(manifest)
	if err != nil {
		_ = listener.Close()
		return mcp.ErrResult(err)
	}

	org, _ := mcp.ArgStr(args, "org")
	ghTarget := "https://github.com/settings/apps/new"
	if org != "" {
		ghTarget = fmt.Sprintf("https://github.com/organizations/%s/settings/apps/new", org)
	}

	codeCh := make(chan string, 1)
	errCh := make(chan error, 1)

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, req *http.Request) {
		data := struct {
			Target   string
			Manifest string
		}{
			Target:   ghTarget,
			Manifest: string(manifestJSON),
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := formPage.Execute(w, data); err != nil {
			http.Error(w, err.Error(), 500)
		}
	})
	mux.HandleFunc("/callback", func(w http.ResponseWriter, req *http.Request) {
		code := req.URL.Query().Get("code")
		if code == "" {
			http.Error(w, "missing code parameter", 400)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = fmt.Fprint(w, `<html><body><h2>GitHub App created!</h2><p>You can close this tab.</p></body></html>`)
		codeCh <- code
	})

	srv := &http.Server{Handler: mux, ReadHeaderTimeout: 10 * time.Second}
	go func() {
		if err := srv.Serve(listener); err != nil && !strings.Contains(err.Error(), "Server closed") {
			errCh <- err
		}
	}()

	localURL := fmt.Sprintf("http://localhost:%d", port)
	_ = openBrowser(localURL)

	botID, _ := mcp.ArgStr(args, "bot_id")

	select {
	case code := <-codeCh:
		shutCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutCtx)

		data, err := b.githubPost(ctx, fmt.Sprintf("/app-manifests/%s/conversions", code), nil)
		if err != nil {
			return mcp.ErrResult(fmt.Errorf("manifest exchange failed: %w", err))
		}

		var appConfig struct {
			ID            int    `json:"id"`
			Slug          string `json:"slug"`
			Name          string `json:"name"`
			ClientID      string `json:"client_id"`
			ClientSecret  string `json:"client_secret"`
			WebhookSecret string `json:"webhook_secret"`
			PEM           string `json:"pem"`
			HTMLURL       string `json:"html_url"`
		}
		if err := json.Unmarshal(data, &appConfig); err != nil {
			return mcp.ErrResult(fmt.Errorf("failed to parse app config: %w", err))
		}

		if botID != "" && b.inv != nil {
			_ = b.inv.update(botID, func(e *BotEntry) {
				e.GitHub.Enabled = true
				e.GitHub.AppID = fmt.Sprintf("%d", appConfig.ID)
				e.GitHub.AppSlug = appConfig.Slug
				e.GitHub.ClientID = appConfig.ClientID
				e.GitHub.ClientSecret = appConfig.ClientSecret
				e.GitHub.WebhookSecret = appConfig.WebhookSecret
				e.GitHub.PrivateKey = appConfig.PEM
			})
		}

		var nextSteps []string
		installURL := fmt.Sprintf("https://github.com/apps/%s/installations/new", appConfig.Slug)
		_ = openBrowser(installURL)
		if appConfig.HTMLURL != "" {
			nextSteps = append(nextSteps, fmt.Sprintf("View your app: %s", appConfig.HTMLURL))
		}
		nextSteps = append(nextSteps,
			fmt.Sprintf("Install page opened in browser (or go to: %s)", installURL),
		)

		return mcp.JSONResult(map[string]any{
			"app_id":             appConfig.ID,
			"app_slug":           appConfig.Slug,
			"name":               appConfig.Name,
			"client_id":          appConfig.ClientID,
			"html_url":           appConfig.HTMLURL,
			"credentials_stored": botID != "",
			"next_steps":         nextSteps,
		})

	case err := <-errCh:
		return mcp.ErrResult(fmt.Errorf("local server error: %w", err))

	case <-ctx.Done():
		shutCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutCtx)
		return mcp.ErrResult(fmt.Errorf("timed out waiting for GitHub approval — a browser window should have opened at %s", localURL))
	}
}

var formPage = template.Must(template.New("form").Parse(`<!DOCTYPE html>
<html><head><title>Create GitHub App</title></head>
<body>
<h2>Creating GitHub App...</h2>
<p>If you are not redirected automatically, click the button below.</p>
<form id="form" action="{{.Target}}" method="post">
  <input type="hidden" name="manifest" value='{{.Manifest}}'>
  <input type="submit" value="Create GitHub App">
</form>
<script>document.getElementById('form').submit();</script>
</body></html>`))
