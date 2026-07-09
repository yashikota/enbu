package github

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/yashikota/enbu/provider"
)

var apiBaseURL = "https://api.github.com"

type Client struct {
	token      string
	httpClient *http.Client
}

func NewClient(token string) *Client {
	return &Client{
		token:      token,
		httpClient: http.DefaultClient,
	}
}

func (c *Client) do(ctx context.Context, method, path string, body io.Reader) (*http.Response, error) {
	url := apiBaseURL + path
	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	return c.httpClient.Do(req)
}

func (c *Client) IsOrganization(ctx context.Context, login string) bool {
	resp, err := c.do(ctx, http.MethodGet, "/users/"+login, nil)
	if err != nil {
		return false
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return false
	}

	var u struct {
		Type string `json:"type"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&u); err != nil {
		return false
	}
	return u.Type == "Organization"
}

func (c *Client) GetUser(ctx context.Context) (*provider.User, error) {
	resp, err := c.do(ctx, http.MethodGet, "/user", nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}

	var user provider.User
	if err := json.NewDecoder(resp.Body).Decode(&user); err != nil {
		return nil, err
	}
	return &user, nil
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

func (c *Client) CreateRepository(ctx context.Context, name string, private bool) (*CreateRepoResult, error) {
	payload, err := json.Marshal(map[string]any{"name": name, "private": private})
	if err != nil {
		return nil, err
	}
	resp, err := c.do(ctx, http.MethodPost, "/user/repos", bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("GitHub API %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var r struct {
		Owner struct {
			Login string `json:"login"`
		} `json:"owner"`
		Name     string `json:"name"`
		SSHURL   string `json:"ssh_url"`
		CloneURL string `json:"clone_url"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		return nil, err
	}
	return &CreateRepoResult{
		Owner:    r.Owner.Login,
		Name:     r.Name,
		SSHURL:   r.SSHURL,
		HTTPSURL: r.CloneURL,
	}, nil
}
