package github

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/yashikota/enbu/provider"
)

func TestGetUser(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/user" {
			t.Errorf("path = %q, want /user", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-token" {
			t.Errorf("Authorization = %q, want %q", got, "Bearer test-token")
		}
		if got := r.Header.Get("X-GitHub-Api-Version"); got != "2022-11-28" {
			t.Errorf("API version = %q, want 2022-11-28", got)
		}
		_ = json.NewEncoder(w).Encode(provider.User{Login: "testuser"})
	}))
	defer server.Close()

	client := &Client{
		token:      "test-token",
		httpClient: http.DefaultClient,
	}
	origBase := apiBaseURL
	defer func() { apiBaseURL = origBase }()
	apiBaseURL = server.URL

	user, err := client.GetUser(context.Background())
	if err != nil {
		t.Fatalf("GetUser: %v", err)
	}
	if user.Login != "testuser" {
		t.Errorf("Login = %q, want %q", user.Login, "testuser")
	}
}

func TestIsOrganization(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/users/my-org" {
			_ = json.NewEncoder(w).Encode(map[string]string{"type": "Organization", "login": "my-org"})
			return
		}
		if r.URL.Path == "/users/my-user" {
			_ = json.NewEncoder(w).Encode(map[string]string{"type": "User", "login": "my-user"})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	client := &Client{
		token:      "test-token",
		httpClient: http.DefaultClient,
	}
	origBase := apiBaseURL
	defer func() { apiBaseURL = origBase }()
	apiBaseURL = server.URL

	if !client.IsOrganization(context.Background(), "my-org") {
		t.Error("expected my-org to be detected as Organization")
	}
	if client.IsOrganization(context.Background(), "my-user") {
		t.Error("expected my-user to not be detected as Organization")
	}
	if client.IsOrganization(context.Background(), "nonexistent") {
		t.Error("expected nonexistent to not be detected as Organization")
	}
}

func TestGetUserUnauthorized(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()

	client := &Client{
		token:      "bad-token",
		httpClient: http.DefaultClient,
	}
	origBase := apiBaseURL
	defer func() { apiBaseURL = origBase }()
	apiBaseURL = server.URL

	_, err := client.GetUser(context.Background())
	if err == nil {
		t.Fatal("expected error for unauthorized request")
	}
}

func TestGetUserTeams(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/orgs/myorg/teams":
			_ = json.NewEncoder(w).Encode([]map[string]string{
				{"slug": "backend"},
				{"slug": "infra"},
			})
		case "/orgs/myorg/teams/backend/memberships/alice":
			_ = json.NewEncoder(w).Encode(map[string]string{"state": "active"})
		case "/orgs/myorg/teams/infra/memberships/alice":
			_ = json.NewEncoder(w).Encode(map[string]string{"state": "active"})
		case "/orgs/myorg/teams/backend/memberships/bob":
			_ = json.NewEncoder(w).Encode(map[string]string{"state": "active"})
		case "/orgs/myorg/teams/infra/memberships/bob":
			w.WriteHeader(http.StatusNotFound)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	client := &Client{token: "test-token", httpClient: http.DefaultClient}
	origBase := apiBaseURL
	defer func() { apiBaseURL = origBase }()
	apiBaseURL = server.URL

	teams, err := client.GetUserTeams(context.Background(), "myorg", "alice")
	if err != nil {
		t.Fatalf("GetUserTeams(alice): %v", err)
	}
	if len(teams) != 2 {
		t.Fatalf("expected 2 teams for alice, got %d: %v", len(teams), teams)
	}

	teams, err = client.GetUserTeams(context.Background(), "myorg", "bob")
	if err != nil {
		t.Fatalf("GetUserTeams(bob): %v", err)
	}
	if len(teams) != 1 || teams[0] != "backend" {
		t.Fatalf("expected [backend] for bob, got %v", teams)
	}
}

func TestGetCollaboratorPermission(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/owner/repo/collaborators/admin-user/permission":
			_ = json.NewEncoder(w).Encode(map[string]string{"permission": "admin"})
		case "/repos/owner/repo/collaborators/write-user/permission":
			_ = json.NewEncoder(w).Encode(map[string]string{"permission": "write"})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	client := &Client{token: "test-token", httpClient: http.DefaultClient}
	origBase := apiBaseURL
	defer func() { apiBaseURL = origBase }()
	apiBaseURL = server.URL

	perm, err := client.GetCollaboratorPermission(context.Background(), "owner", "repo", "admin-user")
	if err != nil {
		t.Fatalf("GetCollaboratorPermission(admin-user): %v", err)
	}
	if perm != "admin" {
		t.Fatalf("expected admin, got %s", perm)
	}

	perm, err = client.GetCollaboratorPermission(context.Background(), "owner", "repo", "write-user")
	if err != nil {
		t.Fatalf("GetCollaboratorPermission(write-user): %v", err)
	}
	if perm != "write" {
		t.Fatalf("expected write, got %s", perm)
	}

	_, err = client.GetCollaboratorPermission(context.Background(), "owner", "repo", "unknown")
	if err == nil {
		t.Fatal("expected error for unknown user")
	}
}
