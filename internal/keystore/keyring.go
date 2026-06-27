package keystore

import (
	"errors"

	"github.com/zalando/go-keyring"
)

type KeyringBackend struct{}

func (k *KeyringBackend) Store(service, key string, secret []byte) error {
	return keyring.Set(service, key, string(secret))
}

func (k *KeyringBackend) Load(service, key string) ([]byte, error) {
	s, err := keyring.Get(service, key)
	if err != nil {
		if errors.Is(err, keyring.ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return []byte(s), nil
}

func (k *KeyringBackend) Delete(service, key string) error {
	return keyring.Delete(service, key)
}
