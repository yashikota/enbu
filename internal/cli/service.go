package cli

import (
	"context"
	"fmt"
	"strings"

	"github.com/yashikota/enbu/internal/auth"
	"github.com/yashikota/enbu/internal/config"
	gh "github.com/yashikota/enbu/internal/github"
	"github.com/yashikota/enbu/internal/keystore"
	"github.com/yashikota/enbu/internal/oci"
)

type Registry interface {
	Push(ctx context.Context, ref string, mediaType string, data []byte, token string, opts *oci.PushOptions) error
	Pull(ctx context.Context, ref string, token string) ([]byte, error)
	ListTags(ctx context.Context, ref string, token string) ([]string, error)
	GetDigest(ctx context.Context, ref string, token string) (string, error)
}

type TokenProvider interface {
	LoadToken() (accessToken string, username string, err error)
}

type KeyStore interface {
	Store(service, key string, value []byte) error
	Load(service, key string) ([]byte, error)
}

type RepoDetector interface {
	LoadRepo() (owner, repo string, err error)
}

type GitHubClient interface {
	GetUser(ctx context.Context) (*gh.User, error)
	IsOrganization(ctx context.Context, login string) bool
}

type Service struct {
	Registry      Registry
	TokenProvider TokenProvider
	KeyStore      KeyStore
	RepoDetector  RepoDetector
	GitHub        GitHubClient
	RegistryHost  string
}

func (s *Service) registryHost() string {
	if s.RegistryHost != "" {
		return s.RegistryHost
	}
	return "ghcr.io"
}

func (s *Service) registryRef(owner, repo string) string {
	return fmt.Sprintf("%s/%s/%s-enbu", s.registryHost(), strings.ToLower(owner), strings.ToLower(repo))
}

func (s *Service) secretsRef(owner, repo string) string {
	return s.registryRef(owner, repo) + ":secrets-default"
}

type defaultRegistry struct{}

func (d *defaultRegistry) Push(ctx context.Context, ref string, mediaType string, data []byte, token string, opts *oci.PushOptions) error {
	return oci.Push(ctx, ref, mediaType, data, token, opts)
}

func (d *defaultRegistry) Pull(ctx context.Context, ref string, token string) ([]byte, error) {
	return oci.Pull(ctx, ref, token)
}

func (d *defaultRegistry) ListTags(ctx context.Context, ref string, token string) ([]string, error) {
	return oci.ListTags(ctx, ref, token)
}

func (d *defaultRegistry) GetDigest(ctx context.Context, ref string, token string) (string, error) {
	return oci.GetDigest(ctx, ref, token)
}

type defaultTokenProvider struct{}

func (d *defaultTokenProvider) LoadToken() (string, string, error) {
	token, err := auth.LoadToken()
	if err != nil {
		return "", "", err
	}
	return token.AccessToken, token.Username, nil
}

type defaultRepoDetector struct{}

func (d *defaultRepoDetector) LoadRepo() (string, string, error) {
	cfg, err := config.LoadRepo()
	if err != nil {
		return "", "", err
	}
	return cfg.Owner, cfg.Repo, nil
}

type defaultKeyStore struct {
	backend keystore.Backend
}

func (d *defaultKeyStore) Store(service, key string, value []byte) error {
	return d.backend.Store(service, key, value)
}

func (d *defaultKeyStore) Load(service, key string) ([]byte, error) {
	return d.backend.Load(service, key)
}

type unavailableKeyStore struct {
	err error
}

func (u *unavailableKeyStore) Store(_, _ string, _ []byte) error {
	return u.err
}

func (u *unavailableKeyStore) Load(_, _ string) ([]byte, error) {
	return nil, u.err
}

func DefaultService() *Service {
	ks, err := keystore.New()
	var keystoreImpl KeyStore
	if err != nil {
		keystoreImpl = &unavailableKeyStore{err: fmt.Errorf("initializing keystore: %w", err)}
	} else {
		keystoreImpl = &defaultKeyStore{backend: ks}
	}
	return &Service{
		Registry:      &defaultRegistry{},
		TokenProvider: &defaultTokenProvider{},
		KeyStore:      keystoreImpl,
		RepoDetector:  &defaultRepoDetector{},
	}
}
