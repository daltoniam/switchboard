package web

import (
	"bytes"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"os/exec"
	"strconv"
	"strings"
	"testing"
	"time"

	mcp "github.com/daltoniam/switchboard"
	"github.com/daltoniam/switchboard/integrations/snipeit"
	"github.com/stretchr/testify/require"
)

const liveSnipeITImage = "snipe/snipe-it:v8.8.0@sha256:973da940ffcca77ccd31fd0197e182b8b8614531bf3e8ed7f8665b83722c67ce"
const liveSnipeITDBImage = "mariadb:11.4.7@sha256:39596f079862334be04f4231664862e55d4febe54309cc62f750f2297de85b06"

func init() {
	registerLiveSpec(liveSpec{
		Integration: "snipeit", Tier: liveContainer, Adapter: snipeit.New(), Provision: liveSnipeITProvision,
		Cases: []liveCase{
			{Tool: "snipeit_list_assets", Args: map[string]any{"search": "SWITCHBOARD-LIVE", "limit": 1}, Expect: []string{"SWITCHBOARD-LIVE-1", "Incident laptop", "Live Laptop", "Office", "user"}},
			{Tool: "snipeit_list_assets", Args: map[string]any{"status": "deployed"}, Expect: []string{"SWITCHBOARD-LIVE-1"}},
			{Tool: "snipeit_list_assets", Args: map[string]any{"status": "ready"}, Expect: []string{"SWITCHBOARD-LIVE-2"}},
			{Tool: "snipeit_list_assets", Args: map[string]any{"search": "SWITCHBOARD-LIVE", "limit": 1, "offset": 1}, Expect: []string{"SWITCHBOARD-LIVE-2"}},
			{Tool: "snipeit_get_asset", Args: map[string]any{"id": "${asset_id}"}, Expect: []string{"SWITCHBOARD-LIVE-1", "LIVE-SERIAL-1", "Live Laptop", "Ready", "Office", "user"}},
			{Tool: "snipeit_get_asset_by_tag", Args: map[string]any{"tag": "SWITCHBOARD-LIVE-1"}, Expect: []string{"SWITCHBOARD-LIVE-1", "Incident laptop", "LIVE-SERIAL-1", "user"}},
		},
	})
}

func liveSnipeITProvision(t *testing.T) liveProvision {
	database := liveStartContainer(t, liveDockerOptions{
		Image: liveSnipeITDBImage,
		Env:   map[string]string{"MARIADB_DATABASE": "snipeit", "MARIADB_USER": "snipeit", "MARIADB_PASSWORD": "disposable-db-password", "MARIADB_ROOT_PASSWORD": "disposable-root-password"},
	})
	liveEventually(t, time.Minute, func() error {
		return exec.CommandContext(t.Context(), "docker", "exec", database.id, "healthcheck.sh", "--connect", "--innodb_initialized").Run()
	})
	dbHost := strings.TrimSpace(liveDocker(t, "inspect", "--format", "{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}", database.id))
	provider := liveStartContainer(t, liveDockerOptions{
		Image: liveSnipeITImage, Ports: []string{"80/tcp"},
		Env: map[string]string{
			"APP_KEY": "base64:YWJjZGVmZ2hpamtsbW5vcHFyc3R1dnd4eXphYmNkZWY=", "APP_URL": "http://localhost", "APP_ENV": "production", "APP_DEBUG": "false",
			"DB_HOST": dbHost, "DB_PORT": "3306", "DB_DATABASE": "snipeit", "DB_USERNAME": "snipeit", "DB_PASSWORD": "disposable-db-password", "MAIL_MAILER": "log",
		},
	})
	host, port := provider.Address(t, "80/tcp")
	base := fmt.Sprintf("http://%s:%s", host, port)
	client := &http.Client{Timeout: 15 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	liveEventually(t, 2*time.Minute, func() error {
		request, err := http.NewRequestWithContext(t.Context(), http.MethodGet, base+"/", nil)
		if err != nil {
			return err
		}
		response, err := client.Do(request)
		if err != nil {
			return err
		}
		_ = response.Body.Close()
		if response.StatusCode >= 500 {
			return fmt.Errorf("Snipe-IT not ready: HTTP %d", response.StatusCode)
		}
		return nil
	})
	provider.Exec(t, "", "php", "artisan", "migrate", "--force")
	provider.Exec(t, `<?php
require '/var/www/html/vendor/autoload.php';
$app = require '/var/www/html/bootstrap/app.php';
$app->make(Illuminate\Contracts\Console\Kernel::class)->bootstrap();
$settings = new App\Models\Setting;
$settings->brand = 1;
$settings->site_name = 'Switchboard Live';
$settings->locale = 'en-US';
$settings->default_currency = 'USD';
$settings->full_multiple_companies_support = 1;
$settings->null_company_is_floater = 0;
$settings->pwd_secure_min = 10;
$settings->saveOrFail();
`, "php")
	provider.Exec(t, "", "php", "artisan", "snipeit:create-admin", "--first_name=Live", "--last_name=Admin", "--username=admin", "--email=admin@localhost.test", "--password=disposable-admin-password")
	provider.Exec(t, "", "php", "artisan", "passport:keys", "--force")
	provider.Exec(t, "", "chown", "docker:root", "/var/lib/snipeit/keys/oauth-public.key", "/var/lib/snipeit/keys/oauth-private.key")
	provider.Exec(t, "", "php", "artisan", "passport:client", "--personal", "--name=live", "--no-interaction")
	adminToken := strings.TrimSpace(provider.Exec(t, "", "php", "artisan", "snipeit:make-api-key", "--user_id=1", "--key-only"))
	call := func(token, method, path string, body any, want int) []byte {
		t.Helper()
		var data []byte
		if body != nil {
			var err error
			data, err = json.Marshal(body)
			require.NoError(t, err)
		}
		request, err := http.NewRequestWithContext(t.Context(), method, base+"/api/v1"+path, bytes.NewReader(data))
		require.NoError(t, err)
		request.Header.Set("Authorization", "Bearer "+token)
		request.Header.Set("Accept", "application/json")
		request.Header.Set("Content-Type", "application/json")
		response, err := client.Do(request)
		require.NoError(t, err)
		defer func() { require.NoError(t, response.Body.Close()) }()
		payload, err := io.ReadAll(response.Body)
		require.NoError(t, err)
		require.Equal(t, want, response.StatusCode, "%s %s", method, path)
		return payload
	}
	create := func(path string, body any) int {
		t.Helper()
		var envelope struct {
			Status  string `json:"status"`
			Payload struct {
				ID int `json:"id"`
			} `json:"payload"`
			Messages any `json:"messages"`
		}
		require.NoError(t, json.Unmarshal(call(adminToken, http.MethodPost, path, body, 200), &envelope))
		require.Equal(t, "success", envelope.Status, "%s: %v", path, envelope.Messages)
		require.Positive(t, envelope.Payload.ID)
		return envelope.Payload.ID
	}
	company := create("/companies", map[string]any{"name": "Approved Company"})
	otherCompany := create("/companies", map[string]any{"name": "Other Company"})
	location := create("/locations", map[string]any{"name": "Office", "company_id": company})
	readerID := create("/users", map[string]any{
		"first_name": "Live", "last_name": "Reader", "username": "switchboard-reader", "email": "SECRET_SENTINEL@localhost.test", "password": "disposable-reader-password", "password_confirmation": "disposable-reader-password", "activated": true,
		"location_id": location, "company_ids": []int{company}, "permissions": map[string]int{"self.api": 1, "assets.view": 1},
	})
	category := create("/categories", map[string]any{"name": "Live Computers", "category_type": "asset"})
	fieldset := create("/fieldsets", map[string]any{"name": "Live security fields"})
	field := create("/fields", map[string]any{"name": "Live encrypted secret", "element": "text", "format": "ANY", "field_encrypted": true})
	create(fmt.Sprintf("/fields/%d/associate", field), map[string]any{"fieldset_id": fieldset})
	var customField struct {
		Column string `json:"db_column_name"`
	}
	require.NoError(t, json.Unmarshal(call(adminToken, http.MethodGet, fmt.Sprintf("/fields/%d", field), nil, 200), &customField))
	require.NotEmpty(t, customField.Column)
	model := create("/models", map[string]any{"name": "Live Laptop", "model_number": "LIVE-1", "category_id": category, "fieldset_id": fieldset})
	status := create("/statuslabels", map[string]any{"name": "Ready", "type": "deployable"})
	assetID := create("/hardware", map[string]any{"name": "Incident laptop", "asset_tag": "SWITCHBOARD-LIVE-1", "serial": "LIVE-SERIAL-1", "model_id": model, "status_id": status, "company_id": company, "rtd_location_id": location, "location_id": location, "notes": "SECRET_SENTINEL", customField.Column: "ENCRYPTED_SECRET_SENTINEL"})
	create("/hardware", map[string]any{"name": "Spare laptop", "asset_tag": "SWITCHBOARD-LIVE-2", "serial": "LIVE-SERIAL-2", "model_id": model, "status_id": status, "company_id": company, "rtd_location_id": location, "location_id": location})
	otherAssetID := create("/hardware", map[string]any{"name": "SECRET_SENTINEL", "asset_tag": "OTHER-COMPANY", "model_id": model, "status_id": status, "company_id": otherCompany})
	var checkout struct {
		Status   string `json:"status"`
		Messages any    `json:"messages"`
	}
	require.NoError(t, json.Unmarshal(call(adminToken, http.MethodPost, fmt.Sprintf("/hardware/%d/checkout", assetID), map[string]any{"checkout_to_type": "user", "assigned_user": readerID}, 200), &checkout))
	require.Equal(t, "success", checkout.Status, checkout.Messages)
	readerToken := strings.TrimSpace(provider.Exec(t, "", "php", "artisan", "snipeit:make-api-key", "--user_id="+strconv.Itoa(readerID), "--key-only"))
	adminAsset := call(adminToken, http.MethodGet, fmt.Sprintf("/hardware/%d", assetID), nil, 200)
	require.Contains(t, string(adminAsset), "ENCRYPTED_SECRET_SENTINEL")
	readerAsset := call(readerToken, http.MethodGet, fmt.Sprintf("/hardware/%d", assetID), nil, 200)
	require.Contains(t, string(readerAsset), "Live encrypted secret")
	require.NotContains(t, string(readerAsset), "ENCRYPTED_SECRET_SENTINEL")
	var page struct {
		Total int `json:"total"`
		Rows  []struct {
			ID int `json:"id"`
		} `json:"rows"`
	}
	require.NoError(t, json.Unmarshal(call(readerToken, http.MethodGet, "/hardware?limit=100", nil, 200), &page))
	require.Equal(t, 2, page.Total)
	require.Len(t, page.Rows, 2)
	for _, row := range page.Rows {
		require.NotEqual(t, otherAssetID, row.ID)
	}
	var hidden struct {
		Status  string `json:"status"`
		Payload any    `json:"payload"`
	}
	hiddenData := call(readerToken, http.MethodGet, fmt.Sprintf("/hardware/%d", otherAssetID), nil, 200)
	require.NoError(t, json.Unmarshal(hiddenData, &hidden))
	require.Equal(t, "error", hidden.Status)
	require.Nil(t, hidden.Payload)
	require.NotContains(t, string(hiddenData), "SECRET_SENTINEL")
	for _, denied := range []struct {
		method, path string
		body         any
	}{
		{http.MethodPost, "/hardware", map[string]any{"asset_tag": "DENIED", "model_id": model, "status_id": status}},
		{http.MethodPatch, fmt.Sprintf("/hardware/%d", assetID), map[string]any{"name": "DENIED"}},
		{http.MethodDelete, fmt.Sprintf("/hardware/%d", assetID), nil},
		{http.MethodPost, fmt.Sprintf("/hardware/%d/checkin", assetID), map[string]any{}},
		{http.MethodPost, fmt.Sprintf("/hardware/%d/checkout", assetID), map[string]any{"checkout_to_type": "user", "assigned_user": readerID}},
		{http.MethodGet, "/users", nil}, {http.MethodGet, "/licenses", nil},
	} {
		call(readerToken, denied.method, denied.path, denied.body, 403)
	}
	target, err := url.Parse(base)
	require.NoError(t, err)
	proxy := httptest.NewTLSServer(httputil.NewSingleHostReverseProxy(target))
	t.Cleanup(proxy.Close)
	cert := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: proxy.Certificate().Raw})
	credentials := map[string]string{"base_url": proxy.URL, "token": readerToken, "ca_cert": strings.ReplaceAll(string(cert), "\n", `\n`)}
	adapter := snipeit.New()
	t.Cleanup(func() { require.NoError(t, adapter.(io.Closer).Close()) })
	require.NoError(t, adapter.Configure(t.Context(), credentials))
	require.True(t, adapter.Healthy(t.Context()))
	for _, tool := range adapter.Tools() {
		args := map[string]any{}
		if tool.Name == "snipeit_get_asset" {
			args["id"] = assetID
		}
		if tool.Name == "snipeit_get_asset_by_tag" {
			args["tag"] = "SWITCHBOARD-LIVE-1"
		}
		result, err := adapter.Execute(t.Context(), tool.Name, args)
		require.NoError(t, err)
		require.False(t, result.IsError, result.Data)
		for _, forbidden := range []string{"SECRET_SENTINEL", "custom_fields", "purchase_cost", "email", "notes", readerToken} {
			require.NotContains(t, result.Data, forbidden)
		}
	}
	for _, args := range []struct {
		tool mcp.ToolName
		args map[string]any
	}{
		{"snipeit_get_asset", map[string]any{"id": otherAssetID}},
		{"snipeit_get_asset_by_tag", map[string]any{"tag": "OTHER-COMPANY"}},
	} {
		result, err := adapter.Execute(t.Context(), args.tool, args.args)
		require.NoError(t, err)
		require.True(t, result.IsError)
		require.NotContains(t, result.Data, "SECRET_SENTINEL")
	}
	return liveProvision{Credentials: credentials, Values: map[string]string{"asset_id": strconv.Itoa(assetID)}}
}
