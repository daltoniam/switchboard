package layouts

import (
	"bytes"
	"context"
	"encoding/base64"
	"strings"
	"testing"

	"github.com/PuerkitoBio/goquery"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBase_ExternalScripts(t *testing.T) {
	var output bytes.Buffer
	require.NoError(t, Base(PageData{Title: "Test"}).Render(context.Background(), &output))
	doc, err := goquery.NewDocumentFromReader(&output)
	require.NoError(t, err)
	scripts := doc.Find(`head script[src^="https://"]`)
	require.Equal(t, 2, scripts.Length())
	for _, src := range []string{
		"https://cdn.jsdelivr.net/npm/htmx.org@2.0.11/dist/htmx.min.js",
		"https://cdn.jsdelivr.net/npm/@tarekraafat/autocomplete.js@10.2.10/dist/autoComplete.min.js",
	} {
		t.Run(src, func(t *testing.T) { checkScript(t, scripts, src) })
	}
}

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

func TestBase_NavigationIcons(t *testing.T) {
	var output bytes.Buffer
	require.NoError(t, Base(PageData{CurrentPath: "/integrations"}).Render(context.Background(), &output))
	doc, err := goquery.NewDocumentFromReader(&output)
	require.NoError(t, err)
	for _, item := range []NavItem{
		{Path: "/", Label: "Dashboard", Icon: "i-lucide:layout-dashboard"},
		{Path: "/projects", Label: "Projects", Icon: "i-lucide:folder"},
		{Path: "/integrations", Label: "Integrations", Icon: "i-lucide:plug"},
		{Path: "/plugins", Label: "Plugin Marketplace", Icon: "i-lucide:store"},
		{Path: "/settings", Label: "Settings", Icon: "i-lucide:settings"},
	} {
		t.Run(item.Label, func(t *testing.T) { checkNavIcon(t, doc, item) })
	}
	assert.Equal(t, "/integrations", doc.Find(`[aria-current="page"]`).AttrOr("href", ""))
}

func TestBase_FlashControls(t *testing.T) {
	var output bytes.Buffer
	data := PageData{FlashSuccess: "Saved", FlashError: "Failed"}
	require.NoError(t, Base(data).Render(context.Background(), &output))
	doc, err := goquery.NewDocumentFromReader(&output)
	require.NoError(t, err)
	assert.False(t, doc.Find(".toast-stack").Is("[aria-label]"))
	for _, key := range []string{"success", "error"} {
		banner := doc.Find(`[data-flash="` + key + `"]`)
		assert.Equal(t, 1, banner.Length())
		button := banner.Find("button[data-dismiss-flash]")
		assert.Equal(t, "button", button.AttrOr("type", ""))
		assert.NotEmpty(t, button.AttrOr("aria-label", ""))
		assert.Empty(t, strings.TrimSpace(button.Text()))
		assert.Equal(t, 1, button.Find(`[class~="i-lucide:x"][aria-hidden="true"]`).Length())
	}
	assert.Equal(t, "-1", doc.Find("main").AttrOr("tabindex", ""))
}

func TestBase_Appearance(t *testing.T) {
	var output bytes.Buffer
	require.NoError(t, Base(PageData{}).Render(context.Background(), &output))
	html := output.String()
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	require.NoError(t, err)
	assert.False(t, doc.Find(".appearance-controls").Is("[aria-label]"))
	assert.Less(t, strings.Index(html, `id="appearance-bootstrap"`), strings.Index(html, `<link`))
	assert.Equal(t, 1, doc.Find(`head #appearance-config`).Length())
	firstControl := doc.Find(`header [data-preference]`).First()
	assert.Equal(t, "fontSize", firstControl.AttrOr("data-preference", ""))
	config := appearanceSettings()
	for _, theme := range config.Themes {
		t.Run(theme.ID, func(t *testing.T) {
			option := doc.Find(`input[data-preference="theme"][value="` + theme.ID + `"]`)
			assert.Equal(t, theme.Name, option.AttrOr("aria-label", ""))
			assert.True(t, option.Is(`[type="radio"][disabled]`))
			preview := option.Parent().Find(`.theme-swatch[data-theme="` + theme.ID + `"]`)
			assert.Equal(t, 4, preview.Children().Length())
			assert.Equal(t, "true", preview.AttrOr("aria-hidden", ""))
		})
	}
	assert.Equal(t, len(config.FontSizes), doc.Find(`input[data-preference="fontSize"]`).Length())
	assert.Equal(t, "Theme", doc.Find(`#theme-label.sr-only`).Text())
}

func TestBase_FontSizeGroup(t *testing.T) {
	var output bytes.Buffer
	require.NoError(t, Base(PageData{}).Render(context.Background(), &output))
	doc, err := goquery.NewDocumentFromReader(&output)
	require.NoError(t, err)
	picker := doc.Find(`[data-font-picker]`)
	assert.Equal(t, "Font size", picker.Find("summary").AttrOr("aria-label", ""))
	group := picker.Find(`[role="radiogroup"][aria-label="Font size"]`)
	require.Equal(t, 1, group.Length())
	assert.Equal(t, 3, group.Find(`input[type="radio"]`).Length())
	for _, size := range []string{"80", "100", "120"} {
		t.Run(size, func(t *testing.T) {
			radio := group.Find(`input[value="` + size + `"]`)
			assert.Equal(t, "font-size", radio.AttrOr("name", ""))
			assert.Equal(t, size+"%", radio.AttrOr("aria-label", ""))
			assert.True(t, radio.Is("[disabled]"))
			assert.Equal(t, size == "100", radio.Is("[checked]"))
			assert.Equal(t, "A", radio.SiblingsFiltered(".font-size-option").Text())
		})
	}
}

func checkNavIcon(t *testing.T, doc *goquery.Document, item NavItem) {
	t.Helper()
	link := doc.Find(`nav a[href="` + item.Path + `"]`)
	assert.Equal(t, item.Label, strings.TrimSpace(link.Text()))
	icon := link.Find("span")
	require.Equal(t, 1, icon.Length())
	assert.True(t, icon.HasClass(item.Icon))
	assert.Equal(t, "true", icon.AttrOr("aria-hidden", ""))
	assert.True(t, icon.HasClass("w-[14px]") && icon.HasClass("h-[14px]"))
}

func checkScript(t *testing.T, scripts *goquery.Selection, src string) {
	t.Helper()
	script := scripts.Filter(`[src="` + src + `"]`)
	require.Equal(t, 1, script.Length())
	assert.Equal(t, "anonymous", script.AttrOr("crossorigin", ""))
	_, deferred := script.Attr("defer")
	assert.True(t, deferred, "dependency scripts must not block HTML parsing")
	integrity := script.AttrOr("integrity", "")
	require.True(t, strings.HasPrefix(integrity, "sha384-"))
	digest, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(integrity, "sha384-"))
	require.NoError(t, err)
	assert.Len(t, digest, 48)
}
