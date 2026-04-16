package view

import (
	"os/exec"
	"runtime"
)

// openBrowser opens url in the user's default browser. Best-effort: any
// error is returned but never logged — caller decides whether to care.
// We keep this in its own file so the platform switch is isolated.
func openBrowser(url string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "linux":
		cmd = exec.Command("xdg-open", url)
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", url)
	default:
		return nil // unsupported; silent no-op
	}
	return cmd.Start()
}
