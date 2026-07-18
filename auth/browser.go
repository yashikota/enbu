package auth

import (
	"os/exec"
	"runtime"
)

func OpenBrowser(url string) error {
	return browserCommand(runtime.GOOS, url).Start()
}

func browserCommand(goos, url string) *exec.Cmd {
	var cmd *exec.Cmd
	switch goos {
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	return cmd
}
