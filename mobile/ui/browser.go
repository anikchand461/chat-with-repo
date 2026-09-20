package ui

import (
	"fmt"
	"os/exec"
	"runtime"
)

// openURL opens rawURL in the system's default browser, so tapping
// "Upgrade to Pro" lands the user on the same hosted payment page the
// web frontend redirects to (see js/app.js: startCheckout /
// upgradeToPro, POST /payment/checkout -> data.checkout_url).
//
// Gio has no built-in "open link" primitive, so this shells out to
// the platform's standard opener. It covers desktop (used for local
// testing per README) and makes a best-effort attempt on Android via
// an intent; callers should treat a non-nil error as "show the link
// as text instead" rather than fatal.
func openURL(rawURL string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", rawURL)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", rawURL)
	case "linux":
		cmd = exec.Command("xdg-open", rawURL)
	case "android":
		return openURLAndroid(rawURL)
	default:
		return fmt.Errorf("open browser: unsupported platform %q", runtime.GOOS)
	}
	return cmd.Start()
}
