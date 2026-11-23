package jenkins

import (
	"context"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"

	mcp "github.com/daltoniam/switchboard"
)

func TestInvalidReconfigureClearsPreviousCredentials(t *testing.T) {
	var calls atomic.Int32
	j, _ := fixture(t, func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(jobsFixture))
	})
	if !j.Healthy(context.Background()) {
		t.Fatal("expected initial configuration to work")
	}
	if err := j.Configure(context.Background(), credentials("http://invalid.example.com")); err == nil {
		t.Fatal("invalid configuration accepted")
	}
	if j.Healthy(context.Background()) || calls.Load() != 1 {
		t.Fatal("invalid reconfiguration must not retain old credentials or make requests")
	}
}

func TestCredentialLengthBounds(t *testing.T) {
	for key, limit := range map[string]int{"username": 512, "api_token": 4096} {
		t.Run(key, func(t *testing.T) {
			j := New().(*jenkins)
			defer func() { _ = j.Close() }()
			creds := credentials("https://jenkins.example.com")
			creds[key] = strings.Repeat("x", limit)
			if err := j.Configure(context.Background(), creds); err != nil {
				t.Fatal(err)
			}
			creds[key] += "x"
			if err := j.Configure(context.Background(), creds); err == nil {
				t.Fatalf("oversized %s accepted", key)
			}
		})
	}
}

func TestRequiredProviderStateNeverFabricated(t *testing.T) {
	cases := []struct {
		name        string
		tool        mcp.ToolName
		args        map[string]any
		body, field string
	}{
		{"job buildable", "jenkins_get_job", map[string]any{"job_path": []string{"feature/foo"}}, jobFixture, `"buildable":true,`},
		{"job inQueue", "jenkins_get_job", map[string]any{"job_path": []string{"feature/foo"}}, jobFixture, `"inQueue":false,`},
		{"build timestamp", "jenkins_get_build", map[string]any{"job_path": []string{"job"}, "build_number": 42}, buildFixture, `"timestamp":1700000000000,`},
		{"build duration", "jenkins_get_build", map[string]any{"job_path": []string{"job"}, "build_number": 42}, buildFixture, `"duration":1234,`},
		{"build estimate", "jenkins_get_build", map[string]any{"job_path": []string{"job"}, "build_number": 42}, buildFixture, `"estimatedDuration":5678,`},
		{"last build timestamp", "jenkins_list_jobs", nil, jobsFixture, `"timestamp":1700000000000,`},
		{"recent duration", "jenkins_get_job", map[string]any{"job_path": []string{"feature/foo"}}, jobFixture, `"duration":0,`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			body := strings.Replace(tc.body, tc.field, "", 1)
			j, _ := fixture(t, func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(body))
			})
			if result := execute(t, j, tc.tool, tc.args); !result.IsError {
				t.Fatal("missing provider state fabricated as zero/false")
			}
		})
	}
}

func TestUnknownEstimatedDurationPreserved(t *testing.T) {
	j, _ := fixture(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(strings.Replace(buildFixture, `"estimatedDuration":5678`, `"estimatedDuration":-1`, 1)))
	})
	out := successJSON(t, execute(t, j, "jenkins_get_build", map[string]any{"job_path": []string{"job"}, "build_number": 42}))
	if out["estimatedDuration"] != float64(-1) {
		t.Fatal("Jenkins unknown-estimate sentinel must be preserved")
	}
}

func TestCompletedNullResultIsValid(t *testing.T) {
	j, _ := fixture(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(strings.Replace(buildFixture, `"building":true`, `"building":false`, 1)))
	})
	out := successJSON(t, execute(t, j, "jenkins_get_build", map[string]any{"job_path": []string{"job"}, "build_number": 42}))
	if out["phase"] != "completed" || out["result"] != nil {
		t.Fatal("phase should depend on building, not result")
	}
}
