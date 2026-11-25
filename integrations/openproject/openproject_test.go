package openproject

import (
	"compress/gzip"
	"context"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"math"
	"net"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"
	"unicode/utf8"

	mcp "github.com/daltoniam/switchboard"
)

const workPackageFixture = `{"_type":"WorkPackage","id":42,"displayId":"PROJ-42","subject":"Fix search bug","updatedAt":"2026-10-16T12:00:00Z","createdAt":"2026-10-15T12:00:00Z","startDate":"2026-10-16","dueDate":"2026-10-20","description":{"format":"markdown","raw":"Untrusted **task** text","html":"<p>PRIVATE-HTML</p>"},"customField1":"PRIVATE-CUSTOM","email":"PRIVATE-EMAIL","_embedded":{"author":{"name":"PRIVATE-USER","email":"PRIVATE-EMAIL"},"attachments":[{"name":"PRIVATE-FILE"}]},"_links":{"project":{"href":"/openproject/api/v3/projects/7","title":"PRIVATE-PROJECT"},"status":{"href":"https://evil.invalid/PRIVATE-STATUS","title":"New"},"type":{"href":"https://evil.invalid/PRIVATE-TYPE","title":"Task"},"author":{"href":"/api/v3/users/1","title":"PRIVATE-USER"},"update":{"href":"https://evil.invalid/PRIVATE-ACTION","method":"patch"},"delete":{"href":"https://evil.invalid/PRIVATE-DELETE","method":"delete"}}}`

func credentials(base string) mcp.Credentials {
	return mcp.Credentials{"base_url": base, "api_token": "secret-token", "workspace_id": "7"}
}

func fixture(t *testing.T, handler http.HandlerFunc) (mcp.Integration, *httptest.Server) {
	t.Helper()
	server := httptest.NewTLSServer(handler)
	t.Cleanup(server.Close)
	creds := credentials(server.URL + "/openproject")
	creds["ca_cert"] = string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: server.Certificate().Raw}))
	integration := New()
	if err := integration.Configure(context.Background(), creds); err != nil {
		t.Fatal(err)
	}
	if closer, ok := integration.(io.Closer); ok {
		t.Cleanup(func() { _ = closer.Close() })
	}
	return integration, server
}

func execute(t *testing.T, integration mcp.Integration, name mcp.ToolName, args map[string]any) *mcp.ToolResult {
	t.Helper()
	result, err := integration.Execute(context.Background(), name, args)
	if err != nil || result == nil {
		t.Fatalf("Execute returned result=%v err=%v", result, err)
	}
	return result
}

func successJSON(t *testing.T, result *mcp.ToolResult) map[string]any {
	t.Helper()
	if result.IsError {
		t.Fatalf("unexpected tool error: %s", result.Data)
	}
	var out map[string]any
	if err := json.Unmarshal([]byte(result.Data), &out); err != nil {
		t.Fatal(err)
	}
	return out
}

func collectionFixture(total, count, page, size int, elements ...string) string {
	return fmt.Sprintf(`{"_type":"WorkPackageCollection","total":%d,"count":%d,"offset":%d,"pageSize":%d,"_embedded":{"elements":[%s]},"_links":{"nextByOffset":{"href":"https://evil.invalid/PRIVATE-NEXT"},"createWorkPackage":{"href":"https://evil.invalid/PRIVATE-CREATE","method":"post"}},"customField1":"PRIVATE-CUSTOM"}`, total, count, page, size, strings.Join(elements, ","))
}

func summaryJSON() map[string]any {
	return map[string]any{"id": float64(42), "displayId": "PROJ-42", "subject": "Fix search bug", "status": "New", "type": "Task", "project_id": float64(7), "updatedAt": "2026-10-16T12:00:00Z"}
}

func detailJSON() map[string]any {
	out := summaryJSON()
	out["createdAt"] = "2026-10-15T12:00:00Z"
	out["startDate"] = "2026-10-16"
	out["dueDate"] = "2026-10-20"
	out["description"] = "Untrusted **task** text"
	out["description_truncated"] = false
	out["truncated"] = false
	return out
}

func listJSON(total, count, page, size, returned, omitted int, next any, rows ...any) map[string]any {
	if rows == nil {
		rows = []any{}
	}
	return map[string]any{"work_packages": rows, "total": float64(total), "count": float64(count), "page": float64(page), "page_size": float64(size), "returned": float64(returned), "omitted": float64(omitted), "truncated": false, "next_page": next}
}

func assertSafe(t *testing.T, data string) {
	t.Helper()
	for _, forbidden := range []string{"PRIVATE", "secret-token", "evil.invalid", "_embedded", "_links", "customField", "description.html", "Authorization", base64.StdEncoding.EncodeToString([]byte("apikey:secret-token"))} {
		if strings.Contains(data, forbidden) {
			t.Fatalf("raw result or error leaked %q: %.300s", forbidden, data)
		}
	}
}

func TestRenderMarkdown_ToolsCovered(t *testing.T) {
	renderer, ok := New().(mcp.MarkdownIntegration)
	if !ok {
		t.Fatal("description content requires the public MarkdownIntegration interface")
	}
	data, err := json.Marshal(detailJSON())
	if err != nil {
		t.Fatal(err)
	}
	for _, tool := range New().Tools() {
		rendered, supported := renderer.RenderMarkdown(tool.Name, data)
		want := tool.Name == "openproject_get_work_package"
		if supported != want || (supported && rendered == "") {
			t.Fatalf("markdown support for %s = %v, want %v", tool.Name, supported, want)
		}
	}
	if _, supported := renderer.RenderMarkdown("openproject_unknown", data); supported {
		t.Fatal("unknown tool must not render markdown")
	}
}

func TestRenderMarkdown_SafeDetail(t *testing.T) {
	renderer, ok := New().(mcp.MarkdownIntegration)
	if !ok {
		t.Fatal("description content requires the public MarkdownIntegration interface")
	}
	out := detailJSON()
	out["PRIVATE-unlisted"] = "PRIVATE-CUSTOM"
	out["_links"] = map[string]any{"update": map[string]any{"href": "https://evil.invalid/PRIVATE-ACTION"}}
	data, err := json.Marshal(out)
	if err != nil {
		t.Fatal(err)
	}
	rendered, supported := renderer.RenderMarkdown("openproject_get_work_package", data)
	if !supported {
		t.Fatal("exact detail should render markdown")
	}
	for _, expected := range []string{"Fix search bug", "PROJ-42", "New", "Task", "Untrusted **task** text"} {
		if !strings.Contains(string(rendered), expected) {
			t.Fatalf("rendered detail omitted %q: %s", expected, rendered)
		}
	}
	assertSafe(t, string(rendered))
}

func TestRenderMarkdown_MalformedFallsThrough(t *testing.T) {
	renderer, ok := New().(mcp.MarkdownIntegration)
	if !ok {
		t.Fatal("description content requires the public MarkdownIntegration interface")
	}
	for _, data := range []string{`{`, `null`, `{}`, `{"id":42}`, `{"id":0,"project_id":7}`, `{"id":42,"project_id":0}`, `{"id":"PRIVATE","project_id":7}`} {
		t.Run(data, func(t *testing.T) {
			rendered, supported := renderer.RenderMarkdown("openproject_get_work_package", []byte(data))
			if supported || rendered != "" {
				t.Fatal("malformed detail should fall through without rendered content")
			}
		})
	}
}

func TestRenderMarkdown_UntrustedContentCannotBreakFence(t *testing.T) {
	renderer, ok := New().(mcp.MarkdownIntegration)
	if !ok {
		t.Fatal("description content requires the public MarkdownIntegration interface")
	}
	for _, ticks := range []int{3, 4, 12} {
		t.Run(strconv.Itoa(ticks), func(t *testing.T) {
			out := detailJSON()
			payload := "before\n" + strings.Repeat("`", ticks) + "\n![tracking](https://tracking.invalid/image)\n<script>unsafe()</script>\nafter"
			out["description"] = payload
			out["subject"] = "![subject](https://tracking.invalid/subject)"
			data, err := json.Marshal(out)
			if err != nil {
				t.Fatal(err)
			}
			rendered, supported := renderer.RenderMarkdown("openproject_get_work_package", data)
			if !supported || !strings.Contains(string(rendered), payload) {
				t.Fatal("untrusted description should remain literal text")
			}
			lines := strings.Split(string(rendered), "\n")
			inside, opened, closed := false, false, false
			fence := ""
			for _, line := range lines {
				trimmed := strings.TrimSpace(line)
				if !opened && strings.HasPrefix(trimmed, "```") {
					run := len(trimmed) - len(strings.TrimLeft(trimmed, "`"))
					if run <= ticks {
						t.Fatal("markdown fence does not dominate untrusted backtick runs")
					}
					fence, inside, opened = strings.Repeat("`", run), true, true
					continue
				}
				if inside && trimmed == fence {
					inside, closed = false, true
					continue
				}
				if (strings.Contains(line, "tracking.invalid") || strings.Contains(line, "<script>")) && !inside {
					t.Fatal("untrusted markdown/HTML escaped the literal text fence")
				}
			}
			if !opened || !closed || inside {
				t.Fatal("rendered description needs balanced literal text fencing")
			}
		})
	}
}

func TestNewAndMetadata(t *testing.T) {
	integration := New()
	if integration.Name() != "openproject" || integration.Healthy(context.Background()) {
		t.Fatal("invalid constructor or unconfigured health")
	}
	tools := integration.Tools()
	if len(tools) != 2 {
		t.Fatalf("expected exactly two read tools, got %d", len(tools))
	}
	seen := map[mcp.ToolName]bool{}
	for _, tool := range tools {
		if seen[tool.Name] || !strings.HasPrefix(string(tool.Name), "openproject_") || tool.Description == "" || tool.Access != mcp.AccessRead {
			t.Fatalf("invalid tool metadata: %+v", tool)
		}
		seen[tool.Name] = true
		var keys []string
		for key := range tool.Parameters {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		switch tool.Name {
		case "openproject_list_work_packages":
			if !strings.Contains(tool.Description, "Start here") || !reflect.DeepEqual(keys, []string{"limit", "page", "subject_or_id"}) || len(tool.Required) != 0 {
				t.Fatalf("invalid list guidance or parameters: %+v", tool)
			}
		case "openproject_get_work_package":
			if !reflect.DeepEqual(keys, []string{"id"}) || !reflect.DeepEqual(tool.Required, []string{"id"}) {
				t.Fatalf("invalid exact-get parameters: %+v", tool)
			}
		default:
			t.Fatalf("unexpected tool %s", tool.Name)
		}
	}
	if !seen["openproject_list_work_packages"] || !seen["openproject_get_work_package"] {
		t.Fatal("unexpected read-tool catalog")
	}
}

func TestCredentialMetadata(t *testing.T) {
	integration := New()
	plain, ok := integration.(mcp.PlainTextCredentials)
	if !ok || !reflect.DeepEqual(plain.PlainTextKeys(), []string{"base_url", "workspace_id", "ca_cert"}) {
		t.Fatal("installation/workspace/CA should be editable plain text, API token must stay secret")
	}
	optional, ok := integration.(mcp.OptionalCredentials)
	if !ok || !reflect.DeepEqual(optional.OptionalKeys(), []string{"ca_cert"}) {
		t.Fatal("only custom CA is optional")
	}
	hints, ok := integration.(mcp.PlaceholderHints)
	if !ok {
		t.Fatal("configuration requires actionable credential hints")
	}
	for _, tc := range []struct{ key, phrase string }{
		{"base_url", "HTTPS"},
		{"workspace_id", "numeric"},
		{"workspace_id", "View work packages"},
		{"api_token", "API token"},
		{"api_token", "apikey"},
		{"ca_cert", "PEM CA certificate"},
	} {
		t.Run(tc.key+"_"+tc.phrase, func(t *testing.T) {
			if !strings.Contains(hints.Placeholders()[tc.key], tc.phrase) {
				t.Fatalf("%s placeholder needs %q", tc.key, tc.phrase)
			}
		})
	}
}

func TestDispatchMap_AllToolsCovered(t *testing.T) {
	for _, tool := range New().Tools() {
		if dispatch[tool.Name] == nil {
			t.Fatalf("tool %s has no handler", tool.Name)
		}
	}
}

func TestDispatchMap_NoOrphanHandlers(t *testing.T) {
	known := map[mcp.ToolName]bool{}
	for _, tool := range New().Tools() {
		known[tool.Name] = true
	}
	for name := range dispatch {
		if !known[name] {
			t.Fatalf("orphan handler %s", name)
		}
	}
}

func TestExecuteUnconfiguredAndUnknown(t *testing.T) {
	cases := []struct {
		name mcp.ToolName
		args map[string]any
	}{
		{"openproject_list_work_packages", nil},
		{"openproject_get_work_package", map[string]any{"id": 42}},
		{"openproject_unknown", nil},
	}
	for _, tc := range cases {
		t.Run(string(tc.name), func(t *testing.T) {
			result := execute(t, New(), tc.name, tc.args)
			if !result.IsError {
				t.Fatal("unconfigured or unknown tool succeeded")
			}
			if tc.name == "openproject_unknown" && !strings.Contains(result.Data, "unknown tool") {
				t.Fatal("unknown tool error should identify the problem")
			}
			assertSafe(t, result.Data)
		})
	}
}

func TestConfigureValidation(t *testing.T) {
	cases := []struct{ key, value string }{
		{"base_url", ""}, {"api_token", ""}, {"workspace_id", ""},
		{"base_url", "http://localhost:8080/openproject"},
		{"base_url", "https://user:PRIVATE@example.com"},
		{"base_url", "https://example.com/?PRIVATE=value"},
		{"base_url", "https://example.com/#PRIVATE"},
		{"base_url", "https://example.com/?"},
		{"base_url", "https://example.com/#"},
		{"base_url", "https:///openproject"},
		{"base_url", "https:example.com"},
		{"base_url", " https://example.com"},
		{"base_url", "https://example.com/a b"},
		{"base_url", "https://example.com/a/../b"},
		{"base_url", "https://example.com/./b"},
		{"base_url", "https://example.com/a//b"},
		{"base_url", "https://example.com/%2e%2e/b"},
		{"base_url", "https://example.com/%2fetc"},
		{"base_url", "https://example.com/a\\b"},
		{"base_url", "https://example.com:0"},
		{"base_url", "https://example.com:65536"},
		{"base_url", "https://example.com:bad"},
		{"base_url", "https://example.com:"},
		{"base_url", "https://[example.com]"},
		{"base_url", "https://-example.com"},
		{"base_url", "https://example-.com"},
		{"base_url", "https://exam_ple.com"},
		{"base_url", "https://example..com"},
		{"base_url", "https://" + strings.Repeat("x", 64) + ".com"},
		{"base_url", "https://" + strings.Repeat("x.", 128) + "com"},
		{"base_url", "https://example.com/" + strings.Repeat("x", 2048)},
		{"base_url", "https://example.com/\xff"},
		{"api_token", " secret-token"}, {"api_token", "secret-token\nPRIVATE"},
		{"api_token", "secret-token\x00PRIVATE"}, {"api_token", "secret-token\tPRIVATE"},
		{"api_token", strings.Repeat("x", 4097)}, {"api_token", "secret-token\xff"},
		{"workspace_id", "0"}, {"workspace_id", "-7"}, {"workspace_id", "7.5"},
		{"workspace_id", "project-seven"}, {"workspace_id", "7/../8"},
		{"workspace_id", "7?PRIVATE=x"}, {"workspace_id", "999999999999999999999999"},
		{"workspace_id", "9223372036854775808"}, {"workspace_id", "07"},
		{"ca_cert", "PRIVATE not a PEM certificate"},
		{"ca_cert", strings.Repeat("x", 128*1024+1)},
		{"ca_cert", "   "},
		{"ca_cert", "-----BEGIN CERTIFICATE-----\nAAAA\n-----END CERTIFICATE-----"},
		{"ca_cert", "-----BEGIN CERTIFICATE-----\nPRIVATE"},
	}
	for index, tc := range cases {
		t.Run(fmt.Sprintf("%d_%s", index, tc.key), func(t *testing.T) {
			creds := credentials("https://openproject.example.com/openproject")
			creds[tc.key] = tc.value
			err := New().Configure(context.Background(), creds)
			if err == nil {
				t.Fatal("invalid credentials accepted")
			}
			assertSafe(t, err.Error())
		})
	}
	for _, base := range []string{"https://openproject.example.com", "https://openproject.example.com/openproject/", "https://[::1]:8443/openproject"} {
		t.Run(base, func(t *testing.T) {
			integration := New()
			if err := integration.Configure(context.Background(), credentials(base)); err != nil {
				t.Fatal(err)
			}
			if closer, ok := integration.(io.Closer); ok {
				_ = closer.Close()
			}
		})
	}
}

func TestConfigureFailureClearsPreviousState(t *testing.T) {
	var calls atomic.Int32
	integration, _ := fixture(t, func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.Header().Set("Content-Type", "application/hal+json")
		_, _ = io.WriteString(w, collectionFixture(1, 1, 1, 20, workPackageFixture))
	})
	_ = successJSON(t, execute(t, integration, "openproject_list_work_packages", nil))
	if err := integration.Configure(context.Background(), credentials("http://invalid.example.com")); err == nil {
		t.Fatal("invalid reconfiguration accepted")
	}
	if integration.Healthy(context.Background()) {
		t.Fatal("failed reconfiguration retained healthy credentials")
	}
	if result := execute(t, integration, "openproject_list_work_packages", nil); !result.IsError {
		t.Fatal("failed reconfiguration retained executable credentials")
	}
	if calls.Load() != 1 {
		t.Fatal("old configured server contacted after failed reconfiguration")
	}
}

func TestRequestAndProjection(t *testing.T) {
	search := `42 + café & "quoted" / ? #`
	searchFilters, err := json.Marshal([]any{map[string]any{"subjectOrId": map[string]any{"operator": "**", "values": []string{search}}}})
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name  mcp.ToolName
		args  map[string]any
		path  string
		query map[string][]string
		body  string
		want  map[string]any
	}{
		{"openproject_list_work_packages", nil, "/openproject/api/v3/workspaces/7/work_packages", map[string][]string{"offset": {"1"}, "pageSize": {"20"}, "filters": {"[]"}, "sortBy": {`[["id","asc"]]`}}, collectionFixture(1, 1, 1, 20, workPackageFixture), listJSON(1, 1, 1, 20, 1, 0, nil, summaryJSON())},
		{"openproject_list_work_packages", map[string]any{"subject_or_id": search, "page": 2, "limit": 1}, "/openproject/api/v3/workspaces/7/work_packages", map[string][]string{"offset": {"2"}, "pageSize": {"1"}, "filters": {string(searchFilters)}, "sortBy": {`[["id","asc"]]`}}, collectionFixture(3, 1, 2, 1, workPackageFixture), listJSON(3, 1, 2, 1, 1, 0, float64(3), summaryJSON())},
		{"openproject_get_work_package", map[string]any{"id": json.Number("42")}, "/openproject/api/v3/work_packages/42", map[string][]string{}, workPackageFixture, detailJSON()},
	}
	for index, tc := range cases {
		t.Run(fmt.Sprintf("%d_%s", index, tc.name), func(t *testing.T) {
			var targetCalls, targetConnections atomic.Int32
			target := httptest.NewUnstartedServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
				targetCalls.Add(1)
			}))
			target.Config.ConnState = func(_ net.Conn, state http.ConnState) {
				if state == http.StateNew {
					targetConnections.Add(1)
				}
			}
			target.StartTLS()
			t.Cleanup(target.Close)
			body := strings.ReplaceAll(tc.body, "https://evil.invalid", target.URL)
			var calls atomic.Int32
			integration, _ := fixture(t, func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				user, token, ok := r.BasicAuth()
				if r.Method != http.MethodGet || !ok || user != "apikey" || token != "secret-token" || r.Header.Get("Accept") != "application/hal+json" {
					t.Error("expected preemptive Basic apikey/token authentication and HAL GET")
				}
				if r.URL.EscapedPath() != tc.path || !reflect.DeepEqual(map[string][]string(r.URL.Query()), tc.query) {
					t.Errorf("unexpected fixed request: %s", r.URL)
				}
				w.Header().Set("Content-Type", "application/hal+json;charset=UTF-8")
				_, _ = io.WriteString(w, body)
			})
			result := execute(t, integration, tc.name, tc.args)
			if got := successJSON(t, result); !reflect.DeepEqual(got, tc.want) {
				t.Errorf("projection=%#v want %#v", got, tc.want)
			}
			assertSafe(t, result.Data)
			if calls.Load() != 1 {
				t.Fatal("one tool call must fetch only one fixed upstream resource")
			}
			if received, connected := targetCalls.Load(), targetConnections.Load(); received != 0 || connected != 0 {
				t.Fatalf("untrusted HAL-link target received %d requests and %d connections", received, connected)
			}
		})
	}
}

func TestListPaginationUsesUpstreamPageSize(t *testing.T) {
	cases := []struct {
		name                     string
		total, count, page, size int
		next                     any
	}{
		{"empty", 0, 0, 1, 20, nil},
		{"smaller installation maximum", 7, 2, 1, 2, float64(2)},
		{"one based page not record offset", 7, 2, 2, 2, float64(3)},
		{"last page", 3, 1, 2, 2, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var rows []string
			var summaries []any
			for range tc.count {
				rows = append(rows, workPackageFixture)
				summaries = append(summaries, summaryJSON())
			}
			var calls atomic.Int32
			integration, _ := fixture(t, func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				if r.URL.Query().Get("offset") != strconv.Itoa(tc.page) || r.URL.Query().Get("pageSize") != "20" {
					t.Error("request changed page semantics")
				}
				w.Header().Set("Content-Type", "application/hal+json")
				_, _ = io.WriteString(w, collectionFixture(tc.total, tc.count, tc.page, tc.size, rows...))
			})
			result := execute(t, integration, "openproject_list_work_packages", map[string]any{"page": tc.page})
			want := listJSON(tc.total, tc.count, tc.page, tc.size, tc.count, 0, tc.next, summaries...)
			if got := successJSON(t, result); !reflect.DeepEqual(got, want) {
				t.Fatalf("paging=%#v want %#v", got, want)
			}
			if calls.Load() != 1 {
				t.Fatal("HAL continuation was followed")
			}
			assertSafe(t, result.Data)
		})
	}
}

func TestInvalidCollectionPagingRejected(t *testing.T) {
	cases := []struct {
		name                     string
		total, count, page, size int
	}{
		{"negative total", -1, 1, 1, 20},
		{"negative count", 1, -1, 1, 20},
		{"count differs from elements", 2, 2, 1, 20},
		{"zero page size", 1, 1, 1, 0},
		{"larger than requested page", 1, 1, 1, 21},
		{"wrong upstream page", 1, 1, 2, 20},
		{"count larger than total", 0, 1, 1, 20},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			integration, _ := fixture(t, func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/hal+json")
				_, _ = io.WriteString(w, collectionFixture(tc.total, tc.count, tc.page, tc.size, workPackageFixture))
			})
			result := execute(t, integration, "openproject_list_work_packages", nil)
			if !result.IsError {
				t.Fatal("inconsistent upstream pagination accepted")
			}
			assertSafe(t, result.Data)
		})
	}
}

func TestWorkspaceReferenceIsolation(t *testing.T) {
	cases := []struct {
		name, project string
		allowed       bool
	}{
		{"released project route", `{"href":"/openproject/api/v3/projects/7"}`, true},
		{"missing installation prefix", `{"href":"/api/v3/projects/7"}`, false},
		{"workspace route", `{"href":"/api/v3/workspaces/7"}`, false},
		{"subpath workspace route", `{"href":"/openproject/api/v3/workspaces/7"}`, false},
		{"other project", `{"href":"/openproject/api/v3/projects/8"}`, false},
		{"descendant workspace", `{"href":"/openproject/api/v3/workspaces/8"}`, false},
		{"portfolio kind", `{"href":"/openproject/api/v3/portfolio/7"}`, false},
		{"missing href", `{}`, false},
		{"null project", `null`, false},
		{"absolute attacker origin", `{"href":"https://evil.invalid/openproject/api/v3/projects/7"}`, false},
		{"protocol relative origin", `{"href":"//evil.invalid/openproject/api/v3/projects/7"}`, false},
		{"matching prefix not id", `{"href":"/openproject/api/v3/projects/70"}`, false},
		{"extra path", `{"href":"/openproject/api/v3/projects/7/work_packages"}`, false},
		{"query suffix", `{"href":"/openproject/api/v3/projects/7?PRIVATE=x"}`, false},
		{"fragment suffix", `{"href":"/openproject/api/v3/projects/7#PRIVATE"}`, false},
		{"traversal", `{"href":"/openproject/api/v3/projects/8/../7"}`, false},
		{"encoded path", `{"href":"/openproject/api/v3/projects/%37"}`, false},
		{"foreign installation", `{"href":"/other/openproject/api/v3/projects/7"}`, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			body := strings.Replace(workPackageFixture, `{"href":"/openproject/api/v3/projects/7","title":"PRIVATE-PROJECT"}`, tc.project, 1)
			var calls atomic.Int32
			integration, _ := fixture(t, func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				w.Header().Set("Content-Type", "application/hal+json")
				if strings.Contains(r.URL.Path, "/workspaces/") {
					_, _ = io.WriteString(w, collectionFixture(5, 1, 1, 1, body))
					return
				}
				_, _ = io.WriteString(w, body)
			})
			listResult := execute(t, integration, "openproject_list_work_packages", map[string]any{"limit": 1})
			listed := successJSON(t, listResult)
			want := listJSON(5, 1, 1, 1, 0, 1, float64(2))
			if tc.allowed {
				want = listJSON(5, 1, 1, 1, 1, 0, float64(2), summaryJSON())
			}
			if !reflect.DeepEqual(listed, want) {
				t.Errorf("workspace-filtered list=%#v want %#v", listed, want)
			}
			getResult := execute(t, integration, "openproject_get_work_package", map[string]any{"id": 42})
			if getResult.IsError == tc.allowed {
				t.Fatalf("exact get workspace decision=%+v allowed=%v", getResult, tc.allowed)
			}
			assertSafe(t, listResult.Data)
			assertSafe(t, getResult.Data)
			if calls.Load() != 2 {
				t.Fatal("project or action href was followed")
			}
		})
	}
}

func TestListCountsMixedWorkspaceOmissions(t *testing.T) {
	other := strings.Replace(workPackageFixture, "/openproject/api/v3/projects/7", "/openproject/api/v3/projects/8", 1)
	missing := strings.Replace(workPackageFixture, `"project":{"href":"/openproject/api/v3/projects/7","title":"PRIVATE-PROJECT"},`, "", 1)
	integration, _ := fixture(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/hal+json")
		_, _ = io.WriteString(w, collectionFixture(20, 3, 2, 3, workPackageFixture, other, missing))
	})
	result := execute(t, integration, "openproject_list_work_packages", map[string]any{"page": 2, "limit": 3})
	want := listJSON(20, 3, 2, 3, 1, 2, float64(3), summaryJSON())
	if got := successJSON(t, result); !reflect.DeepEqual(got, want) {
		t.Fatalf("local omissions must not rewrite upstream paging: %#v", got)
	}
	assertSafe(t, result.Data)
}

func TestArgumentsRejectedBeforeNetwork(t *testing.T) {
	cases := []struct {
		name string
		tool mcp.ToolName
		args map[string]any
	}{
		{"zero page", "openproject_list_work_packages", map[string]any{"page": 0}},
		{"negative page", "openproject_list_work_packages", map[string]any{"page": -1}},
		{"fractional page", "openproject_list_work_packages", map[string]any{"page": 1.5}},
		{"bad page type", "openproject_list_work_packages", map[string]any{"page": []int{1}}},
		{"zero limit", "openproject_list_work_packages", map[string]any{"limit": 0}},
		{"excess limit", "openproject_list_work_packages", map[string]any{"limit": 51}},
		{"negative limit", "openproject_list_work_packages", map[string]any{"limit": -1}},
		{"fractional limit", "openproject_list_work_packages", map[string]any{"limit": 1.5}},
		{"nan limit", "openproject_list_work_packages", map[string]any{"limit": math.NaN()}},
		{"infinite limit", "openproject_list_work_packages", map[string]any{"limit": math.Inf(1)}},
		{"long search", "openproject_list_work_packages", map[string]any{"subject_or_id": strings.Repeat("x", 257)}},
		{"object search", "openproject_list_work_packages", map[string]any{"subject_or_id": map[string]any{"PRIVATE": "x"}}},
		{"caller workspace", "openproject_list_work_packages", map[string]any{"workspace_id": 8}},
		{"caller filters", "openproject_list_work_packages", map[string]any{"filters": []any{}}},
		{"caller sort", "openproject_list_work_packages", map[string]any{"sortBy": "PRIVATE"}},
		{"caller select", "openproject_list_work_packages", map[string]any{"select": "PRIVATE"}},
		{"caller base URL", "openproject_list_work_packages", map[string]any{"base_url": "https://evil.invalid/PRIVATE"}},
		{"caller headers", "openproject_list_work_packages", map[string]any{"headers": map[string]any{"Authorization": "PRIVATE"}}},
		{"caller path", "openproject_list_work_packages", map[string]any{"path": "/PRIVATE"}},
		{"unknown tool", "openproject_delete_work_package", map[string]any{"id": 42}},
		{"missing id", "openproject_get_work_package", nil},
		{"zero id", "openproject_get_work_package", map[string]any{"id": 0}},
		{"negative id", "openproject_get_work_package", map[string]any{"id": -1}},
		{"fractional id", "openproject_get_work_package", map[string]any{"id": 42.5}},
		{"semantic id", "openproject_get_work_package", map[string]any{"id": "PROJ-42"}},
		{"id traversal", "openproject_get_work_package", map[string]any{"id": "42/../43"}},
		{"id query", "openproject_get_work_package", map[string]any{"id": "42?PRIVATE=value"}},
		{"id URL", "openproject_get_work_package", map[string]any{"id": "https://evil.invalid/PRIVATE"}},
		{"id overflow", "openproject_get_work_package", map[string]any{"id": json.Number("999999999999999999999999")}},
		{"get extra argument", "openproject_get_work_package", map[string]any{"id": 42, "workspace_id": 7}},
	}
	var calls atomic.Int32
	integration, _ := fixture(t, func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusInternalServerError)
	})
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			result := execute(t, integration, tc.tool, tc.args)
			if !result.IsError {
				t.Fatal("invalid or injection argument accepted")
			}
			assertSafe(t, result.Data)
		})
	}
	if calls.Load() != 0 {
		t.Fatal("invalid arguments reached upstream")
	}
}

func TestHTTPFailuresAreSafe(t *testing.T) {
	cases := []struct {
		name              string
		status            int
		contentType, body string
	}{
		{"unauthorized", 401, "application/hal+json", `{"message":"PRIVATE secret-token"}`},
		{"forbidden", 403, "text/html", "<p>PRIVATE secret-token</p>"},
		{"permission-hidden 404", 404, "application/hal+json", `{"message":"PRIVATE secret-token"}`},
		{"server failure", 500, "text/plain", "PRIVATE secret-token"},
		{"HTML login", 200, "text/html", "<html>PRIVATE secret-token</html>"},
		{"JSON mislabeled login", 200, "application/hal+json", "<html>PRIVATE secret-token</html>"},
		{"empty", 204, "application/hal+json", ""},
		{"malformed", 200, "application/hal+json", `{"_embedded":`},
		{"missing collection", 200, "application/hal+json", `{}`},
		{"null collection", 200, "application/hal+json", `null`},
		{"wrong elements type", 200, "application/hal+json", `{"total":1,"count":1,"offset":1,"pageSize":20,"_embedded":{"elements":"PRIVATE"}}`},
		{"trailing JSON", 200, "application/hal+json", collectionFixture(0, 0, 1, 20) + `{}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var calls atomic.Int32
			integration, _ := fixture(t, func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				w.Header().Set("Content-Type", tc.contentType)
				w.WriteHeader(tc.status)
				_, _ = io.WriteString(w, tc.body)
				if tc.status >= 400 {
					_, _ = io.WriteString(w, strings.TrimPrefix(r.Header.Get("Authorization"), "Basic "))
				}
			})
			result := execute(t, integration, "openproject_list_work_packages", nil)
			if !result.IsError {
				t.Fatalf("invalid upstream response accepted: %.200s", result.Data)
			}
			assertSafe(t, result.Data)
			lower := strings.ToLower(result.Data)
			switch tc.status {
			case http.StatusUnauthorized:
				if !strings.Contains(lower, "token") && !strings.Contains(lower, "credential") {
					t.Fatal("401 should explain credential/API-token action")
				}
			case http.StatusForbidden:
				if !strings.Contains(lower, "permission") && !strings.Contains(lower, "view work packages") {
					t.Fatal("403 should explain required permission")
				}
			case http.StatusNotFound:
				if !strings.Contains(lower, "visib") && !strings.Contains(lower, "permission") {
					t.Fatal("404 must explain permission-hidden visibility ambiguity")
				}
			}
			if tc.status >= 401 && tc.status <= 404 && calls.Load() != 1 {
				t.Fatal("authentication/authorization failures must not retry")
			}
		})
	}
}

func TestHTTPResponseSizeLimit(t *testing.T) {
	const limit = 2 * 1024 * 1024
	prefix := `{"PRIVATE":"`
	suffix := `",` + strings.TrimPrefix(collectionFixture(0, 0, 1, 20), "{")
	cases := []struct {
		name      string
		size      int
		wantError bool
	}{
		{"below limit", limit - 1, false},
		{"at limit", limit, false},
		{"above limit", limit + 1, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			body := prefix + strings.Repeat("x", tc.size-len(prefix)-len(suffix)) + suffix
			integration, _ := fixture(t, func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/hal+json")
				_, _ = io.WriteString(w, body)
			})
			result := execute(t, integration, "openproject_list_work_packages", nil)
			if tc.wantError {
				if !result.IsError || !strings.Contains(result.Data, "response exceeds 2 MiB limit") {
					t.Fatalf("expected response size error, got error=%v data=%.200s", result.IsError, result.Data)
				}
			} else if got := successJSON(t, result); !reflect.DeepEqual(got, listJSON(0, 0, 1, 20, 0, 0, nil)) {
				t.Fatalf("unexpected collection projection: %#v", got)
			}
			assertSafe(t, result.Data)
		})
	}
}

func TestOversizeDecompressedBodyRejected(t *testing.T) {
	integration, _ := fixture(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/hal+json")
		w.Header().Set("Content-Encoding", "gzip")
		compressed := gzip.NewWriter(w)
		_, _ = io.WriteString(compressed, `{"padding":"`+strings.Repeat("x", 2*1024*1024)+`","_embedded":{"elements":[]},"total":0,"count":0,"offset":1,"pageSize":20}`)
		_ = compressed.Close()
	})
	result := execute(t, integration, "openproject_list_work_packages", nil)
	if !result.IsError || !strings.Contains(result.Data, "response exceeds 2 MiB limit") {
		t.Fatalf("expected decompressed response size error, got error=%v data=%.200s", result.IsError, result.Data)
	}
	assertSafe(t, result.Data)
}

func TestRedirectDenied(t *testing.T) {
	for _, origin := range []string{"same origin", "cross origin"} {
		t.Run(origin, func(t *testing.T) {
			for _, code := range []int{301, 302, 303, 307, 308} {
				t.Run(strconv.Itoa(code), func(t *testing.T) {
					var targetCalls, targetConnections, sourceCalls, redirectedCalls atomic.Int32
					target := httptest.NewUnstartedServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
						targetCalls.Add(1)
					}))
					target.Config.ConnState = func(_ net.Conn, state http.ConnState) {
						if state == http.StateNew {
							targetConnections.Add(1)
						}
					}
					target.StartTLS()
					t.Cleanup(target.Close)
					integration, _ := fixture(t, func(w http.ResponseWriter, r *http.Request) {
						sourceCalls.Add(1)
						if r.URL.Path == "/PRIVATE" {
							redirectedCalls.Add(1)
							w.Header().Set("Content-Type", "application/hal+json")
							_, _ = io.WriteString(w, collectionFixture(0, 0, 1, 20))
							return
						}
						location := target.URL + "/PRIVATE"
						if origin == "same origin" {
							location = "/PRIVATE"
						}
						http.Redirect(w, r, location, code)
					})
					result := execute(t, integration, "openproject_list_work_packages", nil)
					if !result.IsError || !strings.Contains(strings.ToLower(result.Data), "redirect") {
						t.Error("redirect must be rejected explicitly")
					}
					assertSafe(t, result.Data)
					if sourceCalls.Load() != 1 || redirectedCalls.Load() != 0 {
						t.Errorf("source received %d requests, including %d redirected requests", sourceCalls.Load(), redirectedCalls.Load())
					}
					if received, connected := targetCalls.Load(), targetConnections.Load(); received != 0 || connected != 0 {
						t.Errorf("redirect target received %d requests and %d connections", received, connected)
					}
				})
			}
		})
	}
}

func TestTLSVerifiedAndContextCancellation(t *testing.T) {
	integration, server := fixture(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/hal+json")
		_, _ = io.WriteString(w, collectionFixture(0, 0, 1, 20))
	})
	if err := integration.Configure(context.Background(), credentials(server.URL+"/openproject")); err != nil {
		t.Fatal(err)
	}
	if result := execute(t, integration, "openproject_list_work_packages", nil); !result.IsError {
		t.Fatal("untrusted TLS certificate accepted without ca_cert")
	}
	creds := credentials(server.URL + "/openproject")
	creds["ca_cert"] = string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: server.Certificate().Raw}))
	if err := integration.Configure(context.Background(), creds); err != nil {
		t.Fatal(err)
	}
	_ = successJSON(t, execute(t, integration, "openproject_list_work_packages", nil))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	result, err := integration.Execute(ctx, "openproject_list_work_packages", nil)
	if err != nil || result == nil || !result.IsError {
		t.Fatalf("canceled context returned result=%v err=%v", result, err)
	}
	assertSafe(t, result.Data)
	if strings.Contains(result.Data, server.URL) {
		t.Fatal("transport error exposed trusted server URL")
	}
}

func TestBoundedRetryAfter(t *testing.T) {
	cases := []struct {
		name, header string
		failures     int32
		wantCalls    int32
		wantError    bool
	}{
		{"first retry succeeds", "0", 1, 2, false},
		{"second retry succeeds", "0", 2, 3, false},
		{"two retries exhausted", "0", 10, 3, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var calls atomic.Int32
			integration, _ := fixture(t, func(w http.ResponseWriter, _ *http.Request) {
				call := calls.Add(1)
				w.Header().Set("Content-Type", "application/hal+json")
				if call <= tc.failures {
					w.Header().Set("Retry-After", tc.header)
					w.WriteHeader(http.StatusTooManyRequests)
					_, _ = io.WriteString(w, "PRIVATE secret-token")
					return
				}
				_, _ = io.WriteString(w, collectionFixture(0, 0, 1, 20))
			})
			result := execute(t, integration, "openproject_list_work_packages", nil)
			if result.IsError != tc.wantError || calls.Load() != tc.wantCalls {
				t.Fatalf("retry result=%+v calls=%d wantError=%v wantCalls=%d", result, calls.Load(), tc.wantError, tc.wantCalls)
			}
			assertSafe(t, result.Data)
		})
	}
}

func TestRetryAfterInvalidOrUnboundedRejected(t *testing.T) {
	for _, header := range []string{"", "-1", "1.5", "PRIVATE", "999999999999999999999999999", "60", time.Now().Add(time.Hour).UTC().Format(http.TimeFormat)} {
		t.Run(header, func(t *testing.T) {
			var calls atomic.Int32
			integration, _ := fixture(t, func(w http.ResponseWriter, _ *http.Request) {
				calls.Add(1)
				w.Header().Set("Retry-After", header)
				w.WriteHeader(http.StatusTooManyRequests)
				_, _ = io.WriteString(w, "PRIVATE secret-token")
			})
			result := execute(t, integration, "openproject_list_work_packages", nil)
			if !result.IsError || calls.Load() != 1 {
				t.Fatal("invalid or unbounded Retry-After should fail safely without retry")
			}
			assertSafe(t, result.Data)
		})
	}
}

func TestRetryAfterHTTPDateHonored(t *testing.T) {
	var calls atomic.Int32
	requestTimes := make(chan time.Time, 3)
	integration, _ := fixture(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/hal+json")
		if calls.Add(1) == 1 {
			header := time.Now().Add(1500 * time.Millisecond).UTC().Format(http.TimeFormat)
			advertised, err := http.ParseTime(header)
			if err != nil {
				t.Error(err)
			}
			requestTimes <- advertised
			w.Header().Set("Retry-After", header)
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		requestTimes <- time.Now()
		_, _ = io.WriteString(w, collectionFixture(0, 0, 1, 20))
	})
	_ = successJSON(t, execute(t, integration, "openproject_list_work_packages", nil))
	if calls.Load() != 2 {
		t.Fatalf("request count = %d, want 2", calls.Load())
	}
	advertised, retriedAt := <-requestTimes, <-requestTimes
	if retriedAt.Before(advertised) {
		t.Fatalf("retried before HTTP-date deadline: retry=%s deadline=%s", retriedAt, advertised)
	}
}

func TestRetryAfterPositiveDelayHonored(t *testing.T) {
	var calls atomic.Int32
	requestTimes := make(chan time.Time, 3)
	integration, _ := fixture(t, func(w http.ResponseWriter, _ *http.Request) {
		call := calls.Add(1)
		requestTimes <- time.Now()
		w.Header().Set("Content-Type", "application/hal+json")
		if call == 1 {
			w.Header().Set("Retry-After", "1")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		_, _ = io.WriteString(w, collectionFixture(0, 0, 1, 20))
	})
	_ = successJSON(t, execute(t, integration, "openproject_list_work_packages", nil))
	if calls.Load() != 2 {
		t.Fatalf("request count = %d, want 2", calls.Load())
	}
	first, second := <-requestTimes, <-requestTimes
	if second.Sub(first) < time.Second {
		t.Fatal("positive Retry-After was not honored")
	}
}

func TestDescriptionRawOnlyAndBounded(t *testing.T) {
	prefix := strings.Repeat("aé界𐐀", 1999) + "Zé界"
	cases := []struct {
		name, input, want string
		truncated         bool
	}{
		{"normal", "Untrusted **task** text", "Untrusted **task** text", false},
		{"below bound", prefix, prefix, false},
		{"exact bound", prefix + "𐐀", prefix + "𐐀", false},
		{"over bound", prefix + "𐐀!", prefix + "𐐀", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			body := strings.Replace(workPackageFixture, `"raw":"Untrusted **task** text"`, `"raw":`+strconv.Quote(tc.input), 1)
			integration, _ := fixture(t, func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/hal+json")
				_, _ = io.WriteString(w, body)
			})
			result := execute(t, integration, "openproject_get_work_package", map[string]any{"id": 42})
			want := detailJSON()
			want["description"] = tc.want
			want["description_truncated"] = tc.truncated
			want["truncated"] = tc.truncated
			if got := successJSON(t, result); !reflect.DeepEqual(got, want) {
				t.Fatalf("raw description projection differs from expected content or metadata: description matches=%v description_truncated=%v truncated=%v", got["description"] == tc.want, got["description_truncated"], got["truncated"])
			}
			if len(result.Data) >= 40*1024 {
				t.Fatal("detail output reached 40 KiB cap")
			}
			assertSafe(t, result.Data)
		})
	}
}

func TestSummaryStringsBoundedBeforeRawOutput(t *testing.T) {
	prefix := strings.Repeat("aé界𐐀", 127) + "Zé界"
	for _, field := range []struct{ key, original string }{
		{"subject", "Fix search bug"},
		{"displayId", "PROJ-42"},
		{"status", "New"},
		{"type", "Task"},
		{"updatedAt", "2026-10-16T12:00:00Z"},
	} {
		t.Run(field.key, func(t *testing.T) {
			for _, boundary := range []struct {
				name, input, want string
				truncated         bool
			}{
				{"below bound", prefix, prefix, false},
				{"at bound", prefix + "𐐀", prefix + "𐐀", false},
				{"over bound", prefix + "𐐀!", prefix + "𐐀", true},
			} {
				t.Run(boundary.name, func(t *testing.T) {
					body := strings.Replace(workPackageFixture, strconv.Quote(field.original), strconv.Quote(boundary.input), 1)
					integration, _ := fixture(t, func(w http.ResponseWriter, r *http.Request) {
						w.Header().Set("Content-Type", "application/hal+json")
						if strings.Contains(r.URL.Path, "/workspaces/") {
							_, _ = io.WriteString(w, collectionFixture(1, 1, 1, 20, body))
							return
						}
						_, _ = io.WriteString(w, body)
					})
					for _, request := range []struct {
						tool mcp.ToolName
						args map[string]any
					}{
						{"openproject_list_work_packages", nil},
						{"openproject_get_work_package", map[string]any{"id": 42}},
					} {
						t.Run(string(request.tool), func(t *testing.T) {
							result := execute(t, integration, request.tool, request.args)
							assertSafe(t, result.Data)
							want := detailJSON()
							want[field.key] = boundary.want
							if request.tool == "openproject_list_work_packages" {
								summary := summaryJSON()
								summary[field.key] = boundary.want
								want = listJSON(1, 1, 1, 20, 1, 0, nil, summary)
							}
							want["truncated"] = boundary.truncated
							if got := successJSON(t, result); !reflect.DeepEqual(got, want) {
								t.Fatalf("summary projection=%#v want %#v", got, want)
							}
						})
					}
				})
			}
		})
	}
}

func TestFinalListOutputBoundedWithActionableError(t *testing.T) {
	long := strings.Repeat("é", 512)
	row := strings.ReplaceAll(workPackageFixture, "Fix search bug", long)
	row = strings.ReplaceAll(row, "PROJ-42", long)
	row = strings.ReplaceAll(row, `"title":"New"`, `"title":"`+long+`"`)
	row = strings.ReplaceAll(row, `"title":"Task"`, `"title":"`+long+`"`)
	rows := make([]string, 50)
	for index := range rows {
		rows[index] = row
	}
	integration, _ := fixture(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/hal+json")
		_, _ = io.WriteString(w, collectionFixture(100, 50, 1, 50, rows...))
	})
	result := execute(t, integration, "openproject_list_work_packages", map[string]any{"limit": 50})
	if !result.IsError || len(result.Data) >= 40*1024 || !strings.Contains(strings.ToLower(result.Data), "smaller") || !strings.Contains(strings.ToLower(result.Data), "limit") {
		t.Fatalf("oversize projected list needs safe actionable error: bytes=%d error=%v", len(result.Data), result.IsError)
	}
	assertSafe(t, result.Data)
}

func TestHealthyUsesBoundedReadAndClose(t *testing.T) {
	var calls atomic.Int32
	integration, _ := fixture(t, func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.Method != http.MethodGet || r.URL.Path != "/openproject/api/v3/workspaces/7/work_packages" || r.URL.Query().Get("offset") != "1" || r.URL.Query().Get("pageSize") != "1" || r.URL.Query().Get("filters") != "[]" {
			t.Errorf("health must be a minimal fixed-workspace read: %s %s", r.Method, r.URL)
		}
		w.Header().Set("Content-Type", "application/hal+json")
		_, _ = io.WriteString(w, collectionFixture(1, 1, 1, 1, workPackageFixture))
	})
	if !integration.Healthy(context.Background()) || calls.Load() != 1 {
		t.Fatal("configured valid project should be healthy")
	}
	if closer, ok := integration.(io.Closer); ok {
		if err := closer.Close(); err != nil {
			t.Fatal(err)
		}
		if err := closer.Close(); err != nil {
			t.Fatal(err)
		}
		if integration.Healthy(context.Background()) || calls.Load() != 1 {
			t.Fatal("closed adapter retained active credentials")
		}
	}
}

func TestDetailDatesMayBeNull(t *testing.T) {
	body := strings.Replace(workPackageFixture, `"startDate":"2026-10-16","dueDate":"2026-10-20"`, `"startDate":null,"dueDate":null`, 1)
	integration, _ := fixture(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/hal+json")
		_, _ = io.WriteString(w, body)
	})
	result := execute(t, integration, "openproject_get_work_package", map[string]any{"id": 42})
	want := detailJSON()
	want["startDate"], want["dueDate"] = nil, nil
	if got := successJSON(t, result); !reflect.DeepEqual(got, want) {
		t.Fatalf("nullable API dates changed: %#v", got)
	}
	assertSafe(t, result.Data)
}

func TestRequiredSummaryMetadataRejected(t *testing.T) {
	cases := []struct{ name, old, replacement string }{
		{"missing display id", `"displayId":"PROJ-42",`, ""},
		{"null display id", `"displayId":"PROJ-42"`, `"displayId":null`},
		{"missing subject", `"subject":"Fix search bug",`, ""},
		{"null subject", `"subject":"Fix search bug"`, `"subject":null`},
		{"missing updated timestamp", `"updatedAt":"2026-10-16T12:00:00Z",`, ""},
		{"null updated timestamp", `"updatedAt":"2026-10-16T12:00:00Z"`, `"updatedAt":null`},
		{"missing status label", `"title":"New"`, `"name":"PRIVATE"`},
		{"null status label", `"title":"New"`, `"title":null`},
		{"missing type label", `"title":"Task"`, `"name":"PRIVATE"`},
		{"null type label", `"title":"Task"`, `"title":null`},
		{"wrong subject type", `"subject":"Fix search bug"`, `"subject":{"PRIVATE":true}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			body := strings.Replace(workPackageFixture, tc.old, tc.replacement, 1)
			integration, _ := fixture(t, func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/hal+json")
				if strings.Contains(r.URL.Path, "/workspaces/") {
					_, _ = io.WriteString(w, collectionFixture(1, 1, 1, 20, body))
					return
				}
				_, _ = io.WriteString(w, body)
			})
			for _, request := range []struct {
				tool mcp.ToolName
				args map[string]any
			}{{"openproject_list_work_packages", nil}, {"openproject_get_work_package", map[string]any{"id": 42}}} {
				result := execute(t, integration, request.tool, request.args)
				if !result.IsError {
					t.Fatalf("missing/null or incorrectly typed summary accepted by %s", request.tool)
				}
				assertSafe(t, result.Data)
			}
		})
	}
}

func TestRequiredDetailMetadataRejected(t *testing.T) {
	cases := []struct{ name, old, replacement string }{
		{"missing created timestamp", `"createdAt":"2026-10-15T12:00:00Z",`, ""},
		{"null created timestamp", `"createdAt":"2026-10-15T12:00:00Z"`, `"createdAt":null`},
		{"html without raw", `"raw":"Untrusted **task** text",`, ""},
		{"null raw", `"raw":"Untrusted **task** text"`, `"raw":null`},
		{"wrong description type", `"raw":"Untrusted **task** text"`, `"raw":{"PRIVATE":"secret-token"}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			body := strings.Replace(workPackageFixture, tc.old, tc.replacement, 1)
			integration, _ := fixture(t, func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/hal+json")
				_, _ = io.WriteString(w, body)
			})
			result := execute(t, integration, "openproject_get_work_package", map[string]any{"id": 42})
			if !result.IsError {
				t.Fatal("malformed detail accepted or HTML substituted for raw")
			}
			assertSafe(t, result.Data)
		})
	}
}

func TestListSummaryTruncationFlag(t *testing.T) {
	body := strings.Replace(workPackageFixture, `"subject":"Fix search bug"`, `"subject":"`+strings.Repeat("é", 513)+`"`, 1)
	integration, _ := fixture(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/hal+json")
		_, _ = io.WriteString(w, collectionFixture(1, 1, 1, 20, body))
	})
	result := execute(t, integration, "openproject_list_work_packages", nil)
	out := successJSON(t, result)
	if out["truncated"] != true || out["omitted"] != float64(0) || out["returned"] != float64(1) {
		t.Fatal("string truncation must be marked, not counted as a workspace omission")
	}
	packages := out["work_packages"].([]any)
	subject := packages[0].(map[string]any)["subject"].(string)
	if !utf8.ValidString(subject) || utf8.RuneCountInString(subject) != 512 {
		t.Fatal("list subject not truncated at Unicode character bound")
	}
	assertSafe(t, result.Data)
}

func TestGetFailsClosedForWrongResource(t *testing.T) {
	cases := []struct{ name, body string }{
		{"wrong id", strings.Replace(workPackageFixture, `"id":42`, `"id":43`, 1)},
		{"missing id", strings.Replace(workPackageFixture, `"id":42,`, "", 1)},
		{"missing project", strings.Replace(workPackageFixture, `"project":{"href":"/openproject/api/v3/projects/7","title":"PRIVATE-PROJECT"},`, "", 1)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			integration, _ := fixture(t, func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/hal+json")
				_, _ = io.WriteString(w, tc.body)
			})
			result := execute(t, integration, "openproject_get_work_package", map[string]any{"id": 42})
			if !result.IsError {
				t.Fatal("exact get accepted missing or mismatched identity/project")
			}
			assertSafe(t, result.Data)
		})
	}
}
