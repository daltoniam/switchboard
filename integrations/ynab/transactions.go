package ynab

import (
	"context"
	"fmt"

	mcp "github.com/daltoniam/switchboard"
)

func listTransactions(ctx context.Context, y *ynab, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	sinceDate := r.Str("since_date")
	typ := r.Str("type")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	q := queryEncode(map[string]string{
		"since_date": sinceDate,
		"type":       typ,
	})
	data, err := y.get(ctx, "/budgets/%s/transactions%s", budget(args), q)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func getTransaction(ctx context.Context, y *ynab, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	transactionID := r.Str("transaction_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := y.get(ctx, "/budgets/%s/transactions/%s", budget(args), transactionID)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func listAccountTransactions(ctx context.Context, y *ynab, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	accountID := r.Str("account_id")
	sinceDate := r.Str("since_date")
	typ := r.Str("type")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	q := queryEncode(map[string]string{
		"since_date": sinceDate,
		"type":       typ,
	})
	data, err := y.get(ctx, "/budgets/%s/accounts/%s/transactions%s",
		budget(args), accountID, q)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func listCategoryTransactions(ctx context.Context, y *ynab, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	categoryID := r.Str("category_id")
	sinceDate := r.Str("since_date")
	typ := r.Str("type")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	q := queryEncode(map[string]string{
		"since_date": sinceDate,
		"type":       typ,
	})
	data, err := y.get(ctx, "/budgets/%s/categories/%s/transactions%s",
		budget(args), categoryID, q)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func listPayeeTransactions(ctx context.Context, y *ynab, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	payeeID := r.Str("payee_id")
	sinceDate := r.Str("since_date")
	typ := r.Str("type")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	q := queryEncode(map[string]string{
		"since_date": sinceDate,
		"type":       typ,
	})
	data, err := y.get(ctx, "/budgets/%s/payees/%s/transactions%s",
		budget(args), payeeID, q)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func createTransaction(ctx context.Context, y *ynab, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	accountID := r.Str("account_id")
	date := r.Str("date")
	amount := r.Int("amount")
	payeeID := r.Str("payee_id")
	payeeName := r.Str("payee_name")
	categoryID := r.Str("category_id")
	memo := r.Str("memo")
	cleared := r.Str("cleared")
	flagColor := r.Str("flag_color")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}

	txn := map[string]any{
		"account_id": accountID,
		"date":       date,
		"amount":     amount,
	}
	if payeeID != "" {
		txn["payee_id"] = payeeID
	}
	if payeeName != "" {
		txn["payee_name"] = payeeName
	}
	if categoryID != "" {
		txn["category_id"] = categoryID
	}
	if memo != "" {
		txn["memo"] = memo
	}
	if cleared != "" {
		txn["cleared"] = cleared
	}
	if _, ok := args["approved"]; ok {
		v, err := mcp.ArgBool(args, "approved")
		if err != nil {
			return mcp.ErrResult(err)
		}
		txn["approved"] = v
	}
	if flagColor != "" {
		txn["flag_color"] = flagColor
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
	r := mcp.NewArgs(args)
	transactionID := r.Str("transaction_id")
	accountID := r.Str("account_id")
	date := r.Str("date")
	payeeID := r.Str("payee_id")
	payeeName := r.Str("payee_name")
	categoryID := r.Str("category_id")
	memo := r.Str("memo")
	cleared := r.Str("cleared")
	flagColor := r.Str("flag_color")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}

	txn := map[string]any{}
	if accountID != "" {
		txn["account_id"] = accountID
	}
	if date != "" {
		txn["date"] = date
	}
	// Check if amount was explicitly provided (even as 0).
	if _, ok := args["amount"]; ok {
		v, err := mcp.ArgInt(args, "amount")
		if err != nil {
			return mcp.ErrResult(err)
		}
		txn["amount"] = v
	}
	if payeeID != "" {
		txn["payee_id"] = payeeID
	}
	if payeeName != "" {
		txn["payee_name"] = payeeName
	}
	if categoryID != "" {
		txn["category_id"] = categoryID
	}
	if memo != "" {
		txn["memo"] = memo
	}
	if cleared != "" {
		txn["cleared"] = cleared
	}
	if _, ok := args["approved"]; ok {
		v, err := mcp.ArgBool(args, "approved")
		if err != nil {
			return mcp.ErrResult(err)
		}
		txn["approved"] = v
	}
	if flagColor != "" {
		txn["flag_color"] = flagColor
	}

	body := map[string]any{"transaction": txn}
	path := fmt.Sprintf("/budgets/%s/transactions/%s", budget(args), transactionID)
	data, err := y.put(ctx, path, body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func deleteTransaction(ctx context.Context, y *ynab, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	transactionID := r.Str("transaction_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := y.del(ctx, "/budgets/%s/transactions/%s", budget(args), transactionID)
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
	r := mcp.NewArgs(args)
	scheduledTransactionID := r.Str("scheduled_transaction_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := y.get(ctx, "/budgets/%s/scheduled_transactions/%s",
		budget(args), scheduledTransactionID)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func listMonthTransactions(ctx context.Context, y *ynab, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	month := r.Str("month")
	sinceDate := r.Str("since_date")
	typ := r.Str("type")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	q := queryEncode(map[string]string{
		"since_date": sinceDate,
		"type":       typ,
	})
	data, err := y.get(ctx, "/budgets/%s/months/%s/transactions%s",
		budget(args), month, q)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func createScheduledTransaction(ctx context.Context, y *ynab, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	accountID := r.Str("account_id")
	date := r.Str("date")
	amount := r.Int("amount")
	frequency := r.Str("frequency")
	payeeID := r.Str("payee_id")
	payeeName := r.Str("payee_name")
	categoryID := r.Str("category_id")
	memo := r.Str("memo")
	flagColor := r.Str("flag_color")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}

	st := map[string]any{
		"account_id": accountID,
		"date":       date,
		"amount":     amount,
		"frequency":  frequency,
	}
	if payeeID != "" {
		st["payee_id"] = payeeID
	}
	if payeeName != "" {
		st["payee_name"] = payeeName
	}
	if categoryID != "" {
		st["category_id"] = categoryID
	}
	if memo != "" {
		st["memo"] = memo
	}
	if flagColor != "" {
		st["flag_color"] = flagColor
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
	r := mcp.NewArgs(args)
	scheduledTransactionID := r.Str("scheduled_transaction_id")
	accountID := r.Str("account_id")
	date := r.Str("date")
	frequency := r.Str("frequency")
	payeeID := r.Str("payee_id")
	payeeName := r.Str("payee_name")
	categoryID := r.Str("category_id")
	memo := r.Str("memo")
	flagColor := r.Str("flag_color")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}

	st := map[string]any{}
	if accountID != "" {
		st["account_id"] = accountID
	}
	if date != "" {
		st["date"] = date
	}
	// Check if amount was explicitly provided (even as 0).
	if _, ok := args["amount"]; ok {
		v, err := mcp.ArgInt(args, "amount")
		if err != nil {
			return mcp.ErrResult(err)
		}
		st["amount"] = v
	}
	if frequency != "" {
		st["frequency"] = frequency
	}
	if payeeID != "" {
		st["payee_id"] = payeeID
	}
	if payeeName != "" {
		st["payee_name"] = payeeName
	}
	if categoryID != "" {
		st["category_id"] = categoryID
	}
	if memo != "" {
		st["memo"] = memo
	}
	if flagColor != "" {
		st["flag_color"] = flagColor
	}

	body := map[string]any{"scheduled_transaction": st}
	path := fmt.Sprintf("/budgets/%s/scheduled_transactions/%s",
		budget(args), scheduledTransactionID)
	data, err := y.put(ctx, path, body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func deleteScheduledTransaction(ctx context.Context, y *ynab, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	scheduledTransactionID := r.Str("scheduled_transaction_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := y.del(ctx, "/budgets/%s/scheduled_transactions/%s",
		budget(args), scheduledTransactionID)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}
