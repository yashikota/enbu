package config

import (
	"path/filepath"
	"testing"
)

func TestLoadGUIMissing(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())

	cfg, err := LoadGUI()
	if err != nil {
		t.Fatalf("LoadGUI: %v", err)
	}
	if cfg.SelectedRepo != "" {
		t.Fatalf("SelectedRepo = %q, want empty", cfg.SelectedRepo)
	}
}

func TestSaveAndLoadGUI(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	want := filepath.Join(t.TempDir(), "repo")

	if err := SaveGUI(&GUIConfig{SelectedRepo: want}); err != nil {
		t.Fatalf("SaveGUI: %v", err)
	}

	cfg, err := LoadGUI()
	if err != nil {
		t.Fatalf("LoadGUI: %v", err)
	}
	if cfg.SelectedRepo != want {
		t.Fatalf("SelectedRepo = %q, want %q", cfg.SelectedRepo, want)
	}
}
