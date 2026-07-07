package web

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/yashikota/enbu/app"
)

func TestValidateRepoPath(t *testing.T) {
	repoDir := newGitRepo(t)

	repo, err := validateRepoPath(repoDir)
	if err != nil {
		t.Fatalf("validateRepoPath: %v", err)
	}
	if repo.Path != repoDir {
		t.Fatalf("Path = %q, want %q", repo.Path, repoDir)
	}
	if repo.Owner != "octo" || repo.Repo != "hello" {
		t.Fatalf("repo = %s/%s, want octo/hello", repo.Owner, repo.Repo)
	}
}

func TestValidateRepoPathRejectsInvalidPath(t *testing.T) {
	if _, err := validateRepoPath(filepath.Join(t.TempDir(), "missing")); err == nil {
		t.Fatal("validateRepoPath succeeded for missing path")
	}
}

func TestGUIRepoAPISelectsAndPersistsRepo(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	repoDir := newGitRepo(t)

	srv := NewServer(app.New(), "client-id", nil)
	reqBody, err := json.Marshal(map[string]string{"path": repoDir})
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/gui/repo", bytes.NewReader(reqBody))
	req.Header.Set("X-ENBU-Token", srv.CSRFToken())
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/api/gui/repo", nil)
	req.Header.Set("X-ENBU-Token", srv.CSRFToken())
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var resp struct {
		Data struct {
			Selected bool `json:"selected"`
			Repo     struct {
				Path  string `json:"path"`
				Owner string `json:"owner"`
				Repo  string `json:"repo"`
			} `json:"repo"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if !resp.Data.Selected || resp.Data.Repo.Path != repoDir {
		t.Fatalf("repo status = %+v, want selected %q", resp.Data, repoDir)
	}
}

func TestRepoScopedAPIRequiresSelectedRepo(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	srv := NewServer(app.New(), "client-id", nil)

	req := httptest.NewRequest(http.MethodGet, "/api/repo", nil)
	req.Header.Set("X-ENBU-Token", srv.CSRFToken())
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func newGitRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	runGit(t, dir, "init")
	runGit(t, dir, "remote", "add", "origin", "https://github.com/octo/hello.git")
	root := runGit(t, dir, "rev-parse", "--show-toplevel")
	return root
}

func runGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %s: %v", args, out, err)
	}
	return string(bytes.TrimSpace(out))
}
