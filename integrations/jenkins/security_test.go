package jenkins

import (
	"context"
	"encoding/pem"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	mcp "github.com/daltoniam/switchboard"
)

func TestExactItemSegmentsNeverDecodeOrNormalize(t *testing.T) {
	cases := []struct{ segment, escaped string }{
		{"feature/foo", "feature%2Ffoo"},
		{"feature%2Ffoo", "feature%252Ffoo"},
		{"feature%252Ffoo", "feature%25252Ffoo"},
		{"100% coverage", "100%25%20coverage"},
		{"feature/a/../b", "feature%2Fa%2F..%2Fb"},
		{"feature%2f..%2fmain", "feature%252f..%252fmain"},
	}
	for _, tc := range cases {
		t.Run(tc.segment, func(t *testing.T) {
			j, _ := fixture(t, func(w http.ResponseWriter, r *http.Request) {
				if r.URL.EscapedPath() != "/jenkins/job/"+tc.escaped+"/42/api/json" {
					t.Errorf("exact segment changed: %s", r.URL.EscapedPath())
				}
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(buildFixture))
			})
			_ = successJSON(t, execute(t, j, "jenkins_get_build", map[string]any{"job_path": []string{tc.segment}, "build_number": 42}))
		})
	}
}

func TestCustomCAPEMValidation(t *testing.T) {
	ts := httptest.NewTLSServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	defer ts.Close()
	cert := string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: ts.Certificate().Raw}))
	cases := []struct {
		name, cert string
		valid      bool
	}{
		{"literal newline", strings.ReplaceAll(cert, "\n", `\n`), true},
		{"bundle", cert + cert, true},
		{"trailing data", cert + "PRIVATE", false},
		{"leading data", "PRIVATE\n" + cert, false},
		{"wrong block", string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: ts.Certificate().Raw})), false},
		{"invalid cert", "-----BEGIN CERTIFICATE-----\nYWJj\n-----END CERTIFICATE-----", false},
		{"whitespace only", " \n", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			j := New().(*jenkins)
			defer func() { _ = j.Close() }()
			creds := credentials(ts.URL)
			creds["ca_cert"] = tc.cert
			err := j.Configure(context.Background(), creds)
			if (err == nil) != tc.valid {
				t.Fatalf("valid=%v err=%v", tc.valid, err)
			}
			if err != nil && strings.Contains(err.Error(), "PRIVATE") {
				t.Fatal("certificate input echoed")
			}
		})
	}
}

func TestRetryWaitHonorsCancellation(t *testing.T) {
	requested := make(chan struct{})
	j, _ := fixture(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Retry-After", "2")
		w.WriteHeader(429)
		close(requested)
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		<-requested
		cancel()
	}()
	started := time.Now()
	result, err := j.Execute(ctx, "jenkins_list_jobs", nil)
	if err != nil || result == nil || !result.IsError || time.Since(started) > time.Second {
		t.Fatal("retry wait did not honor context")
	}
}

func TestBoundedCollectionAndStringEdges(t *testing.T) {
	job := `{"name":"` + strings.Repeat("é", 512) + `","_class":"hudson.model.FreeStyleProject","color":"blue","lastBuild":null}`
	cases := []struct {
		name, body string
		limit      int
		valid      bool
	}{
		{"512 characters", `{"jobs":[` + job + `]}`, 1, true},
		{"50 records", `{"jobs":[` + strings.TrimSuffix(strings.Repeat(job+",", 50), ",") + `]}`, 50, true},
		{"51 records", `{"jobs":[` + strings.TrimSuffix(strings.Repeat(job+",", 51), ",") + `]}`, 50, false},
		{"null jobs", `{"jobs":null}`, 1, false},
		{"control field", strings.Replace(jobsFixture, `"color":"blue"`, `"color":"blue\u000aPRIVATE"`, 1), 1, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			j, _ := fixture(t, func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(tc.body))
			})
			result := execute(t, j, "jenkins_list_jobs", map[string]any{"limit": tc.limit})
			if result.IsError == tc.valid {
				t.Fatalf("valid=%v got=%+v", tc.valid, result)
			}
			if tc.valid {
				out := successJSON(t, result)
				if len(out["jobs"].([]any)) > 50 {
					t.Fatal("record cap exceeded")
				}
			}
		})
	}
}

func TestReadOnlySurfaceNeverAttemptsWrites(t *testing.T) {
	var writes, reads atomic.Int32
	j, _ := fixture(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			writes.Add(1)
			w.WriteHeader(403)
			return
		}
		reads.Add(1)
		w.Header().Set("Content-Type", "application/json")
		if strings.Contains(r.URL.Path, "/42/") {
			_, _ = w.Write([]byte(buildFixture))
		} else if strings.Contains(r.URL.Path, "/job/") {
			_, _ = w.Write([]byte(jobFixture))
		} else {
			_, _ = w.Write([]byte(jobsFixture))
		}
	})
	_ = successJSON(t, execute(t, j, "jenkins_list_jobs", nil))
	_ = successJSON(t, execute(t, j, "jenkins_get_job", map[string]any{"job_path": []string{"feature/foo"}}))
	_ = successJSON(t, execute(t, j, "jenkins_get_build", map[string]any{"job_path": []string{"feature/foo"}, "build_number": 42}))
	for _, action := range []string{"build", "build_with_parameters", "stop", "delete_job", "script", "get_logs", "get_config", "list_agents", "list_queue", "get_url"} {
		if result := execute(t, j, mcp.ToolName("jenkins_"+action), nil); !result.IsError {
			t.Fatalf("out-of-scope action %s accepted", action)
		}
	}
	if writes.Load() != 0 || reads.Load() != 3 {
		t.Fatalf("writes=%d reads=%d", writes.Load(), reads.Load())
	}
}

func TestConcurrentConfigureExecuteClose(t *testing.T) {
	j, ts := fixture(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(jobsFixture))
	})
	creds := credentials(ts.URL + "/jenkins")
	creds["ca_cert"] = string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: ts.Certificate().Raw}))
	var wg sync.WaitGroup
	for i := range 9 {
		wg.Go(func() {
			for range 3 {
				switch i % 3 {
				case 0:
					if err := j.Configure(context.Background(), creds); err != nil {
						t.Error(err)
					}
				case 1:
					result, err := j.Execute(context.Background(), "jenkins_list_jobs", nil)
					if err != nil || result == nil {
						t.Error(fmt.Errorf("missing result: %w", err))
					}
				case 2:
					if err := j.Close(); err != nil {
						t.Error(err)
					}
				}
			}
		})
	}
	wg.Wait()
}
