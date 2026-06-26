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
	Version string `toml:"version"`
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
