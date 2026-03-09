package notion

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- buildSetOp ---

func TestBuildSetOp_CreatesSetCommandWithPathAndArgs(t *testing.T) {
	op := buildSetOp("block", "abc-123", []string{"properties", "title"}, map[string]any{"value": "Hello"})

	assert.Equal(t, "set", op.Command)
	assert.Equal(t, "block", op.Table)
	assert.Equal(t, "abc-123", op.ID)
	assert.Equal(t, []string{"properties", "title"}, op.Path)
}

// --- buildUpdateOp ---

func TestBuildUpdateOp_CreatesUpdateCommandWithPathAndArgs(t *testing.T) {
	op := buildUpdateOp("block", "abc", []string{"format"}, map[string]any{"page_icon": "🎉"})

	assert.Equal(t, "update", op.Command)
	assert.Equal(t, "block", op.Table)
}

// --- buildListAfterOp ---

func TestBuildListAfterOp_CreatesListAfterCommandForChildInsertion(t *testing.T) {
	op := buildListAfterOp("block", "parent-1", []string{"content"}, map[string]any{"id": "child-1"})

	assert.Equal(t, "listAfter", op.Command)
	assert.Equal(t, "block", op.Table)
	assert.Equal(t, "parent-1", op.ID)
}

// --- buildListRemoveOp ---

func TestBuildListRemoveOp_CreatesListRemoveCommandForChildRemoval(t *testing.T) {
	op := buildListRemoveOp("block", "parent-1", []string{"content"}, map[string]any{"id": "child-1"})

	assert.Equal(t, "listRemove", op.Command)
}

// --- buildTransaction ---

func TestBuildTransaction_WrapsOpsInTransactionEnvelope(t *testing.T) {
	ops := []op{
		buildSetOp("block", "b1", []string{}, map[string]any{"type": "page"}),
		buildListAfterOp("block", "parent", []string{"content"}, map[string]any{"id": "b1"}),
	}

	tx := buildTransaction(ops)
	assert.Len(t, tx.Operations, 2)
	assert.Equal(t, "set", tx.Operations[0].Command)
	assert.Equal(t, "listAfter", tx.Operations[1].Command)
}

func TestBuildTransaction_SerializesToExpectedJSON(t *testing.T) {
	ops := []op{buildSetOp("block", "id-1", []string{"type"}, "page")}
	tx := buildTransaction(ops)

	data, err := json.Marshal(tx)
	require.NoError(t, err)

	var parsed map[string]any
	require.NoError(t, json.Unmarshal(data, &parsed))
	operations := parsed["operations"].([]any)
	assert.Len(t, operations, 1)

	first := operations[0].(map[string]any)
	assert.Equal(t, "set", first["command"])
	assert.Equal(t, "block", first["table"])
	assert.Equal(t, "id-1", first["id"])
}

// --- submitTransaction ---

func TestSubmitTransaction_PostsToCorrectEndpointWithOps(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/api/v3/submitTransaction", r.URL.Path)

		var tx transaction
		err := json.NewDecoder(r.Body).Decode(&tx)
		require.NoError(t, err)
		assert.Len(t, tx.Operations, 1)
		assert.Equal(t, "set", tx.Operations[0].Command)

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{}`))
	}))
	defer ts.Close()

	n := &notion{tokenV2: "test-token", baseURL: ts.URL, client: ts.Client()}
	ops := []op{buildSetOp("block", "b1", []string{"type"}, "page")}

	data, err := submitTransaction(context.Background(), n, ops)
	require.NoError(t, err)
	assert.Equal(t, "{}", string(data))
}

// --- newBlockID ---

func TestNewBlockID_ReturnsValidUUID(t *testing.T) {
	id := newBlockID()
	assert.Len(t, id, 36) // UUID v4 format: 8-4-4-4-12
	assert.Contains(t, id, "-")
}

func TestNewBlockID_ReturnsUniqueValues(t *testing.T) {
	id1 := newBlockID()
	id2 := newBlockID()
	assert.NotEqual(t, id1, id2)
}
