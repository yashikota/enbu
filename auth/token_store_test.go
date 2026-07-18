package auth

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yashikota/enbu/config"
	"github.com/yashikota/enbu/utils/keystore"
)

type fakeTokenBackend struct {
	values    map[string][]byte
	storeErr  error
	loadErr   error
	deleteErr error
}

func (b *fakeTokenBackend) Store(service, key string, secret []byte) error {
	if b.storeErr != nil {
		return b.storeErr
	}
	if b.values == nil {
		b.values = make(map[string][]byte)
	}
	b.values[service+"/"+key] = append([]byte(nil), secret...)
	return nil
}

func (b *fakeTokenBackend) Load(service, key string) ([]byte, error) {
	if b.loadErr != nil {
		return nil, b.loadErr
	}
	value, ok := b.values[service+"/"+key]
	if !ok {
		return nil, keystore.ErrNotFound
	}
	return append([]byte(nil), value...), nil
}

func (b *fakeTokenBackend) Delete(service, key string) error {
	if b.deleteErr != nil {
		return b.deleteErr
	}
	if _, ok := b.values[service+"/"+key]; !ok {
		return keystore.ErrNotFound
	}
	delete(b.values, service+"/"+key)
	return nil
}

func setupTokenStoreTest(t *testing.T) *fakeTokenBackend {
	t.Helper()
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	t.Setenv("GH_TOKEN", "")
	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("GITHUB_ACTOR", "")
	backend := &fakeTokenBackend{values: make(map[string][]byte)}
	previous := newTokenBackend
	newTokenBackend = func() keystore.Backend { return backend }
	t.Cleanup(func() { newTokenBackend = previous })
	return backend
}

func TestTokenStoreSavesAndSwitchesMultipleAccounts(t *testing.T) {
	backend := setupTokenStoreTest(t)
	alice, err := SaveTokenWithAccount(&StoredToken{AccessToken: "alice-token", Username: "Alice"})
	if err != nil {
		t.Fatal(err)
	}
	if alice.ID != "stored:alice" || alice.Storage != storageKeyring || !alice.Active {
		t.Fatalf("alice = %#v", alice)
	}
	if err := SaveToken(&StoredToken{AccessToken: "bob-token", Username: "bob"}); err != nil {
		t.Fatal(err)
	}

	accounts, err := ListAccounts()
	if err != nil {
		t.Fatal(err)
	}
	if len(accounts) != 2 || accounts[0].Username != "Alice" || accounts[0].Active || !accounts[1].Active {
		t.Fatalf("accounts = %#v", accounts)
	}
	if strings.Contains(readTokenStoreFile(t), "alice-token") || strings.Contains(readTokenStoreFile(t), "bob-token") {
		t.Fatal("keychain-backed access tokens were written to token.json")
	}
	if got := string(backend.values[credentialService+"/alice"]); got != "alice-token" {
		t.Fatalf("keychain alice token = %q", got)
	}
	if err := SwitchAccount("stored:ALICE"); err != nil {
		t.Fatal(err)
	}
	token, err := LoadToken()
	if err != nil {
		t.Fatal(err)
	}
	if token.Username != "Alice" || token.AccessToken != "alice-token" {
		t.Fatalf("token = %#v", token)
	}
}

func TestTokenStoreFallsBackToProtectedFile(t *testing.T) {
	backend := setupTokenStoreTest(t)
	backend.storeErr = errors.New("keychain unavailable")

	account, err := SaveTokenWithAccount(&StoredToken{AccessToken: "fallback-token", Username: "fallback"})
	if err != nil {
		t.Fatal(err)
	}
	if account.Storage != storageFile {
		t.Fatalf("storage = %q, want %q", account.Storage, storageFile)
	}
	if !strings.Contains(readTokenStoreFile(t), "fallback-token") {
		t.Fatal("fallback access token was not written to token.json")
	}
	token, err := LoadToken()
	if err != nil || token.AccessToken != "fallback-token" {
		t.Fatalf("LoadToken() = %#v, %v", token, err)
	}
}

func TestTokenStoreMigratesLegacySingleToken(t *testing.T) {
	backend := setupTokenStoreTest(t)
	writeTokenStoreFile(t, []byte(`{"access_token":"legacy-token","username":"legacy"}`))

	token, err := LoadToken()
	if err != nil || token.Username != "legacy" || token.AccessToken != "legacy-token" {
		t.Fatalf("LoadToken() = %#v, %v", token, err)
	}
	if got := string(backend.values[credentialService+"/legacy"]); got != "legacy-token" {
		t.Fatalf("migrated keychain token = %q", got)
	}
	var store tokenStore
	if err := json.Unmarshal([]byte(readTokenStoreFile(t)), &store); err != nil {
		t.Fatal(err)
	}
	if store.Version != tokenStoreVersion || store.Active != "legacy" {
		t.Fatalf("store = %#v", store)
	}
}

func TestTokenStoreMigratesInterimMultipleAccountFormat(t *testing.T) {
	setupTokenStoreTest(t)
	writeTokenStoreFile(t, []byte(`{"active":"Bob","accounts":{"Alice":{"access_token":"alice-token","username":"Alice"},"Bob":{"access_token":"bob-token","username":"Bob"}}}`))

	accounts, err := ListAccounts()
	if err != nil {
		t.Fatal(err)
	}
	if len(accounts) != 2 || accounts[0].Username != "Alice" || accounts[0].Active || accounts[1].Username != "Bob" || !accounts[1].Active {
		t.Fatalf("accounts = %#v", accounts)
	}
}

func TestDeletingActiveAccountSwitchesToRemainingAccount(t *testing.T) {
	setupTokenStoreTest(t)
	if err := SaveToken(&StoredToken{AccessToken: "alice-token", Username: "alice"}); err != nil {
		t.Fatal(err)
	}
	if err := SaveToken(&StoredToken{AccessToken: "bob-token", Username: "bob"}); err != nil {
		t.Fatal(err)
	}
	if err := DeleteToken(); err != nil {
		t.Fatal(err)
	}
	token, err := LoadToken()
	if err != nil || token.Username != "alice" {
		t.Fatalf("LoadToken() = %#v, %v", token, err)
	}
	accounts, err := ListAccounts()
	if err != nil {
		t.Fatal(err)
	}
	if len(accounts) != 1 || accounts[0].Username != "alice" || !accounts[0].Active {
		t.Fatalf("accounts = %#v", accounts)
	}
}

func TestEnvironmentAccountTakesPrecedenceAndBlocksStoredAccountChanges(t *testing.T) {
	setupTokenStoreTest(t)
	t.Setenv("GITHUB_TOKEN", "environment-token")
	t.Setenv("GITHUB_ACTOR", "actions-user")
	if err := SaveToken(&StoredToken{AccessToken: "stored-token", Username: "stored-user"}); err != nil {
		t.Fatal(err)
	}

	token, err := LoadToken()
	if err != nil || token.Username != "actions-user" {
		t.Fatalf("environment LoadToken() = %#v, %v", token, err)
	}
	if err := SwitchAccount("stored-user"); err == nil {
		t.Fatal("SwitchAccount succeeded while GITHUB_TOKEN is set")
	}
	if err := RemoveAccount("stored-user"); err == nil {
		t.Fatal("RemoveAccount succeeded while GITHUB_TOKEN is set")
	}
	stored, err := LoadTokenFor("stored-user")
	if err != nil || stored.AccessToken != "stored-token" {
		t.Fatalf("LoadTokenFor(stored-user) = %#v, %v", stored, err)
	}
	accounts, err := ListAccounts()
	if err != nil {
		t.Fatal(err)
	}
	if len(accounts) != 2 || accounts[0].Active || accounts[1].ID != environmentAccountID || !accounts[1].Active {
		t.Fatalf("accounts = %#v", accounts)
	}
}

func TestGHTokenTakesPrecedenceOverGitHubToken(t *testing.T) {
	setupTokenStoreTest(t)
	t.Setenv("GH_TOKEN", "gh-token")
	t.Setenv("GITHUB_TOKEN", "github-token")
	t.Setenv("GITHUB_ACTOR", "actions-user")
	token, err := LoadToken()
	if err != nil || token.AccessToken != "gh-token" {
		t.Fatalf("LoadToken() = %#v, %v", token, err)
	}
	if source := EnvironmentTokenSource(); source != "GH_TOKEN" {
		t.Fatalf("EnvironmentTokenSource() = %q", source)
	}
}

func TestRemoveNamedAndAllAccounts(t *testing.T) {
	setupTokenStoreTest(t)
	if err := SaveToken(&StoredToken{AccessToken: "alice-token", Username: "alice"}); err != nil {
		t.Fatal(err)
	}
	if err := SaveToken(&StoredToken{AccessToken: "bob-token", Username: "bob"}); err != nil {
		t.Fatal(err)
	}
	if err := RemoveAccount("stored:alice"); err != nil {
		t.Fatal(err)
	}
	accounts, err := ListAccounts()
	if err != nil || len(accounts) != 1 || accounts[0].Username != "bob" {
		t.Fatalf("ListAccounts() = %#v, %v", accounts, err)
	}
	if err := RemoveAllAccounts(); err != nil {
		t.Fatal(err)
	}
	accounts, err = ListAccounts()
	if err != nil || len(accounts) != 0 {
		t.Fatalf("ListAccounts() after remove all = %#v, %v", accounts, err)
	}
	if _, err := os.Stat(TokenPath()); !os.IsNotExist(err) {
		t.Fatalf("token store still exists: %v", err)
	}
}

func TestEnvironmentAccountCannotBeRemoved(t *testing.T) {
	setupTokenStoreTest(t)
	t.Setenv("GITHUB_TOKEN", "environment-token")
	if err := DeleteToken(); err == nil {
		t.Fatal("DeleteToken succeeded for environment account")
	}
	if err := RemoveAccount(environmentAccountID); err == nil {
		t.Fatal("RemoveAccount succeeded for environment account")
	}
}

func TestAccountMetadataCanBeRemovedWhenKeychainEntryIsMissing(t *testing.T) {
	backend := setupTokenStoreTest(t)
	if err := SaveToken(&StoredToken{AccessToken: "alice-token", Username: "alice"}); err != nil {
		t.Fatal(err)
	}
	delete(backend.values, credentialService+"/alice")
	if err := RemoveAccount("alice"); err != nil {
		t.Fatal(err)
	}
	accounts, err := ListAccounts()
	if err != nil || len(accounts) != 0 {
		t.Fatalf("ListAccounts() = %#v, %v", accounts, err)
	}
}

func writeTokenStoreFile(t *testing.T, data []byte) {
	t.Helper()
	if err := os.MkdirAll(config.DataDir(), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(config.DataDir(), "token.json"), data, 0o600); err != nil {
		t.Fatal(err)
	}
}

func readTokenStoreFile(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile(TokenPath())
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}
