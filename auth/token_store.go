package auth

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/gofrs/flock"
	"github.com/yashikota/enbu/config"
	"github.com/yashikota/enbu/utils/keystore"
)

const (
	tokenStoreVersion        = 2
	credentialService        = "enbu-github"
	accountSourceStored      = "stored"
	accountSourceEnvironment = "environment"
	storageKeyring           = "keychain"
	storageFile              = "file"
	storageEnvironment       = "environment"
	environmentAccountID     = "environment"
)

type StoredToken struct {
	AccessToken string `json:"access_token"`
	Username    string `json:"username"`
}

type Account struct {
	ID       string `json:"id"`
	Username string `json:"username"`
	Active   bool   `json:"active"`
	Source   string `json:"source"`
	Storage  string `json:"storage"`
}

type storedAccount struct {
	Username    string `json:"username"`
	Storage     string `json:"storage"`
	AccessToken string `json:"access_token,omitempty"`
}

type tokenStore struct {
	Version  int                      `json:"version"`
	Active   string                   `json:"active,omitempty"`
	Accounts map[string]storedAccount `json:"accounts"`
}

var (
	tokenStoreMu    sync.Mutex
	newTokenBackend = func() keystore.Backend {
		return &keystore.KeyringBackend{}
	}
)

func SaveToken(token *StoredToken) error {
	_, err := SaveTokenWithAccount(token)
	return err
}

func SaveTokenWithAccount(token *StoredToken) (Account, error) {
	if token == nil || strings.TrimSpace(token.Username) == "" || token.AccessToken == "" {
		return Account{}, fmt.Errorf("username and access token are required")
	}
	var result Account
	err := withTokenStore(func() error {
		store, err := loadTokenStoreUnlocked()
		if err != nil {
			return err
		}
		key := accountKey(token.Username)
		account := storeCredential(key, token.Username, token.AccessToken)
		store.Accounts[key] = account
		store.Active = key
		if err := saveTokenStoreUnlocked(store); err != nil {
			return err
		}
		result = publicStoredAccount(key, account, true)
		return nil
	})
	return result, err
}

func LoadToken() (*StoredToken, error) {
	var token *StoredToken
	err := withTokenStore(func() error {
		if loaded, ok := environmentToken(); ok {
			token = loaded
			return nil
		}
		store, err := loadTokenStoreUnlocked()
		if err != nil {
			return err
		}
		if store.Active != "" {
			loaded, err := loadStoredToken(store, store.Active)
			if err != nil {
				return err
			}
			token = loaded
			return nil
		}
		return notLoggedInError()
	})
	return token, err
}

func LoadTokenFor(username string) (*StoredToken, error) {
	var token *StoredToken
	err := withTokenStore(func() error {
		store, err := loadTokenStoreUnlocked()
		if err != nil {
			return err
		}
		key := accountKey(username)
		if _, ok := store.Accounts[key]; ok {
			loaded, err := loadStoredToken(store, key)
			if err != nil {
				return err
			}
			token = loaded
			return nil
		}
		if env, ok := environmentToken(); ok && accountKey(env.Username) == key {
			token = env
			return nil
		}
		return fmt.Errorf("GitHub account %q is not registered", username)
	})
	return token, err
}

func DeleteToken() error {
	return withTokenStore(func() error {
		if source := EnvironmentTokenSource(); source != "" {
			return environmentCredentialsError(source)
		}
		store, err := loadTokenStoreUnlocked()
		if err != nil {
			return err
		}
		if store.Active == "" {
			return nil
		}
		return removeStoredAccountUnlocked(store, store.Active)
	})
}

func RemoveAccount(identifier string) error {
	return withTokenStore(func() error {
		if source := EnvironmentTokenSource(); source != "" {
			return environmentCredentialsError(source)
		}
		if identifier == environmentAccountID {
			return fmt.Errorf("cannot remove an account provided by an environment variable")
		}
		store, err := loadTokenStoreUnlocked()
		if err != nil {
			return err
		}
		key := storedAccountKey(identifier)
		if _, ok := store.Accounts[key]; !ok {
			return fmt.Errorf("GitHub account %q is not registered", identifier)
		}
		return removeStoredAccountUnlocked(store, key)
	})
}

func RemoveAllAccounts() error {
	return withTokenStore(func() error {
		if source := EnvironmentTokenSource(); source != "" {
			return environmentCredentialsError(source)
		}
		store, err := loadTokenStoreUnlocked()
		if err != nil {
			return err
		}
		var removeErrors []error
		for _, key := range sortedAccountKeys(store.Accounts) {
			account := store.Accounts[key]
			if account.Storage == storageKeyring {
				if err := newTokenBackend().Delete(credentialService, key); err != nil && !errors.Is(err, keystore.ErrNotFound) {
					removeErrors = append(removeErrors, fmt.Errorf("removing %s from keychain: %w", account.Username, err))
					continue
				}
			}
			delete(store.Accounts, key)
		}
		store.Active = ""
		if err := saveOrRemoveTokenStoreUnlocked(store); err != nil {
			removeErrors = append(removeErrors, err)
		}
		return errors.Join(removeErrors...)
	})
}

func ListAccounts() ([]Account, error) {
	var accounts []Account
	err := withTokenStore(func() error {
		store, err := loadTokenStoreUnlocked()
		if err != nil {
			return err
		}
		for _, key := range sortedAccountKeys(store.Accounts) {
			accounts = append(accounts, publicStoredAccount(key, store.Accounts[key], key == store.Active && EnvironmentTokenSource() == ""))
		}
		if env, ok := environmentToken(); ok {
			accounts = append(accounts, Account{
				ID:       environmentAccountID,
				Username: env.Username,
				Active:   true,
				Source:   accountSourceEnvironment,
				Storage:  storageEnvironment,
			})
		}
		return nil
	})
	return accounts, err
}

func SwitchAccount(identifier string) error {
	return withTokenStore(func() error {
		if source := EnvironmentTokenSource(); source != "" {
			return environmentCredentialsError(source)
		}
		store, err := loadTokenStoreUnlocked()
		if err != nil {
			return err
		}
		key := storedAccountKey(identifier)
		if _, ok := store.Accounts[key]; !ok {
			return fmt.Errorf("GitHub account %q is not registered", identifier)
		}
		store.Active = key
		return saveTokenStoreUnlocked(store)
	})
}

func UseEnvironmentAccount() error {
	if source := EnvironmentTokenSource(); source == "" {
		return fmt.Errorf("GH_TOKEN or GITHUB_TOKEN is not set")
	}
	return nil
}

func TokenPath() string {
	return filepath.Join(config.DataDir(), "token.json")
}

func withTokenStore(fn func() error) error {
	tokenStoreMu.Lock()
	defer tokenStoreMu.Unlock()
	if err := os.MkdirAll(config.DataDir(), 0o700); err != nil {
		return fmt.Errorf("creating data directory: %w", err)
	}
	fileLock := flock.New(TokenPath() + ".lock")
	if err := fileLock.Lock(); err != nil {
		return fmt.Errorf("locking token store: %w", err)
	}
	defer func() { _ = fileLock.Unlock() }()
	return fn()
}

func loadTokenStoreUnlocked() (*tokenStore, error) {
	data, err := os.ReadFile(TokenPath())
	if err != nil {
		if os.IsNotExist(err) {
			return newTokenStore(), nil
		}
		return nil, fmt.Errorf("reading token store: %w", err)
	}

	var store tokenStore
	if err := json.Unmarshal(data, &store); err == nil && store.Version == tokenStoreVersion && store.Accounts != nil {
		if _, ok := store.Accounts[store.Active]; !ok {
			store.Active = ""
		}
		return &store, nil
	}

	migrated, err := migrateLegacyTokenStore(data)
	if err != nil {
		return nil, err
	}
	if err := saveTokenStoreUnlocked(migrated); err != nil {
		return nil, fmt.Errorf("saving migrated token store: %w", err)
	}
	return migrated, nil
}

func migrateLegacyTokenStore(data []byte) (*tokenStore, error) {
	var interim struct {
		Active   string                 `json:"active"`
		Accounts map[string]StoredToken `json:"accounts"`
	}
	if err := json.Unmarshal(data, &interim); err == nil && interim.Accounts != nil {
		store := newTokenStore()
		for key, token := range interim.Accounts {
			normalized := accountKey(key)
			if token.Username == "" {
				token.Username = key
			}
			store.Accounts[normalized] = storeCredential(normalized, token.Username, token.AccessToken)
		}
		if _, ok := store.Accounts[accountKey(interim.Active)]; ok {
			store.Active = accountKey(interim.Active)
		}
		return store, nil
	}

	var legacy StoredToken
	if err := json.Unmarshal(data, &legacy); err != nil {
		return nil, fmt.Errorf("parsing token: %w", err)
	}
	if legacy.Username == "" || legacy.AccessToken == "" {
		return nil, notLoggedInError()
	}
	key := accountKey(legacy.Username)
	store := newTokenStore()
	store.Active = key
	store.Accounts[key] = storeCredential(key, legacy.Username, legacy.AccessToken)
	return store, nil
}

func newTokenStore() *tokenStore {
	return &tokenStore{Version: tokenStoreVersion, Accounts: make(map[string]storedAccount)}
}

func storeCredential(key, username, accessToken string) storedAccount {
	account := storedAccount{Username: username, Storage: storageKeyring}
	if err := newTokenBackend().Store(credentialService, key, []byte(accessToken)); err != nil {
		account.Storage = storageFile
		account.AccessToken = accessToken
	}
	return account
}

func loadStoredToken(store *tokenStore, key string) (*StoredToken, error) {
	account, ok := store.Accounts[key]
	if !ok {
		return nil, fmt.Errorf("GitHub account %q is not registered", key)
	}
	var accessToken string
	switch account.Storage {
	case storageKeyring:
		data, err := newTokenBackend().Load(credentialService, key)
		if err != nil {
			return nil, fmt.Errorf("loading GitHub token for %s from keychain: %w", account.Username, err)
		}
		accessToken = string(data)
	case storageFile:
		accessToken = account.AccessToken
	default:
		return nil, fmt.Errorf("unsupported token storage %q for %s", account.Storage, account.Username)
	}
	if accessToken == "" {
		return nil, fmt.Errorf("stored GitHub token for %s is empty", account.Username)
	}
	return &StoredToken{AccessToken: accessToken, Username: account.Username}, nil
}

func removeStoredAccountUnlocked(store *tokenStore, key string) error {
	account := store.Accounts[key]
	if account.Storage == storageKeyring {
		if err := newTokenBackend().Delete(credentialService, key); err != nil && !errors.Is(err, keystore.ErrNotFound) {
			return fmt.Errorf("removing %s from keychain: %w", account.Username, err)
		}
	}
	delete(store.Accounts, key)
	if store.Active == key {
		remaining := sortedAccountKeys(store.Accounts)
		if len(remaining) == 0 {
			store.Active = ""
		} else {
			store.Active = remaining[0]
		}
	}
	return saveOrRemoveTokenStoreUnlocked(store)
}

func saveOrRemoveTokenStoreUnlocked(store *tokenStore) error {
	if len(store.Accounts) == 0 && store.Active == "" {
		if err := os.Remove(TokenPath()); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("removing token store: %w", err)
		}
		return nil
	}
	return saveTokenStoreUnlocked(store)
}

func saveTokenStoreUnlocked(store *tokenStore) error {
	store.Version = tokenStoreVersion
	data, err := json.Marshal(store)
	if err != nil {
		return err
	}
	temp, err := os.CreateTemp(config.DataDir(), "token-*.tmp")
	if err != nil {
		return fmt.Errorf("creating temporary token store: %w", err)
	}
	tempPath := temp.Name()
	defer func() { _ = os.Remove(tempPath) }()
	if err := temp.Chmod(0o600); err != nil {
		_ = temp.Close()
		return err
	}
	if _, err := temp.Write(data); err != nil {
		_ = temp.Close()
		return err
	}
	if err := temp.Sync(); err != nil {
		_ = temp.Close()
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tempPath, TokenPath()); err != nil {
		return fmt.Errorf("replacing token store: %w", err)
	}
	return nil
}

func publicStoredAccount(key string, account storedAccount, active bool) Account {
	return Account{
		ID:       "stored:" + key,
		Username: account.Username,
		Active:   active,
		Source:   accountSourceStored,
		Storage:  account.Storage,
	}
}

func sortedAccountKeys(accounts map[string]storedAccount) []string {
	keys := make([]string, 0, len(accounts))
	for key := range accounts {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func storedAccountKey(identifier string) string {
	return accountKey(strings.TrimPrefix(identifier, "stored:"))
}

func accountKey(username string) string {
	return strings.ToLower(strings.TrimSpace(username))
}

func environmentToken() (*StoredToken, bool) {
	accessToken := os.Getenv("GH_TOKEN")
	if accessToken == "" {
		accessToken = os.Getenv("GITHUB_TOKEN")
	}
	if accessToken == "" {
		return nil, false
	}
	username := os.Getenv("GITHUB_ACTOR")
	if username == "" {
		username = "github-actions"
	}
	return &StoredToken{AccessToken: accessToken, Username: username}, true
}

func EnvironmentTokenSource() string {
	if os.Getenv("GH_TOKEN") != "" {
		return "GH_TOKEN"
	}
	if os.Getenv("GITHUB_TOKEN") != "" {
		return "GITHUB_TOKEN"
	}
	return ""
}

func environmentCredentialsError(source string) error {
	return fmt.Errorf("the value of the %s environment variable is being used for authentication; clear it before changing stored accounts", source)
}

func notLoggedInError() error {
	return fmt.Errorf("not logged in: run 'enbu auth login' to authenticate with GitHub")
}
