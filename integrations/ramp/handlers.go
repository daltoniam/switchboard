package ramp

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"

	mcp "github.com/daltoniam/switchboard"
)

func listTransactions(ctx context.Context, r *ramp, args map[string]any) (*mcp.ToolResult, error) {
	rd := mcp.NewArgs(args)
	params := pageParams(args)
	for _, key := range []string{
		"from_date", "to_date", "user_id", "card_id", "department_id", "location_id",
		"merchant_id", "state", "sync_status", "entity_id",
	} {
		if v := rd.Str(key); v != "" {
			params[key] = v
		}
	}
	if err := rd.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := r.get(ctx, "/developer/v1/transactions%s", queryEncode(params))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func getTransaction(ctx context.Context, r *ramp, args map[string]any) (*mcp.ToolResult, error) {
	rd := mcp.NewArgs(args)
	id := rd.Str("transaction_id")
	if err := rd.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := r.get(ctx, "/developer/v1/transactions/%s", url.PathEscape(id))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func setTransactionMemo(ctx context.Context, r *ramp, args map[string]any) (*mcp.ToolResult, error) {
	rd := mcp.NewArgs(args)
	id := rd.Str("transaction_id")
	memo := rd.Str("memo")
	if err := rd.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	body := map[string]any{"memo": memo}
	if _, ok := args["is_memo_recurring"]; ok {
		recurring, err := mcp.ArgBool(args, "is_memo_recurring")
		if err != nil {
			return mcp.ErrResult(err)
		}
		body["is_memo_recurring"] = recurring
	}
	path := fmt.Sprintf("/developer/v1/memos/%s", url.PathEscape(id))
	data, err := r.post(ctx, path, body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func updateTransactionSplits(ctx context.Context, r *ramp, args map[string]any) (*mcp.ToolResult, error) {
	rd := mcp.NewArgs(args)
	id := rd.Str("transaction_id")
	if err := rd.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	lineItems, err := anyJSONArg(args, "line_items")
	if err != nil {
		return mcp.ErrResult(err)
	}
	path := fmt.Sprintf("/developer/v1/transactions/%s", url.PathEscape(id))
	data, err := r.patch(ctx, path, map[string]any{"line_items": lineItems})
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

// anyJSONArg accepts either a native JSON value (array/object) or a JSON string.
func anyJSONArg(args map[string]any, key string) (any, error) {
	v, ok := args[key]
	if !ok || v == nil {
		return nil, fmt.Errorf("%s is required", key)
	}
	switch v := v.(type) {
	case string:
		var out any
		if err := json.Unmarshal([]byte(v), &out); err != nil {
			return nil, fmt.Errorf("invalid JSON for %s: %w", key, err)
		}
		return out, nil
	default:
		return v, nil
	}
}

func listReimbursements(ctx context.Context, r *ramp, args map[string]any) (*mcp.ToolResult, error) {
	data, err := r.get(ctx, "/developer/v1/reimbursements%s", queryEncode(pageParams(args)))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func getReimbursement(ctx context.Context, r *ramp, args map[string]any) (*mcp.ToolResult, error) {
	rd := mcp.NewArgs(args)
	id := rd.Str("reimbursement_id")
	if err := rd.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := r.get(ctx, "/developer/v1/reimbursements/%s", url.PathEscape(id))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func listBills(ctx context.Context, r *ramp, args map[string]any) (*mcp.ToolResult, error) {
	data, err := r.get(ctx, "/developer/v1/bills%s", queryEncode(pageParams(args)))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func getBill(ctx context.Context, r *ramp, args map[string]any) (*mcp.ToolResult, error) {
	rd := mcp.NewArgs(args)
	id := rd.Str("bill_id")
	if err := rd.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := r.get(ctx, "/developer/v1/bills/%s", url.PathEscape(id))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func listUsers(ctx context.Context, r *ramp, args map[string]any) (*mcp.ToolResult, error) {
	rd := mcp.NewArgs(args)
	params := pageParams(args)
	if v := rd.Str("department_id"); v != "" {
		params["department_id"] = v
	}
	if v := rd.Str("location_id"); v != "" {
		params["location_id"] = v
	}
	if err := rd.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := r.get(ctx, "/developer/v1/users%s", queryEncode(params))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func getUser(ctx context.Context, r *ramp, args map[string]any) (*mcp.ToolResult, error) {
	rd := mcp.NewArgs(args)
	id := rd.Str("user_id")
	if err := rd.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := r.get(ctx, "/developer/v1/users/%s", url.PathEscape(id))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func listVirtualCards(ctx context.Context, r *ramp, args map[string]any) (*mcp.ToolResult, error) {
	data, err := r.get(ctx, "/developer/v1/cards/virtual%s", queryEncode(pageParams(args)))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func getVirtualCard(ctx context.Context, r *ramp, args map[string]any) (*mcp.ToolResult, error) {
	rd := mcp.NewArgs(args)
	id := rd.Str("card_id")
	if err := rd.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := r.get(ctx, "/developer/v1/cards/virtual/%s", url.PathEscape(id))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func listPhysicalCards(ctx context.Context, r *ramp, args map[string]any) (*mcp.ToolResult, error) {
	data, err := r.get(ctx, "/developer/v1/cards/physical%s", queryEncode(pageParams(args)))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func getPhysicalCard(ctx context.Context, r *ramp, args map[string]any) (*mcp.ToolResult, error) {
	rd := mcp.NewArgs(args)
	id := rd.Str("card_id")
	if err := rd.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := r.get(ctx, "/developer/v1/cards/physical/%s", url.PathEscape(id))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func listReceipts(ctx context.Context, r *ramp, args map[string]any) (*mcp.ToolResult, error) {
	data, err := r.get(ctx, "/developer/v1/receipts%s", queryEncode(pageParams(args)))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func getReceipt(ctx context.Context, r *ramp, args map[string]any) (*mcp.ToolResult, error) {
	rd := mcp.NewArgs(args)
	id := rd.Str("receipt_id")
	if err := rd.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := r.get(ctx, "/developer/v1/receipts/%s", url.PathEscape(id))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func listDepartments(ctx context.Context, r *ramp, args map[string]any) (*mcp.ToolResult, error) {
	data, err := r.get(ctx, "/developer/v1/departments%s", queryEncode(pageParams(args)))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func getDepartment(ctx context.Context, r *ramp, args map[string]any) (*mcp.ToolResult, error) {
	rd := mcp.NewArgs(args)
	id := rd.Str("department_id")
	if err := rd.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := r.get(ctx, "/developer/v1/departments/%s", url.PathEscape(id))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func listLocations(ctx context.Context, r *ramp, args map[string]any) (*mcp.ToolResult, error) {
	data, err := r.get(ctx, "/developer/v1/locations%s", queryEncode(pageParams(args)))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func getLocation(ctx context.Context, r *ramp, args map[string]any) (*mcp.ToolResult, error) {
	rd := mcp.NewArgs(args)
	id := rd.Str("location_id")
	if err := rd.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := r.get(ctx, "/developer/v1/locations/%s", url.PathEscape(id))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func listMerchants(ctx context.Context, r *ramp, args map[string]any) (*mcp.ToolResult, error) {
	data, err := r.get(ctx, "/developer/v1/merchants%s", queryEncode(pageParams(args)))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func listVendors(ctx context.Context, r *ramp, args map[string]any) (*mcp.ToolResult, error) {
	data, err := r.get(ctx, "/developer/v1/vendors%s", queryEncode(pageParams(args)))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func getVendor(ctx context.Context, r *ramp, args map[string]any) (*mcp.ToolResult, error) {
	rd := mcp.NewArgs(args)
	id := rd.Str("vendor_id")
	if err := rd.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := r.get(ctx, "/developer/v1/vendors/%s", url.PathEscape(id))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func listEntities(ctx context.Context, r *ramp, args map[string]any) (*mcp.ToolResult, error) {
	data, err := r.get(ctx, "/developer/v1/entities%s", queryEncode(pageParams(args)))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func getEntity(ctx context.Context, r *ramp, args map[string]any) (*mcp.ToolResult, error) {
	rd := mcp.NewArgs(args)
	id := rd.Str("entity_id")
	if err := rd.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := r.get(ctx, "/developer/v1/entities/%s", url.PathEscape(id))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func listBankAccounts(ctx context.Context, r *ramp, args map[string]any) (*mcp.ToolResult, error) {
	data, err := r.get(ctx, "/developer/v1/bank-accounts%s", queryEncode(pageParams(args)))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func getBankAccount(ctx context.Context, r *ramp, args map[string]any) (*mcp.ToolResult, error) {
	rd := mcp.NewArgs(args)
	id := rd.Str("bank_account_id")
	if err := rd.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := r.get(ctx, "/developer/v1/bank-accounts/%s", url.PathEscape(id))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}
