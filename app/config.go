package app

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/yashikota/enbu/config"
)

func (a *App) ReadConfig() (string, error) {
	path, err := config.ProjectConfigPathFrom(a.RepositoryDir)
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
	cfg, err := config.ParseProject(content)
	if err != nil {
		return err
	}
	if err := config.ValidateProjectOutputs(cfg); err != nil {
		return err
	}
	path, err := config.ProjectConfigPathFrom(a.RepositoryDir)
	if err != nil {
		path = filepath.Join(a.RepositoryDir, "enbu.toml")
	}
	return os.WriteFile(path, []byte(content), 0o644)
}
