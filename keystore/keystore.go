package keystore

import (
	"errors"
	"fmt"
	"os"
)

var ErrNotFound = errors.New("secret not found")

type Backend interface {
	Store(service, key string, secret []byte) error
	Load(service, key string) ([]byte, error)
	Delete(service, key string) error
}

func New() (Backend, error) {
	backendType := os.Getenv("ENBU_BACKEND")
	if backendType == "" {
		backendType = "keyring"
	}

	switch backendType {
	case "keyring":
		return &KeyringBackend{}, nil
	case "text":
		return &TextBackend{}, nil
	default:
		return nil, fmt.Errorf("unknown backend type: %s (supported: keyring, text)", backendType)
	}
}
