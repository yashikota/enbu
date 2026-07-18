package auth

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/yashikota/enbu/config"
)

type StoredToken struct {
	AccessToken string `json:"access_token"`
	Username    string `json:"username"`
}

func SaveToken(token *StoredToken) error {
	dir := config.DataDir()
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("creating data directory: %w", err)
	}

	data, err := json.Marshal(token)
	if err != nil {
		return err
	}

	path := filepath.Join(dir, "token.json")
	return os.WriteFile(path, data, 0o600)
}

func LoadToken() (*StoredToken, error) {
	if tokenEnv := os.Getenv("GITHUB_TOKEN"); tokenEnv != "" {
		actorEnv := os.Getenv("GITHUB_ACTOR")
		if actorEnv == "" {
			actorEnv = "github-actions"
		}
		return &StoredToken{
			AccessToken: tokenEnv,
			Username:    actorEnv,
		}, nil
	}

	path := filepath.Join(config.DataDir(), "token.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("not logged in: run 'enbu auth login' to authenticate with GitHub")
	}

	var token StoredToken
	if err := json.Unmarshal(data, &token); err != nil {
		return nil, fmt.Errorf("parsing token: %w", err)
	}

	return &token, nil
}

func DeleteToken() error {
	path := filepath.Join(config.DataDir(), "token.json")
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("removing token: %w", err)
	}
	return nil
}

func TokenPath() string {
	return filepath.Join(config.DataDir(), "token.json")
}
