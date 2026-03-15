package ynab

import (
	"context"
	"fmt"

	mcp "github.com/daltoniam/switchboard"
)

func listTransactions(ctx context.Context, y *ynab, args map[string]any) (*mcp.ToolResult, error) {
	q := queryEncode(map[string]string{
		"since_date": argStr(args, "since_date"),
		"type":       argStr(args, "type"),
	})
	data, err := y.get(ctx, "/budgets/%s/transactions%s", budget(args), q)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func getTransaction(ctx context.Context, y *ynab, args map[string]any) (*mcp.ToolResult, error) {
	data, err := y.get(ctx, "/budgets/%s/transactions/%s", budget(args), argStr(args, "transaction_id"))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func listAccountTransactions(ctx context.Context, y *ynab, args map[string]any) (*mcp.ToolResult, error) {
	q := queryEncode(map[string]string{
		"since_date": argStr(args, "since_date"),
		"type":       argStr(args, "type"),
	})
	data, err := y.get(ctx, "/budgets/%s/accounts/%s/transactions%s",
		budget(args), argStr(args, "account_id"), q)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func listCategoryTransactions(ctx context.Context, y *ynab, args map[string]any) (*mcp.ToolResult, error) {
	q := queryEncode(map[string]string{
		"since_date": argStr(args, "since_date"),
		"type":       argStr(args, "type"),
	})
	data, err := y.get(ctx, "/budgets/%s/categories/%s/transactions%s",
		budget(args), argStr(args, "category_id"), q)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func listPayeeTransactions(ctx context.Context, y *ynab, args map[string]any) (*mcp.ToolResult, error) {
	q := queryEncode(map[string]string{
		"since_date": argStr(args, "since_date"),
		"type":       argStr(args, "type"),
	})
	data, err := y.get(ctx, "/budgets/%s/payees/%s/transactions%s",
		budget(args), argStr(args, "payee_id"), q)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func createTransaction(ctx context.Context, y *ynab, args map[string]any) (*mcp.ToolResult, error) {
	txn := map[string]any{
		"account_id": argStr(args, "account_id"),
		"date":       argStr(args, "date"),
		"amount":     argInt(args, "amount"),
	}
	if v := argStr(args, "payee_id"); v != "" {
		txn["payee_id"] = v
	}
	if v := argStr(args, "payee_name"); v != "" {
		txn["payee_name"] = v
	}
	if v := argStr(args, "category_id"); v != "" {
		txn["category_id"] = v
	}
	if v := argStr(args, "memo"); v != "" {
		txn["memo"] = v
	}
	if v := argStr(args, "cleared"); v != "" {
		txn["cleared"] = v
	}
	if v := argStr(args, "approved"); v != "" {
		txn["approved"] = argBool(args, "approved")
	}
	if v := argStr(args, "flag_color"); v != "" {
		txn["flag_color"] = v
	}

	body := map[string]any{"transaction": txn}
	path := fmt.Sprintf("/budgets/%s/transactions", budget(args))
	data, err := y.post(ctx, path, body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func updateTransaction(ctx context.Context, y *ynab, args map[string]any) (*mcp.ToolResult, error) {
	txn := map[string]any{}
	if v := argStr(args, "account_id"); v != "" {
		txn["account_id"] = v
	}
	if v := argStr(args, "date"); v != "" {
		txn["date"] = v
	}
	if argStr(args, "amount") != "" {
		txn["amount"] = argInt(args, "amount")
	}
	if v := argStr(args, "payee_id"); v != "" {
		txn["payee_id"] = v
	}
	if v := argStr(args, "payee_name"); v != "" {
		txn["payee_name"] = v
	}
	if v := argStr(args, "category_id"); v != "" {
		txn["category_id"] = v
	}
	if v := argStr(args, "memo"); v != "" {
		txn["memo"] = v
	}
	if v := argStr(args, "cleared"); v != "" {
		txn["cleared"] = v
	}
	if v := argStr(args, "approved"); v != "" {
		txn["approved"] = argBool(args, "approved")
	}
	if v := argStr(args, "flag_color"); v != "" {
		txn["flag_color"] = v
	}

	body := map[string]any{"transaction": txn}
	path := fmt.Sprintf("/budgets/%s/transactions/%s", budget(args), argStr(args, "transaction_id"))
	data, err := y.put(ctx, path, body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func deleteTransaction(ctx context.Context, y *ynab, args map[string]any) (*mcp.ToolResult, error) {
	data, err := y.del(ctx, "/budgets/%s/transactions/%s", budget(args), argStr(args, "transaction_id"))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func listScheduledTransactions(ctx context.Context, y *ynab, args map[string]any) (*mcp.ToolResult, error) {
	data, err := y.get(ctx, "/budgets/%s/scheduled_transactions", budget(args))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func getScheduledTransaction(ctx context.Context, y *ynab, args map[string]any) (*mcp.ToolResult, error) {
	data, err := y.get(ctx, "/budgets/%s/scheduled_transactions/%s",
		budget(args), argStr(args, "scheduled_transaction_id"))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func listMonthTransactions(ctx context.Context, y *ynab, args map[string]any) (*mcp.ToolResult, error) {
	q := queryEncode(map[string]string{
		"since_date": argStr(args, "since_date"),
		"type":       argStr(args, "type"),
	})
	data, err := y.get(ctx, "/budgets/%s/months/%s/transactions%s",
		budget(args), argStr(args, "month"), q)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func createScheduledTransaction(ctx context.Context, y *ynab, args map[string]any) (*mcp.ToolResult, error) {
	st := map[string]any{
		"account_id": argStr(args, "account_id"),
		"date":       argStr(args, "date"),
		"amount":     argInt(args, "amount"),
		"frequency":  argStr(args, "frequency"),
	}
	if v := argStr(args, "payee_id"); v != "" {
		st["payee_id"] = v
	}
	if v := argStr(args, "payee_name"); v != "" {
		st["payee_name"] = v
	}
	if v := argStr(args, "category_id"); v != "" {
		st["category_id"] = v
	}
	if v := argStr(args, "memo"); v != "" {
		st["memo"] = v
	}
	if v := argStr(args, "flag_color"); v != "" {
		st["flag_color"] = v
	}

	body := map[string]any{"scheduled_transaction": st}
	path := fmt.Sprintf("/budgets/%s/scheduled_transactions", budget(args))
	data, err := y.post(ctx, path, body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func updateScheduledTransaction(ctx context.Context, y *ynab, args map[string]any) (*mcp.ToolResult, error) {
	st := map[string]any{}
	if v := argStr(args, "account_id"); v != "" {
		st["account_id"] = v
	}
	if v := argStr(args, "date"); v != "" {
		st["date"] = v
	}
	if argStr(args, "amount") != "" {
		st["amount"] = argInt(args, "amount")
	}
	if v := argStr(args, "frequency"); v != "" {
		st["frequency"] = v
	}
	if v := argStr(args, "payee_id"); v != "" {
		st["payee_id"] = v
	}
	if v := argStr(args, "payee_name"); v != "" {
		st["payee_name"] = v
	}
	if v := argStr(args, "category_id"); v != "" {
		st["category_id"] = v
	}
	if v := argStr(args, "memo"); v != "" {
		st["memo"] = v
	}
	if v := argStr(args, "flag_color"); v != "" {
		st["flag_color"] = v
	}

	body := map[string]any{"scheduled_transaction": st}
	path := fmt.Sprintf("/budgets/%s/scheduled_transactions/%s",
		budget(args), argStr(args, "scheduled_transaction_id"))
	data, err := y.put(ctx, path, body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func deleteScheduledTransaction(ctx context.Context, y *ynab, args map[string]any) (*mcp.ToolResult, error) {
	data, err := y.del(ctx, "/budgets/%s/scheduled_transactions/%s",
		budget(args), argStr(args, "scheduled_transaction_id"))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}
