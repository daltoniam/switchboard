package daemon

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

const launchdLabel = "com.daltoniam.switchboard"

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

func xmlEscape(s string) string {
	var buf bytes.Buffer
	_ = xml.EscapeText(&buf, []byte(s))
	return buf.String()
}

func buildPlist(label, exePath string, port int, logPath string) string {
	return fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>%s</string>
    <key>ProgramArguments</key>
    <array>
        <string>%s</string>
        <string>--port</string>
        <string>%d</string>
    </array>
    <key>RunAtLoad</key>
    <true/>
    <key>KeepAlive</key>
    <true/>
    <key>StandardOutPath</key>
    <string>%s</string>
    <key>StandardErrorPath</key>
    <string>%s</string>
    <key>EnvironmentVariables</key>
    <dict>
        <key>PATH</key>
        <string>/usr/local/bin:/usr/bin:/bin:/opt/homebrew/bin</string>
        <key>SWITCHBOARD_DAEMON</key>
        <string>1</string>
    </dict>
</dict>
</plist>
`, xmlEscape(label), xmlEscape(exePath), port, xmlEscape(logPath), xmlEscape(logPath))
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

	content := buildPlist(launchdLabel, exe, port, logPath)

	if err := os.WriteFile(plistPath, []byte(content), 0600); err != nil {
		return fmt.Errorf("write plist file: %w", err)
	}

	fmt.Printf("Installed launchd service: %s\n", plistPath)
	return nil
}

func UninstallLaunchd() error {
	plistPath, err := launchdPlistPath()
	if err != nil {
		return err
	}

	_, _ = launchctl("unload", "-w", plistPath)

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

	if output, err := launchctl("load", "-w", plistPath); err != nil {
		return fmt.Errorf("launchctl load: %s (%w)", string(output), err)
	}
	return nil
}

func StopLaunchd() error {
	plistPath, err := launchdPlistPath()
	if err != nil {
		return err
	}

	if output, err := launchctl("unload", plistPath); err != nil {
		return fmt.Errorf("launchctl unload: %s (%w)", string(output), err)
	}
	return nil
}

func launchctl(args ...string) ([]byte, error) {
	return exec.Command("launchctl", args...).CombinedOutput() // #nosec G204
}

func IsLaunchdInstalled() bool {
	plistPath, err := launchdPlistPath()
	if err != nil {
		return false
	}
	_, err = os.Stat(plistPath)
	return err == nil
}
