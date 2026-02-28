package daemon

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWriteAndReadPID(t *testing.T) {
	tmp := t.TempDir()
	origFunc := overridePIDPath(t, filepath.Join(tmp, pidFileName))

	err := WritePID(12345)
	require.NoError(t, err)

	pid, err := ReadPID()
	require.NoError(t, err)
	assert.Equal(t, 12345, pid)

	data, err := os.ReadFile(filepath.Join(tmp, pidFileName))
	require.NoError(t, err)
	assert.Equal(t, "12345", string(data))

	_ = origFunc
}

func TestReadPID_NotExist(t *testing.T) {
	tmp := t.TempDir()
	overridePIDPath(t, filepath.Join(tmp, pidFileName))

	pid, err := ReadPID()
	require.NoError(t, err)
	assert.Equal(t, 0, pid)
}

func TestReadPID_InvalidContent(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, pidFileName)
	overridePIDPath(t, path)

	err := os.WriteFile(path, []byte("notanumber"), 0600)
	require.NoError(t, err)

	_, err = ReadPID()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid PID file")
}

func TestRemovePID(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, pidFileName)
	overridePIDPath(t, path)

	err := WritePID(99999)
	require.NoError(t, err)

	err = RemovePID()
	require.NoError(t, err)

	_, err = os.Stat(path)
	assert.True(t, os.IsNotExist(err))
}

func TestRemovePID_NotExist(t *testing.T) {
	tmp := t.TempDir()
	overridePIDPath(t, filepath.Join(tmp, pidFileName))

	err := RemovePID()
	assert.NoError(t, err)
}

func TestIsRunning_CurrentProcess(t *testing.T) {
	assert.True(t, IsRunning(os.Getpid()))
}

func TestIsRunning_InvalidPID(t *testing.T) {
	assert.False(t, IsRunning(0))
	assert.False(t, IsRunning(-1))
}

func TestCheckHealth_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "healthy"})
	}))
	defer srv.Close()

	var port int
	fmt.Sscanf(srv.URL, "http://127.0.0.1:%d", &port)
	assert.True(t, CheckHealth(port))
}

func TestCheckHealth_Unhealthy(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	var port int
	fmt.Sscanf(srv.URL, "http://127.0.0.1:%d", &port)
	assert.False(t, CheckHealth(port))
}

func TestCheckHealth_NoServer(t *testing.T) {
	assert.False(t, CheckHealth(19999))
}

func TestGetStatus_NotRunning(t *testing.T) {
	tmp := t.TempDir()
	overridePIDPath(t, filepath.Join(tmp, pidFileName))

	status, err := GetStatus(3847)
	require.NoError(t, err)
	assert.False(t, status.Running)
	assert.Equal(t, 0, status.PID)
}

func TestGetStatus_Running(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, pidFileName)
	overridePIDPath(t, path)

	pid := os.Getpid()
	err := os.WriteFile(path, []byte(strconv.Itoa(pid)), 0600)
	require.NoError(t, err)

	status, err := GetStatus(3847)
	require.NoError(t, err)
	assert.True(t, status.Running)
	assert.Equal(t, pid, status.PID)
}

func TestGetStatus_StalePID(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, pidFileName)
	overridePIDPath(t, path)

	err := os.WriteFile(path, []byte("999999999"), 0600)
	require.NoError(t, err)

	status, err := GetStatus(3847)
	require.NoError(t, err)
	assert.False(t, status.Running)
}

func TestConfigDir(t *testing.T) {
	dir, err := ConfigDir()
	require.NoError(t, err)
	assert.Contains(t, dir, "switchboard")
	assert.Contains(t, dir, ".config")
}

func TestLogPath(t *testing.T) {
	path, err := LogPath()
	require.NoError(t, err)
	assert.Contains(t, path, logFileName)
}

func TestExePath(t *testing.T) {
	exe, err := ExePath()
	require.NoError(t, err)
	assert.NotEmpty(t, exe)

	_, err = os.Stat(exe)
	assert.NoError(t, err)
}

func TestStatus_Struct(t *testing.T) {
	s := &Status{
		Running: true,
		PID:     1234,
		Healthy: true,
		Port:    3847,
	}
	assert.True(t, s.Running)
	assert.Equal(t, 1234, s.PID)
	assert.True(t, s.Healthy)
	assert.Equal(t, 3847, s.Port)
}

// overridePIDPath temporarily replaces the PID path lookup for testing.
// It returns a cleanup function (also registered with t.Cleanup).
func overridePIDPath(t *testing.T, path string) func() {
	t.Helper()
	origPIDPathFunc := pidPathFunc
	pidPathFunc = func() (string, error) { return path, nil }
	cleanup := func() { pidPathFunc = origPIDPathFunc }
	t.Cleanup(cleanup)
	return cleanup
}
