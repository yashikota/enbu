package cli

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/yashikota/enbu/pkg/oci"
)

type failingDigestRegistry struct {
	err error
}

func (f *failingDigestRegistry) Push(context.Context, string, string, []byte, string, *oci.PushOptions) error {
	return nil
}

func (f *failingDigestRegistry) Pull(context.Context, string, string) ([]byte, error) {
	return nil, nil
}

func (f *failingDigestRegistry) ListTags(context.Context, string, string) ([]string, error) {
	return nil, nil
}

func (f *failingDigestRegistry) GetDigest(context.Context, string, string) (string, error) {
	return "", f.err
}

func TestDoSyncReturnsNonNotFoundSecretPullErrors(t *testing.T) {
	err := doSync(
		context.Background(),
		&failingDigestRegistry{err: errors.New("unauthorized")},
		"example.com/owner/repo-enbu:secrets-default",
		"example.com/owner/repo-enbu",
		"token",
		nil,
		nil,
		defaultEnvironment,
		[]string{defaultEnvironment},
	)

	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "pulling secrets") || !strings.Contains(err.Error(), "unauthorized") {
		t.Fatalf("expected pulling secrets unauthorized error, got %v", err)
	}
}
