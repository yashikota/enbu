//go:build linux

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRegisterLinuxProtocolHandler(t *testing.T) {
	dataDir := t.TempDir()
	called := false
	if err := registerLinuxProtocolHandler(dataDir, `/opt/enbu build/enbu%desktop`, func() error {
		called = true
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if !called {
		t.Fatal("default URI handler was not set")
	}
	data, err := os.ReadFile(filepath.Join(dataDir, "applications", "enbu.desktop"))
	if err != nil {
		t.Fatal(err)
	}
	entry := string(data)
	for _, want := range []string{
		`Exec="/opt/enbu build/enbu%%desktop" %u`,
		`MimeType=x-scheme-handler/enbu;`,
	} {
		if !strings.Contains(entry, want) {
			t.Fatalf("desktop entry does not contain %q:\n%s", want, entry)
		}
	}
}
