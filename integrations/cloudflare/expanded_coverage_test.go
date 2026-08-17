package cloudflare

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	mcp "github.com/daltoniam/switchboard"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- Workers AI ---

func TestListAIModels_PassesFilters(t *testing.T) {
	c, ts := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Contains(t, r.URL.Path, "/accounts/test-acct/ai/models/search")
		q := r.URL.Query()
		assert.Equal(t, "llama", q.Get("search"))
		assert.Equal(t, "text-generation", q.Get("task"))
		_, _ = w.Write([]byte(`{"success":true,"result":[{"id":"m1","name":"@cf/meta/llama-3"}]}`))
	}))
	defer ts.Close()
	result, err := c.Execute(context.Background(), "cloudflare_list_ai_models", map[string]any{
		"search": "llama", "task": "text-generation",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "llama-3")
}

func TestRunAIModel_PromptShortcut(t *testing.T) {
	c, ts := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Contains(t, r.URL.Path, "/accounts/test-acct/ai/run/@cf/meta/llama-3")
		body := make([]byte, r.ContentLength)
		_, _ = r.Body.Read(body)
		assert.Contains(t, string(body), `"prompt":"hello"`)
		_, _ = w.Write([]byte(`{"success":true,"result":{"response":"hi"}}`))
	}))
	defer ts.Close()
	result, err := c.Execute(context.Background(), "cloudflare_run_ai_model", map[string]any{
		"model_name": "@cf/meta/llama-3", "prompt": "hello",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "response")
}

func TestRunAIModel_RequiresBody(t *testing.T) {
	c, ts := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("should not hit API when body and prompt are missing")
	}))
	defer ts.Close()
	result, err := c.Execute(context.Background(), "cloudflare_run_ai_model", map[string]any{
		"model_name": "@cf/meta/llama-3",
	})
	require.NoError(t, err)
	assert.True(t, result.IsError)
}

// --- Vectorize ---

func TestListVectorizeIndexes(t *testing.T) {
	c, ts := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Contains(t, r.URL.Path, "/accounts/test-acct/vectorize/v2/indexes")
		_, _ = w.Write([]byte(`{"success":true,"result":[{"name":"idx1","config":{"dimensions":768,"metric":"cosine"}}]}`))
	}))
	defer ts.Close()
	result, err := c.Execute(context.Background(), "cloudflare_list_vectorize_indexes", map[string]any{})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "idx1")
}

func TestQueryVectorizeIndex(t *testing.T) {
	c, ts := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Contains(t, r.URL.Path, "/accounts/test-acct/vectorize/v2/indexes/idx1/query")
		_, _ = w.Write([]byte(`{"success":true,"result":{"matches":[{"id":"a","score":0.9}]}}`))
	}))
	defer ts.Close()
	result, err := c.Execute(context.Background(), "cloudflare_query_vectorize_index", map[string]any{
		"index_name": "idx1",
		"vector":     []any{0.1, 0.2, 0.3},
		"topK":       5,
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "matches")
}

func TestQueryVectorizeIndex_RequiresVector(t *testing.T) {
	c, ts := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("should not hit API")
	}))
	defer ts.Close()
	result, err := c.Execute(context.Background(), "cloudflare_query_vectorize_index", map[string]any{
		"index_name": "idx1",
	})
	require.NoError(t, err)
	assert.True(t, result.IsError)
}

// --- Queues ---

func TestListQueues(t *testing.T) {
	c, ts := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Contains(t, r.URL.Path, "/accounts/test-acct/queues")
		_, _ = w.Write([]byte(`{"success":true,"result":[{"queue_id":"q1","queue_name":"main"}]}`))
	}))
	defer ts.Close()
	result, err := c.Execute(context.Background(), "cloudflare_list_queues", map[string]any{})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "main")
}

func TestSendQueueMessages(t *testing.T) {
	c, ts := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Contains(t, r.URL.Path, "/accounts/test-acct/queues/q1/messages")
		_, _ = w.Write([]byte(`{"success":true,"result":{"sent":2}}`))
	}))
	defer ts.Close()
	result, err := c.Execute(context.Background(), "cloudflare_send_queue_messages", map[string]any{
		"queue_id": "q1",
		"messages": []any{map[string]any{"body": "hello"}, map[string]any{"body": "world"}},
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
}

// --- Hyperdrive ---

func TestListHyperdriveConfigs(t *testing.T) {
	c, ts := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Contains(t, r.URL.Path, "/accounts/test-acct/hyperdrive/configs")
		_, _ = w.Write([]byte(`{"success":true,"result":[{"id":"hd1","name":"pg-main"}]}`))
	}))
	defer ts.Close()
	result, err := c.Execute(context.Background(), "cloudflare_list_hyperdrive_configs", map[string]any{})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "pg-main")
}

// --- Workers extras ---

func TestListWorkerSecrets(t *testing.T) {
	c, ts := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Contains(t, r.URL.Path, "/accounts/test-acct/workers/scripts/api/secrets")
		_, _ = w.Write([]byte(`{"success":true,"result":[{"name":"API_KEY","type":"secret_text"}]}`))
	}))
	defer ts.Close()
	result, err := c.Execute(context.Background(), "cloudflare_list_worker_secrets", map[string]any{
		"script_name": "api",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "API_KEY")
}

func TestGetWorkerSubdomain(t *testing.T) {
	c, ts := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Contains(t, r.URL.Path, "/accounts/test-acct/workers/subdomain")
		_, _ = w.Write([]byte(`{"success":true,"result":{"name":"acme"}}`))
	}))
	defer ts.Close()
	result, err := c.Execute(context.Background(), "cloudflare_get_worker_subdomain", map[string]any{})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "acme")
}

// --- Pages extras ---

func TestCreatePagesProject(t *testing.T) {
	c, ts := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Contains(t, r.URL.Path, "/accounts/test-acct/pages/projects")
		_, _ = w.Write([]byte(`{"success":true,"result":{"name":"site"}}`))
	}))
	defer ts.Close()
	result, err := c.Execute(context.Background(), "cloudflare_create_pages_project", map[string]any{
		"name":              "site",
		"production_branch": "main",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
}

func TestListPagesDomains(t *testing.T) {
	c, ts := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Contains(t, r.URL.Path, "/accounts/test-acct/pages/projects/site/domains")
		_, _ = w.Write([]byte(`{"success":true,"result":[{"id":"d1","name":"example.com","status":"active"}]}`))
	}))
	defer ts.Close()
	result, err := c.Execute(context.Background(), "cloudflare_list_pages_domains", map[string]any{
		"project_name": "site",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "example.com")
}

// --- KV bulk ---

func TestBulkDeleteKVValues(t *testing.T) {
	c, ts := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Contains(t, r.URL.Path, "/accounts/test-acct/storage/kv/namespaces/ns1/bulk/delete")
		var body []string
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		assert.Equal(t, []string{"a", "b"}, body)
		_, _ = w.Write([]byte(`{"success":true,"result":{"successful_key_count":2}}`))
	}))
	defer ts.Close()
	result, err := c.Execute(context.Background(), "cloudflare_bulk_delete_kv_values", map[string]any{
		"namespace_id": "ns1",
		"keys":         []any{"a", "b"},
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
}

// --- Stream / Images ---

func TestListStreamVideos(t *testing.T) {
	c, ts := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Contains(t, r.URL.Path, "/accounts/test-acct/stream")
		_, _ = w.Write([]byte(`{"success":true,"result":[{"uid":"v1","meta":{"name":"intro"}}]}`))
	}))
	defer ts.Close()
	result, err := c.Execute(context.Background(), "cloudflare_list_stream_videos", map[string]any{})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "intro")
}

func TestDeleteImage(t *testing.T) {
	c, ts := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "DELETE", r.Method)
		assert.Contains(t, r.URL.Path, "/accounts/test-acct/images/v1/img1")
		_, _ = w.Write([]byte(`{"success":true,"result":{}}`))
	}))
	defer ts.Close()
	result, err := c.Execute(context.Background(), "cloudflare_delete_image", map[string]any{
		"image_id": "img1",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
}

// --- Zero Trust Access ---

func TestListAccessApps(t *testing.T) {
	c, ts := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Contains(t, r.URL.Path, "/accounts/test-acct/access/apps")
		_, _ = w.Write([]byte(`{"success":true,"result":[{"id":"a1","name":"internal","domain":"internal.acme.com"}]}`))
	}))
	defer ts.Close()
	result, err := c.Execute(context.Background(), "cloudflare_list_access_apps", map[string]any{})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "internal")
}

func TestListAccessAppPolicies(t *testing.T) {
	c, ts := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Contains(t, r.URL.Path, "/accounts/test-acct/access/apps/a1/policies")
		_, _ = w.Write([]byte(`{"success":true,"result":[{"id":"p1","name":"engineers","decision":"allow"}]}`))
	}))
	defer ts.Close()
	result, err := c.Execute(context.Background(), "cloudflare_list_access_app_policies", map[string]any{
		"app_id": "a1",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "engineers")
}

// --- Tunnels ---

func TestListTunnels(t *testing.T) {
	c, ts := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Contains(t, r.URL.Path, "/accounts/test-acct/cfd_tunnel")
		_, _ = w.Write([]byte(`{"success":true,"result":[{"id":"t1","name":"home","status":"healthy"}]}`))
	}))
	defer ts.Close()
	result, err := c.Execute(context.Background(), "cloudflare_list_tunnels", map[string]any{})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "home")
}

// --- Email Routing ---

func TestListEmailRoutingRules(t *testing.T) {
	c, ts := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Contains(t, r.URL.Path, "/zones/z1/email/routing/rules")
		_, _ = w.Write([]byte(`{"success":true,"result":[{"tag":"r1","name":"catch-all","enabled":true}]}`))
	}))
	defer ts.Close()
	result, err := c.Execute(context.Background(), "cloudflare_list_email_routing_rules", map[string]any{
		"zone_id": "z1",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "catch-all")
}

func TestGetEmailRoutingSettings(t *testing.T) {
	c, ts := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.True(t, strings.HasSuffix(r.URL.Path, "/zones/z1/email/routing"))
		_, _ = w.Write([]byte(`{"success":true,"result":{"enabled":true,"name":"acme"}}`))
	}))
	defer ts.Close()
	result, err := c.Execute(context.Background(), "cloudflare_get_email_routing_settings", map[string]any{
		"zone_id": "z1",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "enabled")
}

// --- Logpush ---

func TestListLogpushJobs(t *testing.T) {
	c, ts := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Contains(t, r.URL.Path, "/accounts/test-acct/logpush/jobs")
		_, _ = w.Write([]byte(`{"success":true,"result":[{"id":1,"dataset":"http_requests","enabled":true}]}`))
	}))
	defer ts.Close()
	result, err := c.Execute(context.Background(), "cloudflare_list_logpush_jobs", map[string]any{})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "http_requests")
}

func TestGetLogpushJob(t *testing.T) {
	c, ts := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Contains(t, r.URL.Path, "/accounts/test-acct/logpush/jobs/42")
		_, _ = w.Write([]byte(`{"success":true,"result":{"id":42,"dataset":"http_requests"}}`))
	}))
	defer ts.Close()
	result, err := c.Execute(context.Background(), "cloudflare_get_logpush_job", map[string]any{
		"job_id": 42,
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
}

// --- Page Rules ---

func TestListPageRules(t *testing.T) {
	c, ts := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Contains(t, r.URL.Path, "/zones/z1/pagerules")
		_, _ = w.Write([]byte(`{"success":true,"result":[{"id":"pr1","status":"active","priority":1}]}`))
	}))
	defer ts.Close()
	result, err := c.Execute(context.Background(), "cloudflare_list_page_rules", map[string]any{
		"zone_id": "z1",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "pr1")
}

// --- Notifications ---

func TestListNotificationPolicies(t *testing.T) {
	c, ts := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Contains(t, r.URL.Path, "/accounts/test-acct/alerting/v3/policies")
		_, _ = w.Write([]byte(`{"success":true,"result":[{"id":"np1","name":"prod-alerts","enabled":true}]}`))
	}))
	defer ts.Close()
	result, err := c.Execute(context.Background(), "cloudflare_list_notification_policies", map[string]any{})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "prod-alerts")
}

// --- API Tokens ---

func TestListAPITokens(t *testing.T) {
	c, ts := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.True(t, strings.HasSuffix(r.URL.Path, "/user/tokens"))
		_, _ = w.Write([]byte(`{"success":true,"result":[{"id":"tok1","name":"ci","status":"active"}]}`))
	}))
	defer ts.Close()
	result, err := c.Execute(context.Background(), "cloudflare_list_api_tokens", map[string]any{})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "ci")
}

func TestGetAPIToken(t *testing.T) {
	c, ts := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.True(t, strings.HasSuffix(r.URL.Path, "/user/tokens/tok1"))
		_, _ = w.Write([]byte(`{"success":true,"result":{"id":"tok1","name":"ci","status":"active"}}`))
	}))
	defer ts.Close()
	result, err := c.Execute(context.Background(), "cloudflare_get_api_token", map[string]any{
		"token_id": "tok1",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Data, "ci")
}

func TestDeleteAPIToken(t *testing.T) {
	c, ts := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "DELETE", r.Method)
		assert.True(t, strings.HasSuffix(r.URL.Path, "/user/tokens/tok1"))
		_, _ = w.Write([]byte(`{"success":true,"result":{}}`))
	}))
	defer ts.Close()
	result, err := c.Execute(context.Background(), "cloudflare_delete_api_token", map[string]any{
		"token_id": "tok1",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
}

func TestListLogpushJobs_DoesNotCompactDestinationCredentials(t *testing.T) {
	fields, ok := fieldCompactionSpecs["cloudflare_list_logpush_jobs"]
	require.True(t, ok)

	var payload any
	require.NoError(t, json.Unmarshal([]byte(`{
		"result":[{
			"id":1,
			"dataset":"http_requests",
			"enabled":true,
			"name":"prod",
			"destination_conf":"s3://bucket/path?access-key-id=AKIAEXAMPLE&secret-access-key=supersecret",
			"logpull_options":"fields=RayID"
		}]
	}`), &payload))

	compacted := mcp.CompactAny(payload, fields)
	encoded, err := json.Marshal(compacted)
	require.NoError(t, err)
	assert.NotContains(t, string(encoded), "destination_conf")
	assert.NotContains(t, string(encoded), "supersecret")
	assert.Contains(t, string(encoded), "http_requests")
}
