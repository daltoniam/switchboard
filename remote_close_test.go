package mcp

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

var closeMethod = regexp.MustCompile(`(?m)^func \(\w+ \*\w+\) Close\(\) error \{`)

// TestRemoteMCPAdapters_ImplementClose: every adapter that holds a remote MCP
// session (it imports remotemcp) must implement io.Closer, so a host that
// drops an instance (a per-request pool, a reconfigure) can end the session
// instead of leaking it.
func TestRemoteMCPAdapters_ImplementClose(t *testing.T) {
	dirs, err := filepath.Glob("integrations/*")
	require.NoError(t, err)
	for _, dir := range dirs {
		files, err := filepath.Glob(filepath.Join(dir, "*.go"))
		require.NoError(t, err)
		usesRemote, hasClose := false, false
		for _, f := range files {
			if strings.HasSuffix(f, "_test.go") {
				continue
			}
			data, err := os.ReadFile(f)
			require.NoError(t, err)
			src := string(data)
			if strings.Contains(src, `"github.com/daltoniam/switchboard/remotemcp"`) {
				usesRemote = true
			}
			if closeMethod.MatchString(src) {
				hasClose = true
			}
		}
		if usesRemote && !hasClose {
			t.Errorf("%s uses remotemcp but has no Close() error method; implement io.Closer to end its session", dir)
		}
	}
}
