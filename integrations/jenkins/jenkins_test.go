package jenkins

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	mcp "github.com/daltoniam/switchboard"
)

const jobsFixture = `{"_class":"hudson.model.Hudson","jobs":[{"name":"team","_class":"com.cloudbees.hudson.plugins.folder.Folder","color":"blue","lastBuild":{"number":42,"result":"SUCCESS","timestamp":1700000000000,"actions":[{"secret":"PRIVATE"}]},"url":"https://evil.invalid/","description":"PRIVATE","actions":[{"parameters":[{"value":"PRIVATE"}]}]}],"secret":"PRIVATE"}`
const jobFixture = `{"name":"feature/foo","_class":"org.jenkinsci.plugins.workflow.job.WorkflowJob","color":"blue_anime","buildable":true,"inQueue":false,"lastBuild":{"number":42,"result":null,"timestamp":1700000000000},"allBuilds":[{"number":42,"result":null,"timestamp":1700000000000,"duration":0,"building":true,"actions":[{"secret":"PRIVATE"}]},{"number":41,"result":"SUCCESS","timestamp":1699999999000,"duration":1000,"building":false}],"actions":[{"parameters":[{"value":"PRIVATE"}]}],"url":"https://evil.invalid/","description":"PRIVATE"}`
const buildFixture = `{"number":42,"result":null,"building":true,"timestamp":1700000000000,"duration":1234,"estimatedDuration":5678,"actions":[{"parameters":[{"value":"PRIVATE"}]}],"changeSets":[{"author":{"email":"PRIVATE"}}],"url":"https://evil.invalid/","console":"PRIVATE"}`

func credentials(base string) mcp.Credentials {
	return mcp.Credentials{"base_url": base, "username": "read-user", "api_token": "secret-token"}
}

func fixture(t *testing.T, handler http.HandlerFunc) (*jenkins, *httptest.Server) {
	t.Helper()
	ts := httptest.NewTLSServer(handler)
	t.Cleanup(ts.Close)
	creds := credentials(ts.URL + "/jenkins")
	creds["ca_cert"] = string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: ts.Certificate().Raw}))
	j := New().(*jenkins)
	if err := j.Configure(context.Background(), creds); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = j.Close() })
	return j, ts
}

func execute(t *testing.T, j *jenkins, name mcp.ToolName, args map[string]any) *mcp.ToolResult {
	t.Helper()
	result, err := j.Execute(context.Background(), name, args)
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

func TestNewAndMetadata(t *testing.T) {
	j := New().(*jenkins)
	if j.Name() != "jenkins" || j.Healthy(context.Background()) {
		t.Fatal("invalid constructor or unconfigured health")
	}
	if len(j.Tools()) != 3 || !strings.Contains(j.Tools()[0].Description, "Start here") {
		t.Fatal("expected exactly three reads with entry point guidance")
	}
	seen := map[mcp.ToolName]bool{}
	for _, tool := range j.Tools() {
		if seen[tool.Name] || !strings.HasPrefix(string(tool.Name), "jenkins_") || tool.Description == "" || tool.Access != mcp.AccessRead {
			t.Fatalf("invalid tool metadata: %+v", tool)
		}
		seen[tool.Name] = true
	}
	if !reflect.DeepEqual(j.OptionalKeys(), []string{"ca_cert"}) {
		t.Fatal("only ca_cert is optional")
	}
	for key, phrase := range map[string]string{"base_url": "HTTPS Jenkins", "username": "Overall/Read", "api_token": "API token", "ca_cert": "PEM CA certificate"} {
		if !strings.Contains(j.Placeholders()[key], phrase) {
			t.Fatalf("placeholder %s needs %s", key, phrase)
		}
	}
	if !reflect.DeepEqual(j.PlainTextKeys(), []string{"base_url", "username", "ca_cert"}) {
		t.Fatal("api_token must remain secret")
	}
	if result := execute(t, j, "jenkins_unknown", nil); !result.IsError || !strings.Contains(result.Data, "unknown tool") {
		t.Fatal("unknown tool accepted")
	}
	for _, tool := range j.Tools() {
		args := map[string]any{"job_path": []string{"job"}, "build_number": 1}
		switch tool.Name {
		case "jenkins_list_jobs":
			args = nil
		case "jenkins_get_job":
			delete(args, "build_number")
		}
		if result := execute(t, j, tool.Name, args); !result.IsError {
			t.Fatal("unconfigured adapter accepted request")
		}
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

func TestConfigureValidation(t *testing.T) {
	cases := []struct{ key, value string }{
		{"base_url", ""}, {"username", ""}, {"api_token", ""},
		{"base_url", "http://127.0.0.1:8080/jenkins"},
		{"base_url", "https://user:secret@example.com"},
		{"base_url", "https://example.com/?secret=value"},
		{"base_url", "https://example.com/#secret"},
		{"base_url", "https://example.com/?"},
		{"base_url", "https://example.com/#"},
		{"base_url", "https://example.com:0"},
		{"base_url", "https://example.com:65536"},
		{"base_url", "https://example.com:"},
		{"base_url", "https://example.com:bad"},
		{"base_url", "https:///jenkins"},
		{"base_url", "https:example.com"},
		{"base_url", " https://example.com"},
		{"base_url", "https://example.com/a b"},
		{"base_url", "https://example.com/a/../b"},
		{"base_url", "https://example.com/./b"},
		{"base_url", "https://example.com/a//b"},
		{"base_url", "https://example.com/%2e%2e/b"},
		{"base_url", "https://example.com/a\\b"},
		{"base_url", "https://example.com/%2fetc"},
		{"username", "bad:name"}, {"username", " bad"}, {"username", "bad\nname"},
		{"api_token", " bad"}, {"api_token", "bad\r\nname"}, {"api_token", "bad\x00name"},
		{"api_token", "bad\tname"}, {"api_token", "bad\u0085name"},
		{"ca_cert", "not a certificate"},
	}
	for i, tc := range cases {
		t.Run(fmt.Sprintf("%d_%s", i, tc.key), func(t *testing.T) {
			creds := credentials("https://jenkins.example.com/jenkins")
			creds[tc.key] = tc.value
			err := New().Configure(context.Background(), creds)
			if err == nil {
				t.Fatal("invalid credentials accepted")
			}
			token := creds["api_token"]
			jsonToken, marshalErr := json.Marshal(token)
			if marshalErr != nil {
				t.Fatal(marshalErr)
			}
			quotedToken, asciiToken := strconv.Quote(token), strconv.QuoteToASCII(token)
			for _, sensitive := range []string{token, quotedToken[1 : len(quotedToken)-1], asciiToken[1 : len(asciiToken)-1], string(jsonToken[1 : len(jsonToken)-1]), "user:secret"} {
				if sensitive != "" && strings.Contains(err.Error(), sensitive) {
					t.Fatal("credential echoed in validation error")
				}
			}
		})
	}
	for _, base := range []string{"https://jenkins.example.com", "https://jenkins.example.com/jenkins/", "https://[::1]:8443/jenkins"} {
		t.Run(base, func(t *testing.T) {
			j := New().(*jenkins)
			if err := j.Configure(context.Background(), credentials(base)); err != nil {
				t.Fatal(err)
			}
			defer func() { _ = j.Close() }()
			if j.client.Timeout != 15*time.Second {
				t.Fatal("expected 15 second HTTP timeout")
			}
		})
	}
}

func TestRequestAndProjection(t *testing.T) {
	last := map[string]any{"number": float64(42), "result": "SUCCESS", "timestamp": float64(1700000000000)}
	jobs := []any{map[string]any{"name": "team", "_class": "com.cloudbees.hudson.plugins.folder.Folder", "color": "blue", "lastBuild": last}}
	jobLast := map[string]any{"number": float64(42), "result": nil, "timestamp": float64(1700000000000)}
	builds := []any{
		map[string]any{"number": float64(42), "result": nil, "timestamp": float64(1700000000000), "duration": float64(0), "building": true},
		map[string]any{"number": float64(41), "result": "SUCCESS", "timestamp": float64(1699999999000), "duration": float64(1000), "building": false},
	}
	cases := []struct {
		name             mcp.ToolName
		args             map[string]any
		path, tree, body string
		want             map[string]any
	}{
		{"jenkins_list_jobs", nil, "/jenkins/api/json", "jobs[name,_class,color,lastBuild[number,result,timestamp]]{0,20}", jobsFixture, map[string]any{"jobs": jobs, "returned_count": float64(1), "offset": float64(0), "limit": float64(20), "possibly_more": false}},
		{"jenkins_list_jobs", map[string]any{"folder_path": []any{"team", "a %2F#?+é"}, "offset": "2", "limit": 1}, "/jenkins/job/team/job/a%20%252F%23%3F+%C3%A9/api/json", "jobs[name,_class,color,lastBuild[number,result,timestamp]]{2,3}", jobsFixture, map[string]any{"jobs": jobs, "returned_count": float64(1), "offset": float64(2), "limit": float64(1), "possibly_more": true}},
		{"jenkins_get_job", map[string]any{"job_path": `["team","feature/foo"]`, "offset": 5, "limit": 2}, "/jenkins/job/team/job/feature%2Ffoo/api/json", "name,_class,color,buildable,inQueue,lastBuild[number,result,timestamp],allBuilds[number,result,timestamp,duration,building]{5,7}", jobFixture, map[string]any{"name": "feature/foo", "_class": "org.jenkinsci.plugins.workflow.job.WorkflowJob", "color": "blue_anime", "lastBuild": jobLast, "buildable": true, "inQueue": false, "builds": builds, "returned_count": float64(2), "offset": float64(5), "limit": float64(2), "possibly_more": true}},
		{"jenkins_get_build", map[string]any{"job_path": []string{"team", "feature/foo"}, "build_number": json.Number("42")}, "/jenkins/job/team/job/feature%2Ffoo/42/api/json", "number,result,building,timestamp,duration,estimatedDuration", buildFixture, map[string]any{"number": float64(42), "result": nil, "building": true, "phase": "building", "timestamp": float64(1700000000000), "duration": float64(1234), "estimatedDuration": float64(5678)}},
	}
	for i, tc := range cases {
		t.Run(fmt.Sprintf("%d_%s", i, tc.name), func(t *testing.T) {
			var calls atomic.Int32
			j, _ := fixture(t, func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				user, token, ok := r.BasicAuth()
				if r.Method != http.MethodGet || !ok || user != "read-user" || token != "secret-token" || r.Header.Get("Accept") != "application/json" {
					t.Error("invalid method or preemptive authentication")
				}
				if r.URL.EscapedPath() != tc.path || r.URL.Query().Get("tree") != tc.tree || len(r.URL.Query()) != 1 {
					t.Errorf("unexpected fixed request: %s", r.URL)
				}
				w.Header().Set("Content-Type", "application/json;charset=UTF-8")
				_, _ = w.Write([]byte(tc.body))
			})
			result := execute(t, j, tc.name, tc.args)
			out := successJSON(t, result)
			if !reflect.DeepEqual(out, tc.want) {
				t.Errorf("projection=%#v want %#v", out, tc.want)
			}
			if strings.Contains(result.Data, "PRIVATE") || strings.Contains(result.Data, "evil.invalid") || strings.Contains(result.Data, "actions") {
				t.Fatal("raw/script result leaked fields outside typed allowlist")
			}
			if calls.Load() != 1 {
				t.Fatal("unexpected extra requests")
			}
		})
	}
}

func TestBuildHistoryBeyondLatestHundred(t *testing.T) {
	j, _ := fixture(t, func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("tree"); got != "name,_class,color,buildable,inQueue,lastBuild[number,result,timestamp],allBuilds[number,result,timestamp,duration,building]{100,102}" {
			t.Errorf("history must select a bounded allBuilds window, got %s", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"name":"history","_class":"hudson.model.FreeStyleProject","color":"blue","buildable":true,"inQueue":false,"lastBuild":{"number":101,"result":"SUCCESS","timestamp":1700000000000},"builds":[],"allBuilds":[{"number":1,"result":"SUCCESS","timestamp":1699999999000,"duration":1000,"building":false}]}`))
	})
	out := successJSON(t, execute(t, j, "jenkins_get_job", map[string]any{"job_path": []string{"history"}, "offset": 100, "limit": 2}))
	want := map[string]any{
		"name": "history", "_class": "hudson.model.FreeStyleProject", "color": "blue", "buildable": true, "inQueue": false,
		"lastBuild":      map[string]any{"number": float64(101), "result": "SUCCESS", "timestamp": float64(1700000000000)},
		"builds":         []any{map[string]any{"number": float64(1), "result": "SUCCESS", "timestamp": float64(1699999999000), "duration": float64(1000), "building": false}},
		"returned_count": float64(1), "offset": float64(100), "limit": float64(2), "possibly_more": false,
	}
	if !reflect.DeepEqual(out, want) {
		t.Fatalf("older build projection=%#v want %#v", out, want)
	}
}

func TestEmptyAndCompletedShapes(t *testing.T) {
	cases := []struct {
		tool mcp.ToolName
		args map[string]any
		body string
		want string
	}{
		{"jenkins_list_jobs", map[string]any{"folder_path": []any{}}, `{"jobs":[]}`, `"jobs":[]`},
		{"jenkins_list_jobs", map[string]any{"folder_path": "[]"}, `{"jobs":[]}`, `"possibly_more":false`},
		{"jenkins_get_job", map[string]any{"job_path": []string{"new"}}, `{"name":"new","_class":"hudson.model.FreeStyleProject","color":"notbuilt","buildable":false,"inQueue":true,"lastBuild":null,"allBuilds":[]}`, `"builds":[]`},
		{"jenkins_get_build", map[string]any{"job_path": []string{"job"}, "build_number": 42}, strings.ReplaceAll(strings.ReplaceAll(buildFixture, `"building":true`, `"building":false`), `"result":null`, `"result":"SUCCESS"`), `"phase":"completed"`},
	}
	for i, tc := range cases {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			j, _ := fixture(t, func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(tc.body))
			})
			result := execute(t, j, tc.tool, tc.args)
			_ = successJSON(t, result)
			if !strings.Contains(result.Data, tc.want) {
				t.Fatalf("missing %s in %s", tc.want, result.Data)
			}
		})
	}
}

func TestInvalidArgumentsNeverSendRequest(t *testing.T) {
	var calls atomic.Int32
	j, _ := fixture(t, func(http.ResponseWriter, *http.Request) { calls.Add(1) })
	cases := []struct {
		name string
		tool mcp.ToolName
		args map[string]any
	}{
		{"missing job", "jenkins_get_job", nil},
		{"empty job", "jenkins_get_job", map[string]any{"job_path": []string{}}},
		{"string path", "jenkins_get_job", map[string]any{"job_path": "team/job"}},
		{"null path", "jenkins_list_jobs", map[string]any{"folder_path": nil}},
		{"null JSON path", "jenkins_list_jobs", map[string]any{"folder_path": "null"}},
		{"mixed array", "jenkins_get_job", map[string]any{"job_path": []any{"job", 1}}},
		{"object path", "jenkins_get_job", map[string]any{"job_path": map[string]any{"x": "job"}}},
		{"unknown tree", "jenkins_list_jobs", map[string]any{"tree": "secrets"}},
		{"unknown depth", "jenkins_list_jobs", map[string]any{"depth": 10}},
		{"unexpected pagination", "jenkins_get_build", map[string]any{"job_path": []string{"job"}, "build_number": 42, "limit": 1}},
	}
	for _, segment := range []string{"", ".", "..", "https://evil.invalid", "//evil.invalid", "/root", "file:secret", "job\x00name", "job\nname", "job\u0085name", "job\\name", strings.Repeat("a", 513)} {
		cases = append(cases, struct {
			name string
			tool mcp.ToolName
			args map[string]any
		}{"segment " + strconv.Quote(segment), "jenkins_get_job", map[string]any{"job_path": []string{segment}}})
	}
	for _, key := range []string{"offset", "limit", "build_number"} {
		values := []any{nil, true, 1.5, math.NaN(), math.Inf(1), "1.2", "secret-token", []int{1}, json.Number("1e3"), "9999999999999999999999999999", -1}
		if key != "offset" {
			values = append(values, 0)
		}
		if key == "limit" {
			values = append(values, 51)
		}
		for i, value := range values {
			args := map[string]any{key: value}
			tool := mcp.ToolName("jenkins_list_jobs")
			if key == "build_number" {
				tool = "jenkins_get_build"
				args["job_path"] = []string{"job"}
			}
			cases = append(cases, struct {
				name string
				tool mcp.ToolName
				args map[string]any
			}{fmt.Sprintf("%s_%d", key, i), tool, args})
		}
	}
	cases = append(cases, struct {
		name string
		tool mcp.ToolName
		args map[string]any
	}{"range overflow", "jenkins_list_jobs", map[string]any{"offset": int(^uint(0) >> 1), "limit": 1}})
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			result := execute(t, j, tc.tool, tc.args)
			if !result.IsError || strings.Contains(result.Data, "secret-token") {
				t.Fatalf("expected safe argument rejection, got %+v", result)
			}
		})
	}
	if calls.Load() != 0 {
		t.Fatal("invalid arguments reached network")
	}
}

func TestHTTPFailuresAreSafe(t *testing.T) {
	cases := []struct {
		name              string
		status            int
		contentType, body string
	}{
		{"unauthorized", 401, "text/plain", "PRIVATE secret-token read-user"},
		{"forbidden", 403, "text/html", "PRIVATE secret-token read-user"},
		{"visibility", 404, "application/json", `{"message":"PRIVATE secret-token read-user"}`},
		{"server", 500, "text/plain", "PRIVATE secret-token read-user"},
		{"login", 200, "text/html", "<html>PRIVATE secret-token read-user</html>"},
		{"JSON login", 200, "application/json", "<html>PRIVATE secret-token read-user</html>"},
		{"empty", 204, "application/json", ""},
		{"invalid", 200, "application/json", `{"jobs":`},
		{"missing", 200, "application/json", `{}`},
		{"null", 200, "application/json", `null`},
		{"wrong type", 200, "application/json", `{"jobs":"PRIVATE secret-token"}`},
		{"trailing data", 200, "application/json", jobsFixture + `{}`},
		{"oversize", 200, "application/json", `{"junk":"` + strings.Repeat("x", 2*1024*1024) + `","jobs":[]}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var calls atomic.Int32
			j, _ := fixture(t, func(w http.ResponseWriter, _ *http.Request) {
				calls.Add(1)
				w.Header().Set("Content-Type", tc.contentType)
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(tc.body))
			})
			result := execute(t, j, "jenkins_list_jobs", nil)
			if !result.IsError || strings.Contains(result.Data, "PRIVATE") || strings.Contains(result.Data, "secret-token") || strings.Contains(result.Data, "read-user") {
				t.Fatalf("unsafe/non-error result: %+v", result)
			}
			if tc.status == 404 && !strings.Contains(result.Data, "visibility") {
				t.Fatal("404 must explain visibility ambiguity")
			}
			if tc.name == "oversize" && !strings.Contains(result.Data, "2 MiB") {
				t.Fatal("oversize should fail clearly")
			}
			if calls.Load() != 1 {
				t.Fatal("unexpected retry")
			}
		})
	}
}

func TestRedirectDenied(t *testing.T) {
	var targetCalls atomic.Int32
	target := httptest.NewTLSServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { targetCalls.Add(1) }))
	defer target.Close()
	for _, code := range []int{301, 302, 303, 307, 308} {
		t.Run(strconv.Itoa(code), func(t *testing.T) {
			j, _ := fixture(t, func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, target.URL+"/PRIVATE", code) })
			result := execute(t, j, "jenkins_list_jobs", nil)
			if !result.IsError || !strings.Contains(result.Data, "redirect") || strings.Contains(result.Data, "PRIVATE") {
				t.Fatal("redirect must fail without echoing location")
			}
		})
	}
	if targetCalls.Load() != 0 {
		t.Fatal("credentials may have followed redirect")
	}
}

func TestTLSVerifiedAndContextCancellation(t *testing.T) {
	j, ts := fixture(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(jobsFixture))
	})
	if err := j.Configure(context.Background(), credentials(ts.URL)); err != nil {
		t.Fatal(err)
	}
	if result := execute(t, j, "jenkins_list_jobs", nil); !result.IsError {
		t.Fatal("untrusted server certificate accepted")
	}
	creds := credentials(ts.URL + "/jenkins")
	creds["ca_cert"] = string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: ts.Certificate().Raw}))
	if err := j.Configure(context.Background(), creds); err != nil {
		t.Fatal(err)
	}
	_ = successJSON(t, execute(t, j, "jenkins_list_jobs", nil))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	result, err := j.Execute(ctx, "jenkins_list_jobs", nil)
	if err != nil || result == nil || !result.IsError || strings.Contains(result.Data, ts.URL) || strings.Contains(result.Data, "secret-token") {
		t.Fatal("cancellation should return safe error")
	}
}

func TestHealthyBoundedAndClose(t *testing.T) {
	var calls atomic.Int32
	j, _ := fixture(t, func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.URL.Query().Get("tree") != "jobs[name,_class,color,lastBuild[number,result,timestamp]]{0,1}" {
			t.Error("health must use lightweight limit 1")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(jobsFixture))
	})
	if !j.Healthy(context.Background()) || calls.Load() != 1 {
		t.Fatal("configured fixture should be healthy")
	}
	if err := j.Close(); err != nil {
		t.Fatal(err)
	}
	if err := j.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestRetryAfterHTTPDateWaitsUntilDeadline(t *testing.T) {
	cases := []struct {
		name   string
		offset time.Duration
	}{
		{"whole second", 0},
		{"quarter second", 250 * time.Millisecond},
		{"last millisecond", 999 * time.Millisecond},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				time.Sleep(tc.offset)
				requests := 0
				var advertised, retriedAt time.Time
				j := &jenkins{
					baseURL: "https://jenkins.example.com",
					client: &http.Client{Transport: retryWaitTransport(func(*http.Request) (*http.Response, error) {
						requests++
						if requests == 1 {
							header := time.Now().Add(2 * time.Second).UTC().Format(http.TimeFormat)
							var err error
							advertised, err = http.ParseTime(header)
							if err != nil {
								t.Fatal(err)
							}
							return &http.Response{StatusCode: http.StatusTooManyRequests, Header: http.Header{"Retry-After": {header}}, Body: http.NoBody}, nil
						}
						retriedAt = time.Now()
						return &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": {"application/json"}}, Body: io.NopCloser(strings.NewReader(jobsFixture))}, nil
					})},
				}
				started := time.Now()
				_ = successJSON(t, execute(t, j, "jenkins_list_jobs", nil))
				if requests != 2 {
					t.Fatalf("requests = %d, want 2", requests)
				}
				if !advertised.After(started) || advertised.Sub(started) > maxRetryDelay {
					t.Fatalf("advertised retry delay %s is not positive and bounded", advertised.Sub(started))
				}
				if !retriedAt.Equal(advertised) || !time.Now().Equal(advertised) {
					t.Fatalf("retry at %s after %s, want advertised deadline %s after %s", retriedAt, retriedAt.Sub(started), advertised, advertised.Sub(started))
				}
			})
		})
	}
}

func TestBoundedRetryAfter(t *testing.T) {
	cases := []struct {
		name, header string
		status       int
		wantCalls    int32
		wantError    bool
	}{
		{"zero", "0", 200, 2, false},
		{"still limited", "0", 429, 2, true},
		{"missing", "", 200, 1, true},
		{"negative", "-1", 200, 1, true},
		{"invalid", "PRIVATE", 200, 1, true},
		{"overflow", "99999999999999999999", 200, 1, true},
		{"over bound", "60", 200, 1, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var calls atomic.Int32
			j, _ := fixture(t, func(w http.ResponseWriter, _ *http.Request) {
				call := calls.Add(1)
				w.Header().Set("Content-Type", "application/json")
				if call == 1 || tc.status == 429 {
					w.Header().Set("Retry-After", tc.header)
					w.WriteHeader(429)
					_, _ = w.Write([]byte("PRIVATE secret-token"))
					return
				}
				_, _ = w.Write([]byte(jobsFixture))
			})
			result := execute(t, j, "jenkins_list_jobs", nil)
			if result.IsError != tc.wantError || calls.Load() != tc.wantCalls || strings.Contains(result.Data, "PRIVATE") || strings.Contains(result.Data, "secret-token") {
				t.Fatalf("unexpected retry result=%+v calls=%d", result, calls.Load())
			}
		})
	}
}

func TestProjectionRejectsOversizeAndMalformed(t *testing.T) {
	job := `{"name":"x","_class":"job","color":"blue","lastBuild":null}`
	cases := []struct {
		name string
		tool mcp.ToolName
		body string
		args map[string]any
	}{
		{"too many jobs", "jenkins_list_jobs", `{"jobs":[` + strings.TrimSuffix(strings.Repeat(job+",", 51), ",") + `]}`, map[string]any{"limit": 50}},
		{"ignored limit", "jenkins_list_jobs", `{"jobs":[` + job + "," + job + `]}`, map[string]any{"limit": 1}},
		{"long name", "jenkins_list_jobs", strings.Replace(jobsFixture, `"name":"team"`, `"name":"`+strings.Repeat("é", 513)+`"`, 1), nil},
		{"long class", "jenkins_list_jobs", strings.Replace(jobsFixture, `"_class":"com.cloudbees.hudson.plugins.folder.Folder"`, `"_class":"`+strings.Repeat("x", 513)+`"`, 1), nil},
		{"long color", "jenkins_list_jobs", strings.Replace(jobsFixture, `"color":"blue"`, `"color":"`+strings.Repeat("x", 513)+`"`, 1), nil},
		{"long result", "jenkins_get_build", strings.Replace(buildFixture, `"result":null`, `"result":"`+strings.Repeat("x", 513)+`"`, 1), map[string]any{"job_path": []string{"job"}, "build_number": 42}},
		{"missing building", "jenkins_get_build", `{"number":42,"result":null}`, map[string]any{"job_path": []string{"job"}, "build_number": 42}},
		{"wrong build", "jenkins_get_build", strings.Replace(buildFixture, `"number":42`, `"number":43`, 1), map[string]any{"job_path": []string{"job"}, "build_number": 42}},
		{"wrong job", "jenkins_get_job", jobFixture, map[string]any{"job_path": []string{"other"}}},
		{"missing builds", "jenkins_get_job", `{"name":"x"}`, map[string]any{"job_path": []string{"x"}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			j, _ := fixture(t, func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(tc.body))
			})
			result := execute(t, j, tc.tool, tc.args)
			if !result.IsError || strings.Contains(result.Data, "PRIVATE") {
				t.Fatalf("invalid projection accepted: %+v", result)
			}
		})
	}
}

func TestErrorNeverLeaksBasicHeader(t *testing.T) {
	j, _ := fixture(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(403)
		_, _ = w.Write([]byte(r.Header.Get("Authorization")))
	})
	result := execute(t, j, "jenkins_list_jobs", nil)
	if !result.IsError || strings.Contains(result.Data, base64.StdEncoding.EncodeToString([]byte("read-user:secret-token"))) {
		t.Fatal("authorization leaked into error")
	}
}
