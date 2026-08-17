package stripe

import (
	"context"
	"fmt"

	mcp "github.com/daltoniam/switchboard"
)

// --- Subscriptions ---

func listSubscriptions(ctx context.Context, s *stripe, args map[string]any) (*mcp.ToolResult, error) {
	params := listParamsFrom(args, "customer", "price", "status", "collection_method", "created", "current_period_end", "current_period_start")
	data, err := s.get(ctx, "/subscriptions", params)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func retrieveSubscription(ctx context.Context, s *stripe, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	id := r.Str("id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := s.get(ctx, fmt.Sprintf("/subscriptions/%s", id), nil)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func searchSubscriptions(ctx context.Context, s *stripe, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	_ = r.Str("query")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := s.get(ctx, "/subscriptions/search", searchParamsFrom(args))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func createSubscription(ctx context.Context, s *stripe, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	customer := r.Str("customer")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if _, ok := args["items"]; !ok {
		return mcp.ErrResult(fmt.Errorf("stripe: items is required"))
	}
	body := map[string]any{
		"customer": customer,
		"items":    args["items"],
	}
	copyIfPresent(body, args, "default_payment_method", "default_source", "trial_period_days", "trial_end", "trial_from_plan", "collection_method", "days_until_due", "coupon", "promotion_code", "metadata", "off_session", "payment_behavior", "proration_behavior", "billing_cycle_anchor", "cancel_at", "cancel_at_period_end", "automatic_tax", "description", "currency")
	data, err := s.post(ctx, "/subscriptions", body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func updateSubscription(ctx context.Context, s *stripe, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	id := r.Str("id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	body := map[string]any{}
	copyIfPresent(body, args, "items", "cancel_at_period_end", "cancel_at", "default_payment_method", "default_source", "proration_behavior", "trial_end", "metadata", "pause_collection", "collection_method", "days_until_due", "coupon", "promotion_code", "description", "billing_cycle_anchor", "payment_behavior", "automatic_tax")
	data, err := s.post(ctx, fmt.Sprintf("/subscriptions/%s", id), body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func cancelSubscription(ctx context.Context, s *stripe, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	id := r.Str("id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	// Stripe's cancel uses DELETE for immediate cancellation.
	// Pass invoice_now / prorate / cancellation_details as form params.
	body := map[string]any{}
	copyIfPresent(body, args, "invoice_now", "prorate", "cancellation_details")
	data, err := s.del(ctx, fmt.Sprintf("/subscriptions/%s", id), body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

// --- Subscription Items ---

func listSubscriptionItems(ctx context.Context, s *stripe, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	sub := r.Str("subscription")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	params := listParamsFrom(args)
	params["subscription"] = sub
	data, err := s.get(ctx, "/subscription_items", params)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func retrieveSubscriptionItem(ctx context.Context, s *stripe, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	id := r.Str("id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := s.get(ctx, fmt.Sprintf("/subscription_items/%s", id), nil)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

// --- Products ---

func listProducts(ctx context.Context, s *stripe, args map[string]any) (*mcp.ToolResult, error) {
	params := listParamsFrom(args, "active", "ids", "shippable", "url", "created")
	data, err := s.get(ctx, "/products", params)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func retrieveProduct(ctx context.Context, s *stripe, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	id := r.Str("id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := s.get(ctx, fmt.Sprintf("/products/%s", id), nil)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func createProduct(ctx context.Context, s *stripe, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	name := r.Str("name")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	body := map[string]any{"name": name}
	copyIfPresent(body, args, "id", "description", "active", "default_price_data", "images", "metadata", "shippable", "tax_code", "url", "package_dimensions", "statement_descriptor", "unit_label", "features")
	data, err := s.post(ctx, "/products", body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func updateProduct(ctx context.Context, s *stripe, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	id := r.Str("id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	body := map[string]any{}
	copyIfPresent(body, args, "name", "description", "active", "default_price", "images", "metadata", "shippable", "url", "tax_code", "package_dimensions", "statement_descriptor", "unit_label", "features")
	data, err := s.post(ctx, fmt.Sprintf("/products/%s", id), body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func deleteProduct(ctx context.Context, s *stripe, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	id := r.Str("id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := s.del(ctx, fmt.Sprintf("/products/%s", id), nil)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

// --- Prices ---

func listPrices(ctx context.Context, s *stripe, args map[string]any) (*mcp.ToolResult, error) {
	params := listParamsFrom(args, "product", "active", "currency", "type", "created", "lookup_keys")
	data, err := s.get(ctx, "/prices", params)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func retrievePrice(ctx context.Context, s *stripe, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	id := r.Str("id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := s.get(ctx, fmt.Sprintf("/prices/%s", id), nil)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func createPrice(ctx context.Context, s *stripe, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	currency := r.Str("currency")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	body := map[string]any{"currency": currency}
	copyIfPresent(body, args, "product", "product_data", "unit_amount", "unit_amount_decimal", "recurring", "nickname", "active", "billing_scheme", "tiers", "tiers_mode", "metadata", "lookup_key", "transfer_lookup_key", "tax_behavior", "currency_options")
	data, err := s.post(ctx, "/prices", body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func updatePrice(ctx context.Context, s *stripe, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	id := r.Str("id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	body := map[string]any{}
	copyIfPresent(body, args, "active", "nickname", "metadata", "lookup_key", "transfer_lookup_key", "tax_behavior", "currency_options")
	data, err := s.post(ctx, fmt.Sprintf("/prices/%s", id), body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}
