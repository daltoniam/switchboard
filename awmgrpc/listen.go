package awmgrpc

import (
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
)

const defaultListenHost = "127.0.0.1"

// TCPListenAddr builds the native gRPC / HTTP listen address. An empty host
// defaults to loopback so local installations are not accidentally exposed.
// Operators may opt into another host (including 0.0.0.0 / ::) explicitly.
func TCPListenAddr(host string, port int) (string, error) {
	if port <= 0 || port > 65535 {
		return "", fmt.Errorf("port must be between 1 and 65535")
	}
	host = strings.TrimSpace(host)
	if host == "" {
		host = defaultListenHost
	}
	if _, _, err := net.SplitHostPort(host); err == nil {
		return "", fmt.Errorf("listen host %q must not include a port", host)
	}
	host = strings.TrimPrefix(strings.TrimSuffix(host, "]"), "[")
	switch host {
	case "*", "0.0.0.0":
		return fmt.Sprintf("0.0.0.0:%d", port), nil
	case "::":
		return fmt.Sprintf("[::]:%d", port), nil
	}
	return net.JoinHostPort(host, fmt.Sprintf("%d", port)), nil
}

// ListenUnix opens a Unix-domain socket for native gRPC only. A leftover
// socket from a previous process is removed; a live listener or regular file
// is refused. The resulting socket is chmod 0600 on Unix platforms.
func ListenUnix(path string) (net.Listener, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, errors.New("unix socket path is required")
	}
	if runtime.GOOS == "windows" {
		return nil, errors.New("unix domain sockets are not supported on windows")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, fmt.Errorf("create unix socket directory: %w", err)
	}
	if err := removeStaleUnixSocket(path); err != nil {
		return nil, err
	}
	ln, err := net.Listen("unix", path)
	if err != nil {
		return nil, fmt.Errorf("listen unix %s: %w", path, err)
	}
	if err := os.Chmod(path, 0o600); err != nil {
		_ = ln.Close()
		_ = os.Remove(path)
		return nil, fmt.Errorf("chmod unix socket: %w", err)
	}
	return &unixListener{Listener: ln, path: path}, nil
}

func removeStaleUnixSocket(path string) error {
	info, err := os.Lstat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if info.Mode()&os.ModeSocket == 0 {
		return fmt.Errorf("unix socket path %s exists and is not a socket", path)
	}
	conn, err := net.DialTimeout("unix", path, 0)
	if err == nil {
		_ = conn.Close()
		return fmt.Errorf("unix socket %s is already in use", path)
	}
	if !isStaleUnixDialError(err) {
		return fmt.Errorf("probe unix socket %s: %w", path, err)
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove stale unix socket: %w", err)
	}
	return nil
}

func isStaleUnixDialError(err error) bool {
	if errors.Is(err, syscall.ECONNREFUSED) || errors.Is(err, syscall.ENOENT) {
		return true
	}
	var opErr *net.OpError
	if errors.As(err, &opErr) {
		if errors.Is(opErr.Err, syscall.ECONNREFUSED) || errors.Is(opErr.Err, syscall.ENOENT) {
			return true
		}
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "connection refused") || strings.Contains(msg, "no such file")
}

type unixListener struct {
	net.Listener
	path string
}

func (l *unixListener) Close() error {
	err := l.Listener.Close()
	if rmErr := os.Remove(l.path); rmErr != nil && !os.IsNotExist(rmErr) && err == nil {
		return rmErr
	}
	return err
}
