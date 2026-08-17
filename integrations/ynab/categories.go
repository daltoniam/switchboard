package ynab

import (
	"context"
	"fmt"

	mcp "github.com/daltoniam/switchboard"
)

func listCategories(ctx context.Context, y *ynab, args map[string]any) (*mcp.ToolResult, error) {
	data, err := y.get(ctx, "/budgets/%s/categories", budget(args))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func getCategory(ctx context.Context, y *ynab, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	categoryID := r.Str("category_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := y.get(ctx, "/budgets/%s/categories/%s", budget(args), categoryID)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func createCategory(ctx context.Context, y *ynab, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	name := r.Str("name")
	categoryGroupID := r.Str("category_group_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	body := map[string]any{
		"category": map[string]any{
			"name":              name,
			"category_group_id": categoryGroupID,
		},
	}
	path := fmt.Sprintf("/budgets/%s/categories", budget(args))
	data, err := y.post(ctx, path, body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func updateCategory(ctx context.Context, y *ynab, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	categoryID := r.Str("category_id")
	name := r.Str("name")
	note := r.Str("note")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	cat := map[string]any{}
	if name != "" {
		cat["name"] = name
	}
	if note != "" {
		cat["note"] = note
	}
	body := map[string]any{"category": cat}
	path := fmt.Sprintf("/budgets/%s/categories/%s", budget(args), categoryID)
	data, err := y.patch(ctx, path, body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func getMonthCategory(ctx context.Context, y *ynab, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	month := r.Str("month")
	categoryID := r.Str("category_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := y.get(ctx, "/budgets/%s/months/%s/categories/%s",
		budget(args), month, categoryID)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func updateMonthCategory(ctx context.Context, y *ynab, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	month := r.Str("month")
	categoryID := r.Str("category_id")
	budgeted := r.Int("budgeted")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	body := map[string]any{
		"category": map[string]any{
			"budgeted": budgeted,
		},
	}
	path := fmt.Sprintf("/budgets/%s/months/%s/categories/%s",
		budget(args), month, categoryID)
	data, err := y.patch(ctx, path, body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func listPayees(ctx context.Context, y *ynab, args map[string]any) (*mcp.ToolResult, error) {
	data, err := y.get(ctx, "/budgets/%s/payees", budget(args))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func getPayee(ctx context.Context, y *ynab, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	payeeID := r.Str("payee_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := y.get(ctx, "/budgets/%s/payees/%s", budget(args), payeeID)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func listMonths(ctx context.Context, y *ynab, args map[string]any) (*mcp.ToolResult, error) {
	data, err := y.get(ctx, "/budgets/%s/months", budget(args))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func getMonth(ctx context.Context, y *ynab, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	month := r.Str("month")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := y.get(ctx, "/budgets/%s/months/%s", budget(args), month)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func createCategoryGroup(ctx context.Context, y *ynab, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	name := r.Str("name")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	body := map[string]any{
		"category_group": map[string]any{
			"name": name,
		},
	}
	path := fmt.Sprintf("/budgets/%s/category_groups", budget(args))
	data, err := y.post(ctx, path, body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func updateCategoryGroup(ctx context.Context, y *ynab, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	categoryGroupID := r.Str("category_group_id")
	name := r.Str("name")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	body := map[string]any{
		"category_group": map[string]any{
			"name": name,
		},
	}
	path := fmt.Sprintf("/budgets/%s/category_groups/%s", budget(args), categoryGroupID)
	data, err := y.patch(ctx, path, body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func updatePayee(ctx context.Context, y *ynab, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	payeeID := r.Str("payee_id")
	name := r.Str("name")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	body := map[string]any{
		"payee": map[string]any{
			"name": name,
		},
	}
	path := fmt.Sprintf("/budgets/%s/payees/%s", budget(args), payeeID)
	data, err := y.patch(ctx, path, body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func listPayeeLocations(ctx context.Context, y *ynab, args map[string]any) (*mcp.ToolResult, error) {
	data, err := y.get(ctx, "/budgets/%s/payee_locations", budget(args))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func getPayeeLocation(ctx context.Context, y *ynab, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	payeeLocationID := r.Str("payee_location_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := y.get(ctx, "/budgets/%s/payee_locations/%s", budget(args), payeeLocationID)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func listLocationsForPayee(ctx context.Context, y *ynab, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	payeeID := r.Str("payee_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := y.get(ctx, "/budgets/%s/payees/%s/payee_locations", budget(args), payeeID)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}
