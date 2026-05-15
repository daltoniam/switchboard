package stripe

import mcp "github.com/daltoniam/switchboard"

// dispatch routes a tool name to its handler.
var dispatch = map[mcp.ToolName]handlerFunc{
	// Balance
	mcp.ToolName("stripe_get_balance"):                  getBalance,
	mcp.ToolName("stripe_list_balance_transactions"):    listBalanceTransactions,
	mcp.ToolName("stripe_retrieve_balance_transaction"): retrieveBalanceTransaction,

	// Customers
	mcp.ToolName("stripe_list_customers"):    listCustomers,
	mcp.ToolName("stripe_retrieve_customer"): retrieveCustomer,
	mcp.ToolName("stripe_search_customers"):  searchCustomers,
	mcp.ToolName("stripe_create_customer"):   createCustomer,
	mcp.ToolName("stripe_update_customer"):   updateCustomer,
	mcp.ToolName("stripe_delete_customer"):   deleteCustomer,

	// Charges
	mcp.ToolName("stripe_list_charges"):    listCharges,
	mcp.ToolName("stripe_retrieve_charge"): retrieveCharge,
	mcp.ToolName("stripe_search_charges"):  searchCharges,
	mcp.ToolName("stripe_capture_charge"):  captureCharge,

	// Payment Intents
	mcp.ToolName("stripe_list_payment_intents"):    listPaymentIntents,
	mcp.ToolName("stripe_retrieve_payment_intent"): retrievePaymentIntent,
	mcp.ToolName("stripe_create_payment_intent"):   createPaymentIntent,
	mcp.ToolName("stripe_update_payment_intent"):   updatePaymentIntent,
	mcp.ToolName("stripe_confirm_payment_intent"):  confirmPaymentIntent,
	mcp.ToolName("stripe_cancel_payment_intent"):   cancelPaymentIntent,
	mcp.ToolName("stripe_search_payment_intents"):  searchPaymentIntents,

	// Refunds
	mcp.ToolName("stripe_list_refunds"):    listRefunds,
	mcp.ToolName("stripe_retrieve_refund"): retrieveRefund,
	mcp.ToolName("stripe_create_refund"):   createRefund,
	mcp.ToolName("stripe_update_refund"):   updateRefund,

	// Disputes
	mcp.ToolName("stripe_list_disputes"):    listDisputes,
	mcp.ToolName("stripe_retrieve_dispute"): retrieveDispute,
	mcp.ToolName("stripe_update_dispute"):   updateDispute,

	// Payouts
	mcp.ToolName("stripe_list_payouts"):    listPayouts,
	mcp.ToolName("stripe_retrieve_payout"): retrievePayout,
	mcp.ToolName("stripe_create_payout"):   createPayout,

	// Subscriptions
	mcp.ToolName("stripe_list_subscriptions"):    listSubscriptions,
	mcp.ToolName("stripe_retrieve_subscription"): retrieveSubscription,
	mcp.ToolName("stripe_search_subscriptions"):  searchSubscriptions,
	mcp.ToolName("stripe_create_subscription"):   createSubscription,
	mcp.ToolName("stripe_update_subscription"):   updateSubscription,
	mcp.ToolName("stripe_cancel_subscription"):   cancelSubscription,

	// Subscription Items
	mcp.ToolName("stripe_list_subscription_items"):    listSubscriptionItems,
	mcp.ToolName("stripe_retrieve_subscription_item"): retrieveSubscriptionItem,

	// Products
	mcp.ToolName("stripe_list_products"):    listProducts,
	mcp.ToolName("stripe_retrieve_product"): retrieveProduct,
	mcp.ToolName("stripe_create_product"):   createProduct,
	mcp.ToolName("stripe_update_product"):   updateProduct,
	mcp.ToolName("stripe_delete_product"):   deleteProduct,

	// Prices
	mcp.ToolName("stripe_list_prices"):    listPrices,
	mcp.ToolName("stripe_retrieve_price"): retrievePrice,
	mcp.ToolName("stripe_create_price"):   createPrice,
	mcp.ToolName("stripe_update_price"):   updatePrice,

	// Invoices
	mcp.ToolName("stripe_list_invoices"):             listInvoices,
	mcp.ToolName("stripe_retrieve_invoice"):          retrieveInvoice,
	mcp.ToolName("stripe_retrieve_upcoming_invoice"): retrieveUpcomingInvoice,
	mcp.ToolName("stripe_search_invoices"):           searchInvoices,
	mcp.ToolName("stripe_create_invoice"):            createInvoice,
	mcp.ToolName("stripe_finalize_invoice"):          finalizeInvoice,
	mcp.ToolName("stripe_pay_invoice"):               payInvoice,
	mcp.ToolName("stripe_send_invoice"):              sendInvoice,
	mcp.ToolName("stripe_void_invoice"):              voidInvoice,
	mcp.ToolName("stripe_delete_invoice"):            deleteInvoice,

	// Invoice Items
	mcp.ToolName("stripe_list_invoice_items"):  listInvoiceItems,
	mcp.ToolName("stripe_create_invoice_item"): createInvoiceItem,

	// Payment Methods
	mcp.ToolName("stripe_list_payment_methods"):    listPaymentMethods,
	mcp.ToolName("stripe_retrieve_payment_method"): retrievePaymentMethod,
	mcp.ToolName("stripe_attach_payment_method"):   attachPaymentMethod,
	mcp.ToolName("stripe_detach_payment_method"):   detachPaymentMethod,

	// Setup Intents
	mcp.ToolName("stripe_list_setup_intents"):    listSetupIntents,
	mcp.ToolName("stripe_retrieve_setup_intent"): retrieveSetupIntent,
	mcp.ToolName("stripe_create_setup_intent"):   createSetupIntent,

	// Coupons / Promotion Codes
	mcp.ToolName("stripe_list_coupons"):          listCoupons,
	mcp.ToolName("stripe_retrieve_coupon"):       retrieveCoupon,
	mcp.ToolName("stripe_create_coupon"):         createCoupon,
	mcp.ToolName("stripe_delete_coupon"):         deleteCoupon,
	mcp.ToolName("stripe_list_promotion_codes"):  listPromotionCodes,
	mcp.ToolName("stripe_create_promotion_code"): createPromotionCode,

	// Events
	mcp.ToolName("stripe_list_events"):    listEvents,
	mcp.ToolName("stripe_retrieve_event"): retrieveEvent,
}
