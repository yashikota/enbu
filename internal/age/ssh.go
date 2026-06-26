package age

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"

	"filippo.io/age"
	"filippo.io/age/agessh"
	"golang.org/x/crypto/ssh"
)

func DefaultSSHKeys() ([]string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	return []string{
		filepath.Join(home, ".ssh", "id_ed25519"),
		filepath.Join(home, ".ssh", "id_rsa"),
	}, nil
}

func GetLocalSSHKey() (pubKeyStr string, privKeyPath string, err error) {
	keyPaths, err := DefaultSSHKeys()
	if err != nil {
		return "", "", fmt.Errorf("getting default SSH key paths: %w", err)
	}

	for _, privPath := range keyPaths {
		pubPath := privPath + ".pub"
		pubBytes, err := os.ReadFile(pubPath)
		if err != nil {
			continue
		}

		localPub, _, _, _, err := ssh.ParseAuthorizedKey(pubBytes)
		if err != nil {
			continue
		}

		sshType := localPub.Type()
		if sshType == "ssh-ed25519" || sshType == "ssh-rsa" {
			cleanPubKeyStr := string(bytes.TrimSpace(ssh.MarshalAuthorizedKey(localPub)))
			return cleanPubKeyStr, privPath, nil
		}
	}

	return "", "", fmt.Errorf("no default SSH keys (Ed25519 or RSA) found on your machine")
}

func LoadSSHIdentities() ([]age.Identity, error) {
	keyPaths, err := DefaultSSHKeys()
	if err != nil {
		return nil, err
	}

	var identities []age.Identity
	for _, privPath := range keyPaths {
		pemBytes, err := os.ReadFile(privPath)
		if err != nil {
			continue
		}

		id, err := agessh.ParseIdentity(pemBytes)
		if err != nil {
			continue
		}
		identities = append(identities, id)
	}

	return identities, nil
}
