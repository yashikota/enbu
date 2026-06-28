package cli

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/yashikota/enbu/pkg/age"
	"github.com/yashikota/enbu/pkg/bundle"
	"github.com/yashikota/enbu/pkg/oci"
)

type deleteTestTokenProvider struct{}

func (d *deleteTestTokenProvider) LoadToken() (string, string, error) {
	return "token", "alice", nil
}

type deleteTestRepoDetector struct{}

func (d *deleteTestRepoDetector) LoadRepo() (string, string, error) {
	return "owner", "repo", nil
}

type failingKeyStore struct {
	err error
}

func (f *failingKeyStore) Store(string, string, []byte) error {
	return nil
}

func (f *failingKeyStore) Load(string, string) ([]byte, error) {
	return nil, f.err
}

type deleteDigestErrorRegistry struct {
	ciphertext []byte
	publicKey  string
	pushes     int
}

func (d *deleteDigestErrorRegistry) Push(context.Context, string, string, []byte, string, *oci.PushOptions) error {
	d.pushes++
	return nil
}

func (d *deleteDigestErrorRegistry) Pull(_ context.Context, ref string, _ string) ([]byte, error) {
	if strings.HasSuffix(ref, ":recipient-alice") {
		return []byte(d.publicKey), nil
	}
	return d.ciphertext, nil
}

func (d *deleteDigestErrorRegistry) ListTags(context.Context, string, string) ([]string, error) {
	return []string{"recipient-alice"}, nil
}

func (d *deleteDigestErrorRegistry) GetDigest(context.Context, string, string) (string, error) {
	return "sha256:base", nil
}

type deleteExpectedDigestRegistry struct {
	ciphertext     []byte
	publicKey      string
	expectedDigest string
	gotExpected    string
	pushes         int
	pushErr        error
}

func (d *deleteExpectedDigestRegistry) Push(_ context.Context, _ string, _ string, _ []byte, _ string, opts *oci.PushOptions) error {
	d.pushes++
	if opts != nil {
		d.gotExpected = opts.ExpectedDigest
	}
	return d.pushErr
}

func (d *deleteExpectedDigestRegistry) Pull(_ context.Context, ref string, _ string) ([]byte, error) {
	if strings.HasSuffix(ref, ":recipient-alice") {
		return []byte(d.publicKey), nil
	}
	return d.ciphertext, nil
}

func (d *deleteExpectedDigestRegistry) ListTags(context.Context, string, string) ([]string, error) {
	return []string{"recipient-alice"}, nil
}

func (d *deleteExpectedDigestRegistry) GetDigest(context.Context, string, string) (string, error) {
	return d.expectedDigest, nil
}

func TestDeleteCommandReturnsKeyStoreError(t *testing.T) {
	svc := &Service{
		Registry:      &deleteDigestErrorRegistry{},
		TokenProvider: &deleteTestTokenProvider{},
		RepoDetector:  &deleteTestRepoDetector{},
		KeyStore:      &failingKeyStore{err: errors.New("keychain locked")},
	}
	cmd := NewWithService("test", svc)
	cmd.SetArgs([]string{"delete", "API_KEY"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "loading decryption keys") || !strings.Contains(err.Error(), "keychain locked") {
		t.Fatalf("expected wrapped key store error, got %v", err)
	}
}

func TestDeleteCommandPassesBaseDigestToPush(t *testing.T) {
	kp, err := age.GenerateKeyPair()
	if err != nil {
		t.Fatalf("GenerateKeyPair: %v", err)
	}
	plaintext := bundle.Marshal(map[string]string{"API_KEY": "secret"})
	ciphertext, err := age.EncryptForPublicKeys(plaintext, []string{kp.PublicKey})
	if err != nil {
		t.Fatalf("EncryptForPublicKeys: %v", err)
	}

	reg := &deleteExpectedDigestRegistry{
		ciphertext:     ciphertext,
		publicKey:      kp.PublicKey,
		expectedDigest: "sha256:base",
	}
	svc := &Service{
		Registry:      reg,
		TokenProvider: &deleteTestTokenProvider{},
		RepoDetector:  &deleteTestRepoDetector{},
		KeyStore: &staticKeyStore{
			key: []byte(kp.Identity.String()),
		},
	}
	cmd := NewWithService("test", svc)
	cmd.SetArgs([]string{"delete", "API_KEY"})

	err = cmd.Execute()
	if err != nil {
		t.Fatalf("delete: %v", err)
	}
	if reg.gotExpected != "sha256:base" {
		t.Fatalf("expected push to receive base digest, got %q", reg.gotExpected)
	}
	if reg.pushes != 1 {
		t.Fatalf("expected 1 push, got %d", reg.pushes)
	}
}

type staticKeyStore struct {
	key []byte
}

func (s *staticKeyStore) Store(string, string, []byte) error {
	return nil
}

func (s *staticKeyStore) Load(string, string) ([]byte, error) {
	return s.key, nil
}
