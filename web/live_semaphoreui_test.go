package web

import (
	"bytes"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/daltoniam/switchboard/integrations/semaphoreui"
	"github.com/stretchr/testify/require"
)

const liveSemaphoreUIImage = "semaphoreui/semaphore:v2.19.12@sha256:3996804607ebb63690528185bb9adc3507ee896851098e4453975f2ce7f8b435"

func init() {
	registerLiveSpec(liveSpec{
		Integration: "semaphoreui", Tier: liveContainer, Adapter: semaphoreui.New(), Provision: liveSemaphoreUIProvision,
		Cases: []liveCase{
			{Tool: "semaphoreui_get_project", Expect: []string{"Switchboard Live"}},
			{Tool: "semaphoreui_list_templates", Expect: []string{"Safe Live Task", "bash", "build", "last_task", "success", "truncated"}},
			{Tool: "semaphoreui_get_template", Args: map[string]any{"template_id": "${template_id}"}, Expect: []string{"Safe Live Task", "bash", "build", "success"}},
			{Tool: "semaphoreui_list_tasks", Args: map[string]any{"count": 1}, Expect: []string{"success", "next_before", "has_next", "pagination_known", "created", "start", "end"}},
			{Tool: "semaphoreui_list_tasks", Args: map[string]any{"count": 1, "before": "${task_id}"}, Expect: []string{"error", "next_before", "has_next", "pagination_known"}},
			{Tool: "semaphoreui_list_tasks", Args: map[string]any{"count": 2, "status": "error"}, Expect: []string{"error", "next_before", "has_next", "pagination_known"}},
			{Tool: "semaphoreui_get_task", Args: map[string]any{"task_id": "${task_id}"}, Expect: []string{"success", "created", "start", "end"}},
		},
	})
}

func liveSemaphoreUIProvision(t *testing.T) liveProvision {
	container := liveStartContainer(t, liveDockerOptions{
		Image: liveSemaphoreUIImage,
		Env: map[string]string{
			"SEMAPHORE_DB_DIALECT":     "sqlite",
			"SEMAPHORE_ADMIN":          "admin",
			"SEMAPHORE_ADMIN_PASSWORD": "disposable-live-admin-password",
			"SEMAPHORE_ADMIN_NAME":     "Admin",
			"SEMAPHORE_ADMIN_EMAIL":    "admin@localhost",
		},
		Ports: []string{"3000/tcp"},
	})
	host, port := container.Address(t, "3000/tcp")
	base := fmt.Sprintf("http://%s:%s", host, port)
	jar, err := cookiejar.New(nil)
	require.NoError(t, err)
	admin := &http.Client{Jar: jar, Timeout: 10 * time.Second}
	liveEventually(t, 60*time.Second, func() error {
		request, err := http.NewRequestWithContext(t.Context(), http.MethodGet, base+"/api/ping", nil)
		if err != nil {
			return err
		}
		response, err := admin.Do(request)
		if err != nil {
			return err
		}
		_ = response.Body.Close()
		if response.StatusCode != http.StatusOK {
			return fmt.Errorf("product not ready: HTTP %d", response.StatusCode)
		}
		return nil
	})
	call := func(client *http.Client, method, path string, body any, want int) map[string]any {
		t.Helper()
		var data []byte
		if body != nil {
			var err error
			data, err = json.Marshal(body)
			require.NoError(t, err)
		}
		request, err := http.NewRequestWithContext(t.Context(), method, base+"/api"+path, bytes.NewReader(data))
		require.NoError(t, err)
		request.Header.Set("Content-Type", "application/json")
		response, err := client.Do(request)
		require.NoError(t, err)
		defer func() { require.NoError(t, response.Body.Close()) }()
		payload, err := io.ReadAll(response.Body)
		require.NoError(t, err)
		require.Equal(t, want, response.StatusCode, "%s %s: %s", method, path, payload)
		if len(payload) == 0 || response.StatusCode != want {
			return nil
		}
		var result map[string]any
		require.NoError(t, json.Unmarshal(payload, &result))
		return result
	}
	call(admin, http.MethodPost, "/auth/login", map[string]any{"auth": "admin", "password": "disposable-live-admin-password"}, 204)
	project := call(admin, http.MethodPost, "/projects", map[string]any{"name": "Switchboard Live"}, 201)
	projectID := strconv.Itoa(int(project["id"].(float64)))
	path := "/project/" + projectID
	user := call(admin, http.MethodPost, "/users", map[string]any{"username": "reader", "name": "Live Reader", "email": "reader@localhost", "password": "disposable-live-reader-password", "admin": false}, 201)
	call(admin, http.MethodPost, path+"/users", map[string]any{"user_id": user["id"], "role": "guest"}, 204)
	container.Exec(t, "", "sh", "-c", `set -e
mkdir -p /tmp/switchboard-live-repo
cd /tmp/switchboard-live-repo
git init -q -b main
git config user.name Live
git config user.email live@localhost
printf 'printf "safe-live-success\\n"\n' > safe.sh
git add safe.sh
git commit -q -m Seed`)
	key := call(admin, http.MethodPost, path+"/keys", map[string]any{"name": "None", "type": "none", "project_id": project["id"]}, 201)
	repo := call(admin, http.MethodPost, path+"/repositories", map[string]any{"name": "Local Live Repo", "project_id": project["id"], "git_url": "/tmp/switchboard-live-repo", "git_branch": "main", "ssh_key_id": key["id"]}, 201)
	template := call(admin, http.MethodPost, path+"/templates", map[string]any{"name": "Safe Live Task", "project_id": project["id"], "repository_id": repo["id"], "app": "bash", "playbook": "safe.sh", "type": "build", "environment_ids": []int{}, "description": "SECRET_SENTINEL"}, 201)
	templateID := strconv.Itoa(int(template["id"].(float64)))
	taskID := ""
	for index := 0; index < 3; index++ {
		command := "exit 0"
		expectedStatus := "success"
		if index == 1 {
			command, expectedStatus = "exit 1", "error"
		}
		container.Exec(t, "", "sh", "-c", "cd /tmp/switchboard-live-repo; printf '%s\\n' '"+command+"' > safe.sh; git add safe.sh; git commit -q -m TaskSeed")
		task := call(admin, http.MethodPost, path+"/tasks", map[string]any{"template_id": template["id"], "message": "SECRET_SENTINEL"}, 201)
		taskID = strconv.Itoa(int(task["id"].(float64)))
		liveEventually(t, 90*time.Second, func() error {
			state := call(admin, http.MethodGet, path+"/tasks/"+taskID, nil, 200)
			if state["status"] != expectedStatus {
				return fmt.Errorf("waiting for seeded task completion: %v", state["status"])
			}
			return nil
		})
	}
	readerJar, err := cookiejar.New(nil)
	require.NoError(t, err)
	reader := &http.Client{Jar: readerJar, Timeout: 10 * time.Second}
	call(reader, http.MethodPost, "/auth/login", map[string]any{"auth": "reader", "password": "disposable-live-reader-password"}, 204)
	token := call(reader, http.MethodPost, "/user/tokens", map[string]any{"name": "switchboard-live"}, 201)["id"].(string)
	target, err := url.Parse(base)
	require.NoError(t, err)
	proxy := httptest.NewTLSServer(httputil.NewSingleHostReverseProxy(target))
	t.Cleanup(proxy.Close)
	cert := string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: proxy.Certificate().Raw}))
	credentials := map[string]string{"base_url": proxy.URL, "api_token": token, "project_id": projectID, "ca_cert": strings.ReplaceAll(cert, "\n", `\n`)}
	adapter := semaphoreui.New()
	t.Cleanup(func() { require.NoError(t, adapter.(io.Closer).Close()) })
	require.NoError(t, adapter.Configure(t.Context(), credentials))
	require.True(t, adapter.Healthy(t.Context()))
	empty, err := adapter.Execute(t.Context(), "semaphoreui_list_tasks", map[string]any{"count": 1, "status": "error"})
	require.NoError(t, err)
	require.False(t, empty.IsError, empty.Data)
	var emptyPage struct {
		Tasks      []any `json:"tasks"`
		HasNext    bool  `json:"has_next"`
		NextBefore int   `json:"next_before"`
	}
	require.NoError(t, json.Unmarshal([]byte(empty.Data), &emptyPage))
	require.Empty(t, emptyPage.Tasks)
	require.True(t, emptyPage.HasNext)
	require.Equal(t, taskID, strconv.Itoa(emptyPage.NextBefore))
	for _, check := range []struct {
		method, path string
		body         any
	}{
		{http.MethodPost, path + "/tasks", map[string]any{"template_id": template["id"]}},
		{http.MethodPut, path + "/templates/" + templateID, template},
		{http.MethodPost, path + "/tasks/" + taskID + "/stop", map[string]any{}},
	} {
		payload, err := json.Marshal(check.body)
		require.NoError(t, err)
		request, err := http.NewRequestWithContext(t.Context(), check.method, base+"/api"+check.path, bytes.NewReader(payload))
		require.NoError(t, err)
		request.Header.Set("Content-Type", "application/json")
		request.Header.Set("Authorization", "Bearer "+token)
		response, err := (&http.Client{Timeout: 10 * time.Second}).Do(request)
		require.NoError(t, err)
		require.NoError(t, response.Body.Close())
		require.Equal(t, http.StatusForbidden, response.StatusCode, "guest mutation must be denied: %s", check.path)
	}
	for _, tool := range adapter.Tools() {
		args := map[string]any{}
		if tool.Name == "semaphoreui_get_task" {
			args["task_id"] = taskID
		}
		if tool.Name == "semaphoreui_get_template" {
			args["template_id"] = templateID
		}
		result, err := adapter.Execute(t.Context(), tool.Name, args)
		require.NoError(t, err)
		require.False(t, result.IsError, result.Data)
		require.NotContains(t, result.Data, "SECRET_SENTINEL")
	}
	return liveProvision{Credentials: credentials, Values: map[string]string{"template_id": templateID, "task_id": taskID}}
}
