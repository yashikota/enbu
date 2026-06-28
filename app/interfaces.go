package app

import (
	"context"

	"github.com/yashikota/enbu/pkg/oci"
	gh "github.com/yashikota/enbu/pkg/provider/github"
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

type EventHandler interface {
	OnProgress(msg string)
	OnConflictRetry(attempt, maxAttempts int)
}
