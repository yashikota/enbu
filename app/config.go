package app

import (
	"fmt"
	"os"

	"github.com/yashikota/enbu/config"
)

func (a *App) ReadConfig() (string, error) {
	path, err := config.ProjectConfigPath()
	if err != nil {
		return "", fmt.Errorf("enbu.toml not found: %w", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("reading enbu.toml: %w", err)
	}
	return string(data), nil
}

func (a *App) WriteConfig(content string) error {
	path, err := config.ProjectConfigPath()
	if err != nil {
		path = "enbu.toml"
	}
	return os.WriteFile(path, []byte(content), 0o644)
}
