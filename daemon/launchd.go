package daemon

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"text/template"
)

const launchdLabel = "com.daltoniam.switchboard"

const launchdPlistTemplate = `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>{{ .Label }}</string>
    <key>ProgramArguments</key>
    <array>
        <string>{{ .ExePath }}</string>
        <string>--port</string>
        <string>{{ .Port }}</string>
    </array>
    <key>RunAtLoad</key>
    <true/>
    <key>KeepAlive</key>
    <true/>
    <key>StandardOutPath</key>
    <string>{{ .LogPath }}</string>
    <key>StandardErrorPath</key>
    <string>{{ .LogPath }}</string>
    <key>EnvironmentVariables</key>
    <dict>
        <key>PATH</key>
        <string>/usr/local/bin:/usr/bin:/bin:/opt/homebrew/bin</string>
    </dict>
</dict>
</plist>
`

type launchdData struct {
	Label   string
	ExePath string
	Port    int
	LogPath string
}

var launchdPlistPathFunc = defaultLaunchdPlistPath

func launchdPlistPath() (string, error) {
	return launchdPlistPathFunc()
}

func defaultLaunchdPlistPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("get home dir: %w", err)
	}
	return filepath.Join(home, "Library", "LaunchAgents", launchdLabel+".plist"), nil
}

func InstallLaunchd(port int) error {
	exe, err := ExePath()
	if err != nil {
		return err
	}

	logPath, err := LogPath()
	if err != nil {
		return err
	}

	plistPath, err := launchdPlistPath()
	if err != nil {
		return err
	}

	dir := filepath.Dir(plistPath)
	if err := os.MkdirAll(dir, 0750); err != nil {
		return fmt.Errorf("create LaunchAgents dir: %w", err)
	}

	data := launchdData{
		Label:   launchdLabel,
		ExePath: exe,
		Port:    port,
		LogPath: logPath,
	}

	tmpl, err := template.New("plist").Parse(launchdPlistTemplate)
	if err != nil {
		return fmt.Errorf("parse plist template: %w", err)
	}

	f, err := os.OpenFile(plistPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return fmt.Errorf("create plist file: %w", err)
	}
	defer func() { _ = f.Close() }()

	if err := tmpl.Execute(f, data); err != nil {
		return fmt.Errorf("write plist: %w", err)
	}

	fmt.Printf("Installed launchd service: %s\n", plistPath)
	return nil
}

func UninstallLaunchd() error {
	plistPath, err := launchdPlistPath()
	if err != nil {
		return err
	}

	_ = exec.Command("launchctl", "bootout", fmt.Sprintf("gui/%d", os.Getuid()), plistPath).Run() // #nosec G204

	if err := os.Remove(plistPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove plist: %w", err)
	}

	fmt.Printf("Uninstalled launchd service: %s\n", plistPath)
	return nil
}

func StartLaunchd() error {
	plistPath, err := launchdPlistPath()
	if err != nil {
		return err
	}

	if _, err := os.Stat(plistPath); os.IsNotExist(err) {
		return fmt.Errorf("service not installed — run 'switchboard daemon install' first")
	}

	cmd := exec.Command("launchctl", "bootstrap", fmt.Sprintf("gui/%d", os.Getuid()), plistPath) // #nosec G204
	if output, err := cmd.CombinedOutput(); err != nil {
		outStr := string(output)
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 37 {
			return fmt.Errorf("service already loaded")
		}
		return fmt.Errorf("launchctl bootstrap: %s (%w)", outStr, err)
	}
	return nil
}

func StopLaunchd() error {
	plistPath, err := launchdPlistPath()
	if err != nil {
		return err
	}

	cmd := exec.Command("launchctl", "bootout", fmt.Sprintf("gui/%d", os.Getuid()), plistPath) // #nosec G204
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("launchctl bootout: %s (%w)", string(output), err)
	}
	return nil
}

func IsLaunchdInstalled() bool {
	plistPath, err := launchdPlistPath()
	if err != nil {
		return false
	}
	_, err = os.Stat(plistPath)
	return err == nil
}
