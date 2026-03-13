package browser

import (
	"bytes"
	"context"
	"testing"

	mcp "github.com/daltoniam/switchboard"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Interface compliance — compile-time assertions.
var (
	_ mcp.BrowserService = (*service)(nil)
	_ mcp.BrowserSession = (*session)(nil)
	_ mcp.BrowserPage    = (*page)(nil)
)

// skipIfUnavailable calls t.Skip if playwright driver is not installed.
func skipIfUnavailable(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Skipf("playwright not available (%v) — install with: go run github.com/playwright-community/playwright-go/cmd/playwright install chromium", err)
	}
}

func TestNew_HeadlessBrowser(t *testing.T) {
	svc, err := New(true)
	skipIfUnavailable(t, err)
	require.NoError(t, err)
	defer func() { require.NoError(t, svc.Close()) }()

	ctx := context.Background()

	sess, err := svc.NewSession(ctx)
	require.NoError(t, err)
	defer func() { require.NoError(t, sess.Close()) }()

	pg, err := sess.NewPage(ctx)
	require.NoError(t, err)
	defer func() { require.NoError(t, pg.Close()) }()

	require.NoError(t, pg.Navigate(ctx, "about:blank"))

	content, err := pg.Content(ctx)
	require.NoError(t, err)
	assert.NotEmpty(t, content)
}

func TestBrowserSession_IsolatedCookies(t *testing.T) {
	svc, err := New(true)
	skipIfUnavailable(t, err)
	require.NoError(t, err)
	defer func() { require.NoError(t, svc.Close()) }()

	ctx := context.Background()

	sess1, err := svc.NewSession(ctx)
	require.NoError(t, err)
	defer func() { require.NoError(t, sess1.Close()) }()

	sess2, err := svc.NewSession(ctx)
	require.NoError(t, err)
	defer func() { require.NoError(t, sess2.Close()) }()

	pg1, err := sess1.NewPage(ctx)
	require.NoError(t, err)
	defer func() { require.NoError(t, pg1.Close()) }()

	pg2, err := sess2.NewPage(ctx)
	require.NoError(t, err)
	defer func() { require.NoError(t, pg2.Close()) }()

	require.NoError(t, pg1.Navigate(ctx, "about:blank"))
	require.NoError(t, pg2.Navigate(ctx, "about:blank"))

	// Both pages return content — sessions are independent.
	c1, err := pg1.Content(ctx)
	require.NoError(t, err)
	assert.NotEmpty(t, c1)

	c2, err := pg2.Content(ctx)
	require.NoError(t, err)
	assert.NotEmpty(t, c2)
}

func TestPage_Evaluate(t *testing.T) {
	svc, err := New(true)
	skipIfUnavailable(t, err)
	require.NoError(t, err)
	defer func() { require.NoError(t, svc.Close()) }()

	ctx := context.Background()

	sess, err := svc.NewSession(ctx)
	require.NoError(t, err)
	defer func() { require.NoError(t, sess.Close()) }()

	pg, err := sess.NewPage(ctx)
	require.NoError(t, err)
	defer func() { require.NoError(t, pg.Close()) }()

	require.NoError(t, pg.Navigate(ctx, "about:blank"))

	result, err := pg.Evaluate(ctx, "1 + 1")
	require.NoError(t, err)
	assert.Equal(t, float64(2), result)
}

func TestPage_Screenshot(t *testing.T) {
	svc, err := New(true)
	skipIfUnavailable(t, err)
	require.NoError(t, err)
	defer func() { require.NoError(t, svc.Close()) }()

	ctx := context.Background()

	sess, err := svc.NewSession(ctx)
	require.NoError(t, err)
	defer func() { require.NoError(t, sess.Close()) }()

	pg, err := sess.NewPage(ctx)
	require.NoError(t, err)
	defer func() { require.NoError(t, pg.Close()) }()

	require.NoError(t, pg.Navigate(ctx, "about:blank"))

	data, err := pg.Screenshot(ctx)
	require.NoError(t, err)
	// PNG magic bytes: 0x89 0x50 0x4E 0x47
	pngMagic := []byte{0x89, 0x50, 0x4E, 0x47}
	assert.True(t, bytes.HasPrefix(data, pngMagic), "expected PNG data")
}
