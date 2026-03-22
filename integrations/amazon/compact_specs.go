package amazon

import (
	"fmt"

	mcp "github.com/daltoniam/switchboard"
)

var rawFieldCompactionSpecs = map[string][]string{
	"amazon_search_products": {
		"asin", "title", "is_sponsored", "brand",
		"price", "reviews.average_rating", "reviews.review_count",
		"is_prime_eligible", "product_url",
	},
	"amazon_get_product": {
		"asin", "title", "price", "can_use_subscribe_and_save",
		"description.overview", "description.features",
		"reviews.average_rating", "reviews.reviews_count",
		"main_image_url",
	},
	"amazon_get_orders": {
		"order_info.order_number", "order_info.order_date",
		"order_info.total", "order_info.status",
		"items[].title", "items[].asin", "items[].return_eligible",
	},
	"amazon_get_cart": {
		"is_empty", "subtotal", "total_items",
		"items[].title", "items[].price", "items[].quantity",
		"items[].asin", "items[].availability",
	},
}

var fieldCompactionSpecs = mustBuildFieldCompactionSpecs(rawFieldCompactionSpecs)

func mustBuildFieldCompactionSpecs(raw map[string][]string) map[string][]mcp.CompactField {
	parsed := make(map[string][]mcp.CompactField, len(raw))
	for tool, specs := range raw {
		fields, err := mcp.ParseCompactSpecs(specs)
		if err != nil {
			panic(fmt.Sprintf("amazon: invalid field compaction spec for %q: %v", tool, err))
		}
		parsed[tool] = fields
	}
	return parsed
}
