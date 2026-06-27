package cli

import (
	"errors"
	"fmt"
	"strings"

	agecrypto "filippo.io/age"
	"github.com/yashikota/enbu/internal/config"
	"github.com/yashikota/enbu/internal/keystore"
)

const keystoreService = "enbu"

func repoKeystoreKey(cfg *config.RepoConfig) string {
	return fmt.Sprintf("%s/%s", strings.ToLower(cfg.Owner), strings.ToLower(cfg.Repo))
}

func loadIdentitiesForRepo(cfg *config.RepoConfig) ([]agecrypto.Identity, error) {
	backend, err := keystore.New()
	if err != nil {
		return nil, err
	}

	key := repoKeystoreKey(cfg)
	privKeyBytes, err := backend.Load(keystoreService, key)
	if err != nil {
		if errors.Is(err, keystore.ErrNotFound) {
			return nil, fmt.Errorf("no private key found (run 'enbu init' first)")
		}
		return nil, fmt.Errorf("loading private key: %w", err)
	}

	id, err := agecrypto.ParseX25519Identity(string(privKeyBytes))
	if err != nil {
		return nil, fmt.Errorf("parsing private key: %w", err)
	}

	return []agecrypto.Identity{id}, nil
}
