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

func TestParseGitRemoteTrailingSlash(t *testing.T) {
	owner, repo, err := ParseGitRemote("https://github.com/yashikota/enbu/")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if owner != "yashikota" || repo != "enbu" {
		t.Fatalf("got %s/%s, want yashikota/enbu", owner, repo)
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
	env, err := cfg.Environment("default")
	if err != nil {
		t.Fatalf("Environment(default): %v", err)
	}
	if env.Output != ".env" {
		t.Fatalf("got output %q, want .env", env.Output)
	}
}

func TestLoadProjectConfigWithEnvironments(t *testing.T) {
	dir := t.TempDir()
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(origDir) })
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}

	content := `version = "0.1"

[env.dev]
output = ".env.dev"

[env.prod]
output = ".env.production"
`
	if err := os.WriteFile(filepath.Join(dir, "enbu.toml"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadProject()
	if err != nil {
		t.Fatalf("LoadProject: %v", err)
	}
	dev, err := cfg.Environment("dev")
	if err != nil {
		t.Fatalf("Environment(dev): %v", err)
	}
	if dev.Output != ".env.dev" {
		t.Fatalf("got dev output %q, want .env.dev", dev.Output)
	}
	prod, err := cfg.Environment("prod")
	if err != nil {
		t.Fatalf("Environment(prod): %v", err)
	}
	if prod.Output != ".env.production" {
		t.Fatalf("got prod output %q, want .env.production", prod.Output)
	}
	if _, err := cfg.Environment("stage"); err == nil {
		t.Fatal("expected undefined environment to fail")
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

	cfg := NewProjectWithEnvironment("dev")
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
	env, err := loaded.Environment("dev")
	if err != nil {
		t.Fatalf("Environment(dev): %v", err)
	}
	if env.Output != ".env.dev" {
		t.Fatalf("got output %q, want .env.dev", env.Output)
	}
	def, err := loaded.Environment("default")
	if err != nil {
		t.Fatalf("Environment(default): %v", err)
	}
	if def.Output != ".env" {
		t.Fatalf("got default output %q, want .env", def.Output)
	}
}

func TestNewProjectWithEnvironmentIncludesDefault(t *testing.T) {
	cfg := NewProjectWithEnvironment("dev")
	dev, err := cfg.Environment("dev")
	if err != nil {
		t.Fatalf("Environment(dev): %v", err)
	}
	if dev.Output != ".env.dev" {
		t.Fatalf("got dev output %q, want .env.dev", dev.Output)
	}
	def, err := cfg.Environment("default")
	if err != nil {
		t.Fatalf("Environment(default): %v", err)
	}
	if def.Output != ".env" {
		t.Fatalf("got default output %q, want .env", def.Output)
	}
}

func TestValidEnvironmentName(t *testing.T) {
	tests := []struct {
		name string
		want bool
	}{
		{"default", true},
		{"dev", true},
		{"stage-1", true},
		{"prod.us", true},
		{"", false},
		{"prod/us", false},
		{"prod us", false},
	}
	for _, tt := range tests {
		if got := ValidEnvironmentName(tt.name); got != tt.want {
			t.Errorf("ValidEnvironmentName(%q) = %v, want %v", tt.name, got, tt.want)
		}
	}
}
