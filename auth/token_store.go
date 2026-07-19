package auth

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/enbu-net/enbu/config"
	"github.com/zalando/go-keyring"
)

const (
	tokenKeyringService = "enbu"
	tokenKeyringAccount = "github-oauth"
)

var (
	keyringSet    = keyring.Set
	keyringGet    = keyring.Get
	keyringDelete = keyring.Delete
)

type StoredToken struct {
	AccessToken string `json:"access_token"`
	Username    string `json:"username"`
	UserID      int64  `json:"user_id"`
}

func SaveToken(token *StoredToken) error {
	if err := validateStoredToken(token); err != nil {
		return err
	}
	data, err := json.Marshal(token)
	if err != nil {
		return err
	}
	if err := keyringSet(tokenKeyringService, tokenKeyringAccount, string(data)); err != nil {
		return fmt.Errorf("storing token in OS keyring: %w", err)
	}
	stored, err := keyringGet(tokenKeyringService, tokenKeyringAccount)
	if err != nil || !bytes.Equal([]byte(stored), data) {
		return errors.New("verifying token in OS keyring failed")
	}
	if err := os.Remove(legacyTokenPath()); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("removing legacy token file (credential was saved to keyring): %w", err)
	}
	return nil
}

func LoadToken() (*StoredToken, error) {
	if tokenEnv := os.Getenv("GITHUB_TOKEN"); tokenEnv != "" {
		actor := os.Getenv("GITHUB_ACTOR")
		if actor == "" {
			actor = "github-actions"
		}
		return &StoredToken{AccessToken: tokenEnv, Username: actor}, nil
	}

	data, err := keyringGet(tokenKeyringService, tokenKeyringAccount)
	if err != nil {
		if errors.Is(err, keyring.ErrNotFound) {
			return nil, notLoggedInError()
		}
		return nil, fmt.Errorf("loading token from OS keyring: %w", err)
	}
	decoder := json.NewDecoder(bytes.NewBufferString(data))
	decoder.DisallowUnknownFields()
	var token StoredToken
	if err := decoder.Decode(&token); err != nil {
		return nil, fmt.Errorf("parsing token from OS keyring: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return nil, errors.New("parsing token from OS keyring: invalid trailing data")
	}
	if err := validateStoredToken(&token); err != nil {
		return nil, fmt.Errorf("parsing token from OS keyring: %w", err)
	}
	return &token, nil
}

func DeleteToken() error {
	var deleteErrors []error
	if err := keyringDelete(tokenKeyringService, tokenKeyringAccount); err != nil && !errors.Is(err, keyring.ErrNotFound) {
		deleteErrors = append(deleteErrors, fmt.Errorf("removing token from OS keyring: %w", err))
	}
	if err := os.Remove(legacyTokenPath()); err != nil && !os.IsNotExist(err) {
		deleteErrors = append(deleteErrors, fmt.Errorf("removing legacy token file: %w", err))
	}
	return errors.Join(deleteErrors...)
}

func validateStoredToken(token *StoredToken) error {
	if token == nil || token.AccessToken == "" || len(token.AccessToken) > 2048 {
		return errors.New("invalid access token")
	}
	if token.Username == "" || len(token.Username) > 256 || token.UserID <= 0 {
		return errors.New("invalid GitHub identity")
	}
	return nil
}

func notLoggedInError() error {
	return errors.New("not logged in: run 'enbu auth login' to authenticate with GitHub")
}

func legacyTokenPath() string {
	return filepath.Join(config.DataDir(), "token.json")
}
