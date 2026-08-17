package metabase

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	mcp "github.com/daltoniam/switchboard"
	"github.com/daltoniam/switchboard/compact"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestFieldCompactionSpecs_AllParse(t *testing.T) {
	require.NotEmpty(t, fieldCompactionSpecs, "fieldCompactionSpecs should not be empty")
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

func TestFieldCompactionSpecs_OnlyReadTools(t *testing.T) {
	mutationPrefixes := []string{"create", "update", "delete", "add"}
	for toolName := range fieldCompactionSpecs {
		for _, prefix := range mutationPrefixes {
			assert.NotContains(t, toolName, "_"+prefix+"_",
				"mutation tool %q should not have a field compaction spec", toolName)
		}
	}
}

func TestFieldCompactionSpec_ReturnsFieldsForListTool(t *testing.T) {
	m := &metabase{}
	fields, ok := m.CompactSpec("metabase_list_databases")
	require.True(t, ok, "metabase_list_databases should have field compaction spec")
	assert.NotEmpty(t, fields)
}

func TestFieldCompactionSpec_ReturnsFalseForMutationTool(t *testing.T) {
	m := &metabase{}
	_, ok := m.CompactSpec("metabase_create_card")
	assert.False(t, ok, "mutation tools should return false")
}

func TestFieldCompactionSpec_ReturnsFalseForUnknownTool(t *testing.T) {
	m := &metabase{}
	_, ok := m.CompactSpec("metabase_nonexistent")
	assert.False(t, ok, "unknown tools should return false")
}

func TestFieldCompactionSpec_ShapeParity_ListDatabases(t *testing.T) {
	// Metabase /api/database returns {"data": [...]} envelope. The handler must
	// unwrap this so compaction specs (which target top-level fields like "id",
	// "name") operate on the array items directly.
	//
	// This test verifies the handler output shape matches what compaction expects.
	// If compaction produces "{}" or "[]", the handler isn't unwrapping correctly.
	apiEnvelope := `{"data":[{"id":1,"name":"Sample Database","engine":"h2","created_at":"2024-01-01","updated_at":"2024-01-02","timezone":"UTC","features":["basic-aggregations"],"native_permissions":"write"}]}`

	// Simulate what the handler should return (unwrapped array).
	var envelope struct {
		Data json.RawMessage `json:"data"`
	}
	require.NoError(t, json.Unmarshal([]byte(apiEnvelope), &envelope))
	handlerOutput := envelope.Data // should be the array

	fields, ok := fieldCompactionSpecs["metabase_list_databases"]
	require.True(t, ok)

	compacted, err := mcp.CompactJSON(handlerOutput, fields)
	require.NoError(t, err)
	assert.NotEqual(t, "{}", string(compacted), "compacted list_databases should not be empty — shape mismatch")
	assert.NotEqual(t, "[]", string(compacted), "compacted list_databases should not be empty array")
	assert.Contains(t, string(compacted), "Sample Database")
}

func TestListDatabases_UnwrapsDataEnvelope(t *testing.T) {
	// The Metabase /api/database endpoint wraps results in {"data": [...]}.
	// The handler must unwrap this so compaction specs work correctly.
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"data":[{"id":1,"name":"Sample DB","engine":"h2","timezone":"UTC"}]}`))
	}))
	defer ts.Close()

	m := &metabase{apiKey: "test", baseURL: ts.URL, client: ts.Client()}
	result, err := listDatabases(context.Background(), m, nil)
	require.NoError(t, err)
	assert.False(t, result.IsError)

	// Handler output should be the unwrapped array, not the envelope.
	assert.True(t, result.Data[0] == '[', "handler should return array, not envelope object; got: %s", result.Data[:50])
	assert.Contains(t, result.Data, "Sample DB")
}
