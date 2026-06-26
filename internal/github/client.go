package github

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"golang.org/x/crypto/nacl/box"
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

type User struct {
	Login string `json:"login"`
}

func (c *Client) GetUser(ctx context.Context) (*User, error) {
	resp, err := c.do(ctx, http.MethodGet, "/user", nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}

	var user User
	if err := json.NewDecoder(resp.Body).Decode(&user); err != nil {
		return nil, err
	}
	return &user, nil
}

type RepoPublicKey struct {
	KeyID string `json:"key_id"`
	Key   string `json:"key"`
}

func (c *Client) GetRepoPublicKey(ctx context.Context, owner, repo string) (*RepoPublicKey, error) {
	path := fmt.Sprintf("/repos/%s/%s/actions/secrets/public-key", owner, repo)
	resp, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("getting repo public key failed (status: %d)", resp.StatusCode)
	}

	var pubKey RepoPublicKey
	if err := json.NewDecoder(resp.Body).Decode(&pubKey); err != nil {
		return nil, err
	}
	return &pubKey, nil
}

type CreateSecretRequest struct {
	EncryptedValue string `json:"encrypted_value"`
	KeyID          string `json:"key_id"`
}

func (c *Client) CreateOrUpdateRepoSecret(ctx context.Context, owner, repo, secretName, secretValue string) error {
	pubKey, err := c.GetRepoPublicKey(ctx, owner, repo)
	if err != nil {
		return err
	}

	decodedKey, err := base64.StdEncoding.DecodeString(pubKey.Key)
	if err != nil {
		return fmt.Errorf("decoding public key: %w", err)
	}

	var peerKey [32]byte
	copy(peerKey[:], decodedKey)

	encryptedBytes, err := box.SealAnonymous(nil, []byte(secretValue), &peerKey, rand.Reader)
	if err != nil {
		return fmt.Errorf("encrypting secret: %w", err)
	}

	encryptedVal := base64.StdEncoding.EncodeToString(encryptedBytes)

	reqBody := CreateSecretRequest{
		EncryptedValue: encryptedVal,
		KeyID:          pubKey.KeyID,
	}

	jsonBytes, err := json.Marshal(reqBody)
	if err != nil {
		return err
	}

	path := fmt.Sprintf("/repos/%s/%s/actions/secrets/%s", owner, repo, secretName)
	resp, err := c.do(ctx, http.MethodPut, path, bytes.NewReader(jsonBytes))
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusNoContent {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to create/update secret (status: %d, body: %s)", resp.StatusCode, string(bodyBytes))
	}

	return nil
}
