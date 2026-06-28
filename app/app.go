package app

import (
	"context"
	"fmt"
	"strings"

	"github.com/yashikota/enbu/pkg/auth"
	"github.com/yashikota/enbu/pkg/config"
	"github.com/yashikota/enbu/pkg/keystore"
	"github.com/yashikota/enbu/pkg/oci"
)

type App struct {
	Registry      Registry
	TokenProvider TokenProvider
	KeyStore      KeyStore
	RepoDetector  RepoDetector
	GitHub        GitHubClient
	Events        EventHandler
	RegistryHost  string
}

func (a *App) registryHost() string {
	if a.RegistryHost != "" {
		return a.RegistryHost
	}
	return "ghcr.io"
}

func (a *App) registryRef(owner, repo string) string {
	return fmt.Sprintf("%s/%s/%s-enbu", a.registryHost(), strings.ToLower(owner), strings.ToLower(repo))
}

func (a *App) secretsRef(owner, repo, env string) string {
	return a.registryRef(owner, repo) + ":" + secretsTag(env)
}

func (a *App) emit(msg string) {
	if a.Events != nil {
		a.Events.OnProgress(msg)
	}
}

func (a *App) emitRetry(attempt, max int) {
	if a.Events != nil {
		a.Events.OnConflictRetry(attempt, max)
	}
}

func New() *App {
	ks, err := keystore.New()
	var keystoreImpl KeyStore
	if err != nil {
		keystoreImpl = &unavailableKeyStore{err: fmt.Errorf("initializing keystore: %w", err)}
	} else {
		keystoreImpl = &defaultKeyStore{backend: ks}
	}
	return &App{
		Registry:      &defaultRegistry{},
		TokenProvider: &defaultTokenProvider{},
		KeyStore:      keystoreImpl,
		RepoDetector:  &defaultRepoDetector{},
	}
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
