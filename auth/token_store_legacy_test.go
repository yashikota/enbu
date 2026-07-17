package auth

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadTokenRejectsUnsupportedStoreFormat(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("XDG_DATA_HOME", t.TempDir())

	if err := os.MkdirAll(filepath.Dir(TokenPath()), 0o700); err != nil {
		t.Fatal(err)
	}
	data := []byte(`{"version":2,"active":"octo","accounts":{"octo":{"username":"octo","storage":"keychain"}}}`)
	if err := os.WriteFile(TokenPath(), data, 0o600); err != nil {
		t.Fatal(err)
	}

	_, err := LoadToken()
	if err == nil {
		t.Fatal("LoadToken returned no error for an unsupported token store")
	}
	if !strings.Contains(err.Error(), "unsupported format") || !strings.Contains(err.Error(), "enbu auth login") {
		t.Fatalf("unexpected error: %v", err)
	}
}
