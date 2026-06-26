package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseGitRemoteSSH(t *testing.T) {
	owner, repo, err := ParseGitRemote("git@github.com:yashikota/enbu.git")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if owner != "yashikota" || repo != "enbu" {
		t.Fatalf("got %s/%s, want yashikota/enbu", owner, repo)
	}
}

func TestParseGitRemoteHTTPS(t *testing.T) {
	owner, repo, err := ParseGitRemote("https://github.com/yashikota/enbu.git")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if owner != "yashikota" || repo != "enbu" {
		t.Fatalf("got %s/%s, want yashikota/enbu", owner, repo)
	}
}

func TestParseGitRemoteHTTPSNoSuffix(t *testing.T) {
	owner, repo, err := ParseGitRemote("https://github.com/yashikota/enbu")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if owner != "yashikota" || repo != "enbu" {
		t.Fatalf("got %s/%s, want yashikota/enbu", owner, repo)
	}
}

func TestParseGitRemoteSSHNoSuffix(t *testing.T) {
	owner, repo, err := ParseGitRemote("git@github.com:org/repo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if owner != "org" || repo != "repo" {
		t.Fatalf("got %s/%s, want org/repo", owner, repo)
	}
}

func TestParseGitRemoteInvalid(t *testing.T) {
	_, _, err := ParseGitRemote("not-a-url")
	if err == nil {
		t.Fatal("expected error for invalid URL")
	}
}

func TestParseGitRemoteInvalidSSH(t *testing.T) {
	_, _, err := ParseGitRemote("git@github.com")
	if err == nil {
		t.Fatal("expected error for invalid SSH URL without colon")
	}
}

func TestLoadProjectConfig(t *testing.T) {
	dir := t.TempDir()
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(origDir) })
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}

	content := `version = "0.1"` + "\n"
	if err := os.WriteFile(filepath.Join(dir, "enbu.toml"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadProject()
	if err != nil {
		t.Fatalf("LoadProject: %v", err)
	}
	if cfg.Version != "0.1" {
		t.Fatalf("got version %q, want %q", cfg.Version, "0.1")
	}
}

func TestLoadProjectConfigNotFound(t *testing.T) {
	dir := t.TempDir()
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(origDir) })
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}

	_, err = LoadProject()
	if err == nil {
		t.Fatal("expected error when enbu.toml not found")
	}
}

func TestSaveProject(t *testing.T) {
	dir := t.TempDir()
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(origDir) })
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}

	cfg := &ProjectConfig{Version: "0.1"}
	if err := SaveProject(cfg); err != nil {
		t.Fatalf("SaveProject: %v", err)
	}

	loaded, err := LoadProject()
	if err != nil {
		t.Fatalf("LoadProject after save: %v", err)
	}
	if loaded.Version != "0.1" {
		t.Fatalf("got version %q, want %q", loaded.Version, "0.1")
	}
}
