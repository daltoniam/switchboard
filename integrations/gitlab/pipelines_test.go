package gitlab

import (
	"context"
	"net/http"
	"strings"
	"testing"

	mcp "github.com/daltoniam/switchboard"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetJobTrace_NearOneMiBPlainText(t *testing.T) {
	const cap = 1024 * 1024
	line := "STEP failed\n"
	repeat := cap / len(line)
	body := strings.Repeat(line, repeat)
	require.GreaterOrEqual(t, len(body), cap-64)

	g, _ := configured(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(body))
	})
	res, err := g.Execute(context.Background(), "gitlab_get_job_trace", map[string]any{
		"project_id": "1",
		"job_id":     1,
	})
	require.NoError(t, err)
	require.False(t, res.IsError, res.Data)
	assert.Equal(t, body, res.Data)
	assert.LessOrEqual(t, len(res.Data), cap)

	wrapped, err := mcp.JSONResult(map[string]string{"trace": body})
	require.NoError(t, err)
	assert.Greater(t, len(wrapped.Data), cap, "JSON wrapping would exceed 1 MiB transport cap for newline-heavy traces")
}
