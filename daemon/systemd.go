package daemon

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"text/template"
)

const systemdUnitName = "switchboard.service"

const systemdUnitTemplate = `[Unit]
Description=Switchboard MCP Server
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
ExecStart={{ .ExePath }} --port {{ .Port }}
Restart=on-failure
RestartSec=5
StandardOutput=append:{{ .LogPath }}
StandardError=append:{{ .LogPath }}
Environment=PATH=/usr/local/bin:/usr/bin:/bin
Environment=SWITCHBOARD_DAEMON=1

[Install]
WantedBy=default.target
`

type systemdData struct {
	ExePath string
	Port    int
	LogPath string
}

var systemdUnitPathFunc = defaultSystemdUnitPath

var execCommand = defaultExecCommand

func defaultExecCommand(name string, args ...string) ([]byte, error) {
	return exec.Command(name, args...).CombinedOutput() // #nosec G204
}

func systemdUnitPath() (string, error) {
	return systemdUnitPathFunc()
}

func defaultSystemdUnitPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("get home dir: %w", err)
	}
	return filepath.Join(home, ".config", "systemd", "user", systemdUnitName), nil
}

func InstallSystemd(port int) error {
	exe, err := ExePath()
	if err != nil {
		return err
	}

	logPath, err := LogPath()
	if err != nil {
		return err
	}

	unitPath, err := systemdUnitPath()
	if err != nil {
		return err
	}

	dir := filepath.Dir(unitPath)
	if err := os.MkdirAll(dir, 0750); err != nil {
		return fmt.Errorf("create systemd user dir: %w", err)
	}

	data := systemdData{
		ExePath: exe,
		Port:    port,
		LogPath: logPath,
	}

	tmpl, err := template.New("unit").Parse(systemdUnitTemplate)
	if err != nil {
		return fmt.Errorf("parse unit template: %w", err)
	}

	f, err := os.OpenFile(unitPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return fmt.Errorf("create unit file: %w", err)
	}
	defer func() { _ = f.Close() }()

	if err := tmpl.Execute(f, data); err != nil {
		return fmt.Errorf("write unit file: %w", err)
	}

	if output, err := execCommand("systemctl", "--user", "daemon-reload"); err != nil {
		return fmt.Errorf("systemctl daemon-reload: %s (%w)", string(output), err)
	}

	if output, err := execCommand("systemctl", "--user", "enable", systemdUnitName); err != nil {
		return fmt.Errorf("systemctl enable: %s (%w)", string(output), err)
	}

	fmt.Printf("Installed systemd user service: %s\n", unitPath)
	return nil
}

func UninstallSystemd() error {
	unitPath, err := systemdUnitPath()
	if err != nil {
		return err
	}

	_, _ = execCommand("systemctl", "--user", "stop", systemdUnitName)
	_, _ = execCommand("systemctl", "--user", "disable", systemdUnitName)

	if err := os.Remove(unitPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove unit file: %w", err)
	}

	_, _ = execCommand("systemctl", "--user", "daemon-reload")

	fmt.Printf("Uninstalled systemd user service: %s\n", unitPath)
	return nil
}

func StartSystemd() error {
	unitPath, err := systemdUnitPath()
	if err != nil {
		return err
	}

	if _, err := os.Stat(unitPath); os.IsNotExist(err) {
		return fmt.Errorf("service not installed — run 'switchboard daemon install' first")
	}

	if output, err := execCommand("systemctl", "--user", "start", systemdUnitName); err != nil {
		return fmt.Errorf("systemctl start: %s (%w)", string(output), err)
	}
	return nil
}

func StopSystemd() error {
	if output, err := execCommand("systemctl", "--user", "stop", systemdUnitName); err != nil {
		return fmt.Errorf("systemctl stop: %s (%w)", string(output), err)
	}
	return nil
}

func IsSystemdInstalled() bool {
	unitPath, err := systemdUnitPath()
	if err != nil {
		return false
	}
	_, err = os.Stat(unitPath)
	return err == nil
}
