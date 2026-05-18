package stripe

import mcp "github.com/daltoniam/switchboard"

// Standard list-pagination params reused across many list tools.
var listParams = []mcp.Parameter{
	{Name: mcp.ParamName("limit"), Description: "Number of objects to return (1-100, default 10)"},
	{Name: mcp.ParamName("starting_after"), Description: "Cursor for pagination — an object ID for the next page"},
	{Name: mcp.ParamName("ending_before"), Description: "Cursor for pagination — an object ID for the previous page"},
}

var tools = []mcp.ToolDefinition{
	// ── Balance ──────────────────────────────────────────────────────
	{
		Name:        mcp.ToolName("stripe_get_balance"),
		Description: "Get the current Stripe account balance (available, pending, connect_reserved). Start here to inspect funds on the Stripe account.",
	},
	{
		Name:        mcp.ToolName("stripe_list_balance_transactions"),
		Description: "List balance transactions (charges, refunds, payouts, fees, transfers) that affected the Stripe account balance. Use for accounting reconciliation, financial audits, fee analysis, and tracking money movement on the Stripe platform.",
		Parameters: mergeParams(listParams, []mcp.Parameter{{Name: mcp.ParamName("type"), Description: "Filter by type (charge, refund, payout, transfer, adjustment, fee, etc.)"}, {Name: mcp.ParamName("currency"), Description: "Three-letter ISO currency code lowercase (e.g. usd)"}, {Name: mcp.ParamName("payout"), Description: "Filter to transactions paid out in this payout ID"}, {Name: mcp.ParamName("source"), Description: "Filter by source ID (charge, refund, etc.)"}}),
	},
	{
		Name:        mcp.ToolName("stripe_retrieve_balance_transaction"),
		Description: "Retrieve (get) a single balance transaction by ID.",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("id"), Description: "Balance transaction ID (e.g. txn_...)", Required: true}},
	},

	// ── Customers ────────────────────────────────────────────────────
	{
		Name:        mcp.ToolName("stripe_list_customers"),
		Description: "List Stripe customers. Use for browsing payers, finding accounts by email, exporting customer rosters, or paginating the customer directory.",
		Parameters: mergeParams(listParams, []mcp.Parameter{{Name: mcp.ParamName("email"), Description: "Filter by exact email match"}, {Name: mcp.ParamName("created"), Description: "Filter by created timestamp (Unix epoch seconds) — pass a number or use nested keys gt/gte/lt/lte for ranges"}}),
	},
	{
		Name:        mcp.ToolName("stripe_retrieve_customer"),
		Description: "Retrieve (get) a single customer by ID including their default payment method, billing address, and metadata.",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("id"), Description: "Customer ID (e.g. cus_...)", Required: true}},
	},
	{
		Name:        mcp.ToolName("stripe_search_customers"),
		Description: "Search Stripe customers using Stripe search query language (e.g. email:\"alice@example.com\" or metadata['plan']:\"pro\"). Returns matching customer records.",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("query"), Description: "Stripe search query syntax", Required: true}, {Name: mcp.ParamName("limit"), Description: "Number of results (1-100)"}, {Name: mcp.ParamName("page"), Description: "Cursor for pagination (from prior response.next_page)"}},
	},
	{
		Name:        mcp.ToolName("stripe_create_customer"),
		Description: "Create a new Stripe customer record for a payer.",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("email"), Description: "Customer email address"}, {Name: mcp.ParamName("name"), Description: "Full customer name"}, {Name: mcp.ParamName("phone"), Description: "Phone number"}, {Name: mcp.ParamName("description"), Description: "Arbitrary description for internal use"}, {Name: mcp.ParamName("metadata"), Description: "Object of key-value strings to attach (max 50 keys)"}, {Name: mcp.ParamName("address"), Description: "Object with line1, line2, city, state, postal_code, country"}, {Name: mcp.ParamName("shipping"), Description: "Object with name, phone, address"}},
	},
	{
		Name:        mcp.ToolName("stripe_update_customer"),
		Description: "Update (edit) a customer's profile, contact info, default payment method, or metadata.",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("id"), Description: "Customer ID", Required: true}, {Name: mcp.ParamName("email"), Description: "New email"}, {Name: mcp.ParamName("name"), Description: "New name"}, {Name: mcp.ParamName("phone"), Description: "New phone"}, {Name: mcp.ParamName("description"), Description: "New description"}, {Name: mcp.ParamName("metadata"), Description: "Object to merge into existing metadata (set key to empty string to clear it)"}, {Name: mcp.ParamName("default_source"), Description: "Default payment source ID"}, {Name: mcp.ParamName("invoice_settings"), Description: "Object with default_payment_method, custom_fields, footer"}, {Name: mcp.ParamName("address"), Description: "Billing address object"}, {Name: mcp.ParamName("shipping"), Description: "Shipping address object"}},
	},
	{
		Name:        mcp.ToolName("stripe_delete_customer"),
		Description: "Delete (remove) a customer permanently. Cancels active subscriptions and disassociates payment methods.",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("id"), Description: "Customer ID", Required: true}},
	},

	// ── Charges ──────────────────────────────────────────────────────
	{
		Name:        mcp.ToolName("stripe_list_charges"),
		Description: "List charges (card and bank transactions) processed on the Stripe account. Use for transaction history, sales reports, revenue analytics, and finding individual successful or failed payments.",
		Parameters: mergeParams(listParams, []mcp.Parameter{{Name: mcp.ParamName("customer"), Description: "Filter by customer ID"}, {Name: mcp.ParamName("payment_intent"), Description: "Filter by payment intent ID"}, {Name: mcp.ParamName("transfer_group"), Description: "Filter by transfer group"}, {Name: mcp.ParamName("created"), Description: "Filter by creation timestamp"}}),
	},
	{
		Name:        mcp.ToolName("stripe_retrieve_charge"),
		Description: "Retrieve (get) a single charge by ID with full details (amount, currency, status, customer, payment_method, refunds, dispute, outcome).",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("id"), Description: "Charge ID (e.g. ch_...)", Required: true}},
	},
	{
		Name:        mcp.ToolName("stripe_search_charges"),
		Description: "Search charges using Stripe search query language (e.g. amount>1000 AND status:\"succeeded\").",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("query"), Description: "Stripe search query", Required: true}, {Name: mcp.ParamName("limit"), Description: "Number of results (1-100)"}, {Name: mcp.ParamName("page"), Description: "Cursor for pagination"}},
	},
	{
		Name:        mcp.ToolName("stripe_capture_charge"),
		Description: "Capture a previously authorized but uncaptured charge (manual capture flow).",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("id"), Description: "Charge ID", Required: true}, {Name: mcp.ParamName("amount"), Description: "Optional amount to capture in smallest currency unit (cents). Defaults to full authorized amount."}},
	},

	// ── Payment Intents ──────────────────────────────────────────────
	{
		Name:        mcp.ToolName("stripe_list_payment_intents"),
		Description: "List PaymentIntents — the modern payment flow object representing intent to collect from a customer. Use for monitoring in-progress, succeeded, requires_action, or failed payments.",
		Parameters: mergeParams(listParams, []mcp.Parameter{{Name: mcp.ParamName("customer"), Description: "Filter by customer ID"}, {Name: mcp.ParamName("created"), Description: "Filter by creation timestamp"}}),
	},
	{
		Name:        mcp.ToolName("stripe_retrieve_payment_intent"),
		Description: "Retrieve (get) a single PaymentIntent by ID with status, next_action, latest_charge, and client_secret.",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("id"), Description: "PaymentIntent ID (e.g. pi_...)", Required: true}},
	},
	{
		Name:        mcp.ToolName("stripe_create_payment_intent"),
		Description: "Create a new PaymentIntent to charge a customer. Amounts are in the smallest currency unit (e.g. cents for USD).",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("amount"), Description: "Amount in smallest currency unit (e.g. 1099 = $10.99)", Required: true}, {Name: mcp.ParamName("currency"), Description: "Three-letter ISO currency lowercase (e.g. usd)", Required: true}, {Name: mcp.ParamName("customer"), Description: "Customer ID to associate with this payment"}, {Name: mcp.ParamName("payment_method"), Description: "Payment method ID to charge"}, {Name: mcp.ParamName("payment_method_types"), Description: `Array of payment method types (e.g. ["card"])`}, {Name: mcp.ParamName("description"), Description: "Arbitrary description shown to the customer"}, {Name: mcp.ParamName("receipt_email"), Description: "Email address to send receipt to"}, {Name: mcp.ParamName("statement_descriptor"), Description: "Up to 22 chars shown on the customer's statement"}, {Name: mcp.ParamName("capture_method"), Description: "automatic or manual"}, {Name: mcp.ParamName("confirm"), Description: "If true, confirm the PaymentIntent in the same request (boolean)"}, {Name: mcp.ParamName("off_session"), Description: "Boolean indicating customer is not present"}, {Name: mcp.ParamName("metadata"), Description: "Object of key-value strings"}},
	},
	{
		Name:        mcp.ToolName("stripe_update_payment_intent"),
		Description: "Update (edit) a PaymentIntent before it is confirmed.",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("id"), Description: "PaymentIntent ID", Required: true}, {Name: mcp.ParamName("amount"), Description: "New amount in smallest currency unit"}, {Name: mcp.ParamName("currency"), Description: "Currency code"}, {Name: mcp.ParamName("customer"), Description: "Customer ID"}, {Name: mcp.ParamName("description"), Description: "New description"}, {Name: mcp.ParamName("metadata"), Description: "Metadata object"}, {Name: mcp.ParamName("payment_method"), Description: "Payment method ID"}, {Name: mcp.ParamName("receipt_email"), Description: "Receipt email address"}},
	},
	{
		Name:        mcp.ToolName("stripe_confirm_payment_intent"),
		Description: "Confirm a PaymentIntent to attempt to collect payment.",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("id"), Description: "PaymentIntent ID", Required: true}, {Name: mcp.ParamName("payment_method"), Description: "Payment method ID to attach for confirmation"}, {Name: mcp.ParamName("return_url"), Description: "URL to redirect after 3DS/redirect-based authentication"}, {Name: mcp.ParamName("off_session"), Description: "Boolean — confirming on behalf of an absent customer"}},
	},
	{
		Name:        mcp.ToolName("stripe_cancel_payment_intent"),
		Description: "Cancel a PaymentIntent that is in a cancelable state (requires_payment_method, requires_capture, requires_confirmation, requires_action, processing).",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("id"), Description: "PaymentIntent ID", Required: true}, {Name: mcp.ParamName("cancellation_reason"), Description: "duplicate, fraudulent, requested_by_customer, abandoned"}},
	},
	{
		Name:        mcp.ToolName("stripe_search_payment_intents"),
		Description: "Search PaymentIntents with Stripe search query (e.g. status:\"requires_action\" AND amount>5000).",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("query"), Description: "Stripe search query", Required: true}, {Name: mcp.ParamName("limit"), Description: "Number of results"}, {Name: mcp.ParamName(

		// ── Refunds ──────────────────────────────────────────────────────
		"page"), Description: "Cursor for pagination"}},
	},

	{
		Name:        mcp.ToolName("stripe_list_refunds"),
		Description: "List refunds processed on the Stripe account. Use for reviewing returned money, refund audits, and tracking refund status.",
		Parameters: mergeParams(listParams, []mcp.Parameter{{Name: mcp.ParamName("charge"), Description: "Filter by charge ID"}, {Name: mcp.ParamName("payment_intent"), Description: "Filter by payment intent ID"}, {Name: mcp.ParamName("created"), Description: "Filter by creation timestamp"}}),
	},
	{
		Name:        mcp.ToolName("stripe_retrieve_refund"),
		Description: "Retrieve (get) a single refund by ID.",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("id"), Description: "Refund ID (e.g. re_...)", Required: true}},
	},
	{
		Name:        mcp.ToolName("stripe_create_refund"),
		Description: "Create a refund (full or partial) for a charge or PaymentIntent. Returns money to the customer.",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("charge"), Description: "Charge ID to refund (one of charge or payment_intent required)"}, {Name: mcp.ParamName("payment_intent"), Description: "PaymentIntent ID to refund"}, {Name: mcp.ParamName("amount"), Description: "Amount to refund in smallest currency unit (defaults to full)"}, {Name: mcp.ParamName("reason"), Description: "duplicate, fraudulent, or requested_by_customer"}, {Name: mcp.ParamName("refund_application_fee"), Description: "Boolean — whether to refund the application fee"}, {Name: mcp.ParamName("reverse_transfer"), Description: "Boolean — reverse the transfer to a connected account"}, {Name: mcp.ParamName("metadata"), Description: "Metadata object"}},
	},
	{
		Name:        mcp.ToolName("stripe_update_refund"),
		Description: "Update (edit) a refund's metadata.",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("id"), Description: "Refund ID", Required: true}, {Name: mcp.ParamName("metadata"), Description: "Metadata object to merge"}},
	},

	// ── Disputes ─────────────────────────────────────────────────────
	{
		Name:        mcp.ToolName("stripe_list_disputes"),
		Description: "List chargeback disputes filed against charges on the Stripe account. Use for fraud monitoring, chargeback response workflows, and dispute analytics.",
		Parameters: mergeParams(listParams, []mcp.Parameter{{Name: mcp.ParamName("charge"), Description: "Filter by charge ID"}, {Name: mcp.ParamName("payment_intent"), Description: "Filter by payment intent ID"}, {Name: mcp.ParamName("created"), Description: "Filter by creation timestamp"}}),
	},
	{
		Name:        mcp.ToolName("stripe_retrieve_dispute"),
		Description: "Retrieve (get) a single dispute by ID including evidence and status.",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("id"), Description: "Dispute ID (e.g. dp_...)", Required: true}},
	},
	{
		Name:        mcp.ToolName("stripe_update_dispute"),
		Description: "Update (edit) a dispute to submit evidence for chargeback response.",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("id"), Description: "Dispute ID", Required: true}, {Name: mcp.ParamName("evidence"), Description: "Evidence object (e.g. customer_communication, receipt, service_documentation, shipping_documentation, etc.)"}, {Name: mcp.ParamName("submit"), Description: "Boolean — submit evidence immediately"},

		// ── Payouts ──────────────────────────────────────────────────────
		{Name: mcp.ParamName("metadata"), Description: "Metadata object"}},
	},

	{
		Name:        mcp.ToolName("stripe_list_payouts"),
		Description: "List payouts (transfers from the Stripe balance to a bank account). Use for cash-out tracking, settlement reconciliation, and finance reports.",
		Parameters: mergeParams(listParams, []mcp.Parameter{{Name: mcp.ParamName("status"), Description: "Filter by status (paid, pending, in_transit, canceled, failed)"}, {Name: mcp.ParamName("destination"), Description: "Filter by bank account or card destination ID"}, {Name: mcp.ParamName("arrival_date"), Description: "Filter by arrival_date timestamp"}, {Name: mcp.ParamName("created"), Description: "Filter by creation timestamp"}}),
	},
	{
		Name:        mcp.ToolName("stripe_retrieve_payout"),
		Description: "Retrieve (get) a single payout by ID.",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("id"), Description: "Payout ID (e.g. po_...)", Required: true}},
	},
	{
		Name:        mcp.ToolName("stripe_create_payout"),
		Description: "Create a manual payout from the Stripe balance to the default bank account.",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("amount"), Description: "Amount in smallest currency unit", Required: true}, {Name: mcp.ParamName("currency"), Description: "Three-letter ISO currency lowercase", Required: true}, {Name: mcp.ParamName("description"), Description: "Description"}, {Name: mcp.ParamName("method"), Description: "standard or instant"}, {Name: mcp.ParamName("destination"), Description:

		// ── Subscriptions ────────────────────────────────────────────────
		"Bank account or debit card ID (optional override)"}, {Name: mcp.ParamName("metadata"), Description: "Metadata object"}},
	},

	{
		Name:        mcp.ToolName("stripe_list_subscriptions"),
		Description: "List recurring billing subscriptions. Use for MRR/ARR reports, churn analysis, active customer counts, and finding subscriptions in trial, past_due, or canceled state.",
		Parameters: mergeParams(listParams, []mcp.Parameter{{Name: mcp.ParamName("customer"), Description: "Filter by customer ID"}, {Name: mcp.ParamName("price"), Description: "Filter by price ID"}, {Name: mcp.ParamName("status"), Description: "Filter by status (active, past_due, unpaid, canceled, incomplete, incomplete_expired, trialing, all)"}, {Name: mcp.ParamName("collection_method"), Description: "charge_automatically or send_invoice"}, {Name: mcp.ParamName("created"), Description: "Filter by creation timestamp"}}),
	},
	{
		Name:        mcp.ToolName("stripe_retrieve_subscription"),
		Description: "Retrieve (get) a single subscription by ID including items, current period, and billing cycle.",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("id"), Description: "Subscription ID (e.g. sub_...)", Required: true}},
	},
	{
		Name:        mcp.ToolName("stripe_search_subscriptions"),
		Description: "Search subscriptions using Stripe search query (e.g. status:\"trialing\" AND created>1700000000).",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("query"), Description: "Stripe search query", Required: true}, {Name: mcp.ParamName("limit"), Description: "Number of results"}, {Name: mcp.ParamName("page"), Description: "Cursor for pagination"}},
	},
	{
		Name:        mcp.ToolName("stripe_create_subscription"),
		Description: "Create a new subscription that recurringly bills a customer for one or more prices.",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("customer"), Description: "Customer ID to subscribe (required)", Required: true}, {Name: mcp.ParamName("items"), Description: "Array of subscription items, each with a 'price' ID and optional 'quantity'", Required: true}, {Name: mcp.ParamName("default_payment_method"), Description: "Payment method ID to use for invoices"}, {Name: mcp.ParamName("trial_period_days"), Description: "Number of days for free trial"}, {Name: mcp.ParamName("trial_end"), Description: "Unix timestamp when trial ends (or 'now')"}, {Name: mcp.ParamName("collection_method"), Description: "charge_automatically or send_invoice"}, {Name: mcp.ParamName("days_until_due"), Description: "Days until invoice is due (only for send_invoice)"}, {Name: mcp.ParamName("coupon"), Description: "Coupon ID to apply"}, {Name: mcp.ParamName("metadata"), Description: "Metadata object"}},
	},
	{
		Name:        mcp.ToolName("stripe_update_subscription"),
		Description: "Update (edit) a subscription's items, prices, quantities, billing settings, or metadata.",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("id"), Description: "Subscription ID", Required: true}, {Name: mcp.ParamName("items"), Description: "Updated items array — each item may include 'id' (existing item), 'price', 'quantity', or 'deleted':true"}, {Name: mcp.ParamName("cancel_at_period_end"), Description: "Boolean — cancel at end of current period"}, {Name: mcp.ParamName("default_payment_method"), Description: "Payment method ID"}, {Name: mcp.ParamName("proration_behavior"), Description: "create_prorations, none, or always_invoice"}, {Name: mcp.ParamName("trial_end"), Description: "Unix timestamp or 'now'"}, {Name: mcp.ParamName("metadata"), Description: "Metadata object"}, {Name: mcp.ParamName("pause_collection"), Description: "Object with behavior (keep_as_draft, mark_uncollectible, void) and resumes_at"}},
	},
	{
		Name:        mcp.ToolName("stripe_cancel_subscription"),
		Description: "Cancel a subscription immediately (or at period end via stripe_update_subscription with cancel_at_period_end).",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("id"), Description: "Subscription ID", Required: true}, {Name: mcp.ParamName("invoice_now"), Description: "Boolean — generate a final invoice for unbilled usage"}, {Name: mcp.ParamName("prorate"), Description: "Boolean — credit prorated unused time"}, {Name: mcp.ParamName("cancellation_details"), Description:

		// ── Subscription Items ───────────────────────────────────────────
		"Object with comment and feedback"}},
	},

	{
		Name:        mcp.ToolName("stripe_list_subscription_items"),
		Description: "List items (line items) belonging to a subscription.",
		Parameters: mergeParams(listParams, []mcp.Parameter{
			{Name: mcp.ParamName("subscription"), Description: "Subscription ID", Required: true},
		}),
	},
	{
		Name:        mcp.ToolName("stripe_retrieve_subscription_item"),
		Description: "Retrieve (get) a single subscription item by ID.",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("id"), Description: "Subscription item ID (e.g. si_...)", Required: true}},
	},

	// ── Products ─────────────────────────────────────────────────────
	{
		Name:        mcp.ToolName("stripe_list_products"),
		Description: "List products (goods or services sold on Stripe). Use for product catalog browsing and finding SKUs.",
		Parameters: mergeParams(listParams, []mcp.Parameter{{Name: mcp.ParamName("active"), Description: "Filter by active (true/false)"}, {Name: mcp.ParamName("ids"), Description: "Comma-separated product IDs"}, {Name: mcp.ParamName("shippable"), Description: "Filter shippable goods (true/false)"}, {Name: mcp.ParamName("url"), Description: "Filter by URL"}}),
	},
	{
		Name:        mcp.ToolName("stripe_retrieve_product"),
		Description: "Retrieve (get) a single product by ID.",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("id"), Description: "Product ID (e.g. prod_...)", Required: true}},
	},
	{
		Name:        mcp.ToolName("stripe_create_product"),
		Description: "Create a new product to sell.",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("name"), Description: "Product name (required)", Required: true}, {Name: mcp.ParamName("description"), Description: "Product description"}, {Name: mcp.ParamName("active"), Description: "Boolean — whether the product can be used"}, {Name: mcp.ParamName("default_price_data"), Description: "Object describing the default price (currency, unit_amount, recurring)"}, {Name: mcp.ParamName("images"), Description: "Array of image URLs"}, {Name: mcp.ParamName("metadata"), Description: "Metadata object"}, {Name: mcp.ParamName("shippable"), Description: "Boolean — whether the product is shippable"}, {Name: mcp.ParamName("tax_code"), Description: "Tax code ID"}, {Name: mcp.ParamName("url"), Description: "Product URL"}},
	},
	{
		Name:        mcp.ToolName("stripe_update_product"),
		Description: "Update (edit) a product's name, description, images, or metadata.",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("id"), Description: "Product ID", Required: true}, {Name: mcp.ParamName("name"), Description: "New name"}, {Name: mcp.ParamName("description"), Description: "New description"}, {Name: mcp.ParamName("active"), Description: "Boolean"}, {Name: mcp.ParamName("default_price"), Description: "Default price ID"}, {Name: mcp.ParamName("images"), Description: "Array of image URLs"}, {Name: mcp.ParamName("metadata"), Description: "Metadata object"}, {Name: mcp.ParamName("url"), Description: "Product URL"}},
	},
	{
		Name:        mcp.ToolName("stripe_delete_product"),
		Description: "Delete (remove) a product. Only succeeds if no prices reference it.",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("id"), Description: "Product ID", Required: true}},
	},

	// ── Prices ───────────────────────────────────────────────────────
	{
		Name:        mcp.ToolName("stripe_list_prices"),
		Description: "List prices attached to products. Each price defines how much and how often to charge.",
		Parameters: mergeParams(listParams, []mcp.Parameter{{Name: mcp.ParamName("product"), Description: "Filter by product ID"}, {Name: mcp.ParamName("active"), Description: "Filter by active"}, {Name: mcp.ParamName("currency"), Description: "Filter by currency"}, {Name: mcp.ParamName("type"), Description: "one_time or recurring"}}),
	},
	{
		Name:        mcp.ToolName("stripe_retrieve_price"),
		Description: "Retrieve (get) a single price by ID.",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("id"), Description: "Price ID (e.g. price_...)", Required: true}},
	},
	{
		Name:        mcp.ToolName("stripe_create_price"),
		Description: "Create a new price for a product (one-time or recurring).",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("product"), Description: "Product ID (required if product_data not given)"}, {Name: mcp.ParamName("product_data"), Description: "Inline product object with name to create alongside the price"}, {Name: mcp.ParamName("currency"), Description: "Three-letter ISO currency lowercase (required)", Required: true}, {Name: mcp.ParamName("unit_amount"), Description: "Price in smallest currency unit (required for fixed pricing)"}, {Name: mcp.ParamName("recurring"), Description: "Object with interval (day/week/month/year) and interval_count"}, {Name: mcp.ParamName("nickname"), Description: "Internal display name"}, {Name: mcp.ParamName("active"), Description: "Boolean"}, {Name: mcp.ParamName("billing_scheme"), Description: "per_unit or tiered"}, {Name: mcp.ParamName("tiers"), Description: "Array of tier objects (for tiered pricing)"}, {Name: mcp.ParamName("tiers_mode"), Description: "graduated or volume"}, {Name: mcp.ParamName("metadata"), Description: "Metadata object"}},
	},
	{
		Name:        mcp.ToolName("stripe_update_price"),
		Description: "Update (edit) a price's nickname, active flag, or metadata. Note: most fields are immutable on existing prices.",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("id"), Description: "Price ID", Required: true}, {Name: mcp.ParamName("active"), Description: "Boolean"}, {Name: mcp.ParamName("nickname"), Description:

		// ── Invoices ─────────────────────────────────────────────────────
		"Nickname"}, {Name: mcp.ParamName("metadata"), Description: "Metadata object"}},
	},

	{
		Name:        mcp.ToolName("stripe_list_invoices"),
		Description: "List invoices on the Stripe account. Use for billing audits, finding past_due/open/paid invoices, and revenue reporting.",
		Parameters: mergeParams(listParams, []mcp.Parameter{{Name: mcp.ParamName("customer"), Description: "Filter by customer ID"}, {Name: mcp.ParamName("subscription"), Description: "Filter by subscription ID"}, {Name: mcp.ParamName("status"), Description: "draft, open, paid, uncollectible, void"}, {Name: mcp.ParamName("collection_method"), Description: "charge_automatically or send_invoice"}, {Name: mcp.ParamName("created"), Description: "Filter by creation timestamp"}, {Name: mcp.ParamName("due_date"), Description: "Filter by due_date timestamp"}}),
	},
	{
		Name:        mcp.ToolName("stripe_retrieve_invoice"),
		Description: "Retrieve (get) a single invoice by ID with line items, totals, and status.",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("id"), Description: "Invoice ID (e.g. in_...)", Required: true}},
	},
	{
		Name:        mcp.ToolName("stripe_retrieve_upcoming_invoice"),
		Description: "Retrieve (get) the upcoming (preview) invoice for a customer or subscription before it is finalized. Useful for previewing the next bill.",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("customer"), Description: "Customer ID"}, {Name: mcp.ParamName("subscription"), Description: "Subscription ID"}, {Name: mcp.ParamName("coupon"), Description: "Coupon ID for preview"}},
	},
	{
		Name:        mcp.ToolName("stripe_search_invoices"),
		Description: "Search invoices with Stripe search query (e.g. status:\"open\" AND total>10000).",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("query"), Description: "Stripe search query", Required: true}, {Name: mcp.ParamName("limit"), Description: "Number of results"}, {Name: mcp.ParamName("page"), Description: "Cursor for pagination"}},
	},
	{
		Name:        mcp.ToolName("stripe_create_invoice"),
		Description: "Create a draft invoice for a customer. Add invoice items separately or use auto_advance to finalize automatically.",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("customer"), Description: "Customer ID (required)", Required: true}, {Name: mcp.ParamName("auto_advance"), Description: "Boolean — finalize and attempt collection automatically"}, {Name: mcp.ParamName("collection_method"), Description: "charge_automatically or send_invoice"}, {Name: mcp.ParamName("days_until_due"), Description: "Days until due (for send_invoice)"}, {Name: mcp.ParamName("description"), Description: "Description"}, {Name: mcp.ParamName("subscription"), Description: "Subscription ID to associate"}, {Name: mcp.ParamName("metadata"), Description: "Metadata object"}},
	},
	{
		Name:        mcp.ToolName("stripe_finalize_invoice"),
		Description: "Finalize a draft invoice — locks line items and generates the final amount.",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("id"), Description: "Invoice ID", Required: true}, {Name: mcp.ParamName("auto_advance"), Description: "Boolean — proceed with payment collection"}},
	},
	{
		Name:        mcp.ToolName("stripe_pay_invoice"),
		Description: "Pay an open invoice using a payment method or by charging the customer's default source.",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("id"), Description: "Invoice ID", Required: true}, {Name: mcp.ParamName("payment_method"), Description: "Payment method ID to charge"}, {Name: mcp.ParamName("paid_out_of_band"), Description: "Boolean — mark as paid externally without collecting funds"}, {Name: mcp.ParamName("off_session"), Description: "Boolean"}},
	},
	{
		Name:        mcp.ToolName("stripe_send_invoice"),
		Description: "Email an open invoice to the customer.",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("id"), Description: "Invoice ID", Required: true}},
	},
	{
		Name:        mcp.ToolName("stripe_void_invoice"),
		Description: "Void an open invoice. Cannot be undone.",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("id"), Description: "Invoice ID", Required: true}},
	},
	{
		Name:        mcp.ToolName("stripe_delete_invoice"),
		Description: "Delete (remove) a draft invoice permanently. Only allowed for draft invoices.",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("id"), Description: "Invoice ID", Required: true}},
	},

	// ── Invoice Items ────────────────────────────────────────────────
	{
		Name:        mcp.ToolName("stripe_list_invoice_items"),
		Description: "List pending or attached invoice items (line items added to a customer's next invoice).",
		Parameters: mergeParams(listParams, []mcp.Parameter{{Name: mcp.ParamName("customer"), Description: "Filter by customer ID"}, {Name: mcp.ParamName("invoice"), Description: "Filter by invoice ID"}, {Name: mcp.ParamName("pending"), Description: "Only return pending items (true/false)"}}),
	},
	{
		Name:        mcp.ToolName("stripe_create_invoice_item"),
		Description: "Create an invoice item that will be added to a customer's next invoice or attached to a specific invoice.",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("customer"), Description: "Customer ID (required)", Required: true}, {Name: mcp.ParamName("amount"), Description: "Amount in smallest currency unit"}, {Name: mcp.ParamName("currency"), Description: "Currency code"}, {Name: mcp.ParamName("description"), Description: "Line item description"}, {Name: mcp.ParamName("price"), Description: "Price ID (alternative to amount+currency)"}, {Name: mcp.ParamName("quantity"), Description: "Quantity (defaults to 1)"}, {Name: mcp.ParamName("invoice"), Description:

		// ── Payment Methods ──────────────────────────────────────────────
		"Optional invoice ID to attach to"}, {Name: mcp.ParamName("subscription"), Description: "Subscription ID to attach to"}, {Name: mcp.ParamName("metadata"), Description: "Metadata object"}},
	},

	{
		Name:        mcp.ToolName("stripe_list_payment_methods"),
		Description: "List payment methods attached to a customer (cards, bank accounts, wallets).",
		Parameters: mergeParams(listParams, []mcp.Parameter{
			{Name: mcp.ParamName("customer"), Description: "Customer ID (required)", Required: true},
			{Name: mcp.ParamName("type"), Description: "card, us_bank_account, sepa_debit, etc."},
		}),
	},
	{
		Name:        mcp.ToolName("stripe_retrieve_payment_method"),
		Description: "Retrieve (get) a single payment method by ID.",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("id"), Description: "Payment method ID (e.g. pm_...)", Required: true}},
	},
	{
		Name:        mcp.ToolName("stripe_attach_payment_method"),
		Description: "Attach a payment method to a customer for later reuse.",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("id"), Description: "Payment method ID", Required: true}, {Name: mcp.ParamName("customer"), Description: "Customer ID to attach to", Required: true}},
	},
	{
		Name:        mcp.ToolName("stripe_detach_payment_method"),
		Description: "Detach (remove) a payment method from its customer.",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("id"), Description: "Payment method ID", Required: true}},
	},

	// ── Setup Intents ────────────────────────────────────────────────
	{
		Name:        mcp.ToolName("stripe_list_setup_intents"),
		Description: "List SetupIntents (objects representing intent to save a payment method for future use).",
		Parameters: mergeParams(listParams, []mcp.Parameter{{Name: mcp.ParamName("customer"), Description: "Filter by customer ID"}, {Name: mcp.ParamName("payment_method"), Description: "Filter by payment method ID"}, {Name: mcp.ParamName("created"), Description: "Filter by creation timestamp"}}),
	},
	{
		Name:        mcp.ToolName("stripe_retrieve_setup_intent"),
		Description: "Retrieve (get) a single SetupIntent by ID.",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("id"), Description: "SetupIntent ID (e.g. seti_...)", Required: true}},
	},
	{
		Name:        mcp.ToolName("stripe_create_setup_intent"),
		Description: "Create a SetupIntent to collect a customer's payment method for future use.",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("customer"), Description: "Customer ID"}, {Name: mcp.ParamName("payment_method"), Description: "Payment method ID"}, {Name: mcp.ParamName("payment_method_types"), Description: "Array of allowed payment method types"}, {Name: mcp.ParamName("usage"), Description: "on_session or off_session"}, {Name: mcp.ParamName("confirm"), Description: "Boolean — confirm immediately"}, {Name:

		// ── Coupons / Promotion Codes ────────────────────────────────────
		mcp.ParamName("description"), Description: "Description"}, {Name: mcp.ParamName("metadata"), Description: "Metadata object"}},
	},

	{
		Name:        mcp.ToolName("stripe_list_coupons"),
		Description: "List discount coupons defined on the Stripe account.",
		Parameters:  mergeParams(listParams, nil),
	},
	{
		Name:        mcp.ToolName("stripe_retrieve_coupon"),
		Description: "Retrieve (get) a single coupon by ID.",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("id"), Description: "Coupon ID", Required: true}},
	},
	{
		Name:        mcp.ToolName("stripe_create_coupon"),
		Description: "Create a discount coupon that can be applied to customers, invoices, or subscriptions.",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("id"), Description: "Optional coupon ID (auto-generated if omitted)"}, {Name: mcp.ParamName("name"), Description: "Display name shown on receipts/invoices"}, {Name: mcp.ParamName("percent_off"), Description: "Percentage discount (0-100). Use this or amount_off."}, {Name: mcp.ParamName("amount_off"), Description: "Fixed amount discount in smallest currency unit"}, {Name: mcp.ParamName("currency"), Description: "Required when amount_off is set"}, {Name: mcp.ParamName("duration"), Description: "once, repeating, or forever", Required: true}, {Name: mcp.ParamName("duration_in_months"), Description: "Number of months (for duration=repeating)"}, {Name: mcp.ParamName("max_redemptions"), Description: "Maximum total redemptions"}, {Name: mcp.ParamName("redeem_by"), Description: "Unix timestamp coupon expires for new redemptions"}, {Name: mcp.ParamName("metadata"), Description: "Metadata object"}},
	},
	{
		Name:        mcp.ToolName("stripe_delete_coupon"),
		Description: "Delete (remove) a coupon.",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("id"), Description: "Coupon ID", Required: true}},
	},
	{
		Name:        mcp.ToolName("stripe_list_promotion_codes"),
		Description: "List customer-facing promotion codes (the code strings tied to a coupon).",
		Parameters: mergeParams(listParams, []mcp.Parameter{{Name: mcp.ParamName("coupon"), Description: "Filter by coupon ID"}, {Name: mcp.ParamName("customer"), Description: "Filter by customer ID"}, {Name: mcp.ParamName("active"), Description: "Filter by active"}, {Name: mcp.ParamName("code"), Description: "Filter by code string"}}),
	},
	{
		Name:        mcp.ToolName("stripe_create_promotion_code"),
		Description: "Create a customer-facing promotion code tied to a coupon.",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("coupon"), Description: "Coupon ID (required)", Required: true}, {Name: mcp.ParamName("code"), Description: "Customer-facing code (auto-generated if omitted)"}, {Name: mcp.ParamName("customer"), Description: "Restrict to a specific customer ID"}, {Name: mcp.ParamName("max_redemptions"), Description: "Maximum total redemptions"}, {Name: mcp.ParamName("expires_at"), Description: "Unix timestamp"},

		// ── Events ───────────────────────────────────────────────────────
		{Name: mcp.ParamName("active"), Description: "Boolean"}, {Name: mcp.ParamName("metadata"), Description: "Metadata object"}},
	},

	{
		Name:        mcp.ToolName("stripe_list_events"),
		Description: "List Stripe events (webhook event log). Use for auditing webhook history, replaying missed events, or finding when a specific object changed.",
		Parameters: mergeParams(listParams, []mcp.Parameter{{Name: mcp.ParamName("type"), Description: "Filter by event type (e.g. charge.succeeded, invoice.paid)"}, {Name: mcp.ParamName("types"), Description: "Array of event types"}, {Name: mcp.ParamName("created"), Description: "Filter by creation timestamp"}, {Name: mcp.ParamName("delivery_success"), Description: "Boolean — filter by webhook delivery outcome"}}),
	},
	{
		Name:        mcp.ToolName("stripe_retrieve_event"),
		Description: "Retrieve (get) a single event by ID.",
		Parameters:  []mcp.Parameter{{Name: mcp.ParamName("id"), Description: "Event ID (e.g. evt_...)", Required: true}},
	},
}

// mergeParams appends extra parameters after base list pagination parameters.
// Base entries come first; extra entries follow in declaration order.
func mergeParams(base []mcp.Parameter, extra []mcp.Parameter) []mcp.Parameter {
	if len(extra) == 0 {
		return base
	}
	out := make([]mcp.Parameter, 0, len(base)+len(extra))
	out = append(out, extra...)
	out = append(out, base...)
	return out
}
