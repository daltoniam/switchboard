package stripe

import mcp "github.com/daltoniam/switchboard"

// Standard list-pagination params reused across many list tools.
var listParams = map[string]string{
	"limit":          "Number of objects to return (1-100, default 10)",
	"starting_after": "Cursor for pagination — an object ID for the next page",
	"ending_before":  "Cursor for pagination — an object ID for the previous page",
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
		Parameters: mergeParams(listParams, map[string]string{
			"type":     "Filter by type (charge, refund, payout, transfer, adjustment, fee, etc.)",
			"currency": "Three-letter ISO currency code lowercase (e.g. usd)",
			"payout":   "Filter to transactions paid out in this payout ID",
			"source":   "Filter by source ID (charge, refund, etc.)",
		}),
	},
	{
		Name:        mcp.ToolName("stripe_retrieve_balance_transaction"),
		Description: "Retrieve (get) a single balance transaction by ID.",
		Parameters:  map[string]string{"id": "Balance transaction ID (e.g. txn_...)"},
		Required:    []string{"id"},
	},

	// ── Customers ────────────────────────────────────────────────────
	{
		Name:        mcp.ToolName("stripe_list_customers"),
		Description: "List Stripe customers. Use for browsing payers, finding accounts by email, exporting customer rosters, or paginating the customer directory.",
		Parameters: mergeParams(listParams, map[string]string{
			"email":   "Filter by exact email match",
			"created": "Filter by created timestamp (Unix epoch seconds) — pass a number or use nested keys gt/gte/lt/lte for ranges",
		}),
	},
	{
		Name:        mcp.ToolName("stripe_retrieve_customer"),
		Description: "Retrieve (get) a single customer by ID including their default payment method, billing address, and metadata.",
		Parameters:  map[string]string{"id": "Customer ID (e.g. cus_...)"},
		Required:    []string{"id"},
	},
	{
		Name:        mcp.ToolName("stripe_search_customers"),
		Description: "Search Stripe customers using Stripe search query language (e.g. email:\"alice@example.com\" or metadata['plan']:\"pro\"). Returns matching customer records.",
		Parameters: map[string]string{
			"query": "Stripe search query syntax",
			"limit": "Number of results (1-100)",
			"page":  "Cursor for pagination (from prior response.next_page)",
		},
		Required: []string{"query"},
	},
	{
		Name:        mcp.ToolName("stripe_create_customer"),
		Description: "Create a new Stripe customer record for a payer.",
		Parameters: map[string]string{
			"email":       "Customer email address",
			"name":        "Full customer name",
			"phone":       "Phone number",
			"description": "Arbitrary description for internal use",
			"metadata":    "Object of key-value strings to attach (max 50 keys)",
			"address":     "Object with line1, line2, city, state, postal_code, country",
			"shipping":    "Object with name, phone, address",
		},
	},
	{
		Name:        mcp.ToolName("stripe_update_customer"),
		Description: "Update (edit) a customer's profile, contact info, default payment method, or metadata.",
		Parameters: map[string]string{
			"id":               "Customer ID",
			"email":            "New email",
			"name":             "New name",
			"phone":            "New phone",
			"description":      "New description",
			"metadata":         "Object to merge into existing metadata (set key to empty string to clear it)",
			"default_source":   "Default payment source ID",
			"invoice_settings": "Object with default_payment_method, custom_fields, footer",
			"address":          "Billing address object",
			"shipping":         "Shipping address object",
		},
		Required: []string{"id"},
	},
	{
		Name:        mcp.ToolName("stripe_delete_customer"),
		Description: "Delete (remove) a customer permanently. Cancels active subscriptions and disassociates payment methods.",
		Parameters:  map[string]string{"id": "Customer ID"},
		Required:    []string{"id"},
	},

	// ── Charges ──────────────────────────────────────────────────────
	{
		Name:        mcp.ToolName("stripe_list_charges"),
		Description: "List charges (card and bank transactions) processed on the Stripe account. Use for transaction history, sales reports, revenue analytics, and finding individual successful or failed payments.",
		Parameters: mergeParams(listParams, map[string]string{
			"customer":       "Filter by customer ID",
			"payment_intent": "Filter by payment intent ID",
			"transfer_group": "Filter by transfer group",
			"created":        "Filter by creation timestamp",
		}),
	},
	{
		Name:        mcp.ToolName("stripe_retrieve_charge"),
		Description: "Retrieve (get) a single charge by ID with full details (amount, currency, status, customer, payment_method, refunds, dispute, outcome).",
		Parameters:  map[string]string{"id": "Charge ID (e.g. ch_...)"},
		Required:    []string{"id"},
	},
	{
		Name:        mcp.ToolName("stripe_search_charges"),
		Description: "Search charges using Stripe search query language (e.g. amount>1000 AND status:\"succeeded\").",
		Parameters: map[string]string{
			"query": "Stripe search query",
			"limit": "Number of results (1-100)",
			"page":  "Cursor for pagination",
		},
		Required: []string{"query"},
	},
	{
		Name:        mcp.ToolName("stripe_capture_charge"),
		Description: "Capture a previously authorized but uncaptured charge (manual capture flow).",
		Parameters: map[string]string{
			"id":     "Charge ID",
			"amount": "Optional amount to capture in smallest currency unit (cents). Defaults to full authorized amount.",
		},
		Required: []string{"id"},
	},

	// ── Payment Intents ──────────────────────────────────────────────
	{
		Name:        mcp.ToolName("stripe_list_payment_intents"),
		Description: "List PaymentIntents — the modern payment flow object representing intent to collect from a customer. Use for monitoring in-progress, succeeded, requires_action, or failed payments.",
		Parameters: mergeParams(listParams, map[string]string{
			"customer": "Filter by customer ID",
			"created":  "Filter by creation timestamp",
		}),
	},
	{
		Name:        mcp.ToolName("stripe_retrieve_payment_intent"),
		Description: "Retrieve (get) a single PaymentIntent by ID with status, next_action, latest_charge, and client_secret.",
		Parameters:  map[string]string{"id": "PaymentIntent ID (e.g. pi_...)"},
		Required:    []string{"id"},
	},
	{
		Name:        mcp.ToolName("stripe_create_payment_intent"),
		Description: "Create a new PaymentIntent to charge a customer. Amounts are in the smallest currency unit (e.g. cents for USD).",
		Parameters: map[string]string{
			"amount":               "Amount in smallest currency unit (e.g. 1099 = $10.99)",
			"currency":             "Three-letter ISO currency lowercase (e.g. usd)",
			"customer":             "Customer ID to associate with this payment",
			"payment_method":       "Payment method ID to charge",
			"payment_method_types": "Array of payment method types (e.g. [\"card\"])",
			"description":          "Arbitrary description shown to the customer",
			"receipt_email":        "Email address to send receipt to",
			"statement_descriptor": "Up to 22 chars shown on the customer's statement",
			"capture_method":       "automatic or manual",
			"confirm":              "If true, confirm the PaymentIntent in the same request (boolean)",
			"off_session":          "Boolean indicating customer is not present",
			"metadata":             "Object of key-value strings",
		},
		Required: []string{"amount", "currency"},
	},
	{
		Name:        mcp.ToolName("stripe_update_payment_intent"),
		Description: "Update (edit) a PaymentIntent before it is confirmed.",
		Parameters: map[string]string{
			"id":             "PaymentIntent ID",
			"amount":         "New amount in smallest currency unit",
			"currency":       "Currency code",
			"customer":       "Customer ID",
			"description":    "New description",
			"metadata":       "Metadata object",
			"payment_method": "Payment method ID",
			"receipt_email":  "Receipt email address",
		},
		Required: []string{"id"},
	},
	{
		Name:        mcp.ToolName("stripe_confirm_payment_intent"),
		Description: "Confirm a PaymentIntent to attempt to collect payment.",
		Parameters: map[string]string{
			"id":             "PaymentIntent ID",
			"payment_method": "Payment method ID to attach for confirmation",
			"return_url":     "URL to redirect after 3DS/redirect-based authentication",
			"off_session":    "Boolean — confirming on behalf of an absent customer",
		},
		Required: []string{"id"},
	},
	{
		Name:        mcp.ToolName("stripe_cancel_payment_intent"),
		Description: "Cancel a PaymentIntent that is in a cancelable state (requires_payment_method, requires_capture, requires_confirmation, requires_action, processing).",
		Parameters: map[string]string{
			"id":                  "PaymentIntent ID",
			"cancellation_reason": "duplicate, fraudulent, requested_by_customer, abandoned",
		},
		Required: []string{"id"},
	},
	{
		Name:        mcp.ToolName("stripe_search_payment_intents"),
		Description: "Search PaymentIntents with Stripe search query (e.g. status:\"requires_action\" AND amount>5000).",
		Parameters: map[string]string{
			"query": "Stripe search query",
			"limit": "Number of results",
			"page":  "Cursor for pagination",
		},
		Required: []string{"query"},
	},

	// ── Refunds ──────────────────────────────────────────────────────
	{
		Name:        mcp.ToolName("stripe_list_refunds"),
		Description: "List refunds processed on the Stripe account. Use for reviewing returned money, refund audits, and tracking refund status.",
		Parameters: mergeParams(listParams, map[string]string{
			"charge":         "Filter by charge ID",
			"payment_intent": "Filter by payment intent ID",
			"created":        "Filter by creation timestamp",
		}),
	},
	{
		Name:        mcp.ToolName("stripe_retrieve_refund"),
		Description: "Retrieve (get) a single refund by ID.",
		Parameters:  map[string]string{"id": "Refund ID (e.g. re_...)"},
		Required:    []string{"id"},
	},
	{
		Name:        mcp.ToolName("stripe_create_refund"),
		Description: "Create a refund (full or partial) for a charge or PaymentIntent. Returns money to the customer.",
		Parameters: map[string]string{
			"charge":                 "Charge ID to refund (one of charge or payment_intent required)",
			"payment_intent":         "PaymentIntent ID to refund",
			"amount":                 "Amount to refund in smallest currency unit (defaults to full)",
			"reason":                 "duplicate, fraudulent, or requested_by_customer",
			"refund_application_fee": "Boolean — whether to refund the application fee",
			"reverse_transfer":       "Boolean — reverse the transfer to a connected account",
			"metadata":               "Metadata object",
		},
	},
	{
		Name:        mcp.ToolName("stripe_update_refund"),
		Description: "Update (edit) a refund's metadata.",
		Parameters: map[string]string{
			"id":       "Refund ID",
			"metadata": "Metadata object to merge",
		},
		Required: []string{"id"},
	},

	// ── Disputes ─────────────────────────────────────────────────────
	{
		Name:        mcp.ToolName("stripe_list_disputes"),
		Description: "List chargeback disputes filed against charges on the Stripe account. Use for fraud monitoring, chargeback response workflows, and dispute analytics.",
		Parameters: mergeParams(listParams, map[string]string{
			"charge":         "Filter by charge ID",
			"payment_intent": "Filter by payment intent ID",
			"created":        "Filter by creation timestamp",
		}),
	},
	{
		Name:        mcp.ToolName("stripe_retrieve_dispute"),
		Description: "Retrieve (get) a single dispute by ID including evidence and status.",
		Parameters:  map[string]string{"id": "Dispute ID (e.g. dp_...)"},
		Required:    []string{"id"},
	},
	{
		Name:        mcp.ToolName("stripe_update_dispute"),
		Description: "Update (edit) a dispute to submit evidence for chargeback response.",
		Parameters: map[string]string{
			"id":       "Dispute ID",
			"evidence": "Evidence object (e.g. customer_communication, receipt, service_documentation, shipping_documentation, etc.)",
			"submit":   "Boolean — submit evidence immediately",
			"metadata": "Metadata object",
		},
		Required: []string{"id"},
	},

	// ── Payouts ──────────────────────────────────────────────────────
	{
		Name:        mcp.ToolName("stripe_list_payouts"),
		Description: "List payouts (transfers from the Stripe balance to a bank account). Use for cash-out tracking, settlement reconciliation, and finance reports.",
		Parameters: mergeParams(listParams, map[string]string{
			"status":       "Filter by status (paid, pending, in_transit, canceled, failed)",
			"destination":  "Filter by bank account or card destination ID",
			"arrival_date": "Filter by arrival_date timestamp",
			"created":      "Filter by creation timestamp",
		}),
	},
	{
		Name:        mcp.ToolName("stripe_retrieve_payout"),
		Description: "Retrieve (get) a single payout by ID.",
		Parameters:  map[string]string{"id": "Payout ID (e.g. po_...)"},
		Required:    []string{"id"},
	},
	{
		Name:        mcp.ToolName("stripe_create_payout"),
		Description: "Create a manual payout from the Stripe balance to the default bank account.",
		Parameters: map[string]string{
			"amount":      "Amount in smallest currency unit",
			"currency":    "Three-letter ISO currency lowercase",
			"description": "Description",
			"method":      "standard or instant",
			"destination": "Bank account or debit card ID (optional override)",
			"metadata":    "Metadata object",
		},
		Required: []string{"amount", "currency"},
	},

	// ── Subscriptions ────────────────────────────────────────────────
	{
		Name:        mcp.ToolName("stripe_list_subscriptions"),
		Description: "List recurring billing subscriptions. Use for MRR/ARR reports, churn analysis, active customer counts, and finding subscriptions in trial, past_due, or canceled state.",
		Parameters: mergeParams(listParams, map[string]string{
			"customer":          "Filter by customer ID",
			"price":             "Filter by price ID",
			"status":            "Filter by status (active, past_due, unpaid, canceled, incomplete, incomplete_expired, trialing, all)",
			"collection_method": "charge_automatically or send_invoice",
			"created":           "Filter by creation timestamp",
		}),
	},
	{
		Name:        mcp.ToolName("stripe_retrieve_subscription"),
		Description: "Retrieve (get) a single subscription by ID including items, current period, and billing cycle.",
		Parameters:  map[string]string{"id": "Subscription ID (e.g. sub_...)"},
		Required:    []string{"id"},
	},
	{
		Name:        mcp.ToolName("stripe_search_subscriptions"),
		Description: "Search subscriptions using Stripe search query (e.g. status:\"trialing\" AND created>1700000000).",
		Parameters: map[string]string{
			"query": "Stripe search query",
			"limit": "Number of results",
			"page":  "Cursor for pagination",
		},
		Required: []string{"query"},
	},
	{
		Name:        mcp.ToolName("stripe_create_subscription"),
		Description: "Create a new subscription that recurringly bills a customer for one or more prices.",
		Parameters: map[string]string{
			"customer":               "Customer ID to subscribe (required)",
			"items":                  "Array of subscription items, each with a 'price' ID and optional 'quantity'",
			"default_payment_method": "Payment method ID to use for invoices",
			"trial_period_days":      "Number of days for free trial",
			"trial_end":              "Unix timestamp when trial ends (or 'now')",
			"collection_method":      "charge_automatically or send_invoice",
			"days_until_due":         "Days until invoice is due (only for send_invoice)",
			"coupon":                 "Coupon ID to apply",
			"metadata":               "Metadata object",
		},
		Required: []string{"customer", "items"},
	},
	{
		Name:        mcp.ToolName("stripe_update_subscription"),
		Description: "Update (edit) a subscription's items, prices, quantities, billing settings, or metadata.",
		Parameters: map[string]string{
			"id":                     "Subscription ID",
			"items":                  "Updated items array — each item may include 'id' (existing item), 'price', 'quantity', or 'deleted':true",
			"cancel_at_period_end":   "Boolean — cancel at end of current period",
			"default_payment_method": "Payment method ID",
			"proration_behavior":     "create_prorations, none, or always_invoice",
			"trial_end":              "Unix timestamp or 'now'",
			"metadata":               "Metadata object",
			"pause_collection":       "Object with behavior (keep_as_draft, mark_uncollectible, void) and resumes_at",
		},
		Required: []string{"id"},
	},
	{
		Name:        mcp.ToolName("stripe_cancel_subscription"),
		Description: "Cancel a subscription immediately (or at period end via stripe_update_subscription with cancel_at_period_end).",
		Parameters: map[string]string{
			"id":                   "Subscription ID",
			"invoice_now":          "Boolean — generate a final invoice for unbilled usage",
			"prorate":              "Boolean — credit prorated unused time",
			"cancellation_details": "Object with comment and feedback",
		},
		Required: []string{"id"},
	},

	// ── Subscription Items ───────────────────────────────────────────
	{
		Name:        mcp.ToolName("stripe_list_subscription_items"),
		Description: "List items (line items) belonging to a subscription.",
		Parameters: mergeParams(listParams, map[string]string{
			"subscription": "Subscription ID",
		}),
		Required: []string{"subscription"},
	},
	{
		Name:        mcp.ToolName("stripe_retrieve_subscription_item"),
		Description: "Retrieve (get) a single subscription item by ID.",
		Parameters:  map[string]string{"id": "Subscription item ID (e.g. si_...)"},
		Required:    []string{"id"},
	},

	// ── Products ─────────────────────────────────────────────────────
	{
		Name:        mcp.ToolName("stripe_list_products"),
		Description: "List products (goods or services sold on Stripe). Use for product catalog browsing and finding SKUs.",
		Parameters: mergeParams(listParams, map[string]string{
			"active":    "Filter by active (true/false)",
			"ids":       "Comma-separated product IDs",
			"shippable": "Filter shippable goods (true/false)",
			"url":       "Filter by URL",
		}),
	},
	{
		Name:        mcp.ToolName("stripe_retrieve_product"),
		Description: "Retrieve (get) a single product by ID.",
		Parameters:  map[string]string{"id": "Product ID (e.g. prod_...)"},
		Required:    []string{"id"},
	},
	{
		Name:        mcp.ToolName("stripe_create_product"),
		Description: "Create a new product to sell.",
		Parameters: map[string]string{
			"name":               "Product name (required)",
			"description":        "Product description",
			"active":             "Boolean — whether the product can be used",
			"default_price_data": "Object describing the default price (currency, unit_amount, recurring)",
			"images":             "Array of image URLs",
			"metadata":           "Metadata object",
			"shippable":          "Boolean — whether the product is shippable",
			"tax_code":           "Tax code ID",
			"url":                "Product URL",
		},
		Required: []string{"name"},
	},
	{
		Name:        mcp.ToolName("stripe_update_product"),
		Description: "Update (edit) a product's name, description, images, or metadata.",
		Parameters: map[string]string{
			"id":            "Product ID",
			"name":          "New name",
			"description":   "New description",
			"active":        "Boolean",
			"default_price": "Default price ID",
			"images":        "Array of image URLs",
			"metadata":      "Metadata object",
			"url":           "Product URL",
		},
		Required: []string{"id"},
	},
	{
		Name:        mcp.ToolName("stripe_delete_product"),
		Description: "Delete (remove) a product. Only succeeds if no prices reference it.",
		Parameters:  map[string]string{"id": "Product ID"},
		Required:    []string{"id"},
	},

	// ── Prices ───────────────────────────────────────────────────────
	{
		Name:        mcp.ToolName("stripe_list_prices"),
		Description: "List prices attached to products. Each price defines how much and how often to charge.",
		Parameters: mergeParams(listParams, map[string]string{
			"product":  "Filter by product ID",
			"active":   "Filter by active",
			"currency": "Filter by currency",
			"type":     "one_time or recurring",
		}),
	},
	{
		Name:        mcp.ToolName("stripe_retrieve_price"),
		Description: "Retrieve (get) a single price by ID.",
		Parameters:  map[string]string{"id": "Price ID (e.g. price_...)"},
		Required:    []string{"id"},
	},
	{
		Name:        mcp.ToolName("stripe_create_price"),
		Description: "Create a new price for a product (one-time or recurring).",
		Parameters: map[string]string{
			"product":        "Product ID (required if product_data not given)",
			"product_data":   "Inline product object with name to create alongside the price",
			"currency":       "Three-letter ISO currency lowercase (required)",
			"unit_amount":    "Price in smallest currency unit (required for fixed pricing)",
			"recurring":      "Object with interval (day/week/month/year) and interval_count",
			"nickname":       "Internal display name",
			"active":         "Boolean",
			"billing_scheme": "per_unit or tiered",
			"tiers":          "Array of tier objects (for tiered pricing)",
			"tiers_mode":     "graduated or volume",
			"metadata":       "Metadata object",
		},
		Required: []string{"currency"},
	},
	{
		Name:        mcp.ToolName("stripe_update_price"),
		Description: "Update (edit) a price's nickname, active flag, or metadata. Note: most fields are immutable on existing prices.",
		Parameters: map[string]string{
			"id":       "Price ID",
			"active":   "Boolean",
			"nickname": "Nickname",
			"metadata": "Metadata object",
		},
		Required: []string{"id"},
	},

	// ── Invoices ─────────────────────────────────────────────────────
	{
		Name:        mcp.ToolName("stripe_list_invoices"),
		Description: "List invoices on the Stripe account. Use for billing audits, finding past_due/open/paid invoices, and revenue reporting.",
		Parameters: mergeParams(listParams, map[string]string{
			"customer":          "Filter by customer ID",
			"subscription":      "Filter by subscription ID",
			"status":            "draft, open, paid, uncollectible, void",
			"collection_method": "charge_automatically or send_invoice",
			"created":           "Filter by creation timestamp",
			"due_date":          "Filter by due_date timestamp",
		}),
	},
	{
		Name:        mcp.ToolName("stripe_retrieve_invoice"),
		Description: "Retrieve (get) a single invoice by ID with line items, totals, and status.",
		Parameters:  map[string]string{"id": "Invoice ID (e.g. in_...)"},
		Required:    []string{"id"},
	},
	{
		Name:        mcp.ToolName("stripe_retrieve_upcoming_invoice"),
		Description: "Retrieve (get) the upcoming (preview) invoice for a customer or subscription before it is finalized. Useful for previewing the next bill.",
		Parameters: map[string]string{
			"customer":     "Customer ID",
			"subscription": "Subscription ID",
			"coupon":       "Coupon ID for preview",
		},
	},
	{
		Name:        mcp.ToolName("stripe_search_invoices"),
		Description: "Search invoices with Stripe search query (e.g. status:\"open\" AND total>10000).",
		Parameters: map[string]string{
			"query": "Stripe search query",
			"limit": "Number of results",
			"page":  "Cursor for pagination",
		},
		Required: []string{"query"},
	},
	{
		Name:        mcp.ToolName("stripe_create_invoice"),
		Description: "Create a draft invoice for a customer. Add invoice items separately or use auto_advance to finalize automatically.",
		Parameters: map[string]string{
			"customer":          "Customer ID (required)",
			"auto_advance":      "Boolean — finalize and attempt collection automatically",
			"collection_method": "charge_automatically or send_invoice",
			"days_until_due":    "Days until due (for send_invoice)",
			"description":       "Description",
			"subscription":      "Subscription ID to associate",
			"metadata":          "Metadata object",
		},
		Required: []string{"customer"},
	},
	{
		Name:        mcp.ToolName("stripe_finalize_invoice"),
		Description: "Finalize a draft invoice — locks line items and generates the final amount.",
		Parameters: map[string]string{
			"id":           "Invoice ID",
			"auto_advance": "Boolean — proceed with payment collection",
		},
		Required: []string{"id"},
	},
	{
		Name:        mcp.ToolName("stripe_pay_invoice"),
		Description: "Pay an open invoice using a payment method or by charging the customer's default source.",
		Parameters: map[string]string{
			"id":               "Invoice ID",
			"payment_method":   "Payment method ID to charge",
			"paid_out_of_band": "Boolean — mark as paid externally without collecting funds",
			"off_session":      "Boolean",
		},
		Required: []string{"id"},
	},
	{
		Name:        mcp.ToolName("stripe_send_invoice"),
		Description: "Email an open invoice to the customer.",
		Parameters:  map[string]string{"id": "Invoice ID"},
		Required:    []string{"id"},
	},
	{
		Name:        mcp.ToolName("stripe_void_invoice"),
		Description: "Void an open invoice. Cannot be undone.",
		Parameters:  map[string]string{"id": "Invoice ID"},
		Required:    []string{"id"},
	},
	{
		Name:        mcp.ToolName("stripe_delete_invoice"),
		Description: "Delete (remove) a draft invoice permanently. Only allowed for draft invoices.",
		Parameters:  map[string]string{"id": "Invoice ID"},
		Required:    []string{"id"},
	},

	// ── Invoice Items ────────────────────────────────────────────────
	{
		Name:        mcp.ToolName("stripe_list_invoice_items"),
		Description: "List pending or attached invoice items (line items added to a customer's next invoice).",
		Parameters: mergeParams(listParams, map[string]string{
			"customer": "Filter by customer ID",
			"invoice":  "Filter by invoice ID",
			"pending":  "Only return pending items (true/false)",
		}),
	},
	{
		Name:        mcp.ToolName("stripe_create_invoice_item"),
		Description: "Create an invoice item that will be added to a customer's next invoice or attached to a specific invoice.",
		Parameters: map[string]string{
			"customer":     "Customer ID (required)",
			"amount":       "Amount in smallest currency unit",
			"currency":     "Currency code",
			"description":  "Line item description",
			"price":        "Price ID (alternative to amount+currency)",
			"quantity":     "Quantity (defaults to 1)",
			"invoice":      "Optional invoice ID to attach to",
			"subscription": "Subscription ID to attach to",
			"metadata":     "Metadata object",
		},
		Required: []string{"customer"},
	},

	// ── Payment Methods ──────────────────────────────────────────────
	{
		Name:        mcp.ToolName("stripe_list_payment_methods"),
		Description: "List payment methods attached to a customer (cards, bank accounts, wallets).",
		Parameters: mergeParams(listParams, map[string]string{
			"customer": "Customer ID (required)",
			"type":     "card, us_bank_account, sepa_debit, etc.",
		}),
		Required: []string{"customer"},
	},
	{
		Name:        mcp.ToolName("stripe_retrieve_payment_method"),
		Description: "Retrieve (get) a single payment method by ID.",
		Parameters:  map[string]string{"id": "Payment method ID (e.g. pm_...)"},
		Required:    []string{"id"},
	},
	{
		Name:        mcp.ToolName("stripe_attach_payment_method"),
		Description: "Attach a payment method to a customer for later reuse.",
		Parameters: map[string]string{
			"id":       "Payment method ID",
			"customer": "Customer ID to attach to",
		},
		Required: []string{"id", "customer"},
	},
	{
		Name:        mcp.ToolName("stripe_detach_payment_method"),
		Description: "Detach (remove) a payment method from its customer.",
		Parameters:  map[string]string{"id": "Payment method ID"},
		Required:    []string{"id"},
	},

	// ── Setup Intents ────────────────────────────────────────────────
	{
		Name:        mcp.ToolName("stripe_list_setup_intents"),
		Description: "List SetupIntents (objects representing intent to save a payment method for future use).",
		Parameters: mergeParams(listParams, map[string]string{
			"customer":       "Filter by customer ID",
			"payment_method": "Filter by payment method ID",
			"created":        "Filter by creation timestamp",
		}),
	},
	{
		Name:        mcp.ToolName("stripe_retrieve_setup_intent"),
		Description: "Retrieve (get) a single SetupIntent by ID.",
		Parameters:  map[string]string{"id": "SetupIntent ID (e.g. seti_...)"},
		Required:    []string{"id"},
	},
	{
		Name:        mcp.ToolName("stripe_create_setup_intent"),
		Description: "Create a SetupIntent to collect a customer's payment method for future use.",
		Parameters: map[string]string{
			"customer":             "Customer ID",
			"payment_method":       "Payment method ID",
			"payment_method_types": "Array of allowed payment method types",
			"usage":                "on_session or off_session",
			"confirm":              "Boolean — confirm immediately",
			"description":          "Description",
			"metadata":             "Metadata object",
		},
	},

	// ── Coupons / Promotion Codes ────────────────────────────────────
	{
		Name:        mcp.ToolName("stripe_list_coupons"),
		Description: "List discount coupons defined on the Stripe account.",
		Parameters:  mergeParams(listParams, nil),
	},
	{
		Name:        mcp.ToolName("stripe_retrieve_coupon"),
		Description: "Retrieve (get) a single coupon by ID.",
		Parameters:  map[string]string{"id": "Coupon ID"},
		Required:    []string{"id"},
	},
	{
		Name:        mcp.ToolName("stripe_create_coupon"),
		Description: "Create a discount coupon that can be applied to customers, invoices, or subscriptions.",
		Parameters: map[string]string{
			"id":                 "Optional coupon ID (auto-generated if omitted)",
			"name":               "Display name shown on receipts/invoices",
			"percent_off":        "Percentage discount (0-100). Use this or amount_off.",
			"amount_off":         "Fixed amount discount in smallest currency unit",
			"currency":           "Required when amount_off is set",
			"duration":           "once, repeating, or forever",
			"duration_in_months": "Number of months (for duration=repeating)",
			"max_redemptions":    "Maximum total redemptions",
			"redeem_by":          "Unix timestamp coupon expires for new redemptions",
			"metadata":           "Metadata object",
		},
		Required: []string{"duration"},
	},
	{
		Name:        mcp.ToolName("stripe_delete_coupon"),
		Description: "Delete (remove) a coupon.",
		Parameters:  map[string]string{"id": "Coupon ID"},
		Required:    []string{"id"},
	},
	{
		Name:        mcp.ToolName("stripe_list_promotion_codes"),
		Description: "List customer-facing promotion codes (the code strings tied to a coupon).",
		Parameters: mergeParams(listParams, map[string]string{
			"coupon":   "Filter by coupon ID",
			"customer": "Filter by customer ID",
			"active":   "Filter by active",
			"code":     "Filter by code string",
		}),
	},
	{
		Name:        mcp.ToolName("stripe_create_promotion_code"),
		Description: "Create a customer-facing promotion code tied to a coupon.",
		Parameters: map[string]string{
			"coupon":          "Coupon ID (required)",
			"code":            "Customer-facing code (auto-generated if omitted)",
			"customer":        "Restrict to a specific customer ID",
			"max_redemptions": "Maximum total redemptions",
			"expires_at":      "Unix timestamp",
			"active":          "Boolean",
			"metadata":        "Metadata object",
		},
		Required: []string{"coupon"},
	},

	// ── Events ───────────────────────────────────────────────────────
	{
		Name:        mcp.ToolName("stripe_list_events"),
		Description: "List Stripe events (webhook event log). Use for auditing webhook history, replaying missed events, or finding when a specific object changed.",
		Parameters: mergeParams(listParams, map[string]string{
			"type":             "Filter by event type (e.g. charge.succeeded, invoice.paid)",
			"types":            "Array of event types",
			"created":          "Filter by creation timestamp",
			"delivery_success": "Boolean — filter by webhook delivery outcome",
		}),
	},
	{
		Name:        mcp.ToolName("stripe_retrieve_event"),
		Description: "Retrieve (get) a single event by ID.",
		Parameters:  map[string]string{"id": "Event ID (e.g. evt_...)"},
		Required:    []string{"id"},
	},
}

// mergeParams merges base + extra into a fresh map (base takes precedence for duplicates).
func mergeParams(base, extra map[string]string) map[string]string {
	out := make(map[string]string, len(base)+len(extra))
	for k, v := range extra {
		out[k] = v
	}
	for k, v := range base {
		out[k] = v
	}
	return out
}
