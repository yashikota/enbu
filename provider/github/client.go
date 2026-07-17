package github

import (
	"context"
	"fmt"
	"net/http"

	githubsdk "github.com/google/go-github/v89/github"
	"github.com/yashikota/enbu/provider"
)

type Client struct {
	sdk     *githubsdk.Client
	initErr error
}

func NewClient(token string) *Client {
	return NewClientWithHTTPClient(token, http.DefaultClient)
}

func NewClientWithHTTPClient(token string, httpClient *http.Client) *Client {
	return newClient(token, httpClient, "")
}

func newClient(token string, httpClient *http.Client, baseURL string) *Client {
	options := []githubsdk.ClientOptionsFunc{
		githubsdk.WithHTTPClient(httpClient),
		githubsdk.WithAuthToken(token),
	}
	if baseURL != "" {
		options = append(options, githubsdk.WithURLs(&baseURL, &baseURL))
	}
	sdk, err := githubsdk.NewClient(options...)
	return &Client{sdk: sdk, initErr: err}
}

func (c *Client) IsOrganization(ctx context.Context, login string) bool {
	if c.initErr != nil {
		return false
	}
	user, _, err := c.sdk.Users.Get(ctx, login)
	return err == nil && user.GetType() == "Organization"
}

func (c *Client) GetUser(ctx context.Context) (*provider.User, error) {
	if c.initErr != nil {
		return nil, c.initErr
	}
	user, _, err := c.sdk.Users.Get(ctx, "")
	if err != nil {
		return nil, err
	}
	return &provider.User{Login: user.GetLogin(), Type: user.GetType()}, nil
}

func (c *Client) SourceRepoURL(owner, repo string) string {
	return fmt.Sprintf("https://github.com/%s/%s", owner, repo)
}

type CreateRepoResult struct {
	Owner    string
	Name     string
	SSHURL   string
	HTTPSURL string
}

func (c *Client) CreateRepository(
	ctx context.Context,
	name string,
	private bool,
) (*CreateRepoResult, error) {
	if c.initErr != nil {
		return nil, c.initErr
	}
	repository, _, err := c.sdk.Repositories.Create(ctx, "", &githubsdk.Repository{
		Name:    githubsdk.Ptr(name),
		Private: githubsdk.Ptr(private),
	})
	if err != nil {
		return nil, err
	}
	return &CreateRepoResult{
		Owner:    repository.GetOwner().GetLogin(),
		Name:     repository.GetName(),
		SSHURL:   repository.GetSSHURL(),
		HTTPSURL: repository.GetCloneURL(),
	}, nil
}
