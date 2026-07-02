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
	"strconv"
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

// apiError captures a Zitadel API error response body. Note: Code here is the
// JSON body code (a gRPC status code), NOT the HTTP status — see StatusError for
// the HTTP-status-carrying error that callers classify on.
type apiError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (e *apiError) Error() string {
	return fmt.Sprintf("zitadel api %d: %s", e.Code, e.Message)
}

// StatusError is the typed transport error returned by doRequest. Code is the
// HTTP status code observed on the final (non-retried) response, so callers can
// classify deterministically by status — e.g. the propagation drain treating
// 409 AlreadyExists as idempotent success and 429/408 as transient — without
// string-sniffing error text. Message carries the server-provided detail when
// available.
type StatusError struct {
	Code    int
	Message string
}

func (e *StatusError) Error() string {
	if e.Message != "" {
		return fmt.Sprintf("zitadel api %d: %s", e.Code, e.Message)
	}
	return fmt.Sprintf("zitadel api %d", e.Code)
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

func (c *managementClient) ListUserGrants(ctx context.Context, userID string, p SearchParams) (*SearchResult[UserGrant], error) {
	// Zitadel v1: POST /management/v1/users/grants/_search with user ID as a query filter.
	path := "/management/v1/users/grants/_search"
	body := map[string]any{
		"query": searchQuery(p),
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
		Details searchDetails `json:"details"`
		Result  []UserGrant   `json:"result"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("decode list grants response: %w", err)
	}
	return &SearchResult[UserGrant]{Items: result.Result, Total: result.Details.totalInt()}, nil
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

func (c *managementClient) ListProjectRoles(ctx context.Context, projectID string, p SearchParams) (*SearchResult[ProjectRoleResult], error) {
	path := fmt.Sprintf("/management/v1/projects/%s/roles/_search", projectID)
	body := map[string]any{
		"query": searchQuery(p),
	}

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
		Details searchDetails       `json:"details"`
		Result  []ProjectRoleResult `json:"result"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("decode list project roles response: %w", err)
	}
	return &SearchResult[ProjectRoleResult]{Items: result.Result, Total: result.Details.totalInt()}, nil
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

func (c *managementClient) ListUsers(ctx context.Context, p SearchParams) (*SearchResult[ZitadelUser], error) {
	path := "/management/v1/users/_search"
	body := map[string]any{
		"query": searchQuery(p),
	}

	resp, err := c.doRequest(ctx, http.MethodPost, path, body)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("read list users response: %w", err)
	}

	// Search results nest human data directly (no outer "user" wrapper per item).
	var result struct {
		Details searchDetails `json:"details"`
		Result  []struct {
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
		} `json:"result"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("decode list users response: %w", err)
	}

	users := make([]ZitadelUser, len(result.Result))
	for i, r := range result.Result {
		users[i] = ZitadelUser{
			ID:          r.ID,
			Username:    r.UserName,
			DisplayName: r.Human.Profile.DisplayName,
			Email:       r.Human.Email.Email,
			State:       r.State,
		}
	}
	return &SearchResult[ZitadelUser]{Items: users, Total: result.Details.totalInt()}, nil
}

func (c *managementClient) ListProjects(ctx context.Context, p SearchParams) (*SearchResult[ZitadelProject], error) {
	path := "/management/v1/projects/_search"
	body := map[string]any{
		"query": searchQuery(p),
	}

	resp, err := c.doRequest(ctx, http.MethodPost, path, body)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("read list projects response: %w", err)
	}

	var result struct {
		Details searchDetails `json:"details"`
		Result  []struct {
			ID    string `json:"id"`
			Name  string `json:"name"`
			State string `json:"state"`
		} `json:"result"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("decode list projects response: %w", err)
	}

	projects := make([]ZitadelProject, len(result.Result))
	for i, r := range result.Result {
		projects[i] = ZitadelProject{
			ID:    r.ID,
			Name:  r.Name,
			State: r.State,
		}
	}
	return &SearchResult[ZitadelProject]{Items: projects, Total: result.Details.totalInt()}, nil
}

func (c *managementClient) ListAllGrants(ctx context.Context, p SearchParams) (*SearchResult[UserGrant], error) {
	// Same endpoint as ListUserGrants but with no user filter — returns all org grants.
	path := "/management/v1/users/grants/_search"
	body := map[string]any{
		"query": searchQuery(p),
	}

	resp, err := c.doRequest(ctx, http.MethodPost, path, body)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("read list all grants response: %w", err)
	}

	var result struct {
		Details searchDetails `json:"details"`
		Result  []UserGrant   `json:"result"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("decode list all grants response: %w", err)
	}
	return &SearchResult[UserGrant]{Items: result.Result, Total: result.Details.totalInt()}, nil
}

func (c *managementClient) DeleteProjectRole(ctx context.Context, projectID, roleKey string) error {
	path := fmt.Sprintf("/management/v1/projects/%s/roles/%s", projectID, roleKey)

	resp, err := c.doRequest(ctx, http.MethodDelete, path, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}

// --- Search helpers ---

// searchQuery builds the pagination object for Zitadel v1 _search endpoints.
func searchQuery(p SearchParams) map[string]any {
	limit := p.Limit
	if limit <= 0 {
		limit = DefaultSearchLimit
	}
	q := map[string]any{
		"limit": limit,
		"asc":   true,
	}
	if p.Offset > 0 {
		q["offset"] = strconv.Itoa(p.Offset)
	}
	return q
}

// searchDetails captures the pagination metadata from Zitadel _search responses.
type searchDetails struct {
	TotalResult string `json:"totalResult"`
}

func (d searchDetails) totalInt() int {
	n, _ := strconv.Atoi(d.TotalResult)
	return n
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
	lastStatus := 0
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
			// Terminal: surface as a 401 StatusError so callers classify it as a
			// non-retryable failure rather than an opaque transient error.
			return nil, &StatusError{Code: http.StatusUnauthorized, Message: "unauthorized after token refresh"}

		case resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode == http.StatusServiceUnavailable:
			resp.Body.Close()
			lastStatus = resp.StatusCode
			lastErr = fmt.Errorf("retryable status %d on %s %s", resp.StatusCode, method, path)
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff):
				backoff *= 2
				continue
			}

		default:
			// Non-retryable error — read body for diagnostics and return a
			// StatusError carrying the HTTP status so callers classify by code.
			defer resp.Body.Close()
			errBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
			var ae apiError
			if json.Unmarshal(errBody, &ae) == nil && ae.Message != "" {
				return nil, &StatusError{Code: resp.StatusCode, Message: ae.Message}
			}
			return nil, &StatusError{Code: resp.StatusCode, Message: string(errBody)}
		}
	}

	// Retries exhausted on 429/503 — preserve the last HTTP status so the caller
	// can still treat it as transient (load-bearing for the propagation drain's
	// 429 handling) rather than collapsing it to an unclassifiable error.
	if lastStatus != 0 {
		return nil, &StatusError{Code: lastStatus, Message: fmt.Sprintf("max retries exceeded on %s %s", method, path)}
	}
	if lastErr != nil {
		return nil, fmt.Errorf("max retries exceeded: %w", lastErr)
	}
	return nil, fmt.Errorf("max retries exceeded")
}
