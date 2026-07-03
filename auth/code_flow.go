package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

var authorizeURL = "https://github.com/login/oauth/authorize"

func AuthorizeURL(clientID, state string) string {
	params := url.Values{
		"client_id": {clientID},
		"scope":     {strings.Join(requiredScopes, " ")},
		"state":     {state},
	}
	return authorizeURL + "?" + params.Encode()
}

func ExchangeCode(ctx context.Context, clientID, clientSecret, code string) (*TokenResponse, error) {
	data := url.Values{
		"client_id":     {clientID},
		"client_secret": {clientSecret},
		"code":          {code},
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("exchanging code: unexpected status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var result TokenResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("parsing token response: %w", err)
	}

	if result.Error != "" {
		return nil, fmt.Errorf("OAuth error: %s", result.Error)
	}

	return &result, nil
}
