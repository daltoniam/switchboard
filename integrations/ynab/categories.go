package ynab

import (
	"context"
	"fmt"

	mcp "github.com/daltoniam/switchboard"
)

func listCategories(ctx context.Context, y *ynab, args map[string]any) (*mcp.ToolResult, error) {
	data, err := y.get(ctx, "/budgets/%s/categories", budget(args))
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func getCategory(ctx context.Context, y *ynab, args map[string]any) (*mcp.ToolResult, error) {
	data, err := y.get(ctx, "/budgets/%s/categories/%s", budget(args), argStr(args, "category_id"))
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func createCategory(ctx context.Context, y *ynab, args map[string]any) (*mcp.ToolResult, error) {
	body := map[string]any{
		"category": map[string]any{
			"name":              argStr(args, "name"),
			"category_group_id": argStr(args, "category_group_id"),
		},
	}
	path := fmt.Sprintf("/budgets/%s/categories", budget(args))
	data, err := y.post(ctx, path, body)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func updateCategory(ctx context.Context, y *ynab, args map[string]any) (*mcp.ToolResult, error) {
	cat := map[string]any{}
	if v := argStr(args, "name"); v != "" {
		cat["name"] = v
	}
	if v := argStr(args, "note"); v != "" {
		cat["note"] = v
	}
	body := map[string]any{"category": cat}
	path := fmt.Sprintf("/budgets/%s/categories/%s", budget(args), argStr(args, "category_id"))
	data, err := y.patch(ctx, path, body)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func getMonthCategory(ctx context.Context, y *ynab, args map[string]any) (*mcp.ToolResult, error) {
	data, err := y.get(ctx, "/budgets/%s/months/%s/categories/%s",
		budget(args), argStr(args, "month"), argStr(args, "category_id"))
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func updateMonthCategory(ctx context.Context, y *ynab, args map[string]any) (*mcp.ToolResult, error) {
	body := map[string]any{
		"category": map[string]any{
			"budgeted": argInt(args, "budgeted"),
		},
	}
	path := fmt.Sprintf("/budgets/%s/months/%s/categories/%s",
		budget(args), argStr(args, "month"), argStr(args, "category_id"))
	data, err := y.patch(ctx, path, body)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func listPayees(ctx context.Context, y *ynab, args map[string]any) (*mcp.ToolResult, error) {
	data, err := y.get(ctx, "/budgets/%s/payees", budget(args))
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func getPayee(ctx context.Context, y *ynab, args map[string]any) (*mcp.ToolResult, error) {
	data, err := y.get(ctx, "/budgets/%s/payees/%s", budget(args), argStr(args, "payee_id"))
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func listMonths(ctx context.Context, y *ynab, args map[string]any) (*mcp.ToolResult, error) {
	data, err := y.get(ctx, "/budgets/%s/months", budget(args))
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func getMonth(ctx context.Context, y *ynab, args map[string]any) (*mcp.ToolResult, error) {
	data, err := y.get(ctx, "/budgets/%s/months/%s", budget(args), argStr(args, "month"))
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func createCategoryGroup(ctx context.Context, y *ynab, args map[string]any) (*mcp.ToolResult, error) {
	body := map[string]any{
		"category_group": map[string]any{
			"name": argStr(args, "name"),
		},
	}
	path := fmt.Sprintf("/budgets/%s/category_groups", budget(args))
	data, err := y.post(ctx, path, body)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func updateCategoryGroup(ctx context.Context, y *ynab, args map[string]any) (*mcp.ToolResult, error) {
	body := map[string]any{
		"category_group": map[string]any{
			"name": argStr(args, "name"),
		},
	}
	path := fmt.Sprintf("/budgets/%s/category_groups/%s", budget(args), argStr(args, "category_group_id"))
	data, err := y.patch(ctx, path, body)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func updatePayee(ctx context.Context, y *ynab, args map[string]any) (*mcp.ToolResult, error) {
	body := map[string]any{
		"payee": map[string]any{
			"name": argStr(args, "name"),
		},
	}
	path := fmt.Sprintf("/budgets/%s/payees/%s", budget(args), argStr(args, "payee_id"))
	data, err := y.patch(ctx, path, body)
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func listPayeeLocations(ctx context.Context, y *ynab, args map[string]any) (*mcp.ToolResult, error) {
	data, err := y.get(ctx, "/budgets/%s/payee_locations", budget(args))
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func getPayeeLocation(ctx context.Context, y *ynab, args map[string]any) (*mcp.ToolResult, error) {
	data, err := y.get(ctx, "/budgets/%s/payee_locations/%s", budget(args), argStr(args, "payee_location_id"))
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}

func listLocationsForPayee(ctx context.Context, y *ynab, args map[string]any) (*mcp.ToolResult, error) {
	data, err := y.get(ctx, "/budgets/%s/payees/%s/payee_locations", budget(args), argStr(args, "payee_id"))
	if err != nil {
		return errResult(err)
	}
	return rawResult(data)
}
