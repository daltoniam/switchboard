package stripe

import (
	"context"
	"fmt"

	mcp "github.com/daltoniam/switchboard"
)

// --- Balance ---

func getBalance(ctx context.Context, s *stripe, _ map[string]any) (*mcp.ToolResult, error) {
	data, err := s.get(ctx, "/balance", nil)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func listBalanceTransactions(ctx context.Context, s *stripe, args map[string]any) (*mcp.ToolResult, error) {
	params := listParamsFrom(args, "type", "currency", "payout", "source", "created")
	data, err := s.get(ctx, "/balance_transactions", params)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func retrieveBalanceTransaction(ctx context.Context, s *stripe, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	id := r.Str("id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := s.get(ctx, fmt.Sprintf("/balance_transactions/%s", id), nil)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

// --- Customers ---

func listCustomers(ctx context.Context, s *stripe, args map[string]any) (*mcp.ToolResult, error) {
	params := listParamsFrom(args, "email", "created")
	data, err := s.get(ctx, "/customers", params)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func retrieveCustomer(ctx context.Context, s *stripe, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	id := r.Str("id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := s.get(ctx, fmt.Sprintf("/customers/%s", id), nil)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func searchCustomers(ctx context.Context, s *stripe, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	_ = r.Str("query") // required, validated below
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := s.get(ctx, "/customers/search", searchParamsFrom(args))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func createCustomer(ctx context.Context, s *stripe, args map[string]any) (*mcp.ToolResult, error) {
	body := map[string]any{}
	copyIfPresent(body, args, "email", "name", "phone", "description", "metadata", "address", "shipping", "payment_method", "default_source", "invoice_settings")
	data, err := s.post(ctx, "/customers", body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func updateCustomer(ctx context.Context, s *stripe, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	id := r.Str("id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	body := map[string]any{}
	copyIfPresent(body, args, "email", "name", "phone", "description", "metadata", "address", "shipping", "default_source", "invoice_settings")
	data, err := s.post(ctx, fmt.Sprintf("/customers/%s", id), body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func deleteCustomer(ctx context.Context, s *stripe, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	id := r.Str("id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := s.del(ctx, fmt.Sprintf("/customers/%s", id), nil)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}
