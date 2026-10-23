package linear

import (
	"context"
	"io"
	"testing"

	mcp "github.com/daltoniam/switchboard"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type closeCountingRemote struct {
	token  string
	closed int
}

func (r *closeCountingRemote) Name() string { return "linear" }
func (r *closeCountingRemote) Configure(_ context.Context, creds mcp.Credentials) error {
	r.token = creds["access_token"]
	return nil
}
func (r *closeCountingRemote) Tools() []mcp.ToolDefinition  { return nil }
func (r *closeCountingRemote) Healthy(context.Context) bool { return true }
func (r *closeCountingRemote) Execute(context.Context, mcp.ToolName, map[string]any) (*mcp.ToolResult, error) {
	return &mcp.ToolResult{}, nil
}
func (r *closeCountingRemote) Close() error {
	r.closed++
	return nil
}

func withFakeRemote(t *testing.T) *[]*closeCountingRemote {
	t.Helper()
	var created []*closeCountingRemote
	old := newRemote
	newRemote = func(string) mcp.Integration {
		r := &closeCountingRemote{}
		created = append(created, r)
		return r
	}
	t.Cleanup(func() { newRemote = old })
	return &created
}

func TestClose_EndsRemoteSession(t *testing.T) {
	created := withFakeRemote(t)
	i := New("https://mcp.linear.example")
	require.Implements(t, (*io.Closer)(nil), i)
	require.NoError(t, i.Configure(context.Background(), mcp.Credentials{"mcp_access_token": "tok"}))
	require.True(t, IsRemoteMCP(i))

	require.NoError(t, i.(io.Closer).Close())
	require.Len(t, *created, 1)
	assert.Equal(t, 1, (*created)[0].closed)
	assert.False(t, IsRemoteMCP(i))

	require.NoError(t, i.(io.Closer).Close(), "Close is safe to call again")
	assert.Equal(t, 1, (*created)[0].closed)

	require.NoError(t, i.Configure(context.Background(), mcp.Credentials{"mcp_access_token": "tok2"}))
	require.Len(t, *created, 2, "a Configure after Close starts a new session")
	assert.True(t, IsRemoteMCP(i))
}

func TestClose_WithoutRemoteIsANoop(t *testing.T) {
	i := New()
	require.NoError(t, i.Configure(context.Background(), mcp.Credentials{"api_key": "lin_api_x"}))
	assert.NoError(t, i.(io.Closer).Close())
}

func TestConfigure_SwitchingToAPIKeyEndsRemoteSession(t *testing.T) {
	created := withFakeRemote(t)
	i := New("https://mcp.linear.example")
	require.NoError(t, i.Configure(context.Background(), mcp.Credentials{"mcp_access_token": "tok"}))
	require.NoError(t, i.Configure(context.Background(), mcp.Credentials{"api_key": "lin_api_x"}))
	require.Len(t, *created, 1)
	assert.Equal(t, 1, (*created)[0].closed, "an unused remote session must not stay open")
	assert.False(t, IsRemoteMCP(i))
}
