package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSwitchCreate(t *testing.T) {
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(origDir) })
	_ = os.Chdir(dir)

	content := `version = "0.1"
default = "default"

[env.default]
output = ".env"
`
	_ = os.WriteFile(filepath.Join(dir, "enbu.toml"), []byte(content), 0o644)

	svc := &Service{}
	cmd := NewWithService("test", svc)
	cmd.SetArgs([]string{"switch", "-c", "staging"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("switch -c staging: %v", err)
	}

	data, _ := os.ReadFile(filepath.Join(dir, "enbu.toml"))
	if !strings.Contains(string(data), "staging") {
		t.Fatal("staging not in enbu.toml")
	}
	if !strings.Contains(string(data), `default = "staging"`) {
		t.Fatal("default not set to staging")
	}
}

func TestSwitchToExisting(t *testing.T) {
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(origDir) })
	_ = os.Chdir(dir)

	content := `version = "0.1"
default = "dev"

[env.dev]
output = ".env.dev"

[env.staging]
output = ".env.staging"
`
	_ = os.WriteFile(filepath.Join(dir, "enbu.toml"), []byte(content), 0o644)

	svc := &Service{}
	cmd := NewWithService("test", svc)
	cmd.SetArgs([]string{"switch", "staging"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("switch staging: %v", err)
	}

	data, _ := os.ReadFile(filepath.Join(dir, "enbu.toml"))
	if !strings.Contains(string(data), `default = "staging"`) {
		t.Fatal("default not set to staging")
	}
}

func TestSwitchNonExistent(t *testing.T) {
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(origDir) })
	_ = os.Chdir(dir)

	content := `version = "0.1"
default = "dev"

[env.dev]
output = ".env.dev"
`
	_ = os.WriteFile(filepath.Join(dir, "enbu.toml"), []byte(content), 0o644)

	svc := &Service{}
	cmd := NewWithService("test", svc)
	cmd.SetArgs([]string{"switch", "prod"})
	cmd.SilenceErrors = true
	cmd.SilenceUsage = true
	if err := cmd.Execute(); err == nil {
		t.Fatal("expected error for non-existent env")
	}
}

func TestSwitchPrevious(t *testing.T) {
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(origDir) })
	_ = os.Chdir(dir)

	content := `version = "0.1"
default = "dev"

[env.dev]
output = ".env.dev"

[env.staging]
output = ".env.staging"
`
	_ = os.WriteFile(filepath.Join(dir, "enbu.toml"), []byte(content), 0o644)

	svc := &Service{}

	cmd1 := NewWithService("test", svc)
	cmd1.SetArgs([]string{"switch", "staging"})
	if err := cmd1.Execute(); err != nil {
		t.Fatalf("switch staging: %v", err)
	}

	cmd2 := NewWithService("test", svc)
	cmd2.SetArgs([]string{"switch", "-"})
	if err := cmd2.Execute(); err != nil {
		t.Fatalf("switch -: %v", err)
	}

	data, _ := os.ReadFile(filepath.Join(dir, "enbu.toml"))
	if !strings.Contains(string(data), `default = "dev"`) {
		t.Fatalf("expected default to be dev, got: %s", data)
	}
}

func TestSwitchDelete(t *testing.T) {
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(origDir) })
	_ = os.Chdir(dir)

	content := `version = "0.1"
default = "dev"

[env.dev]
output = ".env.dev"

[env.staging]
output = ".env.staging"
`
	_ = os.WriteFile(filepath.Join(dir, "enbu.toml"), []byte(content), 0o644)

	svc := &Service{}
	cmd := NewWithService("test", svc)
	cmd.SetArgs([]string{"switch", "-d", "staging"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("switch -d staging: %v", err)
	}

	data, _ := os.ReadFile(filepath.Join(dir, "enbu.toml"))
	if strings.Contains(string(data), "staging") {
		t.Fatal("staging still in enbu.toml")
	}
}

func TestSwitchDeleteCurrentFails(t *testing.T) {
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(origDir) })
	_ = os.Chdir(dir)

	content := `version = "0.1"
default = "dev"

[env.dev]
output = ".env.dev"
`
	_ = os.WriteFile(filepath.Join(dir, "enbu.toml"), []byte(content), 0o644)

	svc := &Service{}
	cmd := NewWithService("test", svc)
	cmd.SetArgs([]string{"switch", "-d", "dev"})
	cmd.SilenceErrors = true
	cmd.SilenceUsage = true
	if err := cmd.Execute(); err == nil {
		t.Fatal("expected error when deleting current env")
	}
}

func TestSwitchList(t *testing.T) {
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(origDir) })
	_ = os.Chdir(dir)

	content := `version = "0.1"
default = "dev"

[env.dev]
output = ".env.dev"

[env.staging]
output = ".env.staging"
`
	_ = os.WriteFile(filepath.Join(dir, "enbu.toml"), []byte(content), 0o644)

	svc := &Service{}
	cmd := NewWithService("test", svc)
	cmd.SetArgs([]string{"switch", "--list"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("switch --list: %v", err)
	}
}

func TestSwitchMove(t *testing.T) {
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(origDir) })
	_ = os.Chdir(dir)

	content := `version = "0.1"
default = "dev"

[env.dev]
output = ".env.dev"
`
	_ = os.WriteFile(filepath.Join(dir, "enbu.toml"), []byte(content), 0o644)

	svc := &Service{}
	cmd := NewWithService("test", svc)
	cmd.SetArgs([]string{"switch", "-m", "dev", "development"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("switch -m dev development: %v", err)
	}

	data, _ := os.ReadFile(filepath.Join(dir, "enbu.toml"))
	if strings.Contains(string(data), "[env.dev]") {
		t.Fatal("dev still in enbu.toml")
	}
	if !strings.Contains(string(data), "development") {
		t.Fatal("development not in enbu.toml")
	}
	if !strings.Contains(string(data), `default = "development"`) {
		t.Fatal("default not updated to development")
	}
}
