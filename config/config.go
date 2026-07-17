package config

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/BurntSushi/toml"
)

// ErrConfigNotFound is returned when enbu.toml does not exist.
// Callers that need to distinguish "file missing" from "file invalid"
// should use errors.As(err, &config.ErrConfigNotFound{}).
type ErrConfigNotFound struct{ msg string }

func (e ErrConfigNotFound) Error() string { return e.msg }

const currentVersion = "v1alpha1"

type ProjectConfig struct {
	Version      string                       `toml:"version"`
	DefaultEnv   string                       `toml:"default_env,omitempty"`
	Environments map[string]EnvironmentConfig `toml:"env,omitempty"`
}

type EnvironmentConfig struct {
	Output string `toml:"output"`
}

type LocalConfig struct {
	Previous string `toml:"previous,omitempty"`
}

func LoadProject() (*ProjectConfig, error) {
	path, err := findProjectConfig()
	if err != nil {
		return nil, ErrConfigNotFound{msg: err.Error()}
	}

	var cfg ProjectConfig
	if _, err := toml.DecodeFile(path, &cfg); err != nil {
		return nil, fmt.Errorf("parsing enbu.toml: %w", err)
	}
	if cfg.Version != currentVersion {
		return nil, fmt.Errorf("enbu.toml: unsupported version %q (run 'enbu init' to recreate)", cfg.Version)
	}
	return &cfg, nil
}

func SaveProject(cfg *ProjectConfig) error {
	path, err := findProjectConfig()
	if err != nil {
		path = "enbu.toml"
	}

	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("creating enbu.toml: %w", err)
	}

	encoder := toml.NewEncoder(f)
	if err := encoder.Encode(cfg); err != nil {
		_ = f.Close()
		return err
	}
	return f.Close()
}

func LoadLocal() (*LocalConfig, error) {
	path, err := findLocalConfig()
	if err != nil {
		return &LocalConfig{}, nil
	}

	var cfg LocalConfig
	if _, err := toml.DecodeFile(path, &cfg); err != nil {
		return &LocalConfig{}, nil
	}
	return &cfg, nil
}

func SaveLocal(cfg *LocalConfig) error {
	path, err := findLocalConfig()
	if err != nil {
		path = ".enbu.local"
	}

	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("creating .enbu.local: %w", err)
	}

	encoder := toml.NewEncoder(f)
	if err := encoder.Encode(cfg); err != nil {
		_ = f.Close()
		return err
	}
	return f.Close()
}

func findLocalConfig() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}

	for {
		path := filepath.Join(dir, ".enbu.local")
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	return "", fmt.Errorf(".enbu.local not found")
}

func NewProjectWithEnvironment(name string) *ProjectConfig {
	name = strings.TrimSpace(name)
	if name == "" {
		name = "default"
	}
	envs := map[string]EnvironmentConfig{
		name: {Output: DefaultOutput(name)},
	}
	return &ProjectConfig{
		Version:      currentVersion,
		DefaultEnv:   name,
		Environments: envs,
	}
}

func (cfg *ProjectConfig) CurrentEnvironment() string {
	if cfg.DefaultEnv != "" {
		return cfg.DefaultEnv
	}
	return "default"
}

func (cfg *ProjectConfig) SetDefault(name string) {
	cfg.DefaultEnv = name
}

func (cfg *ProjectConfig) AddEnvironment(name string) error {
	if !ValidEnvironmentName(name) {
		return fmt.Errorf("invalid environment name %q", name)
	}
	if cfg.Environments == nil {
		cfg.Environments = make(map[string]EnvironmentConfig)
	}
	if len(cfg.Environments) == 0 && cfg.DefaultEnv == "" {
		cfg.Environments["default"] = EnvironmentConfig{Output: DefaultOutput("default")}
		cfg.DefaultEnv = "default"
	}
	if _, exists := cfg.Environments[name]; exists {
		return fmt.Errorf("environment %q already exists", name)
	}
	cfg.Environments[name] = EnvironmentConfig{Output: DefaultOutput(name)}
	return nil
}

func (cfg *ProjectConfig) RemoveEnvironment(name string) error {
	if _, exists := cfg.Environments[name]; !exists {
		return fmt.Errorf("environment %q does not exist", name)
	}
	delete(cfg.Environments, name)
	if cfg.DefaultEnv == name {
		cfg.DefaultEnv = ""
	}
	return nil
}

func (cfg *ProjectConfig) RenameEnvironment(oldName, newName string) error {
	if !ValidEnvironmentName(newName) {
		return fmt.Errorf("invalid environment name %q", newName)
	}
	env, exists := cfg.Environments[oldName]
	if !exists {
		return fmt.Errorf("environment %q does not exist", oldName)
	}
	if _, exists := cfg.Environments[newName]; exists {
		return fmt.Errorf("environment %q already exists", newName)
	}
	delete(cfg.Environments, oldName)
	env.Output = DefaultOutput(newName)
	cfg.Environments[newName] = env
	if cfg.DefaultEnv == oldName {
		cfg.DefaultEnv = newName
	}
	return nil
}

func (cfg *ProjectConfig) HasEnvironment(name string) bool {
	_, exists := cfg.Environments[name]
	return exists
}

func (cfg *ProjectConfig) Environment(name string) (EnvironmentConfig, error) {
	if name == "" {
		name = cfg.CurrentEnvironment()
	}
	if !ValidEnvironmentName(name) {
		return EnvironmentConfig{}, fmt.Errorf("invalid environment %q", name)
	}
	if len(cfg.Environments) == 0 {
		if name != "default" {
			return EnvironmentConfig{}, fmt.Errorf("environment %q is not defined in enbu.toml", name)
		}
		return EnvironmentConfig{Output: DefaultOutput(name)}, nil
	}
	env, ok := cfg.Environments[name]
	if !ok {
		return EnvironmentConfig{}, fmt.Errorf("environment %q is not defined in enbu.toml", name)
	}
	if env.Output == "" {
		env.Output = DefaultOutput(name)
	}
	return env, nil
}

func (cfg *ProjectConfig) EnvironmentNames() []string {
	if len(cfg.Environments) == 0 {
		return []string{"default"}
	}
	names := make([]string, 0, len(cfg.Environments))
	for name := range cfg.Environments {
		names = append(names, name)
	}
	return names
}

func DefaultOutput(name string) string {
	if name == "" || name == "default" {
		return ".env"
	}
	return ".env." + name
}

func ValidEnvironmentName(name string) bool {
	if name == "" || len(name) > 100 {
		return false
	}
	for _, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' || r == '-' || r == '.' {
			continue
		}
		return false
	}
	return true
}

// ProjectConfigPath returns the path to enbu.toml by traversing up from cwd.
func ProjectConfigPath() (string, error) {
	return findProjectConfig()
}

func findProjectConfig() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}

	for {
		path := filepath.Join(dir, "enbu.toml")
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	return "", fmt.Errorf("enbu.toml not found (run 'enbu init' first)")
}

func ParseGitRemote(url string) (string, string, error) {
	url = strings.TrimSuffix(url, "/")
	if strings.HasPrefix(url, "git@") {
		parts := strings.SplitN(url, ":", 2)
		if len(parts) != 2 {
			return "", "", fmt.Errorf("invalid SSH remote: %s", url)
		}
		return splitOwnerRepo(strings.TrimSuffix(parts[1], ".git"))
	}

	url = strings.TrimSuffix(url, ".git")
	parts := strings.Split(url, "/")
	if len(parts) < 2 {
		return "", "", fmt.Errorf("invalid remote URL: %s", url)
	}
	return parts[len(parts)-2], parts[len(parts)-1], nil
}

func splitOwnerRepo(path string) (string, string, error) {
	parts := strings.SplitN(path, "/", 2)
	if len(parts) != 2 {
		return "", "", fmt.Errorf("invalid owner/repo: %s", path)
	}
	return parts[0], parts[1], nil
}

func DataDir() string {
	if xdg := os.Getenv("XDG_DATA_HOME"); xdg != "" {
		return filepath.Join(xdg, "enbu")
	}
	switch runtime.GOOS {
	case "darwin":
		return filepath.Join(os.Getenv("HOME"), "Library", "Application Support", "enbu")
	case "windows":
		return filepath.Join(os.Getenv("LOCALAPPDATA"), "enbu")
	default:
		return filepath.Join(os.Getenv("HOME"), ".local", "share", "enbu")
	}
}
