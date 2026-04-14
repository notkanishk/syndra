package zitadel

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"
)

// MgmtClient is the live Zitadel Management API client.
// nil when credentials are absent (local-policy-only mode).
var MgmtClient ZitadelClient

// managementClient implements ZitadelClient using direct HTTP calls
// to the Zitadel Management API v1 endpoints.
type managementClient struct {
	domain string
	tokens *tokenManager
	http   *http.Client
}

// apiError captures a Zitadel API error response.
type apiError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (e *apiError) Error() string {
	return fmt.Sprintf("zitadel api %d: %s", e.Code, e.Message)
}

// InitClient establishes the Service Account connection to Zitadel's Management API.
// If ZITADEL_DOMAIN or ZITADEL_MACHINE_KEY_PATH are not set, it short-circuits
// gracefully — the system operates in local-policy-only mode.
func InitClient() error {
	domain := os.Getenv("ZITADEL_DOMAIN")
	keyPath := os.Getenv("ZITADEL_MACHINE_KEY_PATH")

	if domain == "" || keyPath == "" {
		log.Println("[ZITADEL] M2M credentials not set; operating in local-policy-only mode.")
		return nil
	}

	saKey, privKey, err := LoadServiceAccountKey(keyPath)
	if err != nil {
		return fmt.Errorf("load service account key: %w", err)
	}

	tm := newTokenManager(domain, saKey, privKey)

	MgmtClient = &managementClient{
		domain: domain,
		tokens: tm,
		http:   &http.Client{Timeout: 10 * time.Second},
	}

	log.Printf("[ZITADEL] Management client initialized for domain=%s (user=%s)", domain, saKey.UserID)
	return nil
}

// --- ZitadelClient implementation ---

func (c *managementClient) AddUserGrant(ctx context.Context, userID, projectID string, roleKeys []string) error {
	path := fmt.Sprintf("/management/v1/users/%s/grants", userID)
	body := map[string]any{
		"projectId": projectID,
		"roleKeys":  roleKeys,
	}

	resp, err := c.doRequest(ctx, http.MethodPost, path, body)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}

func (c *managementClient) UpdateUserGrant(ctx context.Context, userID, grantID string, roleKeys []string) error {
	path := fmt.Sprintf("/management/v1/users/%s/grants/%s", userID, grantID)
	body := map[string]any{
		"roleKeys": roleKeys,
	}

	resp, err := c.doRequest(ctx, http.MethodPut, path, body)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}

func (c *managementClient) RemoveUserGrant(ctx context.Context, userID, grantID string) error {
	path := fmt.Sprintf("/management/v1/users/%s/grants/%s", userID, grantID)

	resp, err := c.doRequest(ctx, http.MethodDelete, path, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}

func (c *managementClient) ListUserGrants(ctx context.Context, userID string) ([]UserGrant, error) {
	// Zitadel v1: POST /management/v1/users/grants/_search with user ID as a query filter.
	path := "/management/v1/users/grants/_search"
	body := map[string]any{
		"queries": []map[string]any{
			{
				"userIdQuery": map[string]string{
					"userId": userID,
				},
			},
		},
	}

	resp, err := c.doRequest(ctx, http.MethodPost, path, body)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("read list grants response: %w", err)
	}

	var result struct {
		Result []UserGrant `json:"result"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("decode list grants response: %w", err)
	}
	return result.Result, nil
}

func (c *managementClient) AddProjectRole(ctx context.Context, projectID, roleKey, displayName, group string) error {
	path := fmt.Sprintf("/management/v1/projects/%s/roles", projectID)
	body := map[string]any{
		"roleKey":     roleKey,
		"displayName": displayName,
		"group":       group,
	}

	resp, err := c.doRequest(ctx, http.MethodPost, path, body)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}

func (c *managementClient) ListProjectRoles(ctx context.Context, projectID string) ([]ProjectRoleResult, error) {
	path := fmt.Sprintf("/management/v1/projects/%s/roles/_search", projectID)
	body := map[string]any{}

	resp, err := c.doRequest(ctx, http.MethodPost, path, body)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("read list project roles response: %w", err)
	}

	var result struct {
		Result []ProjectRoleResult `json:"result"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("decode list project roles response: %w", err)
	}
	return result.Result, nil
}

func (c *managementClient) UpdateProjectRole(ctx context.Context, projectID, roleKey, displayName, group string) error {
	path := fmt.Sprintf("/management/v1/projects/%s/roles/%s", projectID, roleKey)
	body := map[string]any{
		"displayName": displayName,
		"group":       group,
	}

	resp, err := c.doRequest(ctx, http.MethodPut, path, body)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}

func (c *managementClient) GetUser(ctx context.Context, userID string) (*ZitadelUser, error) {
	path := fmt.Sprintf("/management/v1/users/%s", userID)

	resp, err := c.doRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("read get user response: %w", err)
	}

	// Zitadel v1 nests human user data: user.human.profile.displayName, user.human.email.email.
	var raw struct {
		User struct {
			ID       string `json:"id"`
			UserName string `json:"userName"`
			State    string `json:"state"`
			Human    struct {
				Profile struct {
					DisplayName string `json:"displayName"`
				} `json:"profile"`
				Email struct {
					Email string `json:"email"`
				} `json:"email"`
			} `json:"human"`
		} `json:"user"`
	}
	if err := json.Unmarshal(respBody, &raw); err != nil {
		return nil, fmt.Errorf("decode get user response: %w", err)
	}

	return &ZitadelUser{
		ID:          raw.User.ID,
		Username:    raw.User.UserName,
		DisplayName: raw.User.Human.Profile.DisplayName,
		Email:       raw.User.Human.Email.Email,
		State:       raw.User.State,
	}, nil
}

// --- HTTP transport layer ---

const maxRetries = 3

// doRequest executes an authenticated request to the Zitadel Management API.
// Handles token injection, 401 refresh-and-retry, and retries on 429/503.
func (c *managementClient) doRequest(ctx context.Context, method, path string, body any) (*http.Response, error) {
	var reqBody []byte
	if body != nil {
		var err error
		reqBody, err = json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("marshal request body: %w", err)
		}
	}

	var lastErr error
	backoff := 100 * time.Millisecond

	for attempt := 0; attempt < maxRetries; attempt++ {
		token, err := c.tokens.Token(ctx)
		if err != nil {
			return nil, fmt.Errorf("obtain access token: %w", err)
		}

		url := fmt.Sprintf("https://%s%s", c.domain, path)
		var bodyReader io.Reader
		if reqBody != nil {
			bodyReader = bytes.NewReader(reqBody)
		}

		req, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
		if err != nil {
			return nil, fmt.Errorf("build request: %w", err)
		}
		req.Header.Set("Authorization", "Bearer "+token)
		if reqBody != nil {
			req.Header.Set("Content-Type", "application/json")
		}
		req.Header.Set("Accept", "application/json")

		resp, err := httpDo(c.http, req)
		if err != nil {
			return nil, fmt.Errorf("execute request: %w", err)
		}

		switch {
		case resp.StatusCode >= 200 && resp.StatusCode < 300:
			return resp, nil

		case resp.StatusCode == http.StatusUnauthorized:
			resp.Body.Close()
			if attempt == 0 {
				// Token may be expired — force refresh and retry once.
				c.tokens.ForceRefresh()
				continue
			}
			return nil, fmt.Errorf("unauthorized after token refresh (401)")

		case resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode == http.StatusServiceUnavailable:
			resp.Body.Close()
			lastErr = fmt.Errorf("retryable status %d on %s %s", resp.StatusCode, method, path)
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff):
				backoff *= 2
				continue
			}

		default:
			// Non-retryable error — read body for diagnostics and return.
			defer resp.Body.Close()
			errBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
			var ae apiError
			if json.Unmarshal(errBody, &ae) == nil && ae.Message != "" {
				return nil, &ae
			}
			return nil, fmt.Errorf("zitadel api %d: %s", resp.StatusCode, errBody)
		}
	}

	if lastErr != nil {
		return nil, fmt.Errorf("max retries exceeded: %w", lastErr)
	}
	return nil, fmt.Errorf("max retries exceeded")
}
