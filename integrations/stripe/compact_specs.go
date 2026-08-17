package stripe

import (
	"fmt"

	mcp "github.com/daltoniam/switchboard"
)

// rawFieldCompactionSpecs maps Stripe list/retrieve tools to the subset of
// JSON fields the LLM needs for routing decisions. Stripe list responses
// always have shape: {"object":"list","data":[...],"has_more":bool}.
// We always include "object" and "has_more" so the LLM can paginate.
//
// Single-object retrieve specs use top-level dotted paths.
var rawFieldCompactionSpecs = map[mcp.ToolName][]string{
	// ── Balance ──────────────────────────────────────────────────────
	// stripe_get_balance returns a balance object (no list), keep top-level only.
	mcp.ToolName("stripe_get_balance"): {
		"object", "livemode",
		"available[].amount", "available[].currency",
		"pending[].amount", "pending[].currency",
		"connect_reserved[].amount", "connect_reserved[].currency",
		"instant_available[].amount", "instant_available[].currency",
	},
	mcp.ToolName("stripe_list_balance_transactions"): {
		"object", "has_more",
		"data[].id", "data[].type", "data[].status",
		"data[].amount", "data[].fee", "data[].net", "data[].currency",
		"data[].created", "data[].available_on",
		"data[].source", "data[].description",
	},
	mcp.ToolName("stripe_retrieve_balance_transaction"): {
		"id", "object", "type", "status", "amount", "fee", "net", "currency",
		"created", "available_on", "source", "description",
	},

	// ── Customers ────────────────────────────────────────────────────
	mcp.ToolName("stripe_list_customers"): {
		"object", "has_more",
		"data[].id", "data[].email", "data[].name", "data[].phone",
		"data[].description", "data[].created", "data[].currency",
		"data[].balance", "data[].delinquent", "data[].default_source",
		"data[].invoice_prefix",
	},
	mcp.ToolName("stripe_retrieve_customer"): {
		"id", "object", "email", "name", "phone", "description",
		"created", "currency", "balance", "delinquent", "default_source",
		"invoice_prefix", "address", "shipping",
		"invoice_settings.default_payment_method",
	},
	mcp.ToolName("stripe_search_customers"): {
		"object", "has_more", "next_page", "total_count",
		"data[].id", "data[].email", "data[].name", "data[].created",
		"data[].balance", "data[].delinquent", "data[].description",
	},

	// ── Charges ──────────────────────────────────────────────────────
	mcp.ToolName("stripe_list_charges"): {
		"object", "has_more",
		"data[].id", "data[].amount", "data[].amount_captured", "data[].amount_refunded",
		"data[].currency", "data[].status", "data[].paid", "data[].captured", "data[].refunded",
		"data[].created", "data[].customer", "data[].payment_intent", "data[].payment_method",
		"data[].description", "data[].failure_code", "data[].failure_message",
		"data[].receipt_email", "data[].disputed",
		"data[].outcome.network_status", "data[].outcome.risk_level", "data[].outcome.seller_message",
	},
	mcp.ToolName("stripe_retrieve_charge"): {
		"id", "object", "amount", "amount_captured", "amount_refunded",
		"currency", "status", "paid", "captured", "refunded", "disputed",
		"created", "customer", "payment_intent", "payment_method",
		"description", "failure_code", "failure_message", "receipt_email",
		"receipt_url", "billing_details.email", "billing_details.name",
		"outcome.network_status", "outcome.risk_level", "outcome.seller_message",
		"payment_method_details.type",
		"payment_method_details.card.brand", "payment_method_details.card.last4",
		"payment_method_details.card.exp_month", "payment_method_details.card.exp_year",
		"payment_method_details.card.country",
		"refunds.data[].id", "refunds.data[].amount", "refunds.data[].status",
	},
	mcp.ToolName("stripe_search_charges"): {
		"object", "has_more", "next_page", "total_count",
		"data[].id", "data[].amount", "data[].currency", "data[].status",
		"data[].created", "data[].customer", "data[].payment_intent",
		"data[].description",
	},

	// ── Payment Intents ──────────────────────────────────────────────
	mcp.ToolName("stripe_list_payment_intents"): {
		"object", "has_more",
		"data[].id", "data[].amount", "data[].amount_received", "data[].currency",
		"data[].status", "data[].created", "data[].customer", "data[].payment_method",
		"data[].latest_charge", "data[].description", "data[].capture_method",
		"data[].cancellation_reason", "data[].canceled_at",
		"data[].next_action.type",
	},
	mcp.ToolName("stripe_retrieve_payment_intent"): {
		"id", "object", "amount", "amount_received", "amount_capturable",
		"currency", "status", "created", "customer", "payment_method",
		"latest_charge", "description", "receipt_email", "capture_method",
		"confirmation_method", "cancellation_reason", "canceled_at",
		"client_secret", "next_action.type", "next_action.redirect_to_url.url",
		"payment_method_types",
		"last_payment_error.code", "last_payment_error.message", "last_payment_error.type",
	},
	mcp.ToolName("stripe_search_payment_intents"): {
		"object", "has_more", "next_page", "total_count",
		"data[].id", "data[].amount", "data[].currency", "data[].status",
		"data[].created", "data[].customer", "data[].latest_charge",
	},

	// ── Refunds ──────────────────────────────────────────────────────
	mcp.ToolName("stripe_list_refunds"): {
		"object", "has_more",
		"data[].id", "data[].amount", "data[].currency", "data[].status",
		"data[].created", "data[].charge", "data[].payment_intent",
		"data[].reason", "data[].description",
	},
	mcp.ToolName("stripe_retrieve_refund"): {
		"id", "object", "amount", "currency", "status", "created",
		"charge", "payment_intent", "reason", "description",
		"failure_reason", "receipt_number",
	},

	// ── Disputes ─────────────────────────────────────────────────────
	mcp.ToolName("stripe_list_disputes"): {
		"object", "has_more",
		"data[].id", "data[].amount", "data[].currency", "data[].status",
		"data[].reason", "data[].created", "data[].charge", "data[].payment_intent",
		"data[].is_charge_refundable", "data[].evidence_details.due_by",
		"data[].evidence_details.has_evidence", "data[].evidence_details.submission_count",
	},
	mcp.ToolName("stripe_retrieve_dispute"): {
		"id", "object", "amount", "currency", "status", "reason",
		"created", "charge", "payment_intent", "is_charge_refundable",
		"evidence_details.due_by", "evidence_details.has_evidence",
		"evidence_details.past_due", "evidence_details.submission_count",
		"network_reason_code",
	},

	// ── Payouts ──────────────────────────────────────────────────────
	mcp.ToolName("stripe_list_payouts"): {
		"object", "has_more",
		"data[].id", "data[].amount", "data[].currency", "data[].status",
		"data[].arrival_date", "data[].created", "data[].method", "data[].type",
		"data[].destination", "data[].description", "data[].failure_code",
		"data[].failure_message", "data[].automatic",
	},
	mcp.ToolName("stripe_retrieve_payout"): {
		"id", "object", "amount", "currency", "status", "arrival_date",
		"created", "method", "type", "destination", "description",
		"failure_code", "failure_message", "automatic", "balance_transaction",
		"source_type", "statement_descriptor",
	},

	// ── Subscriptions ────────────────────────────────────────────────
	mcp.ToolName("stripe_list_subscriptions"): {
		"object", "has_more",
		"data[].id", "data[].status", "data[].customer", "data[].created",
		"data[].current_period_start", "data[].current_period_end",
		"data[].cancel_at_period_end", "data[].canceled_at", "data[].cancel_at",
		"data[].trial_start", "data[].trial_end", "data[].collection_method",
		"data[].currency", "data[].default_payment_method",
		"data[].items.data[].id", "data[].items.data[].quantity",
		"data[].items.data[].price.id", "data[].items.data[].price.nickname",
		"data[].items.data[].price.unit_amount", "data[].items.data[].price.currency",
		"data[].items.data[].price.recurring.interval", "data[].items.data[].price.recurring.interval_count",
		"data[].items.data[].price.product",
	},
	mcp.ToolName("stripe_retrieve_subscription"): {
		"id", "object", "status", "customer", "created",
		"current_period_start", "current_period_end",
		"cancel_at_period_end", "canceled_at", "cancel_at", "ended_at",
		"trial_start", "trial_end", "collection_method", "currency",
		"default_payment_method", "default_source", "latest_invoice",
		"items.data[].id", "items.data[].quantity",
		"items.data[].price.id", "items.data[].price.nickname",
		"items.data[].price.unit_amount", "items.data[].price.currency",
		"items.data[].price.recurring.interval", "items.data[].price.recurring.interval_count",
		"items.data[].price.product",
	},
	mcp.ToolName("stripe_search_subscriptions"): {
		"object", "has_more", "next_page", "total_count",
		"data[].id", "data[].status", "data[].customer", "data[].created",
		"data[].current_period_end", "data[].cancel_at_period_end",
	},
	mcp.ToolName("stripe_list_subscription_items"): {
		"object", "has_more",
		"data[].id", "data[].quantity", "data[].subscription",
		"data[].created",
		"data[].price.id", "data[].price.nickname", "data[].price.unit_amount",
		"data[].price.currency", "data[].price.recurring.interval",
		"data[].price.recurring.interval_count", "data[].price.product",
	},
	mcp.ToolName("stripe_retrieve_subscription_item"): {
		"id", "object", "quantity", "subscription", "created",
		"price.id", "price.nickname", "price.unit_amount", "price.currency",
		"price.recurring.interval", "price.recurring.interval_count", "price.product",
	},

	// ── Products ─────────────────────────────────────────────────────
	mcp.ToolName("stripe_list_products"): {
		"object", "has_more",
		"data[].id", "data[].name", "data[].description", "data[].active",
		"data[].created", "data[].updated", "data[].default_price",
		"data[].shippable", "data[].type", "data[].url", "data[].unit_label",
	},
	mcp.ToolName("stripe_retrieve_product"): {
		"id", "object", "name", "description", "active", "created", "updated",
		"default_price", "shippable", "type", "url", "unit_label", "images",
		"tax_code", "statement_descriptor",
	},

	// ── Prices ───────────────────────────────────────────────────────
	mcp.ToolName("stripe_list_prices"): {
		"object", "has_more",
		"data[].id", "data[].product", "data[].nickname", "data[].active",
		"data[].currency", "data[].unit_amount", "data[].unit_amount_decimal",
		"data[].type", "data[].billing_scheme", "data[].lookup_key",
		"data[].recurring.interval", "data[].recurring.interval_count",
		"data[].recurring.usage_type", "data[].created",
	},
	mcp.ToolName("stripe_retrieve_price"): {
		"id", "object", "product", "nickname", "active", "currency",
		"unit_amount", "unit_amount_decimal", "type", "billing_scheme",
		"lookup_key", "tax_behavior", "created",
		"recurring.interval", "recurring.interval_count", "recurring.usage_type",
	},

	// ── Invoices ─────────────────────────────────────────────────────
	mcp.ToolName("stripe_list_invoices"): {
		"object", "has_more",
		"data[].id", "data[].number", "data[].status", "data[].customer",
		"data[].subscription", "data[].created", "data[].due_date",
		"data[].period_start", "data[].period_end",
		"data[].amount_due", "data[].amount_paid", "data[].amount_remaining",
		"data[].total", "data[].subtotal", "data[].currency",
		"data[].collection_method", "data[].paid", "data[].attempted",
		"data[].hosted_invoice_url", "data[].invoice_pdf",
	},
	mcp.ToolName("stripe_retrieve_invoice"): {
		"id", "object", "number", "status", "customer", "subscription",
		"created", "due_date", "period_start", "period_end",
		"amount_due", "amount_paid", "amount_remaining", "total", "subtotal",
		"currency", "collection_method", "paid", "attempted",
		"hosted_invoice_url", "invoice_pdf", "billing_reason", "description",
		"default_payment_method", "default_source",
		"lines.data[].id", "lines.data[].amount", "lines.data[].currency",
		"lines.data[].description", "lines.data[].quantity",
		"lines.data[].price.id", "lines.data[].price.unit_amount",
		"lines.data[].price.nickname", "lines.data[].price.product",
		"lines.data[].period.start", "lines.data[].period.end",
	},
	mcp.ToolName("stripe_retrieve_upcoming_invoice"): {
		"id", "object", "status", "customer", "subscription",
		"period_start", "period_end",
		"amount_due", "amount_paid", "amount_remaining", "total", "subtotal",
		"currency", "billing_reason",
		"lines.data[].amount", "lines.data[].currency", "lines.data[].description",
		"lines.data[].quantity",
		"lines.data[].price.id", "lines.data[].price.unit_amount",
		"lines.data[].price.nickname", "lines.data[].price.product",
		"lines.data[].period.start", "lines.data[].period.end",
	},
	mcp.ToolName("stripe_search_invoices"): {
		"object", "has_more", "next_page", "total_count",
		"data[].id", "data[].number", "data[].status", "data[].customer",
		"data[].total", "data[].currency", "data[].created", "data[].due_date",
	},
	mcp.ToolName("stripe_list_invoice_items"): {
		"object", "has_more",
		"data[].id", "data[].amount", "data[].currency", "data[].description",
		"data[].customer", "data[].invoice", "data[].subscription",
		"data[].quantity", "data[].date",
		"data[].price.id", "data[].price.unit_amount", "data[].price.nickname",
		"data[].price.product",
	},

	// ── Payment Methods ──────────────────────────────────────────────
	mcp.ToolName("stripe_list_payment_methods"): {
		"object", "has_more",
		"data[].id", "data[].type", "data[].customer", "data[].created",
		"data[].billing_details.email", "data[].billing_details.name",
		"data[].card.brand", "data[].card.last4", "data[].card.exp_month",
		"data[].card.exp_year", "data[].card.country", "data[].card.funding",
		"data[].us_bank_account.bank_name", "data[].us_bank_account.last4",
	},
	mcp.ToolName("stripe_retrieve_payment_method"): {
		"id", "object", "type", "customer", "created",
		"billing_details.email", "billing_details.name", "billing_details.phone",
		"billing_details.address",
		"card.brand", "card.last4", "card.exp_month", "card.exp_year",
		"card.country", "card.funding", "card.fingerprint",
		"us_bank_account.bank_name", "us_bank_account.last4",
	},

	// ── Setup Intents ────────────────────────────────────────────────
	mcp.ToolName("stripe_list_setup_intents"): {
		"object", "has_more",
		"data[].id", "data[].status", "data[].customer", "data[].payment_method",
		"data[].created", "data[].usage", "data[].description",
		"data[].cancellation_reason", "data[].latest_attempt",
		"data[].next_action.type",
	},
	mcp.ToolName("stripe_retrieve_setup_intent"): {
		"id", "object", "status", "customer", "payment_method", "created",
		"usage", "description", "cancellation_reason", "latest_attempt",
		"client_secret", "next_action.type", "next_action.redirect_to_url.url",
		"payment_method_types",
		"last_setup_error.code", "last_setup_error.message", "last_setup_error.type",
	},

	// ── Coupons / Promotion Codes ────────────────────────────────────
	mcp.ToolName("stripe_list_coupons"): {
		"object", "has_more",
		"data[].id", "data[].name", "data[].valid", "data[].duration",
		"data[].duration_in_months", "data[].percent_off", "data[].amount_off",
		"data[].currency", "data[].max_redemptions", "data[].times_redeemed",
		"data[].redeem_by", "data[].created",
	},
	mcp.ToolName("stripe_retrieve_coupon"): {
		"id", "object", "name", "valid", "duration", "duration_in_months",
		"percent_off", "amount_off", "currency", "max_redemptions",
		"times_redeemed", "redeem_by", "created",
	},
	mcp.ToolName("stripe_list_promotion_codes"): {
		"object", "has_more",
		"data[].id", "data[].code", "data[].active", "data[].coupon.id",
		"data[].coupon.percent_off", "data[].coupon.amount_off",
		"data[].customer", "data[].expires_at", "data[].max_redemptions",
		"data[].times_redeemed", "data[].created",
	},

	// ── Events ───────────────────────────────────────────────────────
	mcp.ToolName("stripe_list_events"): {
		"object", "has_more",
		"data[].id", "data[].type", "data[].created", "data[].livemode",
		"data[].api_version", "data[].pending_webhooks",
		"data[].data.object.id", "data[].data.object.object",
	},
	mcp.ToolName("stripe_retrieve_event"): {
		"id", "object", "type", "created", "livemode", "api_version",
		"pending_webhooks",
		"data.object.id", "data.object.object",
		"request.id", "request.idempotency_key",
	},
}

var fieldCompactionSpecs = mustBuildFieldCompactionSpecs(rawFieldCompactionSpecs)

func mustBuildFieldCompactionSpecs(raw map[mcp.ToolName][]string) map[mcp.ToolName][]mcp.CompactField {
	parsed := make(map[mcp.ToolName][]mcp.CompactField, len(raw))
	for tool, specs := range raw {
		fields, err := mcp.ParseCompactSpecs(specs)
		if err != nil {
			panic(fmt.Sprintf("stripe: invalid field compaction spec for %q: %v", tool, err))
		}
		parsed[tool] = fields
	}
	return parsed
}
