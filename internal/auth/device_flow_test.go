package auth

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRequestDeviceCode(t *testing.T) {
	expected := DeviceCodeResponse{
		DeviceCode:      "test-device-code",
		UserCode:        "ABCD-1234",
		VerificationURI: "https://github.com/login/device",
		ExpiresIn:       900,
		Interval:        5,
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.Header.Get("Accept") != "application/json" {
			t.Errorf("expected Accept: application/json")
		}
		if err := r.ParseForm(); err != nil {
			t.Fatal(err)
		}
		if got := r.FormValue("client_id"); got != "test-client" {
			t.Errorf("client_id = %q, want %q", got, "test-client")
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(expected)
	}))
	defer server.Close()

	origURL := githubDeviceCodeURL
	defer func() { githubDeviceCodeURL = origURL }()
	githubDeviceCodeURL = server.URL

	resp, err := RequestDeviceCode(context.Background(), "test-client")
	if err != nil {
		t.Fatalf("RequestDeviceCode: %v", err)
	}
	if resp.DeviceCode != expected.DeviceCode {
		t.Errorf("DeviceCode = %q, want %q", resp.DeviceCode, expected.DeviceCode)
	}
	if resp.UserCode != expected.UserCode {
		t.Errorf("UserCode = %q, want %q", resp.UserCode, expected.UserCode)
	}
}

func TestPollForToken_Success(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.Header().Set("Content-Type", "application/json")
		if callCount < 3 {
			_ = json.NewEncoder(w).Encode(TokenResponse{Error: "authorization_pending"})
			return
		}
		_ = json.NewEncoder(w).Encode(TokenResponse{
			AccessToken: "gho_test_token",
			TokenType:   "bearer",
			Scope:       "repo",
		})
	}))
	defer server.Close()

	origURL := githubTokenURL
	defer func() { githubTokenURL = origURL }()
	githubTokenURL = server.URL

	resp, err := PollForToken(context.Background(), "test-client", "device-code", 1)
	if err != nil {
		t.Fatalf("PollForToken: %v", err)
	}
	if resp.AccessToken != "gho_test_token" {
		t.Errorf("AccessToken = %q, want %q", resp.AccessToken, "gho_test_token")
	}
	if callCount < 3 {
		t.Errorf("expected at least 3 calls, got %d", callCount)
	}
}

func TestPollForToken_Expired(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(TokenResponse{Error: "expired_token"})
	}))
	defer server.Close()

	origURL := githubTokenURL
	defer func() { githubTokenURL = origURL }()
	githubTokenURL = server.URL

	_, err := PollForToken(context.Background(), "test-client", "device-code", 1)
	if err == nil {
		t.Fatal("expected error for expired token")
	}
}

func TestPollForToken_AccessDenied(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(TokenResponse{Error: "access_denied"})
	}))
	defer server.Close()

	origURL := githubTokenURL
	defer func() { githubTokenURL = origURL }()
	githubTokenURL = server.URL

	_, err := PollForToken(context.Background(), "test-client", "device-code", 1)
	if err == nil {
		t.Fatal("expected error for access denied")
	}
}
