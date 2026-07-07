package desktop

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/yashikota/enbu/config"
)

type SelectedRepo struct {
	Path  string `json:"path,omitempty"`
	Owner string `json:"owner,omitempty"`
	Repo  string `json:"repo,omitempty"`
}

func ValidateRepoPath(path string) (*SelectedRepo, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, fmt.Errorf("path is required")
	}

	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("resolving path: %w", err)
	}

	info, err := os.Stat(abs)
	if err != nil {
		return nil, fmt.Errorf("reading path: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("path must be a directory")
	}

	root, err := gitOutput(context.Background(), abs, "rev-parse", "--show-toplevel")
	if err != nil {
		return nil, fmt.Errorf("not a Git repository: %w", err)
	}

	remote, err := gitOutput(context.Background(), root, "remote", "get-url", "origin")
	if err != nil {
		return nil, fmt.Errorf("origin remote not found: %w", err)
	}

	owner, repo, err := config.ParseGitRemote(remote)
	if err != nil {
		return nil, err
	}

	return &SelectedRepo{Path: root, Owner: owner, Repo: repo}, nil
}

func gitOutput(ctx context.Context, dir string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("%s: %w", strings.TrimSpace(string(out)), err)
	}
	return strings.TrimSpace(string(out)), nil
}
