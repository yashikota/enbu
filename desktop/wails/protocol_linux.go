//go:build linux

package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func registerProtocolHandler() error {
	dataDir, err := linuxUserDataDir()
	if err != nil {
		return err
	}
	executable, err := os.Executable()
	if err != nil {
		return err
	}
	return registerLinuxProtocolHandler(dataDir, executable, func() error {
		output, err := exec.Command("xdg-mime", "default", "enbu.desktop", "x-scheme-handler/enbu").CombinedOutput()
		if err != nil {
			return fmt.Errorf("setting default URI handler: %w: %s", err, strings.TrimSpace(string(output)))
		}
		return nil
	})
}

func linuxUserDataDir() (string, error) {
	if dataDir := os.Getenv("XDG_DATA_HOME"); filepath.IsAbs(dataDir) {
		return dataDir, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".local", "share"), nil
}

func registerLinuxProtocolHandler(dataDir, executable string, setDefault func() error) error {
	applicationsDir := filepath.Join(dataDir, "applications")
	if err := os.MkdirAll(applicationsDir, 0o700); err != nil {
		return fmt.Errorf("creating applications directory: %w", err)
	}
	entry := fmt.Sprintf("[Desktop Entry]\nType=Application\nName=enbu\nNoDisplay=true\nTerminal=false\nExec=%s %%u\nMimeType=x-scheme-handler/enbu;\n", quoteDesktopExec(executable))
	if err := os.WriteFile(filepath.Join(applicationsDir, "enbu.desktop"), []byte(entry), 0o600); err != nil {
		return fmt.Errorf("writing desktop entry: %w", err)
	}
	return setDefault()
}

func quoteDesktopExec(value string) string {
	replacer := strings.NewReplacer(
		`\`, `\\`,
		`"`, `\"`,
		"`", "\\`",
		"$", `\$`,
		"%", "%%",
	)
	return `"` + replacer.Replace(value) + `"`
}
