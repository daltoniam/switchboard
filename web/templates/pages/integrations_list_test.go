package pages

import (
	"bytes"
	"context"
	"testing"

	"github.com/PuerkitoBio/goquery"
	"github.com/daltoniam/switchboard/web/templates/layouts"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIntegrationsList_HealthForm(t *testing.T) {
	var output bytes.Buffer
	require.NoError(t, IntegrationsList(layouts.PageData{}, IntegrationsListData{}).Render(context.Background(), &output))
	doc, err := goquery.NewDocumentFromReader(&output)
	require.NoError(t, err)
	headerActions := doc.Find(".page-title").Next()
	badge := headerActions.Find(".badge")
	assert.Equal(t, "0 integrations", badge.Text())
	assert.False(t, badge.Is("[aria-label]"), "counter text must be exposed without naming a generic span")
	form := doc.Find("#health-refresh")
	require.Equal(t, 1, form.Length())
	for key, value := range map[string]string{"method": "POST", "action": "/api/health/refresh", "hx-post": "/api/health/refresh", "hx-target": "#integration-results", "hx-swap": "outerHTML"} {
		assert.Equal(t, value, form.AttrOr(key, ""), key)
	}
	assert.Equal(t, 1, doc.Find("#integration-results").Length())
	assert.Equal(t, 1, doc.Find("#integration-results #health-refresh[hx-preserve]").Length())
	assert.Empty(t, form.Find("form").Nodes)
	assert.Equal(t, "page-refresh-status", form.AttrOr("aria-describedby", ""))
	assert.Equal(t, "status", doc.Find("#page-refresh-status").AttrOr("role", ""))
}
