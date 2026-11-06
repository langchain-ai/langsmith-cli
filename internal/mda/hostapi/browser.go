package hostapi

import (
	"os/exec"
	"runtime"
)

// OpenBrowser sends the developer to url, ignoring failure: the URL is printed
// either way, and a machine without a browser is not an error.
func OpenBrowser(url string) {
	_ = TryOpenBrowser(url)
}

// TryOpenBrowser opens url in the default browser, reporting whether the
// opener succeeded.
func TryOpenBrowser(url string) bool {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	return cmd.Run() == nil
}
