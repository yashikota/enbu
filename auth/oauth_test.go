package auth

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestNewSecrets(t *testing.T) {
	poll1, hash1, verifier1, challenge1, err := newSecrets()
	if err != nil {
		t.Fatal(err)
	}
	poll2, _, verifier2, _, err := newSecrets()
	if err != nil {
		t.Fatal(err)
	}
	if poll1 == poll2 || verifier1 == verifier2 {
		t.Fatal("generated secrets were reused")
	}
	if len(poll1) != 43 || len(verifier1) != 43 || len(challenge1) != 43 {
		t.Fatalf("unexpected lengths: poll=%d verifier=%d challenge=%d", len(poll1), len(verifier1), len(challenge1))
	}
	pollSum := sha256.Sum256([]byte(poll1))
	if hash1 != hex.EncodeToString(pollSum[:]) || len(hash1) != 64 {
		t.Fatal("poll secret hash is invalid")
	}
	verifierSum := sha256.Sum256([]byte(verifier1))
	if challenge1 != base64.RawURLEncoding.EncodeToString(verifierSum[:]) {
		t.Fatal("PKCE challenge is invalid")
	}
}

func TestCallbackRejectsInvalidRequestsThenAcceptsValidCallback(t *testing.T) {
	sessionID := strings.Repeat("c", 32)
	state := sessionID + ":" + strings.Repeat("b", 32)
	results := make(chan callbackResult, 1)
	server := newCallbackServer(state, results)

	tests := []string{
		"?code=ok",
		"?code=ok&state=wrong",
		"?code=one&code=two&state=" + state,
		"?code=ok&state=" + state + "&state=" + state,
		"?code=" + strings.Repeat("x", maxOAuthCodeLength+1) + "&state=" + state,
	}
	for _, rawQuery := range tests {
		req := httptest.NewRequest(http.MethodGet, "http://127.0.0.1"+callbackPath+rawQuery, nil)
		req.RemoteAddr = "127.0.0.1:12345"
		response := httptest.NewRecorder()
		server.Handler.ServeHTTP(response, req)
		if response.Code != http.StatusBadRequest {
			t.Fatalf("query %q returned %d", rawQuery, response.Code)
		}
		select {
		case <-results:
			t.Fatalf("query %q delivered a callback", rawQuery)
		default:
		}
	}

	req := httptest.NewRequest(http.MethodGet, "http://127.0.0.1"+callbackPath+"?code=secret-code&state="+state, nil)
	req.RemoteAddr = "127.0.0.1:12345"
	response := httptest.NewRecorder()
	server.Handler.ServeHTTP(response, req)
	if response.Code != http.StatusOK {
		t.Fatalf("valid callback returned %d", response.Code)
	}
	result := <-results
	if result.code != "secret-code" || result.state != state || result.denied {
		t.Fatalf("unexpected callback result: %#v", result)
	}
	if strings.Contains(response.Body.String(), "secret-code") || strings.Contains(response.Body.String(), state) {
		t.Fatal("callback HTML leaked query values")
	}
	second := httptest.NewRecorder()
	server.Handler.ServeHTTP(second, req)
	if second.Code != http.StatusConflict {
		t.Fatalf("duplicate callback returned %d", second.Code)
	}
	for _, header := range []string{"Cache-Control", "Pragma", "Referrer-Policy", "X-Content-Type-Options"} {
		if response.Header().Get(header) == "" {
			t.Fatalf("missing callback header %s", header)
		}
	}
}

func TestCallbackRejectsNonLoopbackAndWrongMethod(t *testing.T) {
	sessionID := strings.Repeat("c", 32)
	state := sessionID + ":" + strings.Repeat("b", 32)
	server := newCallbackServer(state, make(chan callbackResult, 1))
	for _, tc := range []struct {
		method string
		path   string
		remote string
	}{
		{http.MethodPost, callbackPath, "127.0.0.1:1234"},
		{http.MethodGet, "/wrong", "127.0.0.1:1234"},
		{http.MethodGet, callbackPath, "192.0.2.1:1234"},
		{http.MethodGet, callbackPath, "[::1]:1234"},
	} {
		req := httptest.NewRequest(tc.method, "http://127.0.0.1"+tc.path+"?code=x&state="+state, nil)
		req.RemoteAddr = tc.remote
		response := httptest.NewRecorder()
		server.Handler.ServeHTTP(response, req)
		if response.Code != http.StatusBadRequest {
			t.Fatalf("%#v returned %d", tc, response.Code)
		}
	}
}

func TestCallbackDenialDoesNotLeakDescription(t *testing.T) {
	state := strings.Repeat("a", 32) + ":" + strings.Repeat("b", 32)
	results := make(chan callbackResult, 1)
	server := newCallbackServer(state, results)
	req := httptest.NewRequest(
		http.MethodGet,
		"http://127.0.0.1"+callbackPath+"?error=access_denied&error_description=private&state="+state,
		nil,
	)
	req.RemoteAddr = "127.0.0.1:12345"
	response := httptest.NewRecorder()
	server.Handler.ServeHTTP(response, req)
	if response.Code != http.StatusOK || !((<-results).denied) {
		t.Fatalf("denial callback returned %d", response.Code)
	}
	if strings.Contains(response.Body.String(), "private") || strings.Contains(response.Body.String(), state) {
		t.Fatal("denial HTML leaked callback values")
	}
}

func TestOAuthLoginEndToEnd(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	stubKeyring(t)
	originalUser := getGitHubUser
	getGitHubUser = func(context.Context, string) (string, int64, error) { return "octo", 123, nil }
	t.Cleanup(func() { getGitHubUser = originalUser })

	sessionID := strings.Repeat("c", 32)
	state := sessionID + ":" + strings.Repeat("b", 32)
	var redirectURI string
	var pollSecret string
	var mu sync.Mutex
	broker := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/v1/oauth/sessions":
			var body createSessionRequest
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Error(err)
			}
			mu.Lock()
			redirectURI = body.RedirectURI
			mu.Unlock()
			if !strings.HasPrefix(redirectURI, "http://127.0.0.1:") {
				t.Errorf("redirect URI = %q", redirectURI)
			}
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(createSessionResponse{
				SessionID:    sessionID,
				AuthorizeURL: "https://github.com/login/oauth/authorize?state=" + state,
				State:        state,
				ExpiresAt:    time.Now().Add(10 * time.Minute).UnixMilli(),
			})
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/exchange"):
			pollSecret = strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
			var body exchangeRequest
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Error(err)
			}
			if body.Code != "github-code" || body.State != state || len(body.CodeVerifier) != 43 {
				t.Errorf("invalid exchange request")
			}
			_ = json.NewEncoder(w).Encode(exchangeResponse{
				AccessToken: "github-token",
				TokenType:   "Bearer",
				Scope:       "repo,read:org,write:packages,read:packages",
			})
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer broker.Close()

	client := newOAuthClient(broker.URL, broker.Client())
	token, err := client.login(context.Background(), func(string) error {
		mu.Lock()
		callbackURL := redirectURI
		mu.Unlock()
		response, err := http.Get(callbackURL + "?code=github-code&state=" + url.QueryEscape(state))
		if err == nil {
			_ = response.Body.Close()
		}
		return err
	})
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	if token.Username != "octo" || token.UserID != 123 || token.AccessToken != "github-token" {
		t.Fatalf("stored token = %#v", token)
	}
	if len(pollSecret) != 43 {
		t.Fatalf("poll secret length = %d", len(pollSecret))
	}
}

func TestOAuthLoginCancelsSessionWhenBrowserFails(t *testing.T) {
	sessionID := strings.Repeat("c", 32)
	state := sessionID + ":" + strings.Repeat("b", 32)
	cancelled := make(chan struct{}, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.Method {
		case http.MethodPost:
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(createSessionResponse{
				SessionID:    sessionID,
				AuthorizeURL: "https://github.com/login/oauth/authorize?state=" + state,
				State:        state,
				ExpiresAt:    time.Now().Add(10 * time.Minute).UnixMilli(),
			})
		case http.MethodDelete:
			if !strings.HasPrefix(r.Header.Get("Authorization"), "Bearer ") {
				t.Error("cancel request has no bearer token")
			}
			cancelled <- struct{}{}
			_ = json.NewEncoder(w).Encode(map[string]bool{"cancelled": true})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	client := newOAuthClient(server.URL, server.Client())
	_, err := client.login(context.Background(), func(string) error { return io.ErrClosedPipe })
	if err == nil || err.Error() != "opening browser failed" {
		t.Fatalf("login error = %v", err)
	}
	select {
	case <-cancelled:
	case <-time.After(time.Second):
		t.Fatal("OAuth session was not cancelled")
	}
}

func TestOAuthLoginCancellationWhileWaitingForCallback(t *testing.T) {
	sessionID := strings.Repeat("c", 32)
	state := sessionID + ":" + strings.Repeat("d", 32)
	cancelled := make(chan struct{}, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.Method {
		case http.MethodPost:
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(createSessionResponse{
				SessionID:    sessionID,
				AuthorizeURL: "https://github.com/login/oauth/authorize?state=" + state,
				State:        state,
				ExpiresAt:    time.Now().Add(10 * time.Minute).UnixMilli(),
			})
		case http.MethodDelete:
			cancelled <- struct{}{}
			_ = json.NewEncoder(w).Encode(map[string]bool{"cancelled": true})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	client := newOAuthClient(server.URL, server.Client())
	_, err := client.login(ctx, func(string) error {
		cancel()
		return nil
	})
	if err == nil || !strings.Contains(err.Error(), context.Canceled.Error()) {
		t.Fatalf("login error = %v", err)
	}
	select {
	case <-cancelled:
	case <-time.After(time.Second):
		t.Fatal("OAuth session was not cancelled")
	}
}

func TestOAuthHTTPStagesHonorTimeouts(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(200 * time.Millisecond)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusGatewayTimeout)
	}))
	defer server.Close()

	httpClient := server.Client()
	httpClient.Timeout = 25 * time.Millisecond
	client := newOAuthClient(server.URL, httpClient)
	started := time.Now()
	_, err := client.createSession(context.Background(), createSessionRequest{})
	if err == nil || time.Since(started) > 150*time.Millisecond {
		t.Fatalf("session creation timeout err=%v elapsed=%s", err, time.Since(started))
	}

	started = time.Now()
	_, err = client.exchange(
		context.Background(),
		strings.Repeat("a", 32),
		"poll-secret",
		exchangeRequest{Code: "code", State: "state", CodeVerifier: "verifier"},
		time.Now().Add(time.Minute),
	)
	if err == nil || time.Since(started) > 150*time.Millisecond {
		t.Fatalf("exchange timeout err=%v elapsed=%s", err, time.Since(started))
	}

	started = time.Now()
	client.cancelSession(strings.Repeat("a", 32), "poll-secret")
	if time.Since(started) > 150*time.Millisecond {
		t.Fatalf("cancel timeout elapsed=%s", time.Since(started))
	}
}

func TestOAuthErrorsDoNotExposeSecrets(t *testing.T) {
	secrets := []string{"poll-secret", "authorization-code", "state-secret", "pkce-verifier", "access-token"}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadGateway)
		_, _ = io.WriteString(w, strings.Join(secrets, " "))
	}))
	defer server.Close()

	client := newOAuthClient(server.URL, server.Client())
	_, err := client.exchange(
		context.Background(),
		strings.Repeat("a", 32),
		secrets[0],
		exchangeRequest{Code: secrets[1], State: secrets[2], CodeVerifier: secrets[3]},
		time.Now().Add(time.Minute),
	)
	if err == nil {
		t.Fatal("exchange unexpectedly succeeded")
	}
	for _, secret := range secrets {
		if strings.Contains(err.Error(), secret) {
			t.Fatalf("error leaked secret %q", secret)
		}
	}
}

func TestExchangeRetriesOnlyRateLimit(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		w.Header().Set("Content-Type", "application/json")
		if calls == 1 {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		_ = json.NewEncoder(w).Encode(exchangeResponse{
			AccessToken: "token",
			TokenType:   "bearer",
			Scope:       "repo read:org write:packages read:packages",
		})
	}))
	defer server.Close()
	client := newOAuthClient(server.URL, server.Client())
	_, err := client.exchange(context.Background(), strings.Repeat("a", 32), "poll", exchangeRequest{}, time.Now().Add(time.Minute))
	if err != nil || calls != 2 {
		t.Fatalf("exchange err=%v calls=%d", err, calls)
	}

	calls = 0
	badGatewayServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer badGatewayServer.Close()
	client = newOAuthClient(badGatewayServer.URL, badGatewayServer.Client())
	_, err = client.exchange(context.Background(), strings.Repeat("a", 32), "poll", exchangeRequest{}, time.Now().Add(time.Minute))
	if err == nil || calls != 1 {
		t.Fatalf("5xx exchange err=%v calls=%d", err, calls)
	}
}

func TestRetryAfterAcceptsPastHTTPDate(t *testing.T) {
	now := time.Now()
	delay, ok := retryAfter(now.Add(-time.Second).UTC().Format(http.TimeFormat), now)
	if !ok || delay != 0 {
		t.Fatalf("retryAfter returned delay=%s ok=%v", delay, ok)
	}
}

func TestRetryDelayUsesBoundedExponentialMinimum(t *testing.T) {
	tests := []struct {
		serverDelay time.Duration
		attempt     int
		want        time.Duration
	}{
		{attempt: 0, want: 100 * time.Millisecond},
		{attempt: 1, want: 200 * time.Millisecond},
		{attempt: 20, want: 5 * time.Second},
		{serverDelay: 10 * time.Second, attempt: 20, want: 10 * time.Second},
	}
	for _, tt := range tests {
		if got := retryDelay(tt.serverDelay, tt.attempt); got != tt.want {
			t.Fatalf("retryDelay(%s, %d) = %s, want %s", tt.serverDelay, tt.attempt, got, tt.want)
		}
	}
}

func TestValidAuthorizeURLRequiresMatchingSingleState(t *testing.T) {
	state := strings.Repeat("a", 32) + ":" + strings.Repeat("b", 32)
	base := "https://github.com/login/oauth/authorize"
	tests := []struct {
		name string
		url  string
		want bool
	}{
		{name: "valid", url: base + "?state=" + state, want: true},
		{name: "missing", url: base},
		{name: "duplicate", url: base + "?state=" + state + "&state=" + state},
		{name: "mismatch", url: base + "?state=wrong"},
		{name: "wrong host", url: "https://example.com/login/oauth/authorize?state=" + state},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := validAuthorizeURL(tt.url, state); got != tt.want {
				t.Fatalf("validAuthorizeURL() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestValidateTokenRequiresBearerAndAllScopes(t *testing.T) {
	valid := exchangeResponse{
		AccessToken: "token",
		TokenType:   "BEARER",
		Scope:       "repo read:org write:packages read:packages",
	}
	if err := validateToken(valid); err != nil {
		t.Fatalf("valid token rejected: %v", err)
	}
	for _, mutate := range []func(*exchangeResponse){
		func(token *exchangeResponse) { token.AccessToken = "" },
		func(token *exchangeResponse) { token.TokenType = "mac" },
		func(token *exchangeResponse) { token.Scope = "repo read:org write:packages" },
	} {
		token := valid
		mutate(&token)
		if err := validateToken(token); err == nil {
			t.Fatalf("invalid token accepted: %#v", token)
		}
	}
}

func TestDecodeJSONRejectsOversizeAndUnknownFields(t *testing.T) {
	for _, body := range []string{
		`{"access_token":"token","token_type":"bearer","scope":"repo","extra":true}`,
		`{"value":"` + strings.Repeat("x", maxResponseBytes) + `"}`,
	} {
		response := &http.Response{
			Header: http.Header{"Content-Type": []string{"application/json"}},
			Body:   io.NopCloser(strings.NewReader(body)),
		}
		var result exchangeResponse
		if err := decodeJSON(response, &result); err == nil {
			t.Fatalf("decodeJSON accepted invalid body")
		}
	}
}
