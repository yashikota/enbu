package web

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/yashikota/enbu/config"
	"github.com/yashikota/enbu/utils/process"
)

type selectedRepo struct {
	Path  string `json:"path,omitempty"`
	Owner string `json:"owner,omitempty"`
	Repo  string `json:"repo,omitempty"`
}

func (s *Server) loadSelectedRepo() {
	cfg, err := config.LoadGUI()
	if err != nil || strings.TrimSpace(cfg.SelectedRepo) == "" {
		return
	}

	repo, err := validateRepoPath(cfg.SelectedRepo)
	if err != nil {
		return
	}
	s.selectedRepo = repo.Path
}

func (s *Server) handleGUIRepoStatus(w http.ResponseWriter, r *http.Request) {
	repo, err := s.currentRepo()
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{
			"selected": false,
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"selected": true,
		"repo":     repo,
	})
}

func (s *Server) handleSelectGUIRepo(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Path string `json:"path"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_BODY", "invalid request body")
		return
	}

	repo, err := validateRepoPath(req.Path)
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REPO", err.Error())
		return
	}

	if err := config.SaveGUI(&config.GUIConfig{SelectedRepo: repo.Path}); err != nil {
		writeError(w, http.StatusInternalServerError, "GUI_CONFIG_SAVE_ERROR", err.Error())
		return
	}

	s.repoMu.Lock()
	s.selectedRepo = repo.Path
	s.repoMu.Unlock()

	writeJSON(w, http.StatusOK, map[string]any{
		"selected": true,
		"repo":     repo,
	})
}

func (s *Server) currentRepo() (*selectedRepo, error) {
	s.repoMu.Lock()
	path := s.selectedRepo
	s.repoMu.Unlock()
	if path == "" {
		return nil, fmt.Errorf("no repository selected")
	}
	return validateRepoPath(path)
}

func (s *Server) withSelectedRepo(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		s.repoMu.Lock()
		defer s.repoMu.Unlock()

		if s.selectedRepo == "" {
			writeError(w, http.StatusBadRequest, "NO_REPO_SELECTED", "select a Git repository first")
			return
		}

		wd, err := os.Getwd()
		if err != nil {
			writeError(w, http.StatusInternalServerError, "GETWD_ERROR", err.Error())
			return
		}
		if err := os.Chdir(s.selectedRepo); err != nil {
			writeError(w, http.StatusBadRequest, "REPO_CHDIR_ERROR", err.Error())
			return
		}
		defer func() {
			_ = os.Chdir(wd)
		}()

		next(w, r)
	}
}

func validateRepoPath(path string) (*selectedRepo, error) {
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

	return &selectedRepo{
		Path:  root,
		Owner: owner,
		Repo:  repo,
	}, nil
}

func gitOutput(ctx context.Context, dir string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	process.HideWindow(cmd)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("%s: %w", strings.TrimSpace(string(out)), err)
	}
	return strings.TrimSpace(string(out)), nil
}
