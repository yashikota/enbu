package config

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/BurntSushi/toml"
)

type RepoConfig struct {
	Owner string
	Repo  string
}

type ProjectConfig struct {
	Version      string                       `toml:"version"`
	Environments map[string]EnvironmentConfig `toml:"env,omitempty"`
}

type EnvironmentConfig struct {
	Output string `toml:"output"`
}

func LoadRepo() (*RepoConfig, error) {
	owner, repo, err := detectRepo()
	if err != nil {
		return nil, fmt.Errorf("detecting repository: %w", err)
	}
	return &RepoConfig{Owner: owner, Repo: repo}, nil
}

func LoadProject() (*ProjectConfig, error) {
	path, err := findProjectConfig()
	if err != nil {
		return nil, err
	}

	var cfg ProjectConfig
	if _, err := toml.DecodeFile(path, &cfg); err != nil {
		return nil, fmt.Errorf("parsing enbu.toml: %w", err)
	}
	return &cfg, nil
}

func SaveProject(cfg *ProjectConfig) error {
	f, err := os.Create("enbu.toml")
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

func NewProjectWithEnvironment(name string) *ProjectConfig {
	name = strings.TrimSpace(name)
	if name == "" {
		name = "default"
	}
	envs := map[string]EnvironmentConfig{
		"default": {Output: DefaultOutput("default")},
	}
	envs[name] = EnvironmentConfig{Output: DefaultOutput(name)}
	return &ProjectConfig{
		Version:      "0.1",
		Environments: envs,
	}
}

func (cfg *ProjectConfig) Environment(name string) (EnvironmentConfig, error) {
	if name == "" {
		name = "default"
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

func detectRepo() (string, string, error) {
	out, err := exec.Command("git", "remote", "get-url", "origin").Output()
	if err != nil {
		return "", "", fmt.Errorf("git remote not found: %w", err)
	}
	return ParseGitRemote(strings.TrimSpace(string(out)))
}

func RepoRoot() (string, error) {
	out, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
	if err != nil {
		return "", fmt.Errorf("not in a git repository: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
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
