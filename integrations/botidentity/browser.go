package botidentity

import (
	"os/exec"
	"runtime"
)

func openBrowser(url string) error {
	switch runtime.GOOS {
	case "darwin":
		return exec.Command("open", url).Start() // #nosec G204 -- url is always a localhost URL we construct
	case "windows":
		return exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start() // #nosec G204 -- url is always a localhost URL we construct
	default:
		return exec.Command("xdg-open", url).Start() // #nosec G204 -- url is always a localhost URL we construct
	}
}
