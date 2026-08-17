package stripe

import (
	"context"
	"fmt"

	mcp "github.com/daltoniam/switchboard"
)

// --- Payment Methods ---

func listPaymentMethods(ctx context.Context, s *stripe, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	customer := r.Str("customer")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	params := listParamsFrom(args, "type")
	params["customer"] = customer
	data, err := s.get(ctx, "/payment_methods", params)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func retrievePaymentMethod(ctx context.Context, s *stripe, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	id := r.Str("id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := s.get(ctx, fmt.Sprintf("/payment_methods/%s", id), nil)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func attachPaymentMethod(ctx context.Context, s *stripe, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	id := r.Str("id")
	customer := r.Str("customer")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	body := map[string]any{"customer": customer}
	data, err := s.post(ctx, fmt.Sprintf("/payment_methods/%s/attach", id), body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func detachPaymentMethod(ctx context.Context, s *stripe, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	id := r.Str("id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := s.post(ctx, fmt.Sprintf("/payment_methods/%s/detach", id), nil)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

// --- Setup Intents ---

func listSetupIntents(ctx context.Context, s *stripe, args map[string]any) (*mcp.ToolResult, error) {
	params := listParamsFrom(args, "customer", "payment_method", "created", "attach_to_self")
	data, err := s.get(ctx, "/setup_intents", params)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func retrieveSetupIntent(ctx context.Context, s *stripe, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	id := r.Str("id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := s.get(ctx, fmt.Sprintf("/setup_intents/%s", id), nil)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func createSetupIntent(ctx context.Context, s *stripe, args map[string]any) (*mcp.ToolResult, error) {
	body := map[string]any{}
	copyIfPresent(body, args, "customer", "payment_method", "payment_method_types", "usage", "confirm", "description", "metadata", "automatic_payment_methods", "return_url", "on_behalf_of", "single_use", "mandate_data")
	data, err := s.post(ctx, "/setup_intents", body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

// --- Coupons ---

func listCoupons(ctx context.Context, s *stripe, args map[string]any) (*mcp.ToolResult, error) {
	params := listParamsFrom(args, "created")
	data, err := s.get(ctx, "/coupons", params)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func retrieveCoupon(ctx context.Context, s *stripe, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	id := r.Str("id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := s.get(ctx, fmt.Sprintf("/coupons/%s", id), nil)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func createCoupon(ctx context.Context, s *stripe, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	duration := r.Str("duration")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	body := map[string]any{"duration": duration}
	copyIfPresent(body, args, "id", "name", "percent_off", "amount_off", "currency", "duration_in_months", "max_redemptions", "redeem_by", "metadata", "applies_to", "currency_options")
	data, err := s.post(ctx, "/coupons", body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func deleteCoupon(ctx context.Context, s *stripe, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	id := r.Str("id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := s.del(ctx, fmt.Sprintf("/coupons/%s", id), nil)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func listPromotionCodes(ctx context.Context, s *stripe, args map[string]any) (*mcp.ToolResult, error) {
	params := listParamsFrom(args, "coupon", "customer", "active", "code", "created")
	data, err := s.get(ctx, "/promotion_codes", params)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func createPromotionCode(ctx context.Context, s *stripe, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	coupon := r.Str("coupon")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	body := map[string]any{"coupon": coupon}
	copyIfPresent(body, args, "code", "customer", "max_redemptions", "expires_at", "active", "metadata", "restrictions")
	data, err := s.post(ctx, "/promotion_codes", body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

// --- Events ---

func listEvents(ctx context.Context, s *stripe, args map[string]any) (*mcp.ToolResult, error) {
	params := listParamsFrom(args, "type", "types", "created", "delivery_success")
	data, err := s.get(ctx, "/events", params)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func retrieveEvent(ctx context.Context, s *stripe, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	id := r.Str("id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := s.get(ctx, fmt.Sprintf("/events/%s", id), nil)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}
