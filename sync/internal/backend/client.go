package backend

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Client communicates with the MkAuth backend API.
type Client struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
}

// NewClient creates a backend API client.
func NewClient(baseURL, apiKey string) *Client {
	return &Client{
		baseURL: baseURL,
		apiKey:  apiKey,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// ClaimIntents atomically claims up to limit pending provisioning intents.
func (c *Client) ClaimIntents(ctx context.Context, limit int) ([]ProvisioningIntent, error) {
	url := fmt.Sprintf("%s/api/v1/intents/claim?limit=%d", c.baseURL, limit)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, nil)
	if err != nil {
		return nil, fmt.Errorf("build claim request: %w", err)
	}
	c.setAuth(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("claim intents: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, c.readError(resp)
	}

	var intents []ProvisioningIntent
	if err := json.NewDecoder(resp.Body).Decode(&intents); err != nil {
		return nil, fmt.Errorf("decode claim response: %w", err)
	}
	return intents, nil
}

// CompleteIntent marks a provisioning intent as successfully processed.
func (c *Client) CompleteIntent(ctx context.Context, intentID string) error {
	url := fmt.Sprintf("%s/api/v1/intents/%s/complete", c.baseURL, intentID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, nil)
	if err != nil {
		return fmt.Errorf("build complete request: %w", err)
	}
	c.setAuth(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("complete intent %s: %w", intentID, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return c.readError(resp)
	}
	return nil
}

// FailIntent marks a provisioning intent as failed with an error message.
func (c *Client) FailIntent(ctx context.Context, intentID, errMsg string) error {
	body, _ := json.Marshal(map[string]string{"error_message": errMsg})
	url := fmt.Sprintf("%s/api/v1/intents/%s/fail", c.baseURL, intentID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("build fail request: %w", err)
	}
	c.setAuth(req)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("fail intent %s: %w", intentID, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return c.readError(resp)
	}
	return nil
}

// GetShadowCredentialHash retrieves a user's shadow password hash.
// Returns ("", nil) when the user has no shadow credential (HTTP 404).
func (c *Client) GetShadowCredentialHash(ctx context.Context, uid string) (string, error) {
	url := fmt.Sprintf("%s/api/v1/shadow-credentials/%s/hash", c.baseURL, uid)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", fmt.Errorf("build shadow credential request: %w", err)
	}
	c.setAuth(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("get shadow credential for %s: %w", uid, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return "", nil // No credential — not an error.
	}
	if resp.StatusCode != http.StatusOK {
		return "", c.readError(resp)
	}

	var cred ShadowCredentialHash
	if err := json.NewDecoder(resp.Body).Decode(&cred); err != nil {
		return "", fmt.Errorf("decode shadow credential response: %w", err)
	}
	return cred.CredentialHash, nil
}

// GetUserProfile retrieves a user's display name and email.
func (c *Client) GetUserProfile(ctx context.Context, uid string) (UserProfile, error) {
	url := fmt.Sprintf("%s/api/v1/users/%s/profile", c.baseURL, uid)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return UserProfile{}, fmt.Errorf("build user profile request: %w", err)
	}
	c.setAuth(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return UserProfile{}, fmt.Errorf("get user profile for %s: %w", uid, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return UserProfile{}, c.readError(resp)
	}

	var profile UserProfile
	if err := json.NewDecoder(resp.Body).Decode(&profile); err != nil {
		return UserProfile{}, fmt.Errorf("decode user profile response: %w", err)
	}
	return profile, nil
}

func (c *Client) setAuth(req *http.Request) {
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
}

func (c *Client) readError(resp *http.Response) error {
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
	return fmt.Errorf("backend returned %d: %s", resp.StatusCode, string(body))
}
