package cli

import (
	"strings"
	"testing"

	"github.com/yashikota/enbu/app"
)

func TestDefaultAppWithInvalidBackendReturnsKeystoreError(t *testing.T) {
	t.Setenv("ENBU_BACKEND", "invalid")

	a := app.New()
	_, err := app.LoadIdentitiesForRepo(a.KeyStore, "owner", "repo")
	if err == nil {
		t.Fatal("expected keystore initialization error")
	}
	if !strings.Contains(err.Error(), "unknown backend type") {
		t.Fatalf("expected unknown backend type error, got %v", err)
	}
}
