package github

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
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
		json.NewEncoder(w).Encode(User{Login: "testuser"})
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
