package sourcebot

import (
	"encoding/json"
	"io"
	"net/http"
	"testing"

	mcp "github.com/daltoniam/switchboard"
	"github.com/daltoniam/switchboard/compact"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestFieldCompactionSpecs_AllParse(t *testing.T) {
	require.NotEmpty(t, fieldCompactionSpecs)
}

func TestFieldCompactionSpecs_NoDuplicateTools(t *testing.T) {
	var sf compact.SpecFile
	require.NoError(t, yaml.Unmarshal(compactYAML, &sf))
	assert.Equal(t, len(sf.Tools), len(fieldCompactionSpecs))
}

func TestFieldCompactionSpecs_NoOrphanSpecs(t *testing.T) {
	for toolName := range fieldCompactionSpecs {
		_, ok := dispatch[toolName]
		assert.True(t, ok, "field compaction spec for %q has no dispatch handler", toolName)
	}
}

func TestFieldCompactionSpecs_EveryReadToolHasSpec(t *testing.T) {
	for _, tool := range tools {
		if tool.Access != mcp.AccessRead {
			continue
		}
		_, ok := fieldCompactionSpecs[tool.Name]
		assert.True(t, ok, "read tool %s has no compaction spec", tool.Name)
	}
}

func TestCompactSpec_Lookup(t *testing.T) {
	s := &sourcebot{}
	fields, ok := s.CompactSpec("sourcebot_search_code")
	require.True(t, ok)
	assert.NotEmpty(t, fields)
	_, ok = s.CompactSpec("sourcebot_nonexistent")
	assert.False(t, ok)
	_, ok = s.MaxBytes("sourcebot_nonexistent")
	assert.False(t, ok)
}

// TestFieldCompactionSpecs_ShapeParity runs each read tool against a fixture
// shaped like Sourcebot's response and checks the compacted output keeps
// the fields an agent needs and drops the noise.
func TestFieldCompactionSpecs_ShapeParity(t *testing.T) {
	tests := []struct {
		tool     string
		args     map[string]any
		fixture  string
		header   map[string]string
		keep     []string
		drop     []string
		wantKeys []string
	}{
		{
			tool: "sourcebot_search_code", args: map[string]any{"query": "ComputeInvoiceTotal"}, fixture: searchFixture,
			keep:     []string{"internal/billing/invoice.go", "github.com/acme/api", "https://github.com/acme/api/blob/main", "sourcebot.example.com/browse", "func ComputeInvoiceTotal", `"startLine":2`, `"matchLines":[3,4]`, `"language":"Go"`, "refs/heads/main"},
			drop:     []string{"byteOffset", "repositoryId", "contentBytesLoaded", "repositoryInfo", "symbols"},
			wantKeys: []string{"query", "matchCount", "totalMatchCount", "fileCount", "isSearchExhaustive", "files"},
		},
		{
			tool: "sourcebot_list_repos", fixture: `[{"codeHostType":"github","repoId":1,"repoName":"github.com/acme/api","repoDisplayName":"acme/api","webUrl":"https://sourcebot.example.com/browse/github.com/acme/api","externalWebUrl":"https://github.com/acme/api","imageUrl":"https://avatars.example.com/acme.png","indexedAt":"2026-01-02T00:00:00Z","pushedAt":"2026-01-01T00:00:00Z","defaultBranch":"main","isFork":false,"isArchived":false}]`,
			header:   map[string]string{"X-Total-Count": "1"},
			keep:     []string{"github.com/acme/api", "https://github.com/acme/api", `"defaultBranch":"main"`, "pushedAt", "indexedAt", `"isArchived":false`},
			drop:     []string{"imageUrl", "repoId", "repoDisplayName"},
			wantKeys: []string{"repos", "totalCount", "page", "hasMore"},
		},
		{
			tool: "sourcebot_read_file", args: map[string]any{"repo": "github.com/acme/api", "path": "internal/billing/invoice.go"}, fixture: sourceFixture,
			keep:     []string{"package billing", `"totalLines":5`, `"startLine":1`, `"endLine":5`, "https://github.com/acme/api/blob/main/internal/billing/invoice.go"},
			wantKeys: []string{"repo", "path", "language", "webUrl", "externalWebUrl", "totalLines", "startLine", "endLine", "source"},
		},
		{
			tool: "sourcebot_list_tree", args: map[string]any{"repo": "github.com/acme/api"}, fixture: `{"tree":{"name":"root","path":"","type":"tree","children":[{"name":"cmd","path":"cmd","type":"tree","children":[]},{"name":"README.md","path":"README.md","type":"blob","children":[]}]}}`,
			keep:     []string{`{"path":"cmd","type":"tree"}`, `{"path":"README.md","type":"blob"}`, `"totalReturned":2`},
			drop:     []string{"parentPath", "depth"},
			wantKeys: []string{"repo", "ref", "path", "entries", "totalReturned", "truncated"},
		},
		{
			tool: "sourcebot_list_commits", args: map[string]any{"repo": "github.com/acme/api"}, fixture: `[{"hash":"abc123","date":"2026-01-02T03:04:05Z","message":"Add invoice totals","refs":"HEAD -> main","body":"Totals now include tax.","authorName":"Ada Lovelace","authorEmail":"ada@example.com"}]`,
			header:   map[string]string{"X-Total-Count": "1"},
			keep:     []string{"abc123", "Add invoice totals", "Ada Lovelace", "ada@example.com", "Totals now include tax.", `"refs":"HEAD -`},
			wantKeys: []string{"commits", "totalCount", "page", "hasMore"},
		},
		{
			tool: "sourcebot_find_references", args: map[string]any{"symbol": "ComputeInvoiceTotal", "language": "Go"}, fixture: symbolFixture,
			keep:     []string{"internal/billing/invoice.go", `"line":4`, "func ComputeInvoiceTotal"},
			drop:     []string{"repositoryId", "byteOffset"},
			wantKeys: []string{"symbol", "matchCount", "files"},
		},
		{
			tool: "sourcebot_find_definitions", args: map[string]any{"symbol": "ComputeInvoiceTotal", "language": "Go"}, fixture: symbolFixture,
			keep:     []string{"internal/billing/invoice.go", `"line":4`},
			drop:     []string{"repositoryId", "byteOffset"},
			wantKeys: []string{"symbol", "matchCount", "files"},
		},
		{
			tool: "sourcebot_list_language_models", fixture: `[{"provider":"anthropic","model":"claude-sonnet","displayName":"Claude"}]`,
			keep:     []string{"anthropic", "claude-sonnet", "Claude"},
			wantKeys: []string{"models"},
		},
		{
			tool: "sourcebot_ask_codebase", args: map[string]any{"query": "How are totals computed?"}, fixture: `{"answer":"See ComputeInvoiceTotal.","chatId":"c1","chatUrl":"https://sourcebot.example.com/chat/c1","languageModel":{"provider":"anthropic","model":"claude-sonnet","displayName":"Claude"}}`,
			keep:     []string{"See ComputeInvoiceTotal.", "https://sourcebot.example.com/chat/c1", "claude-sonnet"},
			drop:     []string{"chatId", "displayName"},
			wantKeys: []string{"answer", "chatUrl", "languageModel"},
		},
	}
	covered := map[string]bool{}
	for _, tt := range tests {
		covered[tt.tool] = true
		t.Run(tt.tool, func(t *testing.T) {
			s := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
				for k, v := range tt.header {
					w.Header().Set(k, v)
				}
				_, _ = io.WriteString(w, tt.fixture)
			})
			result := execute(t, s, tt.tool, tt.args)
			require.False(t, result.IsError, result.Data)
			fields, ok := fieldCompactionSpecs[mcp.ToolName(tt.tool)]
			require.True(t, ok)
			compacted, err := mcp.CompactJSON([]byte(result.Data), fields)
			require.NoError(t, err)
			text := string(compacted)
			for _, want := range tt.keep {
				assert.Contains(t, text, want)
			}
			for _, unwanted := range tt.drop {
				assert.NotContains(t, text, unwanted)
			}
			var top map[string]any
			require.NoError(t, json.Unmarshal(compacted, &top))
			for _, key := range tt.wantKeys {
				assert.Contains(t, top, key)
			}
		})
	}
	for name := range fieldCompactionSpecs {
		assert.True(t, covered[string(name)], "compaction spec for %s has no shape test", name)
	}
}
