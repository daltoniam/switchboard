package daemon

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

const (
	pidFileName = "switchboard.pid"
	logFileName = "switchboard.log"
)

var pidPathFunc = defaultPIDPath

type Status struct {
	Running bool   `json:"running"`
	PID     int    `json:"pid,omitempty"`
	Healthy bool   `json:"healthy,omitempty"`
	Port    int    `json:"port,omitempty"`
	Uptime  string `json:"uptime,omitempty"`
}

func ConfigDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("get home dir: %w", err)
	}
	return filepath.Join(home, ".config", "switchboard"), nil
}

func PIDPath() (string, error) {
	return pidPathFunc()
}

func defaultPIDPath() (string, error) {
	dir, err := ConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, pidFileName), nil
}

func LogPath() (string, error) {
	dir, err := ConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, logFileName), nil
}

func WritePID(pid int) error {
	path, err := PIDPath()
	if err != nil {
		return err
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}
	return os.WriteFile(path, []byte(strconv.Itoa(pid)), 0600)
}

func ReadPID() (int, error) {
	path, err := PIDPath()
	if err != nil {
		return 0, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, err
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return 0, fmt.Errorf("invalid PID file: %w", err)
	}
	return pid, nil
}

func RemovePID() error {
	path, err := PIDPath()
	if err != nil {
		return err
	}
	err = os.Remove(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func IsRunning(pid int) bool {
	if pid <= 0 {
		return false
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	err = proc.Signal(syscall.Signal(0))
	return err == nil
}

func CheckHealth(port int) bool {
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get(fmt.Sprintf("http://localhost:%d/api/health", port))
	if err != nil {
		return false
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return false
	}
	limited := io.LimitReader(resp.Body, 1024)
	var result map[string]string
	if err := json.NewDecoder(limited).Decode(&result); err != nil {
		return false
	}
	return result["status"] == "healthy"
}

func GetStatus(port int) (*Status, error) {
	pid, err := ReadPID()
	if err != nil {
		return nil, fmt.Errorf("read PID: %w", err)
	}

	status := &Status{Port: port}
	if pid > 0 && IsRunning(pid) {
		status.Running = true
		status.PID = pid
		status.Healthy = CheckHealth(port)
	}
	return status, nil
}

func StopProcess() error {
	pid, err := ReadPID()
	if err != nil {
		return fmt.Errorf("read PID: %w", err)
	}
	if pid == 0 || !IsRunning(pid) {
		_ = RemovePID()
		return errors.New("switchboard is not running")
	}

	proc, err := os.FindProcess(pid)
	if err != nil {
		return fmt.Errorf("find process: %w", err)
	}

	if err := proc.Signal(syscall.SIGTERM); err != nil {
		return fmt.Errorf("send SIGTERM: %w", err)
	}

	for range 50 {
		time.Sleep(100 * time.Millisecond)
		if !IsRunning(pid) {
			_ = RemovePID()
			return nil
		}
	}

	_ = proc.Signal(syscall.SIGKILL)
	_ = RemovePID()
	return fmt.Errorf("process %d did not exit after SIGTERM, sent SIGKILL", pid)
}

func ExePath() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("get executable path: %w", err)
	}
	exe, err = filepath.EvalSymlinks(exe)
	if err != nil {
		return "", fmt.Errorf("resolve symlinks: %w", err)
	}
	return exe, nil
}
