package amazon

import (
	"testing"

	mcp "github.com/daltoniam/switchboard"
	"github.com/daltoniam/switchboard/compactyaml"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestFieldCompactionSpecs_AllParse(t *testing.T) {
	require.NotEmpty(t, fieldCompactionSpecs, "fieldCompactionSpecs should not be empty")
}

func TestFieldCompactionSpecs_NoDuplicateTools(t *testing.T) {
	var sf compactyaml.SpecFile
	require.NoError(t, yaml.Unmarshal(compactYAML, &sf))
	assert.Equal(t, len(sf.Tools), len(fieldCompactionSpecs))
}

func TestFieldCompactionSpecs_NoOrphanSpecs(t *testing.T) {
	for toolName := range fieldCompactionSpecs {
		_, ok := dispatch[toolName]
		assert.True(t, ok, "field compaction spec for %q has no dispatch handler", toolName)
	}
}

func TestFieldCompactionSpec_ReturnsFieldsForListTool(t *testing.T) {
	a := &amazon{}
	fields, ok := a.CompactSpec("amazon_search_products")
	require.True(t, ok, "amazon_search_products should have field compaction spec")
	assert.NotEmpty(t, fields)
}

func TestFieldCompactionSpec_ReturnsFalseForUnknownTool(t *testing.T) {
	a := &amazon{}
	_, ok := a.CompactSpec("amazon_nonexistent")
	assert.False(t, ok, "unknown tools should return false")
}

func TestFieldCompactionSpec_ReturnsFalseForMutationTool(t *testing.T) {
	a := &amazon{}
	_, ok := a.CompactSpec("amazon_add_to_cart")
	assert.False(t, ok, "mutation tools should not have compaction specs")
}

func TestFieldCompactionSpec_NoMissingReadToolSpecs(t *testing.T) {
	readTools := map[string]bool{
		"amazon_search_products": true,
		"amazon_get_product":     true,
		"amazon_get_orders":      true,
		"amazon_get_cart":        true,
	}
	for toolName := range readTools {
		_, ok := fieldCompactionSpecs[mcp.ToolName(toolName)]
		assert.True(t, ok, "read tool %q should have a field compaction spec", toolName)
	}
}

func TestFieldCompactionSpecs_ShapeParity(t *testing.T) {
	handlerOutputs := map[string]string{
		"amazon_search_products": `[{"asin":"B0CHXKM5GK","title":"Test Product","is_sponsored":false,"brand":"BrandX","price":"$29.99","reviews":{"average_rating":"4.5 out of 5 stars","review_count":"1,234"},"is_prime_eligible":true,"product_url":"https://www.amazon.com/dp/B0CHXKM5GK"}]`,
		"amazon_get_product":     `{"asin":"B0CHXKM5GK","title":"Test Product","price":"$29.99","can_use_subscribe_and_save":true,"description":{"overview":"Great","features":"Fast"},"reviews":{"average_rating":"4.5","reviews_count":"1,234"},"main_image_url":"https://img.example.com/img.jpg"}`,
		"amazon_get_orders":      `[{"order_info":{"order_number":"123-456","order_date":"March 1","total":"$50.00","status":"Delivered"},"items":[{"title":"Widget","asin":"B0CHXKM5GK","return_eligible":true}]}]`,
		"amazon_get_cart":        `{"is_empty":false,"subtotal":"$49.98","total_items":2,"items":[{"title":"Mouse","price":"$24.99","quantity":2,"asin":"B0CHXKM5GK","availability":"In Stock"}]}`,
	}

	for toolName, payload := range handlerOutputs {
		t.Run(toolName, func(t *testing.T) {
			fields, ok := fieldCompactionSpecs[mcp.ToolName(toolName)]
			require.True(t, ok, "missing compaction spec for %s", toolName)
			compacted, err := mcp.CompactJSON([]byte(payload), fields)
			require.NoError(t, err)
			assert.NotEqual(t, "{}", string(compacted), "compaction returned empty object — spec paths likely don't match response shape")
			assert.NotEqual(t, "[]", string(compacted), "compaction returned empty array — spec paths likely don't match response shape")
			assert.NotEqual(t, "[{}]", string(compacted), "compaction returned array of empty objects — spec paths likely don't match response shape")
		})
	}
}
