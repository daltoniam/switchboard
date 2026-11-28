package panther

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolveInstance(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		want    instance
		wantURL string
	}{
		{
			name:    "SaaS console URL",
			raw:     "https://acme.runpanther.net",
			want:    instance{apiBase: "https://api.acme.runpanther.net", graphqlURL: "https://api.acme.runpanther.net/public/graphql", mcpOrigin: "https://api.acme.runpanther.net"},
			wantURL: "https://acme.runpanther.net",
		},
		{
			name:    "SaaS API URL with GraphQL path and trailing slash",
			raw:     " https://api.acme.runpanther.net/public/graphql/ ",
			want:    instance{apiBase: "https://api.acme.runpanther.net", graphqlURL: "https://api.acme.runpanther.net/public/graphql", mcpOrigin: "https://api.acme.runpanther.net"},
			wantURL: "https://api.acme.runpanther.net",
		},
		{
			name:    "Remote MCP endpoint",
			raw:     "https://api.panther.acme.com/mcp",
			want:    instance{apiBase: "https://api.panther.acme.com", graphqlURL: "https://api.panther.acme.com/public/graphql", mcpOrigin: "https://api.panther.acme.com"},
			wantURL: "https://api.panther.acme.com",
		},
		{
			name:    "custom domain without scheme",
			raw:     "Panther.Acme.com",
			want:    instance{apiBase: "https://api.panther.acme.com", graphqlURL: "https://api.panther.acme.com/public/graphql", mcpOrigin: "https://api.panther.acme.com"},
			wantURL: "https://panther.acme.com",
		},
		{
			name:    "Cloud Connected API URL keeps its /v1 path on the console host",
			raw:     "https://panther.acme.com/v1/public/graphql?x=1#frag",
			want:    instance{apiBase: "https://panther.acme.com/v1", graphqlURL: "https://panther.acme.com/v1/public/graphql", mcpOrigin: "https://api.panther.acme.com"},
			wantURL: "https://panther.acme.com/v1",
		},
		{
			name:    "loopback fixture over http is used as is",
			raw:     "http://127.0.0.1:8080",
			want:    instance{apiBase: "http://127.0.0.1:8080", graphqlURL: "http://127.0.0.1:8080/public/graphql", mcpOrigin: "http://127.0.0.1:8080"},
			wantURL: "http://127.0.0.1:8080",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := resolveInstance(tt.raw)
			require.NoError(t, err)
			tt.want.url = tt.wantURL
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestResolveInstance_Rejects(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want string
	}{
		{name: "empty", raw: "  ", want: "instance_url is required"},
		{name: "plain http to a remote host", raw: "http://acme.runpanther.net", want: "must use https"},
		{name: "other scheme", raw: "ftp://acme.runpanther.net", want: "must use https"},
		{name: "credentials in the URL", raw: "https://user:pass@acme.runpanther.net", want: "invalid instance_url"},
		{name: "no host", raw: "https://", want: "invalid instance_url"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := resolveInstance(tt.raw)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.want)
		})
	}
}

func TestSignInTargetAndNormalize(t *testing.T) {
	origin, resource, err := SignInTarget("https://acme.runpanther.net/")
	require.NoError(t, err)
	assert.Equal(t, "https://api.acme.runpanther.net", origin)
	assert.Equal(t, "https://api.acme.runpanther.net/mcp", resource)

	normalized, err := NormalizeInstanceURL("acme.runpanther.net/")
	require.NoError(t, err)
	assert.Equal(t, "https://acme.runpanther.net", normalized)

	_, err = NormalizeInstanceURL("http://acme.runpanther.net")
	require.Error(t, err)

	base, err := APIBase("https://panther.acme.com/v1")
	require.NoError(t, err)
	assert.Equal(t, "https://panther.acme.com/v1", base)
}
