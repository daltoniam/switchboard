//go:build !cgo && !windows

package onepassword

// The SDK loads the 1Password app's library through cgo on macOS and Linux,
// so a CGO_ENABLED=0 build cannot sign in through the desktop app.
const desktopAppAvailable = false
