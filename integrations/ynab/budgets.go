package ynab

import (
	"context"
	"fmt"

	mcp "github.com/daltoniam/switchboard"
)

func getUser(ctx context.Context, y *ynab, _ map[string]any) (*mcp.ToolResult, error) {
	data, err := y.get(ctx, "/user")
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func listBudgets(ctx context.Context, y *ynab, args map[string]any) (*mcp.ToolResult, error) {
	q := queryEncode(map[string]string{
		"include_accounts": argStr(args, "include_accounts"),
	})
	data, err := y.get(ctx, "/budgets%s", q)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func getBudget(ctx context.Context, y *ynab, args map[string]any) (*mcp.ToolResult, error) {
	data, err := y.get(ctx, "/budgets/%s", budget(args))
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func getBudgetSettings(ctx context.Context, y *ynab, args map[string]any) (*mcp.ToolResult, error) {
	data, err := y.get(ctx, "/budgets/%s/settings", budget(args))
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func listAccounts(ctx context.Context, y *ynab, args map[string]any) (*mcp.ToolResult, error) {
	data, err := y.get(ctx, "/budgets/%s/accounts", budget(args))
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func getAccount(ctx context.Context, y *ynab, args map[string]any) (*mcp.ToolResult, error) {
	data, err := y.get(ctx, "/budgets/%s/accounts/%s", budget(args), argStr(args, "account_id"))
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func createAccount(ctx context.Context, y *ynab, args map[string]any) (*mcp.ToolResult, error) {
	body := map[string]any{
		"account": map[string]any{
			"name":    argStr(args, "name"),
			"type":    argStr(args, "type"),
			"balance": argInt(args, "balance"),
		},
	}
	path := fmt.Sprintf("/budgets/%s/accounts", budget(args))
	data, err := y.post(ctx, path, body)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}
