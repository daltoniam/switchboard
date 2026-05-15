package stripe

import (
	"context"
	"fmt"

	mcp "github.com/daltoniam/switchboard"
)

// --- Invoices ---

func listInvoices(ctx context.Context, s *stripe, args map[string]any) (*mcp.ToolResult, error) {
	params := listParamsFrom(args, "customer", "subscription", "status", "collection_method", "created", "due_date")
	data, err := s.get(ctx, "/invoices", params)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func retrieveInvoice(ctx context.Context, s *stripe, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	id := r.Str("id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := s.get(ctx, fmt.Sprintf("/invoices/%s", id), nil)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func retrieveUpcomingInvoice(ctx context.Context, s *stripe, args map[string]any) (*mcp.ToolResult, error) {
	params := map[string]any{}
	copyIfPresent(params, args, "customer", "subscription", "coupon", "subscription_items", "subscription_proration_date", "subscription_proration_behavior", "subscription_trial_end", "subscription_cancel_at_period_end", "subscription_cancel_now", "subscription_billing_cycle_anchor", "schedule", "automatic_tax", "invoice_items", "discounts")
	data, err := s.get(ctx, "/invoices/upcoming", params)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func searchInvoices(ctx context.Context, s *stripe, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	_ = r.Str("query")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := s.get(ctx, "/invoices/search", searchParamsFrom(args))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func createInvoice(ctx context.Context, s *stripe, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	customer := r.Str("customer")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	body := map[string]any{"customer": customer}
	copyIfPresent(body, args, "auto_advance", "collection_method", "days_until_due", "description", "subscription", "metadata", "default_payment_method", "default_source", "due_date", "footer", "statement_descriptor", "automatic_tax", "discounts", "default_tax_rates", "account_tax_ids", "custom_fields", "payment_settings", "transfer_data", "rendering", "shipping_cost", "shipping_details")
	data, err := s.post(ctx, "/invoices", body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func finalizeInvoice(ctx context.Context, s *stripe, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	id := r.Str("id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	body := map[string]any{}
	copyIfPresent(body, args, "auto_advance")
	data, err := s.post(ctx, fmt.Sprintf("/invoices/%s/finalize", id), body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func payInvoice(ctx context.Context, s *stripe, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	id := r.Str("id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	body := map[string]any{}
	copyIfPresent(body, args, "payment_method", "source", "paid_out_of_band", "off_session", "forgive", "mandate")
	data, err := s.post(ctx, fmt.Sprintf("/invoices/%s/pay", id), body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func sendInvoice(ctx context.Context, s *stripe, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	id := r.Str("id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := s.post(ctx, fmt.Sprintf("/invoices/%s/send", id), nil)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func voidInvoice(ctx context.Context, s *stripe, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	id := r.Str("id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := s.post(ctx, fmt.Sprintf("/invoices/%s/void", id), nil)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func deleteInvoice(ctx context.Context, s *stripe, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	id := r.Str("id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := s.del(ctx, fmt.Sprintf("/invoices/%s", id), nil)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

// --- Invoice Items ---

func listInvoiceItems(ctx context.Context, s *stripe, args map[string]any) (*mcp.ToolResult, error) {
	params := listParamsFrom(args, "customer", "invoice", "pending", "created")
	data, err := s.get(ctx, "/invoiceitems", params)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func createInvoiceItem(ctx context.Context, s *stripe, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	customer := r.Str("customer")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	body := map[string]any{"customer": customer}
	copyIfPresent(body, args, "amount", "currency", "description", "price", "quantity", "invoice", "subscription", "metadata", "discountable", "discounts", "period", "tax_rates", "unit_amount", "unit_amount_decimal", "price_data")
	data, err := s.post(ctx, "/invoiceitems", body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}
