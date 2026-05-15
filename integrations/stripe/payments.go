package stripe

import (
	"context"
	"fmt"

	mcp "github.com/daltoniam/switchboard"
)

// --- Charges ---

func listCharges(ctx context.Context, s *stripe, args map[string]any) (*mcp.ToolResult, error) {
	params := listParamsFrom(args, "customer", "payment_intent", "transfer_group", "created")
	data, err := s.get(ctx, "/charges", params)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func retrieveCharge(ctx context.Context, s *stripe, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	id := r.Str("id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := s.get(ctx, fmt.Sprintf("/charges/%s", id), nil)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func searchCharges(ctx context.Context, s *stripe, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	_ = r.Str("query")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := s.get(ctx, "/charges/search", searchParamsFrom(args))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func captureCharge(ctx context.Context, s *stripe, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	id := r.Str("id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	body := map[string]any{}
	copyIfPresent(body, args, "amount", "application_fee_amount", "receipt_email", "statement_descriptor", "transfer_data", "transfer_group")
	data, err := s.post(ctx, fmt.Sprintf("/charges/%s/capture", id), body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

// --- Payment Intents ---

func listPaymentIntents(ctx context.Context, s *stripe, args map[string]any) (*mcp.ToolResult, error) {
	params := listParamsFrom(args, "customer", "created")
	data, err := s.get(ctx, "/payment_intents", params)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func retrievePaymentIntent(ctx context.Context, s *stripe, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	id := r.Str("id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := s.get(ctx, fmt.Sprintf("/payment_intents/%s", id), nil)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func createPaymentIntent(ctx context.Context, s *stripe, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	amount := r.Int("amount")
	currency := r.Str("currency")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	body := map[string]any{
		"amount":   amount,
		"currency": currency,
	}
	copyIfPresent(body, args, "customer", "payment_method", "payment_method_types", "description", "receipt_email", "statement_descriptor", "statement_descriptor_suffix", "capture_method", "confirm", "off_session", "metadata", "transfer_data", "application_fee_amount", "automatic_payment_methods", "setup_future_usage", "shipping", "return_url")
	data, err := s.post(ctx, "/payment_intents", body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func updatePaymentIntent(ctx context.Context, s *stripe, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	id := r.Str("id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	body := map[string]any{}
	copyIfPresent(body, args, "amount", "currency", "customer", "description", "metadata", "payment_method", "receipt_email", "statement_descriptor", "statement_descriptor_suffix", "shipping", "transfer_data", "application_fee_amount", "setup_future_usage")
	data, err := s.post(ctx, fmt.Sprintf("/payment_intents/%s", id), body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func confirmPaymentIntent(ctx context.Context, s *stripe, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	id := r.Str("id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	body := map[string]any{}
	copyIfPresent(body, args, "payment_method", "return_url", "off_session", "receipt_email", "shipping", "use_stripe_sdk", "mandate", "mandate_data")
	data, err := s.post(ctx, fmt.Sprintf("/payment_intents/%s/confirm", id), body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func cancelPaymentIntent(ctx context.Context, s *stripe, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	id := r.Str("id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	body := map[string]any{}
	copyIfPresent(body, args, "cancellation_reason")
	data, err := s.post(ctx, fmt.Sprintf("/payment_intents/%s/cancel", id), body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func searchPaymentIntents(ctx context.Context, s *stripe, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	_ = r.Str("query")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := s.get(ctx, "/payment_intents/search", searchParamsFrom(args))
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

// --- Refunds ---

func listRefunds(ctx context.Context, s *stripe, args map[string]any) (*mcp.ToolResult, error) {
	params := listParamsFrom(args, "charge", "payment_intent", "created")
	data, err := s.get(ctx, "/refunds", params)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func retrieveRefund(ctx context.Context, s *stripe, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	id := r.Str("id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := s.get(ctx, fmt.Sprintf("/refunds/%s", id), nil)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func createRefund(ctx context.Context, s *stripe, args map[string]any) (*mcp.ToolResult, error) {
	body := map[string]any{}
	copyIfPresent(body, args, "charge", "payment_intent", "amount", "reason", "refund_application_fee", "reverse_transfer", "metadata", "instructions_email")
	data, err := s.post(ctx, "/refunds", body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func updateRefund(ctx context.Context, s *stripe, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	id := r.Str("id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	body := map[string]any{}
	copyIfPresent(body, args, "metadata")
	data, err := s.post(ctx, fmt.Sprintf("/refunds/%s", id), body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

// --- Disputes ---

func listDisputes(ctx context.Context, s *stripe, args map[string]any) (*mcp.ToolResult, error) {
	params := listParamsFrom(args, "charge", "payment_intent", "created")
	data, err := s.get(ctx, "/disputes", params)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func retrieveDispute(ctx context.Context, s *stripe, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	id := r.Str("id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := s.get(ctx, fmt.Sprintf("/disputes/%s", id), nil)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func updateDispute(ctx context.Context, s *stripe, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	id := r.Str("id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	body := map[string]any{}
	copyIfPresent(body, args, "evidence", "submit", "metadata")
	data, err := s.post(ctx, fmt.Sprintf("/disputes/%s", id), body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

// --- Payouts ---

func listPayouts(ctx context.Context, s *stripe, args map[string]any) (*mcp.ToolResult, error) {
	params := listParamsFrom(args, "status", "destination", "arrival_date", "created")
	data, err := s.get(ctx, "/payouts", params)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func retrievePayout(ctx context.Context, s *stripe, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	id := r.Str("id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	data, err := s.get(ctx, fmt.Sprintf("/payouts/%s", id), nil)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

func createPayout(ctx context.Context, s *stripe, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	amount := r.Int("amount")
	currency := r.Str("currency")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	body := map[string]any{
		"amount":   amount,
		"currency": currency,
	}
	copyIfPresent(body, args, "description", "method", "destination", "metadata", "source_type", "statement_descriptor")
	data, err := s.post(ctx, "/payouts", body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}
