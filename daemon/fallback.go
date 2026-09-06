package daemon

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"
)

func StartFallback(port int, verbose bool) error {
	exe, err := ExePath()
	if err != nil {
		return err
	}

	logPath, err := LogPath()
	if err != nil {
		return err
	}

	logDir := filepath.Dir(logPath)
	if err := os.MkdirAll(logDir, 0700); err != nil {
		return fmt.Errorf("create log dir: %w", err)
	}

	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		return fmt.Errorf("open log file: %w", err)
	}

	args := []string{"--port", fmt.Sprintf("%d", port)}
	if verbose {
		args = append(args, "--verbose")
	}
	cmd := exec.Command(exe, args...) // #nosec G204
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	cmd.Env = append(os.Environ(), "SWITCHBOARD_DAEMON=1")

	setSysProcAttr(cmd)

	if err := cmd.Start(); err != nil {
		_ = logFile.Close()
		return fmt.Errorf("start process: %w", err)
	}

	if err := WritePID(cmd.Process.Pid); err != nil {
		_ = logFile.Close()
		return fmt.Errorf("write PID: %w", err)
	}

	_ = logFile.Close()

	time.Sleep(500 * time.Millisecond)
	if !IsRunning(cmd.Process.Pid) {
		_ = RemovePID()
		return fmt.Errorf("process exited immediately — check %s for details", logPath)
	}

	return nil
}

func Install(port int, verbose bool) error {
	switch runtime.GOOS {
	case "darwin":
		return InstallLaunchd(port, verbose)
	case "linux":
		return InstallSystemd(port, verbose)
	default:
		return fmt.Errorf("service install is not supported on %s — use 'switchboard daemon start' instead", runtime.GOOS)
	}
}

func Uninstall() error {
	switch runtime.GOOS {
	case "darwin":
		return UninstallLaunchd()
	case "linux":
		return UninstallSystemd()
	default:
		return fmt.Errorf("service uninstall is not supported on %s", runtime.GOOS)
	}
}

func Start(port int, verbose bool) error {
	switch runtime.GOOS {
	case "darwin":
		if IsLaunchdInstalled() {
			return StartLaunchd()
		}
		return StartFallback(port, verbose)
	case "linux":
		if IsSystemdInstalled() {
			return StartSystemd()
		}
		return StartFallback(port, verbose)
	default:
		return StartFallback(port, verbose)
	}
}

func Stop() error {
	switch runtime.GOOS {
	case "darwin":
		if IsLaunchdInstalled() {
			return StopLaunchd()
		}
		return StopProcess()
	case "linux":
		if IsSystemdInstalled() {
			return StopSystemd()
		}
		return StopProcess()
	default:
		return StopProcess()
	}
}

func IsServiceInstalled() bool {
	switch runtime.GOOS {
	case "darwin":
		return IsLaunchdInstalled()
	case "linux":
		return IsSystemdInstalled()
	default:
		return false
	}
}
