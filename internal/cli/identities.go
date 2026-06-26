package cli

import (
	"os"
	"path/filepath"

	agecrypto "filippo.io/age"
	"github.com/yashikota/enbu/internal/age"
	"github.com/yashikota/enbu/internal/config"
	"github.com/yashikota/enbu/internal/tokenlock"
)

func loadIdentities(token string) ([]agecrypto.Identity, error) {
	var identities []agecrypto.Identity

	if envKey := os.Getenv("ENBU_SECRET_KEY"); envKey != "" {
		id, err := agecrypto.ParseX25519Identity(envKey)
		if err == nil {
			identities = append(identities, id)
		}
	}

	dataDir := config.DataDir()
	if encKey, err := os.ReadFile(filepath.Join(dataDir, "age_key.enc")); err == nil {
		privKeyBytes, err := tokenlock.Decrypt(encKey, token)
		if err == nil {
			if id, err := agecrypto.ParseX25519Identity(string(privKeyBytes)); err == nil {
				identities = append(identities, id)
			}
		}
	}

	sshIds, err := age.LoadSSHIdentities()
	if err == nil {
		identities = append(identities, sshIds...)
	}

	return identities, nil
}
