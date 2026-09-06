package awmgrpc

import (
	"net"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTCPListenAddr_DefaultsToLoopback(t *testing.T) {
	addr, err := TCPListenAddr("", 3847)
	require.NoError(t, err)
	assert.Equal(t, "127.0.0.1:3847", addr)
}

func TestTCPListenAddr_OptInAndRejects(t *testing.T) {
	tests := []struct {
		name    string
		host    string
		port    int
		want    string
		wantErr string
	}{
		{name: "explicit loopback", host: "127.0.0.1", port: 3847, want: "127.0.0.1:3847"},
		{name: "all interfaces ipv4", host: "0.0.0.0", port: 8080, want: "0.0.0.0:8080"},
		{name: "wildcard star", host: "*", port: 9, want: "0.0.0.0:9"},
		{name: "ipv6 all interfaces", host: "::", port: 3847, want: "[::]:3847"},
		{name: "hostname", host: "localhost", port: 1, want: "localhost:1"},
		{name: "host already has port", host: "127.0.0.1:8080", port: 3847, wantErr: "must not include a port"},
		{name: "invalid port", host: "127.0.0.1", port: 0, wantErr: "port"},
		{name: "negative port", host: "127.0.0.1", port: -1, wantErr: "port"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := TCPListenAddr(tt.host, tt.port)
			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestListenUnix_CreatesRestrictiveSocket(t *testing.T) {
	requireUnix(t)
	path := filepath.Join(t.TempDir(), "awm.sock")
	ln, err := ListenUnix(path)
	require.NoError(t, err)
	t.Cleanup(func() { _ = ln.Close(); _ = os.Remove(path) })

	info, err := os.Stat(path)
	require.NoError(t, err)
	assert.True(t, info.Mode()&os.ModeSocket != 0)
	if runtime.GOOS != "windows" {
		assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())
	}

	conn, err := net.DialTimeout("unix", path, time.Second)
	require.NoError(t, err)
	_ = conn.Close()
}

func TestListenUnix_ReplacesStaleSocket(t *testing.T) {
	requireUnix(t)
	path := filepath.Join(t.TempDir(), "stale.sock")
	require.NoError(t, writeStaleUnixSocket(path))
	info, err := os.Lstat(path)
	require.NoError(t, err)
	require.NotZero(t, info.Mode()&os.ModeSocket)

	ln, err := ListenUnix(path)
	require.NoError(t, err)
	t.Cleanup(func() { _ = ln.Close(); _ = os.Remove(path) })

	conn, err := net.DialTimeout("unix", path, time.Second)
	require.NoError(t, err)
	_ = conn.Close()
}

func TestListenUnix_RefusesLiveSocketAndRegularFile(t *testing.T) {
	requireUnix(t)
	t.Run("live socket", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "live.sock")
		live, err := net.Listen("unix", path)
		require.NoError(t, err)
		t.Cleanup(func() { _ = live.Close(); _ = os.Remove(path) })

		_, err = ListenUnix(path)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "already in use")
	})
	t.Run("regular file", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "not-a-socket")
		require.NoError(t, os.WriteFile(path, []byte("nope"), 0o600))
		_, err := ListenUnix(path)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not a socket")
	})
	t.Run("empty path", func(t *testing.T) {
		_, err := ListenUnix("")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "path is required")
	})
}

func requireUnix(t *testing.T) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "probe.sock")
	ln, err := net.Listen("unix", path)
	if err != nil {
		t.Skipf("unix sockets unavailable: %v", err)
	}
	_ = ln.Close()
	_ = os.Remove(path)
}

func writeStaleUnixSocket(path string) error {
	// unixgram creates a socket inode without a stream listener, which is
	// what a crashed previous process leaves behind.
	conn, err := net.ListenUnixgram("unixgram", &net.UnixAddr{Name: path, Net: "unixgram"})
	if err != nil {
		return err
	}
	return conn.Close()
}
