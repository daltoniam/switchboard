package web

import (
	"context"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	mcp "github.com/daltoniam/switchboard"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
	playwright "github.com/mxschmitt/playwright-go"
	"github.com/stretchr/testify/require"
)

const uiJenkinsFolder = `{"name":"team","_class":"com.cloudbees.hudson.plugins.folder.Folder","color":"blue","lastBuild":{"number":42,"result":"SUCCESS","timestamp":1700000000000,"actions":[{"secret":"SECRET_SENTINEL"}]},"jobs":[{"name":"SECRET_SENTINEL"}],"actions":[{"parameters":[{"value":"SECRET_SENTINEL"}]}],"credentials":"SECRET_SENTINEL","url":"https://SECRET_SENTINEL.invalid/"}`
const uiJenkinsJob = `{"name":"feature/foo","_class":"org.jenkinsci.plugins.workflow.job.WorkflowJob","color":"blue","buildable":true,"inQueue":false,"lastBuild":{"number":42,"result":"SUCCESS","timestamp":1700000000000,"credentials":"SECRET_SENTINEL"},"allBuilds":[{"number":42,"result":"SUCCESS","timestamp":1700000000000,"duration":1234,"building":false,"actions":[{"secret":"SECRET_SENTINEL"}]},{"number":41,"result":"FAILURE","timestamp":1699999999000,"duration":1000,"building":false,"logs":"SECRET_SENTINEL"}],"jobs":[{"name":"SECRET_SENTINEL"}],"actions":[{"parameters":[{"value":"SECRET_SENTINEL"}]}],"logs":"SECRET_SENTINEL","credentials":{"password":"SECRET_SENTINEL"},"description":"SECRET_SENTINEL"}`
const uiJenkinsBuild = `{"number":42,"result":"SUCCESS","building":false,"timestamp":1700000000000,"duration":1234,"estimatedDuration":5678,"actions":[{"parameters":[{"value":"SECRET_SENTINEL"}]}],"changeSets":[{"author":{"email":"SECRET_SENTINEL"}}],"logs":"SECRET_SENTINEL","credentials":{"password":"SECRET_SENTINEL"},"url":"https://SECRET_SENTINEL.invalid/"}`

func uiJenkinsMCP(t *testing.T, app *uiApp, enabled bool) {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	client := mcpsdk.NewClient(&mcpsdk.Implementation{Name: "jenkins-ui-acceptance", Version: "1"}, nil)
	session, err := client.Connect(ctx, &mcpsdk.StreamableClientTransport{Endpoint: app.URL + "/mcp", DisableStandaloneSSE: true}, nil)
	require.NoError(t, err)
	defer func() { require.NoError(t, session.Close()) }()
	call := func(t *testing.T, name string, args map[string]any) string {
		t.Helper()
		result, err := session.CallTool(ctx, &mcpsdk.CallToolParams{Name: name, Arguments: args})
		require.NoError(t, err)
		require.False(t, result.IsError)
		require.NotEmpty(t, result.Content)
		text, ok := result.Content[0].(*mcpsdk.TextContent)
		require.True(t, ok)
		forbiddenFields := []string{"SECRET_SENTINEL", "ui-jenkins-token", "Authorization"}
		if name == "execute" {
			forbiddenFields = append(forbiddenFields, "credentials", "actions", "changeSets", "logs")
		}
		for _, forbidden := range forbiddenFields {
			require.NotContains(t, text.Text, forbidden)
		}
		return text.Text
	}
	var catalog struct {
		Total int `json:"total"`
	}
	require.NoError(t, json.Unmarshal([]byte(call(t, "search", map[string]any{"limit": 0})), &catalog))
	if enabled {
		require.Equal(t, 3, catalog.Total)
	} else {
		require.Zero(t, catalog.Total, "disabled Jenkins tools must be absent from default discovery")
	}
	for _, query := range []string{"Jenkins CI jobs folders build status", "Jenkins CI job build number result duration"} {
		names := uiToolNames(t, call(t, "search", map[string]any{"query": query, "integration": "jenkins", "limit": 10}))
		if enabled {
			require.ElementsMatch(t, []string{"jenkins_list_jobs", "jenkins_get_job", "jenkins_get_build"}, names)
		} else {
			require.Empty(t, names, "disabled Jenkins tools must not be discoverable")
		}
	}
	if !enabled {
		return
	}
	jobSummary := map[string]any{
		"name": "feature/foo", "_class": "org.jenkinsci.plugins.workflow.job.WorkflowJob", "color": "blue",
		"lastBuild": map[string]any{"number": float64(42), "result": "SUCCESS", "timestamp": float64(1700000000000)},
	}
	folderSummary := map[string]any{
		"name": "team", "_class": "com.cloudbees.hudson.plugins.folder.Folder", "color": "blue",
		"lastBuild": map[string]any{"number": float64(42), "result": "SUCCESS", "timestamp": float64(1700000000000)},
	}
	buildSummary := map[string]any{"number": float64(42), "result": "SUCCESS", "timestamp": float64(1700000000000), "duration": float64(1234), "building": false}
	olderSummary := map[string]any{"number": float64(41), "result": "FAILURE", "timestamp": float64(1699999999000), "duration": float64(1000), "building": false}
	for _, c := range []struct {
		name   string
		tool   mcp.ToolName
		args   map[string]any
		assert func(*testing.T, map[string]any, bool)
	}{
		{"root_defaults", "jenkins_list_jobs", map[string]any{}, func(t *testing.T, value map[string]any, _ bool) {
			require.Equal(t, map[string]any{"jobs": []any{folderSummary}, "returned_count": float64(1), "offset": float64(0), "limit": float64(20), "possibly_more": false}, value)
		}},
		{"root_empty_path", "jenkins_list_jobs", map[string]any{"folder_path": []string{}, "offset": 0, "limit": 20}, func(t *testing.T, value map[string]any, _ bool) {
			require.Equal(t, map[string]any{"jobs": []any{folderSummary}, "returned_count": float64(1), "offset": float64(0), "limit": float64(20), "possibly_more": false}, value)
		}},
		{"folder_page", "jenkins_list_jobs", map[string]any{"folder_path": []string{"team"}, "offset": 1, "limit": 1}, func(t *testing.T, value map[string]any, _ bool) {
			require.Equal(t, map[string]any{"jobs": []any{jobSummary}, "returned_count": float64(1), "offset": float64(1), "limit": float64(1), "possibly_more": true}, value)
		}},
		{"job_defaults", "jenkins_get_job", map[string]any{"job_path": []string{"team", "feature/foo"}}, func(t *testing.T, value map[string]any, _ bool) {
			want := map[string]any{"name": jobSummary["name"], "_class": jobSummary["_class"], "color": "blue", "lastBuild": jobSummary["lastBuild"], "buildable": true, "inQueue": false, "builds": []any{buildSummary, olderSummary}, "returned_count": float64(2), "offset": float64(0), "limit": float64(20), "possibly_more": false}
			require.Equal(t, want, value)
		}},
		{"job_page", "jenkins_get_job", map[string]any{"job_path": []string{"team", "feature/foo"}, "offset": 1, "limit": 1}, func(t *testing.T, value map[string]any, _ bool) {
			want := map[string]any{"name": jobSummary["name"], "_class": jobSummary["_class"], "color": "blue", "lastBuild": jobSummary["lastBuild"], "buildable": true, "inQueue": false, "builds": []any{olderSummary}, "returned_count": float64(1), "offset": float64(1), "limit": float64(1), "possibly_more": true}
			require.Equal(t, want, value)
		}},
		{"completed_build", "jenkins_get_build", map[string]any{"job_path": []string{"team", "feature/foo"}, "build_number": 42}, func(t *testing.T, value map[string]any, _ bool) {
			require.Equal(t, map[string]any{"number": float64(42), "result": "SUCCESS", "building": false, "phase": "completed", "timestamp": float64(1700000000000), "duration": float64(1234), "estimatedDuration": float64(5678)}, value)
		}},
		{"running_build", "jenkins_get_build", map[string]any{"job_path": []string{"team", "feature/foo"}, "build_number": 43}, func(t *testing.T, value map[string]any, raw bool) {
			want := map[string]any{"number": float64(43), "building": true, "phase": "building", "timestamp": float64(1700000000000), "duration": float64(1234), "estimatedDuration": float64(5678)}
			if raw {
				want["result"] = nil
			}
			require.Equal(t, want, value)
		}},
	} {
		for _, raw := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/raw=%t", c.name, raw), func(t *testing.T) {
				arguments := map[string]any{"tool_name": c.tool, "arguments": c.args}
				if raw {
					args, err := json.Marshal(c.args)
					require.NoError(t, err)
					arguments = map[string]any{"script": fmt.Sprintf("JSON.stringify(api.call(%q, %s));", c.tool, args)}
				}
				text := call(t, "execute", arguments)
				var value map[string]any
				if raw {
					var ok bool
					value, ok = liveDecodeScript(t, c.tool, text).(map[string]any)
					require.True(t, ok)
				} else {
					require.NoError(t, json.Unmarshal([]byte(text), &value))
				}
				c.assert(t, value, raw)
			})
		}
	}
}

func TestUIJenkins(t *testing.T) {
	app := newUIApp(t)
	t.Logf("isolated development URL: %s", app.URL)
	var unavailable atomic.Bool
	var healthCalls atomic.Int32
	var providerCalls atomic.Int32
	provider := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		providerCalls.Add(1)
		username, token, ok := r.BasicAuth()
		if !ok || username != "reader" || token != "ui-jenkins-token" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		if r.Method != http.MethodGet || r.Header.Get("Accept") != "application/json" || len(r.URL.Query()) != 1 {
			t.Errorf("Jenkins must issue fixed, authenticated JSON GET requests")
			http.Error(w, "invalid request", http.StatusBadRequest)
			return
		}
		tree := r.URL.Query().Get("tree")
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.EscapedPath() {
		case "/jenkins/api/json":
			if tree == "jobs[name,_class,color,lastBuild[number,result,timestamp]]{0,1}" {
				healthCalls.Add(1)
				if unavailable.Load() {
					http.Error(w, "unavailable", http.StatusServiceUnavailable)
					return
				}
			} else if tree != "jobs[name,_class,color,lastBuild[number,result,timestamp]]{0,20}" {
				t.Errorf("unexpected Jenkins root tree: %s", tree)
				http.Error(w, "invalid tree", http.StatusBadRequest)
				return
			}
			_, _ = io.WriteString(w, `{"jobs":[`+uiJenkinsFolder+`],"credentials":"SECRET_SENTINEL"}`)
		case "/jenkins/job/team/api/json":
			if tree != "jobs[name,_class,color,lastBuild[number,result,timestamp]]{1,2}" {
				t.Errorf("unexpected Jenkins folder tree: %s", tree)
				http.Error(w, "invalid tree", http.StatusBadRequest)
				return
			}
			_, _ = io.WriteString(w, `{"jobs":[`+uiJenkinsJob+`],"actions":[{"secret":"SECRET_SENTINEL"}]}`)
		case "/jenkins/job/team/job/feature%2Ffoo/api/json":
			if tree == "name,_class,color,buildable,inQueue,lastBuild[number,result,timestamp],allBuilds[number,result,timestamp,duration,building]{0,20}" {
				_, _ = io.WriteString(w, uiJenkinsJob)
				return
			}
			if tree != "name,_class,color,buildable,inQueue,lastBuild[number,result,timestamp],allBuilds[number,result,timestamp,duration,building]{1,2}" {
				t.Errorf("unexpected Jenkins job tree: %s", tree)
				http.Error(w, "invalid tree", http.StatusBadRequest)
				return
			}
			var job map[string]any
			if err := json.Unmarshal([]byte(uiJenkinsJob), &job); err != nil {
				t.Errorf("decode Jenkins fixture: %v", err)
				return
			}
			job["allBuilds"] = job["allBuilds"].([]any)[1:]
			if err := json.NewEncoder(w).Encode(job); err != nil {
				t.Errorf("encode Jenkins fixture: %v", err)
			}
		case "/jenkins/job/team/job/feature%2Ffoo/42/api/json", "/jenkins/job/team/job/feature%2Ffoo/43/api/json":
			if tree != "number,result,building,timestamp,duration,estimatedDuration" {
				t.Errorf("unexpected Jenkins build tree: %s", tree)
				http.Error(w, "invalid tree", http.StatusBadRequest)
				return
			}
			build := uiJenkinsBuild
			if strings.Contains(r.URL.EscapedPath(), "/43/") {
				build = strings.NewReplacer(`"number":42`, `"number":43`, `"result":"SUCCESS"`, `"result":null`, `"building":false`, `"building":true`).Replace(build)
			}
			_, _ = io.WriteString(w, build)
		default:
			t.Errorf("unexpected Jenkins endpoint: %s", r.URL.EscapedPath())
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(provider.Close)
	var plaintextCalls atomic.Int32
	plaintext := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		plaintextCalls.Add(1)
		http.Error(w, "plaintext credentials must never be sent", http.StatusBadRequest)
	}))
	t.Cleanup(plaintext.Close)
	certificate := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: provider.Certificate().Raw})
	credentials := map[string]string{"base_url": provider.URL + "/jenkins", "username": "reader", "api_token": "ui-jenkins-token", "ca_cert": strings.ReplaceAll(string(certificate), "\n", `\n`)}
	page := uiPage(t)
	uiNavigate(t, page, app.URL+"/integrations")
	card := page.Locator(`a.integration-card[href="/integrations/jenkins"]`)
	count, err := card.Count()
	require.NoError(t, err)
	require.Equal(t, 1, count, "fresh installs must register the Jenkins integration")
	uiText(t, card, "Disabled")
	uiJenkinsMCP(t, app, false)
	require.NoError(t, card.Click())
	require.Equal(t, app.URL+"/integrations/jenkins", page.URL())
	form := page.Locator("#integration-configuration")
	action, err := form.GetAttribute("action")
	require.NoError(t, err)
	require.Equal(t, "/integrations/jenkins", action)
	count, err = form.Locator(`input[id^="cred_"]`).Count()
	require.NoError(t, err)
	require.Equal(t, 4, count)
	for _, field := range []struct{ key, inputType, placeholder string }{
		{"base_url", "text", "HTTPS Jenkins"}, {"username", "text", "Overall/Read"},
		{"api_token", "password", "API token"}, {"ca_cert", "text", "PEM CA certificate"},
	} {
		input := form.Locator("#cred_" + field.key)
		inputType, err := input.GetAttribute("type")
		require.NoError(t, err)
		require.Equal(t, field.inputType, inputType)
		name, err := input.GetAttribute("name")
		require.NoError(t, err)
		require.Equal(t, "cred_"+field.key, name)
		value, err := input.InputValue()
		require.NoError(t, err)
		require.Empty(t, value)
		placeholder, err := input.GetAttribute("placeholder")
		require.NoError(t, err)
		require.Contains(t, placeholder, field.placeholder)
		label, err := form.Locator(`label[for="cred_` + field.key + `"]`).TextContent()
		require.NoError(t, err)
		wantLabel := field.key
		if field.key == "ca_cert" {
			wantLabel += " (optional)"
		}
		require.Equal(t, wantLabel, strings.TrimSpace(label))
	}
	uiText(t, page.Locator("body"), "Available Tools (3)")
	for key, value := range credentials {
		require.NoError(t, form.Locator("#cred_"+key).Fill(value))
	}
	require.NoError(t, form.Locator("#cred_api_token").Fill("invalid-token"))
	require.NoError(t, form.Locator(".toggle").Click())
	uiSave(t, page)
	uiText(t, page.Locator(".badge").First(), "Not Verified")
	require.NoError(t, form.Locator("#cred_api_token").Fill(""))
	uiSave(t, page)
	uiText(t, page.Locator(".badge").First(), "Not Verified")
	require.NoError(t, form.Locator("#cred_api_token").Fill(credentials["api_token"]))
	for _, scenario := range []struct{ name, key, value string }{
		{"reject_plaintext", "base_url", plaintext.URL + "/jenkins"},
		{"verify_server_certificate", "ca_cert", ""},
		{"reject_invalid_ca", "ca_cert", "not-a-PEM-certificate"},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			before := providerCalls.Load()
			require.NoError(t, form.Locator("#cred_"+scenario.key).Fill(scenario.value))
			uiSave(t, page)
			uiText(t, page.Locator(".badge").First(), "Not Verified")
			require.Equal(t, before, providerCalls.Load(), "invalid endpoint or untrusted TLS must not send an authenticated request")
			require.Zero(t, plaintextCalls.Load(), "API tokens must never be transmitted over HTTP")
			require.NoError(t, form.Locator("#cred_"+scenario.key).Fill(credentials[scenario.key]))
		})
	}
	uiSave(t, page)
	uiText(t, page.Locator(".badge").First(), "Connected")
	uiJenkinsMCP(t, app, true)
	uiCapture(t, page, "jenkins-desktop")
	require.NoError(t, page.SetViewportSize(390, 844))
	require.NoError(t, page.Locator("[data-sidebar-close]").Click())
	uiCapture(t, page, "jenkins-mobile")
	require.NoError(t, page.SetViewportSize(1280, 900))
	assertSaved := func() {
		for key, expected := range credentials {
			value, err := form.Locator("#cred_" + key).InputValue()
			require.NoError(t, err)
			require.Equal(t, expected, value)
		}
	}
	uiNavigate(t, page, app.URL+"/integrations/jenkins")
	uiText(t, page.Locator(".badge").First(), "Connected")
	assertSaved()
	app.restart()
	t.Logf("restarted isolated development URL: %s", app.URL)
	uiNavigate(t, page, app.URL+"/integrations/jenkins")
	uiText(t, page.Locator(".badge").First(), "Connected")
	assertSaved()
	uiJenkinsMCP(t, app, true)
	uiNavigate(t, page, app.URL+"/integrations")
	before := healthCalls.Load()
	unavailable.Store(true)
	require.NoError(t, page.GetByRole("button", playwright.PageGetByRoleOptions{Name: "Re-check all", Exact: playwright.Bool(true)}).Click())
	uiText(t, card, "Not Verified")
	require.Greater(t, healthCalls.Load(), before)
	unavailable.Store(false)
	require.NoError(t, page.GetByRole("button", playwright.PageGetByRoleOptions{Name: "Re-check all", Exact: playwright.Bool(true)}).Click())
	uiText(t, card, "Connected")
	require.NoError(t, card.Click())
	require.NoError(t, form.Locator(".toggle").Click())
	uiSave(t, page)
	uiText(t, page.Locator(".badge").First(), "Disabled")
	uiJenkinsMCP(t, app, false)
	uiNavigate(t, page, app.URL+"/integrations")
	uiText(t, card, "Disabled")
	app.restart()
	uiNavigate(t, page, app.URL+"/integrations/jenkins")
	uiText(t, page.Locator(".badge").First(), "Disabled")
	assertSaved()
	uiJenkinsMCP(t, app, false)
	data, err := os.ReadFile(filepath.Join(app.home, ".config", "switchboard", "config.json"))
	require.NoError(t, err)
	var saved mcp.Config
	require.NoError(t, json.Unmarshal(data, &saved))
	require.NotNil(t, saved.Integrations["jenkins"])
	require.False(t, saved.Integrations["jenkins"].Enabled)
	require.Equal(t, mcp.Credentials(credentials), saved.Integrations["jenkins"].Credentials)
}
