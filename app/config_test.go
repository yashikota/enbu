package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadWriteConfig(t *testing.T) {
	dir := t.TempDir()
	tomlPath := filepath.Join(dir, "enbu.toml")
	want := "version = \"v1alpha1\"\ndefault_env = \"default\"\n"
	if err := os.WriteFile(tomlPath, []byte(want), 0o644); err != nil {
		t.Fatal(err)
	}

	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(orig) }()

	a := &App{}

	got, err := a.ReadConfig()
	if err != nil {
		t.Fatalf("ReadConfig: %v", err)
	}
	if got != want {
		t.Fatalf("ReadConfig = %q, want %q", got, want)
	}

	newContent := "version = \"v1alpha1\"\ndefault_env = \"production\"\n"
	if err := a.WriteConfig(newContent); err != nil {
		t.Fatalf("WriteConfig: %v", err)
	}

	data, err := os.ReadFile(tomlPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "production") {
		t.Fatalf("written config = %q, expected to contain production", string(data))
	}
}

func TestReadConfigMissing(t *testing.T) {
	dir := t.TempDir()
	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(orig) }()

	a := &App{}
	_, err = a.ReadConfig()
	if err == nil {
		t.Fatal("ReadConfig succeeded for missing enbu.toml, want error")
	}
}
