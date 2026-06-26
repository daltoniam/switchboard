package postgres

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	mcp "github.com/daltoniam/switchboard"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeTunneld returns an httptest server mimicking tunneld's /internal/query
// endpoint. The handler is supplied by each test.
func fakeTunneld(t *testing.T, handler http.HandlerFunc) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/internal/query", handler)
	return httptest.NewServer(mux)
}

func TestNewTunnelRunner_Validation(t *testing.T) {
	_, err := newTunnelRunner("", "org-1", "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "tunnel_url is required")

	_, err = newTunnelRunner("http://x", "", "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "org is required")
}

func TestNewTunnelRunner_EndpointSuffix(t *testing.T) {
	r, err := newTunnelRunner("http://tunneld.svc:8092", "org-1", "tok")
	require.NoError(t, err)
	assert.Equal(t, "http://tunneld.svc:8092/internal/query", r.endpoint)

	r, err = newTunnelRunner("http://tunneld.svc:8092/internal/query/", "org-1", "tok")
	require.NoError(t, err)
	assert.Equal(t, "http://tunneld.svc:8092/internal/query", r.endpoint)
}

func TestTunnelRunner_QueryRows_Transform(t *testing.T) {
	srv := fakeTunneld(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "Bearer secret", r.Header.Get("Authorization"))
		var req tunnelQueryRequest
		require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
		assert.Equal(t, "org-1", req.OrgID)
		assert.Equal(t, "SELECT id, name FROM users", req.SQL)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(tunnelQueryResponse{
			Columns:   []string{"id", "name"},
			Rows:      [][]any{{float64(1), "alice"}, {float64(2), "bob"}},
			Truncated: false,
		})
	})
	defer srv.Close()

	r, err := newTunnelRunner(srv.URL, "org-1", "secret")
	require.NoError(t, err)

	data, err := r.queryRows(context.Background(), "SELECT id, name FROM users")
	require.NoError(t, err)

	var got []map[string]any
	require.NoError(t, json.Unmarshal(data, &got))
	require.Len(t, got, 2)
	assert.Equal(t, "alice", got[0]["name"])
	assert.Equal(t, float64(1), got[0]["id"])
	assert.Equal(t, "bob", got[1]["name"])
}

func TestTunnelRunner_QueryRows_Empty(t *testing.T) {
	srv := fakeTunneld(t, func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(tunnelQueryResponse{Columns: []string{"id"}, Rows: nil})
	})
	defer srv.Close()

	r, err := newTunnelRunner(srv.URL, "org-1", "")
	require.NoError(t, err)

	data, err := r.queryRows(context.Background(), "SELECT id FROM users WHERE false")
	require.NoError(t, err)
	assert.JSONEq(t, `[]`, string(data))
}

func TestTunnelRunner_QueryRow(t *testing.T) {
	srv := fakeTunneld(t, func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(tunnelQueryResponse{
			Columns: []string{"count"},
			Rows:    [][]any{{float64(42)}},
		})
	})
	defer srv.Close()

	r, err := newTunnelRunner(srv.URL, "org-1", "")
	require.NoError(t, err)

	data, err := r.queryRow(context.Background(), "SELECT count(*) FROM users")
	require.NoError(t, err)
	assert.JSONEq(t, `{"count":42}`, string(data))
}

func TestTunnelRunner_QueryRow_Empty(t *testing.T) {
	srv := fakeTunneld(t, func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(tunnelQueryResponse{Columns: []string{"x"}, Rows: nil})
	})
	defer srv.Close()

	r, err := newTunnelRunner(srv.URL, "org-1", "")
	require.NoError(t, err)

	data, err := r.queryRow(context.Background(), "SELECT x FROM t WHERE false")
	require.NoError(t, err)
	assert.JSONEq(t, `{}`, string(data))
}

func TestTunnelRunner_ServerError(t *testing.T) {
	srv := fakeTunneld(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(tunnelQueryResponse{Error: "relation \"nope\" does not exist"})
	})
	defer srv.Close()

	r, err := newTunnelRunner(srv.URL, "org-1", "")
	require.NoError(t, err)

	_, err = r.queryRows(context.Background(), "SELECT * FROM nope")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "does not exist")
}

func TestTunnelRunner_Exec_ReadOnly(t *testing.T) {
	r, err := newTunnelRunner("http://x", "org-1", "")
	require.NoError(t, err)
	_, err = r.exec(context.Background(), "DELETE FROM users")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "read-only")
}

func TestTunnelRunner_Ping(t *testing.T) {
	var gotSQL string
	srv := fakeTunneld(t, func(w http.ResponseWriter, r *http.Request) {
		var req tunnelQueryRequest
		_ = json.NewDecoder(r.Body).Decode(&req)
		gotSQL = req.SQL
		_ = json.NewEncoder(w).Encode(tunnelQueryResponse{Columns: []string{"?column?"}, Rows: [][]any{{float64(1)}}})
	})
	defer srv.Close()

	r, err := newTunnelRunner(srv.URL, "org-1", "")
	require.NoError(t, err)
	require.NoError(t, r.ping(context.Background()))
	assert.Equal(t, "SELECT 1", gotSQL)
}

func TestConfigure_AgentMode(t *testing.T) {
	srv := fakeTunneld(t, func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(tunnelQueryResponse{Columns: []string{"?column?"}, Rows: [][]any{{float64(1)}}})
	})
	defer srv.Close()

	p := &postgres{conns: make(map[string]*pgConn)}
	err := p.Configure(context.Background(), mcp.Credentials{
		"mode":       "agent",
		"tunnel_url": srv.URL,
		"org":        "9a7331b4-73b5-457b-affd-4592962ab6f2",
	})
	require.NoError(t, err)

	conn, ok := p.conns["default"]
	require.True(t, ok)
	assert.True(t, conn.readOnly)
	assert.Equal(t, "agent:9a7331b4-73b5-457b-affd-4592962ab6f2", conn.host)
	assert.True(t, p.Healthy(context.Background()))
}

func TestConfigure_AgentMode_MissingTunnelURL(t *testing.T) {
	p := &postgres{conns: make(map[string]*pgConn)}
	err := p.Configure(context.Background(), mcp.Credentials{"mode": "agent", "org": "org-1"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "tunnel_url is required")
}
