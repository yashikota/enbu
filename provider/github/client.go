package github

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

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

func (c *Client) GetUserTeams(ctx context.Context, org, username string) ([]string, error) {
	teams, err := c.listOrgTeams(ctx, org)
	if err != nil {
		return nil, fmt.Errorf("listing org teams: %w", err)
	}

	var memberTeams []string
	for _, slug := range teams {
		member, err := c.isTeamMember(ctx, org, slug, username)
		if err != nil {
			continue
		}
		if member {
			memberTeams = append(memberTeams, slug)
		}
	}
	return memberTeams, nil
}

const perPage = 100

func (c *Client) listOrgTeams(ctx context.Context, org string) ([]string, error) {
	var allTeams []string
	page := 1
	for {
		path := fmt.Sprintf("/orgs/%s/teams?per_page=%d&page=%d", org, perPage, page)
		resp, err := c.do(ctx, http.MethodGet, path, nil)
		if err != nil {
			return nil, err
		}

		if resp.StatusCode != http.StatusOK {
			_ = resp.Body.Close()
			return nil, fmt.Errorf("list teams: status %d", resp.StatusCode)
		}

		var teams []struct {
			Slug string `json:"slug"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&teams); err != nil {
			_ = resp.Body.Close()
			return nil, err
		}
		_ = resp.Body.Close()

		for _, t := range teams {
			allTeams = append(allTeams, t.Slug)
		}

		if len(teams) < perPage {
			break
		}
		page++
	}
	return allTeams, nil
}

func (c *Client) isTeamMember(ctx context.Context, org, teamSlug, username string) (bool, error) {
	path := fmt.Sprintf("/orgs/%s/teams/%s/memberships/%s", org, teamSlug, username)
	resp, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return false, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusNotFound {
		return false, nil
	}
	if resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("check membership: status %d", resp.StatusCode)
	}

	var m struct {
		State string `json:"state"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&m); err != nil {
		return false, err
	}
	return m.State == "active", nil
}

func (c *Client) GetCollaboratorPermission(ctx context.Context, owner, repo, username string) (string, error) {
	path := fmt.Sprintf("/repos/%s/%s/collaborators/%s/permission", owner, repo, username)
	resp, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("get permission: status %d", resp.StatusCode)
	}

	var p struct {
		Permission string `json:"permission"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&p); err != nil {
		return "", err
	}
	return p.Permission, nil
}
