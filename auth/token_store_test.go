package auth

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/zalando/go-keyring"
)

func stubKeyring(t *testing.T) *string {
	t.Helper()
	originalSet, originalGet, originalDelete := keyringSet, keyringGet, keyringDelete
	var value string
	present := false
	keyringSet = func(_, _, secret string) error {
		value, present = secret, true
		return nil
	}
	keyringGet = func(_, _ string) (string, error) {
		if !present {
			return "", keyring.ErrNotFound
		}
		return value, nil
	}
	keyringDelete = func(_, _ string) error {
		if !present {
			return keyring.ErrNotFound
		}
		present = false
		return nil
	}
	t.Cleanup(func() {
		keyringSet, keyringGet, keyringDelete = originalSet, originalGet, originalDelete
	})
	return &value
}

func TestTokenStoreUsesKeyringAndRemovesLegacyFile(t *testing.T) {
	dataDir := t.TempDir()
	t.Setenv("XDG_DATA_HOME", dataDir)
	t.Setenv("ENBU_BACKEND", "text")
	stubKeyring(t)
	legacy := filepath.Join(dataDir, "enbu", "token.json")
	if err := os.MkdirAll(filepath.Dir(legacy), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(legacy, []byte("legacy-secret"), 0o600); err != nil {
		t.Fatal(err)
	}

	want := &StoredToken{AccessToken: "token", Username: "octo", UserID: 123}
	if err := SaveToken(want); err != nil {
		t.Fatalf("SaveToken: %v", err)
	}
	if _, err := os.Stat(legacy); !os.IsNotExist(err) {
		t.Fatalf("legacy token still exists: %v", err)
	}
	got, err := LoadToken()
	if err != nil || *got != *want {
		t.Fatalf("LoadToken = %#v, %v", got, err)
	}
	renamed := &StoredToken{AccessToken: "new-token", Username: "renamed-octo", UserID: want.UserID}
	if err := SaveToken(renamed); err != nil {
		t.Fatalf("SaveToken after login rename: %v", err)
	}
	got, err = LoadToken()
	if err != nil || *got != *renamed {
		t.Fatalf("LoadToken after login rename = %#v, %v", got, err)
	}
	if err := DeleteToken(); err != nil {
		t.Fatalf("DeleteToken: %v", err)
	}
	if _, err := LoadToken(); err == nil {
		t.Fatal("LoadToken succeeded after deletion")
	}
}

func TestTokenStoreDoesNotFallbackWhenKeyringFails(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	originalSet := keyringSet
	keyringSet = func(_, _, _ string) error { return errors.New("unavailable") }
	t.Cleanup(func() { keyringSet = originalSet })
	err := SaveToken(&StoredToken{AccessToken: "token", Username: "octo", UserID: 123})
	if err == nil || !strings.Contains(err.Error(), "OS keyring") {
		t.Fatalf("SaveToken error = %v", err)
	}
}

func TestTokenStoreReportsLegacyCleanupFailure(t *testing.T) {
	dataDir := t.TempDir()
	t.Setenv("XDG_DATA_HOME", dataDir)
	stubKeyring(t)
	legacy := filepath.Join(dataDir, "enbu", "token.json")
	if err := os.MkdirAll(legacy, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(legacy, "keep"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	err := SaveToken(&StoredToken{AccessToken: "token", Username: "octo", UserID: 123})
	if err == nil || !strings.Contains(err.Error(), "legacy token") {
		t.Fatalf("SaveToken error = %v", err)
	}
	if _, err := LoadToken(); err != nil {
		t.Fatalf("credential was not retained in keyring: %v", err)
	}
}

func TestGitHubTokenIsEphemeral(t *testing.T) {
	stubKeyring(t)
	t.Setenv("GITHUB_TOKEN", "ci-token")
	t.Setenv("GITHUB_ACTOR", "ci-user")
	token, err := LoadToken()
	if err != nil || token.AccessToken != "ci-token" || token.Username != "ci-user" {
		t.Fatalf("LoadToken = %#v, %v", token, err)
	}
	if _, err := keyringGet(tokenKeyringService, tokenKeyringAccount); !errors.Is(err, keyring.ErrNotFound) {
		t.Fatal("GITHUB_TOKEN was persisted")
	}
}

func TestDeleteTokenRemovesLegacyFileEvenWhenKeyringFails(t *testing.T) {
	dataDir := t.TempDir()
	t.Setenv("XDG_DATA_HOME", dataDir)
	legacy := filepath.Join(dataDir, "enbu", "token.json")
	if err := os.MkdirAll(filepath.Dir(legacy), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(legacy, []byte("legacy-secret"), 0o600); err != nil {
		t.Fatal(err)
	}

	originalDelete := keyringDelete
	keyringDelete = func(_, _ string) error { return errors.New("keyring unavailable") }
	t.Cleanup(func() { keyringDelete = originalDelete })
	if err := DeleteToken(); err == nil || !strings.Contains(err.Error(), "OS keyring") {
		t.Fatalf("DeleteToken error = %v", err)
	}
	if _, err := os.Stat(legacy); !os.IsNotExist(err) {
		t.Fatalf("legacy token still exists: %v", err)
	}
}
