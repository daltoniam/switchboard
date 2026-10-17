package layouts

import (
	"bytes"
	"context"
	"testing"

	"github.com/PuerkitoBio/goquery"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBase_Stylesheet(t *testing.T) {
	var output bytes.Buffer
	require.NoError(t, Base(PageData{}).Render(context.Background(), &output))
	doc, err := goquery.NewDocumentFromReader(&output)
	require.NoError(t, err)
	stylesheet := doc.Find(`head link[rel="stylesheet"][href="/static/style.css"]`)
	require.Equal(t, 1, stylesheet.Length())
	assert.Empty(t, stylesheet.AttrOr("media", ""))
	assert.Zero(t, doc.Find(`script[src*="unocss"], script[src="/static/style.js"]`).Length())
}
