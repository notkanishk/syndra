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
	"strings"
	"time"

	"syndra/internal/extcall"
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
//
// ZITADEL_FAKE_WORLD (staging only) loads the stand-in from a world file
// instead, and only when ZITADEL_DOMAIN is unset: both set is
// ErrWorldBesideZitadel, and the caller refuses to start.
func InitClient() error {
	domain := os.Getenv("ZITADEL_DOMAIN")
	keyPath := os.Getenv("ZITADEL_MACHINE_KEY_PATH")

	if world := os.Getenv("ZITADEL_FAKE_WORLD"); world != "" {
		if domain != "" {
			return ErrWorldBesideZitadel
		}
		wc, err := NewWorldClient(world)
		if err != nil {
			return fmt.Errorf("load staging stand-in: %w", err)
		}
		MgmtClient = wc
		u, p, g := wc.Counts()
		log.Printf("[ZITADEL] Source=world (staging stand-in from %s, %d users, %d projects, %d grants)", world, u, p, g)
		return nil
	}

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

	resp, err := c.call(ctx, http.MethodPost, path, body, c.grantHolds(userID, projectID, roleKeys))
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

	resp, err := c.call(ctx, http.MethodPost, path, body, c.roleExists(projectID, roleKey))
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
			Details  struct {
				CreationDate string `json:"creationDate"`
			} `json:"details"`
			Human struct {
				Profile struct {
					DisplayName string `json:"displayName"`
					FirstName   string `json:"firstName"`
					LastName    string `json:"lastName"`
				} `json:"profile"`
				Email struct {
					Email string `json:"email"`
				} `json:"email"`
			} `json:"human"`
			Machine *struct{} `json:"machine"`
		} `json:"user"`
	}
	if err := json.Unmarshal(respBody, &raw); err != nil {
		return nil, fmt.Errorf("decode get user response: %w", err)
	}

	return &ZitadelUser{
		ID:          raw.User.ID,
		Username:    raw.User.UserName,
		DisplayName: raw.User.Human.Profile.DisplayName,
		GivenName:   raw.User.Human.Profile.FirstName,
		FamilyName:  raw.User.Human.Profile.LastName,
		Email:       raw.User.Human.Email.Email,
		State:       raw.User.State,
		Machine:     raw.User.Machine != nil,
		Created:     parseCreationDate(raw.User.Details.CreationDate),
	}, nil
}

// parseCreationDate reads Zitadel's details.creationDate. An absent or
// unreadable value is the zero time, not an error: one odd user must not
// fail the whole directory read.
func parseCreationDate(s string) time.Time {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return time.Time{}
	}
	return t
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
			Details  struct {
				CreationDate string `json:"creationDate"`
			} `json:"details"`
			Human struct {
				Profile struct {
					DisplayName string `json:"displayName"`
					FirstName   string `json:"firstName"`
					LastName    string `json:"lastName"`
				} `json:"profile"`
				Email struct {
					Email string `json:"email"`
				} `json:"email"`
			} `json:"human"`
			// A pointer so the key's presence is the answer: Zitadel sends
			// `human` or `machine` per user, never both.
			Machine *struct{} `json:"machine"`
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
			GivenName:   r.Human.Profile.FirstName,
			FamilyName:  r.Human.Profile.LastName,
			Email:       r.Human.Email.Email,
			State:       r.State,
			Machine:     r.Machine != nil,
			Created:     parseCreationDate(r.Details.CreationDate),
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

// zitadelPolicy is every Management API call's retry budget: three tries,
// 100ms doubling to a 2s cap, 10s per try. A var so tests can shorten it.
var zitadelPolicy = extcall.Policy{Attempts: 3, Base: 100 * time.Millisecond, Max: 2 * time.Second, Timeout: 10 * time.Second}

// idempotent is what may be sent twice: reads. Zitadel's searches are POSTs,
// named `_search` (v1) or `search` (v2). Every other POST, PUT and DELETE is a
// write, and a write is repeated only after a 429 (see extcall).
func idempotent(method, path string) bool {
	return method == http.MethodGet || strings.HasSuffix(path, "/_search") || strings.HasSuffix(path, "/search")
}

// doRequest executes an authenticated request to the Zitadel Management API
// through extcall: token injection, one token refresh on a 401 (on any try),
// and extcall's retry rules. A non-2xx answer comes back as a StatusError
// carrying the HTTP status so callers classify by code.
func (c *managementClient) doRequest(ctx context.Context, method, path string, body any) (*http.Response, error) {
	return c.call(ctx, method, path, body, nil)
}

// call is doRequest with a read-back for a write whose outcome is unknown.
func (c *managementClient) call(ctx context.Context, method, path string, body any, readBack func(context.Context) (bool, error)) (*http.Response, error) {
	var reqBody []byte
	if body != nil {
		var err error
		reqBody, err = json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("marshal request body: %w", err)
		}
	}

	refreshed := false // once per call, not once per first try: a 401 after a 429 still refreshes
	op := func(ctx context.Context) (*http.Response, error) {
		for {
			token, err := c.tokens.Token(ctx)
			if err != nil {
				return nil, fmt.Errorf("obtain access token: %w", err)
			}
			var bodyReader io.Reader
			if reqBody != nil {
				bodyReader = bytes.NewReader(reqBody)
			}
			req, err := http.NewRequestWithContext(ctx, method, fmt.Sprintf("https://%s%s", c.domain, path), bodyReader)
			if err != nil {
				return nil, fmt.Errorf("build request: %w", err)
			}
			req.Header.Set("Authorization", "Bearer "+token)
			if reqBody != nil {
				req.Header.Set("Content-Type", "application/json")
			}
			req.Header.Set("Accept", "application/json")
			req.Header.Set("X-Request-Id", extcall.RequestID(ctx))

			resp, err := httpDo(c.http, req)
			if err != nil {
				return nil, fmt.Errorf("execute request: %w", err)
			}
			if resp.StatusCode == http.StatusUnauthorized && !refreshed {
				// Token may be expired. A 401 is a refusal, not a processed
				// write, so refreshing and resending is safe for any method.
				resp.Body.Close()
				refreshed = true
				c.tokens.ForceRefresh()
				continue
			}
			return resp, nil
		}
	}

	resp, err := extcall.Do(ctx, zitadelPolicy, extcall.Call{
		Name:       method + " " + path,
		Idempotent: idempotent(method, path),
		ReadBack:   readBack,
		Subsystem:  "ZITADEL",
	}, op)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return resp, nil
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusUnauthorized {
		// Terminal: surface as a 401 StatusError so callers classify it as a
		// non-retryable failure rather than an opaque transient error.
		return nil, &StatusError{Code: http.StatusUnauthorized, Message: "unauthorized after token refresh"}
	}
	// Read the body for diagnostics. A retryable status that ran out of tries
	// keeps its code, so the propagation drain still treats it as transient.
	errBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	var ae apiError
	if json.Unmarshal(errBody, &ae) == nil && ae.Message != "" {
		return nil, &StatusError{Code: resp.StatusCode, Message: ae.Message}
	}
	return nil, &StatusError{Code: resp.StatusCode, Message: string(errBody)}
}

// grantHolds is AddUserGrant's read-back: does the user's grant on the project
// carry every role asked for? Only a complete read may answer yes.
func (c *managementClient) grantHolds(userID, projectID string, roleKeys []string) func(context.Context) (bool, error) {
	return func(ctx context.Context) (bool, error) {
		res, err := c.ListUserGrants(ctx, userID, SearchParams{Limit: 100})
		if err != nil {
			return false, err
		}
		if res.Total > len(res.Items) {
			return false, fmt.Errorf("incomplete grant read: %d of %d", len(res.Items), res.Total)
		}
		for _, g := range res.Items {
			if g.ProjectID == projectID && containsAll(g.RoleKeys, roleKeys) {
				return true, nil
			}
		}
		return false, nil
	}
}

// roleExists is AddProjectRole's read-back. Without it a role created by a try
// whose answer was lost read as a failure, the caller rolled back its own row,
// and the Zitadel role was left an orphan only drift would find.
func (c *managementClient) roleExists(projectID, roleKey string) func(context.Context) (bool, error) {
	return func(ctx context.Context) (bool, error) {
		res, err := c.ListProjectRoles(ctx, projectID, SearchParams{Limit: 1000})
		if err != nil {
			return false, err
		}
		if res.Total > len(res.Items) {
			return false, fmt.Errorf("incomplete role read: %d of %d", len(res.Items), res.Total)
		}
		for _, r := range res.Items {
			if r.Key == roleKey {
				return true, nil
			}
		}
		return false, nil
	}
}

func containsAll(have, want []string) bool {
	set := make(map[string]bool, len(have))
	for _, h := range have {
		set[h] = true
	}
	for _, w := range want {
		if !set[w] {
			return false
		}
	}
	return true
}
