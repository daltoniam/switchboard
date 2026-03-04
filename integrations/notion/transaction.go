package notion

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"
)

// op represents a single operation in a Notion v3 transaction.
type op struct {
	Command string   `json:"command"`
	Table   string   `json:"table"`
	ID      string   `json:"id"`
	Path    []string `json:"path"`
	Args    any      `json:"args"`
}

// transaction represents a v3 submitTransaction request body.
type transaction struct {
	Operations []op `json:"operations"`
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

// buildTransaction wraps operations into a transaction request body.
func buildTransaction(ops []op) transaction {
	return transaction{Operations: ops}
}

// submitTransaction sends a transaction to the v3 API and returns the response.
func submitTransaction(ctx context.Context, n *notion, ops []op) (json.RawMessage, error) {
	tx := buildTransaction(ops)
	return n.doRequest(ctx, "/api/v3/submitTransaction", tx)
}

// newBlockID generates a new UUID for use as a block/record ID.
func newBlockID() string {
	return uuid.New().String()
}
