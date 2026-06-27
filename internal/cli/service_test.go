package cli

import (
	"strings"
	"testing"
)

func TestDefaultServiceWithInvalidBackendReturnsKeystoreError(t *testing.T) {
	t.Setenv("ENBU_BACKEND", "invalid")

	svc := DefaultService()
	_, err := loadIdentitiesForRepo(svc.KeyStore, "owner", "repo")
	if err == nil {
		t.Fatal("expected keystore initialization error")
	}
	if !strings.Contains(err.Error(), "unknown backend type") {
		t.Fatalf("expected unknown backend type error, got %v", err)
	}
}
