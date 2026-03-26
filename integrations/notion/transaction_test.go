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

// --- buildSetParentOp ---

func TestBuildSetParentOp_CreatesPointerFormat(t *testing.T) {
	op := buildSetParentOp("space-1", "page-1", "collection-1", "collection")

	assert.Equal(t, "setParent", op.Command)
	require.NotNil(t, op.Pointer)
	assert.Equal(t, "block", op.Pointer.Table)
	assert.Equal(t, "page-1", op.Pointer.ID)
	assert.Equal(t, "space-1", op.Pointer.SpaceID)
	assert.Empty(t, op.Table)
	assert.Empty(t, op.ID)

	args, ok := op.Args.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "collection-1", args["parentId"])
	assert.Equal(t, "collection", args["parentTable"])
}

func TestOp_OmitsTableAndID_WhenPointerSet(t *testing.T) {
	op := buildSetParentOp("space-1", "page-1", "coll-1", "collection")

	data, err := json.Marshal(op)
	require.NoError(t, err)

	var parsed map[string]any
	require.NoError(t, json.Unmarshal(data, &parsed))

	// Must have pointer, must NOT have top-level table/id
	assert.Contains(t, parsed, "pointer")
	assert.NotContains(t, parsed, "table")
	assert.NotContains(t, parsed, "id")

	ptr := parsed["pointer"].(map[string]any)
	assert.Equal(t, "block", ptr["table"])
	assert.Equal(t, "page-1", ptr["id"])
	assert.Equal(t, "space-1", ptr["spaceId"])
}

func TestOp_IncludesTableAndID_WhenFlatFormat(t *testing.T) {
	op := buildSetOp("block", "b1", []string{"type"}, "page")

	data, err := json.Marshal(op)
	require.NoError(t, err)

	var parsed map[string]any
	require.NoError(t, json.Unmarshal(data, &parsed))

	// Must have table/id, must NOT have pointer
	assert.Equal(t, "block", parsed["table"])
	assert.Equal(t, "b1", parsed["id"])
	assert.NotContains(t, parsed, "pointer")
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
