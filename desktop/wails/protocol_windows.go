//go:build windows

package main

import (
	"fmt"
	"os"

	"golang.org/x/sys/windows/registry"
)

func registerProtocolHandler() error {
	executable, err := os.Executable()
	if err != nil {
		return err
	}
	protocolKey, _, err := registry.CreateKey(registry.CURRENT_USER, `Software\Classes\enbu`, registry.SET_VALUE)
	if err != nil {
		return fmt.Errorf("creating protocol key: %w", err)
	}
	defer protocolKey.Close()
	if err := protocolKey.SetStringValue("", "URL:enbu protocol"); err != nil {
		return fmt.Errorf("setting protocol description: %w", err)
	}
	if err := protocolKey.SetStringValue("URL Protocol", ""); err != nil {
		return fmt.Errorf("marking URL protocol: %w", err)
	}
	commandKey, _, err := registry.CreateKey(registry.CURRENT_USER, `Software\Classes\enbu\shell\open\command`, registry.SET_VALUE)
	if err != nil {
		return fmt.Errorf("creating protocol command key: %w", err)
	}
	defer commandKey.Close()
	if err := commandKey.SetStringValue("", fmt.Sprintf(`"%s" "%%1"`, executable)); err != nil {
		return fmt.Errorf("setting protocol command: %w", err)
	}
	return nil
}
