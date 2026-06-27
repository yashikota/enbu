package keystore

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

type TextBackend struct{}

func (t *TextBackend) Store(service, key string, secret []byte) error {
	dir := textBackendDir()
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("creating keystore directory: %w", err)
	}
	path := filepath.Join(dir, service+"_"+key)
	return os.WriteFile(path, secret, 0o600)
}

func (t *TextBackend) Load(service, key string) ([]byte, error) {
	path := filepath.Join(textBackendDir(), service+"_"+key)
	return os.ReadFile(path)
}

func (t *TextBackend) Delete(service, key string) error {
	path := filepath.Join(textBackendDir(), service+"_"+key)
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func textBackendDir() string {
	if dir := os.Getenv("ENBU_TEXT_BACKEND_DIR"); dir != "" {
		return dir
	}
	if xdg := os.Getenv("XDG_DATA_HOME"); xdg != "" {
		return filepath.Join(xdg, "enbu", "keys")
	}
	switch runtime.GOOS {
	case "windows":
		return filepath.Join(os.Getenv("LOCALAPPDATA"), "enbu", "keys")
	default:
		return filepath.Join(os.Getenv("HOME"), ".local", "share", "enbu", "keys")
	}
}
