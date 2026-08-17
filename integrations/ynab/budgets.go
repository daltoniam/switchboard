package ynab

import (
	"context"
	"fmt"

	mcp "github.com/daltoniam/switchboard"
)

func getUser(ctx context.Context, y *ynab, _ map[string]any) (*mcp.ToolResult, error) {
	data, err := y.get(ctx, "/user")
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func listBudgets(ctx context.Context, y *ynab, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	includeAccounts := r.Str("include_accounts")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	q := queryEncode(map[string]string{
		"include_accounts": includeAccounts,
	})
	data, err := y.get(ctx, "/budgets%s", q)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func getBudget(ctx context.Context, y *ynab, args map[string]any) (*mcp.ToolResult, error) {
	data, err := y.get(ctx, "/budgets/%s", budget(args))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func getBudgetSettings(ctx context.Context, y *ynab, args map[string]any) (*mcp.ToolResult, error) {
	data, err := y.get(ctx, "/budgets/%s/settings", budget(args))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func listAccounts(ctx context.Context, y *ynab, args map[string]any) (*mcp.ToolResult, error) {
	data, err := y.get(ctx, "/budgets/%s/accounts", budget(args))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func getAccount(ctx context.Context, y *ynab, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	accountID := r.Str("account_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := y.get(ctx, "/budgets/%s/accounts/%s", budget(args), accountID)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func createAccount(ctx context.Context, y *ynab, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	name := r.Str("name")
	typ := r.Str("type")
	balance := r.Int("balance")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	body := map[string]any{
		"account": map[string]any{
			"name":    name,
			"type":    typ,
			"balance": balance,
		},
	}
	path := fmt.Sprintf("/budgets/%s/accounts", budget(args))
	data, err := y.post(ctx, path, body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}
