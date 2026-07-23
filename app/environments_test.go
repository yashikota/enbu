package app

import (
	"errors"
	"os"
	"strings"
	"testing"
)

type failingRepoDetector struct {
	err error
}

func (d *failingRepoDetector) LoadRepo() (string, string, error) {
	return "", "", d.err
}

// SwitchEnvironment should not panic when .enbu.local does not exist
func TestSwitchEnvironmentWithoutLocalFile(t *testing.T) {
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(origDir) })
	_ = os.Chdir(dir)

	cfg := `version = "v1alpha1"
default_env = "dev"

[env.dev]
output = ".env.dev"

[env.staging]
output = ".env.staging"
`
	if err := os.WriteFile("enbu.toml", []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}

	a := &App{}
	if err := a.SwitchEnvironment("staging"); err != nil {
		t.Fatalf("SwitchEnvironment: %v", err)
	}
}

func TestDeleteEnvironmentWarnsWhenCacheReferenceCannotBeResolved(t *testing.T) {
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(origDir) })
	_ = os.Chdir(dir)

	cfg := `version = "v1alpha1"
default_env = "dev"

[env.dev]
output = ".env.dev"

[env.staging]
output = ".env.staging"
`
	if err := os.WriteFile("enbu.toml", []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}

	events := &recordingEvents{}
	a := &App{
		RepoDetector: &failingRepoDetector{err: errors.New("remote unavailable")},
		Events:       events,
	}
	if err := a.DeleteEnvironment("staging"); err != nil {
		t.Fatalf("DeleteEnvironment: %v", err)
	}
	if len(events.messages) != 1 || !strings.Contains(events.messages[0], "remote unavailable") {
		t.Fatalf("warnings = %q, want cache cleanup warning", events.messages)
	}
}
