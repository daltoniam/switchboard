package notion

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"
)

// pointer identifies a record in the Notion v3 pointer format.
// Used by commands like setParent that require spaceId context.
type pointer struct {
	Table   string `json:"table"`
	ID      string `json:"id"`
	SpaceID string `json:"spaceId"`
}

// op represents a single operation in a Notion v3 transaction.
// Flat-format ops use Table/ID; pointer-format ops (setParent) use Pointer.
type op struct {
	Command string   `json:"command"`
	Table   string   `json:"table,omitempty"`
	ID      string   `json:"id,omitempty"`
	Pointer *pointer `json:"pointer,omitempty"`
	Path    []string `json:"path"`
	Args    any      `json:"args"`
}

type transaction struct {
	ID         string `json:"id"`
	SpaceID    string `json:"spaceId"`
	Operations []op   `json:"operations"`
}

type saveTransactionsRequest struct {
	RequestID    string        `json:"requestId"`
	Transactions []transaction `json:"transactions"`
}

// buildSetOp creates a "set" operation that sets a value at a path.
func buildSetOp(table, id string, path []string, args any) op {
	return op{
		Command: "set",
		Table:   table,
		ID:      id,
		Path:    path,
		Args:    args,
	}
}

// buildUpdateOp creates an "update" operation that merges values at a path.
func buildUpdateOp(table, id string, path []string, args any) op {
	return op{
		Command: "update",
		Table:   table,
		ID:      id,
		Path:    path,
		Args:    args,
	}
}

// buildListAfterOp creates a "listAfter" operation that appends an item to a list.
func buildListAfterOp(table, id string, path []string, args any) op {
	return op{
		Command: "listAfter",
		Table:   table,
		ID:      id,
		Path:    path,
		Args:    args,
	}
}

// buildListRemoveOp creates a "listRemove" operation that removes an item from a list.
func buildListRemoveOp(table, id string, path []string, args any) op {
	return op{
		Command: "listRemove",
		Table:   table,
		ID:      id,
		Path:    path,
		Args:    args,
	}
}

// buildSetParentOp creates a "setParent" operation that links a block to a parent.
// Used for collection (database) parents where listAfter on block table doesn't work.
func buildSetParentOp(spaceID, blockID, parentID, parentTable string) op {
	return op{
		Command: "setParent",
		Pointer: &pointer{Table: "block", ID: blockID, SpaceID: spaceID},
		Path:    []string{},
		Args:    map[string]any{"parentId": parentID, "parentTable": parentTable},
	}
}

// buildTransaction wraps operations into a transaction request body.
func buildTransaction(ops []op) transaction {
	return transaction{Operations: ops}
}

// submitTransaction sends a transaction to the v3 API and returns the response.
func submitTransaction(ctx context.Context, n *notion, ops []op) (json.RawMessage, error) {
	writeOps := make([]op, len(ops))
	for i, operation := range ops {
		if operation.Pointer == nil {
			operation.Pointer = &pointer{Table: operation.Table, ID: operation.ID, SpaceID: n.spaceID}
			operation.Table = ""
			operation.ID = ""
		}
		writeOps[i] = operation
	}
	tx := buildTransaction(writeOps)
	tx.ID = uuid.NewString()
	tx.SpaceID = n.spaceID
	body := saveTransactionsRequest{
		RequestID:    uuid.NewString(),
		Transactions: []transaction{tx},
	}
	return n.doRequest(ctx, "/api/v3/saveTransactions", body)
}

// newBlockID generates a new UUID for use as a block/record ID.
func newBlockID() string {
	return uuid.New().String()
}
