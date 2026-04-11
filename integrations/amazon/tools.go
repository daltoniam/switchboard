package amazon

import mcp "github.com/daltoniam/switchboard"

var tools = []mcp.ToolDefinition{
	// ── Products ──────────────────────────────────────────────────
	{
		Name:        mcp.ToolName("amazon_search_products"),
		Description: "Search for products on Amazon. Returns up to 20 results with title, price, rating, Prime eligibility, and ASIN. Start here for product discovery workflows.",
		Parameters: map[string]string{
			"search_term": "Search query (e.g. 'wireless headphones', 'collagen powder')",
		},
		Required: []string{"search_term"},
	},
	{
		Name:        mcp.ToolName("amazon_get_product"),
		Description: "Get detailed product information by ASIN (Amazon Standard Identification Number, 10 characters). Returns title, price, description sections, reviews, and image URL. Use after amazon_search_products to drill into a specific product.",
		Parameters: map[string]string{
			"asin": "Product ASIN — exactly 10 characters (e.g. 'B0CHXKM5GK')",
		},
		Required: []string{"asin"},
	},

	// ── Orders ────────────────────────────────────────────────────
	{
		Name:        mcp.ToolName("amazon_get_orders"),
		Description: "Get the authenticated user's recent order history. Returns order details including items, delivery address, status, and return eligibility. Requires valid session cookies.",
	},

	// ── Cart ──────────────────────────────────────────────────────
	{
		Name:        mcp.ToolName("amazon_get_cart"),
		Description: "Get the current Amazon cart contents. Returns items with title, price, quantity, availability, and cart subtotal. Requires valid session cookies.",
	},
	{
		Name:        mcp.ToolName("amazon_add_to_cart"),
		Description: "Add a product to the Amazon cart by ASIN. Navigates to product page and submits the add-to-cart form. Requires valid session cookies.",
		Parameters: map[string]string{
			"asin": "Product ASIN — exactly 10 characters (e.g. 'B0CHXKM5GK')",
		},
		Required: []string{"asin"},
	},
	{
		Name:        mcp.ToolName("amazon_clear_cart"),
		Description: "Remove all items from the Amazon cart. Iterates through cart items and deletes each one. Requires valid session cookies.",
	},
}
