package cli

import (
	"errors"
	"fmt"
	"strings"

	agecrypto "filippo.io/age"
	"github.com/yashikota/enbu/internal/keystore"
)

const keystoreService = "enbu"

func repoKeystoreKey(owner, repo string) string {
	return fmt.Sprintf("%s/%s", strings.ToLower(owner), strings.ToLower(repo))
}

func loadIdentitiesForRepo(ks KeyStore, owner, repo string) ([]agecrypto.Identity, error) {
	key := repoKeystoreKey(owner, repo)
	privKeyBytes, err := ks.Load(keystoreService, key)
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
