package web

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	mcp "github.com/daltoniam/switchboard"
	"github.com/daltoniam/switchboard/integrations/prometheus"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
)

const livePrometheusImage = "prom/prometheus:v3.5.0@sha256:63805ebb8d2b3920190daf1cb14a60871b16fd38bed42b857a3182bc621f4996"

func init() {
	registerLiveSpec(liveSpec{
		Integration: "prometheus",
		Tier:        liveContainer,
		Adapter:     prometheus.New(),
		Provision:   livePrometheusProvision,
		Cases: []liveCase{
			{Tool: "prometheus_query", Args: map[string]any{"query": `up{job="live-prometheus"}`}, Expect: []string{"live-prometheus", "vector"}},
			{Tool: "prometheus_query_range", Args: map[string]any{"query": `up{job="live-prometheus"}`, "start": "${start}", "end": "${end}", "step": "1s"}, Expect: []string{"live-prometheus", "matrix", "timestamps"}},
			{Tool: "prometheus_list_alerts", Args: map[string]any{"state": "firing"}, Expect: []string{"LivePrometheusUp", "firing", "Live scrape is healthy"}},
			{Tool: "prometheus_list_rules", Args: map[string]any{"group": "live-rules"}, Expect: []string{"live-rules", "LivePrometheusUp", "live:up", "health"}},
			{Tool: "prometheus_list_targets", Args: map[string]any{"scrape_pool": "live-prometheus"}, Expect: []string{"live-prometheus", "up", "scrapeInterval"}},
			{Tool: "prometheus_search_metrics", Args: map[string]any{"match": []any{`prometheus_build_info{job="live-prometheus"}`}, "limit": 100}, Expect: []string{"prometheus_build_info", "metadata_complete"}},
			{Tool: "prometheus_list_label_values", Args: map[string]any{"label": "job", "match": []any{"up"}}, Expect: []string{"live-prometheus"}},
			{Tool: "prometheus_list_series", Args: map[string]any{"match": []any{`up{job="live-prometheus"}`}}, Expect: []string{"__name__", "live-prometheus", "up"}},
			{Tool: "prometheus_server_info", Expect: []string{"3.5.0", "reloadConfigSuccess", "build", "runtime"}},
		},
	})
}

func livePrometheusProvision(t *testing.T) liveProvision {
	password := "live-reader-password"
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.MinCost)
	require.NoError(t, err)
	seed := fmt.Sprintf(`set -eu
cat > /prometheus/web.yml <<'YAML'
basic_auth_users:
  reader: %s
YAML
cat > /prometheus/prometheus.yml <<'YAML'
global:
  scrape_interval: 1s
  evaluation_interval: 1s
rule_files:
  - /prometheus/rules.yml
scrape_configs:
  - job_name: live-prometheus
    basic_auth:
      username: reader
      password: live-reader-password
    static_configs:
      - targets: ['127.0.0.1:9090']
YAML
cat > /prometheus/rules.yml <<'YAML'
groups:
  - name: live-rules
    rules:
      - record: live:up
        expr: up{job="live-prometheus"}
      - alert: LivePrometheusUp
        expr: up{job="live-prometheus"} == 1
        labels:
          severity: info
        annotations:
          summary: Live scrape is healthy
YAML
exec /bin/prometheus --config.file=/prometheus/prometheus.yml --web.config.file=/prometheus/web.yml --storage.tsdb.path=/prometheus/data
`, hash)
	container := liveStartContainer(t, liveDockerOptions{
		Image:      livePrometheusImage,
		Ports:      []string{"9090/tcp"},
		Entrypoint: "/bin/sh",
		Cmd:        []string{"-c", seed},
	})
	host, port := container.Address(t, "9090/tcp")
	baseURL := fmt.Sprintf("http://%s:%s", host, port)
	client := &http.Client{Timeout: 5 * time.Second}
	liveEventually(t, time.Minute, func() error {
		request, err := http.NewRequestWithContext(t.Context(), http.MethodGet, baseURL+"/-/ready", nil)
		if err != nil {
			return err
		}
		request.SetBasicAuth("reader", password)
		response, err := client.Do(request)
		if err != nil {
			return err
		}
		defer func() { _ = response.Body.Close() }()
		if response.StatusCode != http.StatusOK {
			return fmt.Errorf("Prometheus not ready: %d", response.StatusCode)
		}
		return nil
	})
	adapter := prometheus.New()
	credentials := mcp.Credentials{"base_url": baseURL, "username": "reader", "password": password}
	require.NoError(t, adapter.Configure(context.Background(), credentials))
	started := time.Now().Add(-time.Minute)
	liveEventually(t, time.Minute, func() error {
		result, err := adapter.Execute(t.Context(), "prometheus_list_alerts", nil)
		if err != nil {
			return err
		}
		if result.IsError || !liveContainsAll(result.Data, []string{"LivePrometheusUp", "firing"}) {
			return fmt.Errorf("waiting for self-scrape and rule evaluation")
		}
		return nil
	})
	// The range must hold real samples. Times are sent with whole-second
	// precision, so an end taken in the same second as the first scrape is
	// rounded down to before it and the range comes back empty. Poll the
	// exact window the case will use until it returns data.
	var start, end string
	liveEventually(t, time.Minute, func() error {
		start = started.UTC().Format(time.RFC3339)
		end = time.Now().UTC().Format(time.RFC3339)
		result, err := adapter.Execute(t.Context(), "prometheus_query_range", map[string]any{
			"query": `up{job="live-prometheus"}`, "start": start, "end": end, "step": "1s",
		})
		if err != nil {
			return err
		}
		if result.IsError || !liveContainsAll(result.Data, []string{"live-prometheus", "timestamps"}) {
			return fmt.Errorf("waiting for range samples")
		}
		return nil
	})
	return liveProvision{
		Credentials: map[string]string{"base_url": baseURL, "username": "reader", "password": password},
		Values: map[string]string{
			"start": start,
			"end":   end,
		},
	}
}
