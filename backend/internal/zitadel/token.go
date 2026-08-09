package zitadel

import (
	"context"
	"crypto/rsa"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// tokenManager handles M2M access token lifecycle for the Zitadel Management API.
// Tokens are obtained via JWT profile grant (RFC 7523) and cached until near-expiry.
type tokenManager struct {
	mu          sync.RWMutex
	domain      string
	saKey       *ServiceAccountKey
	privKey     *rsa.PrivateKey
	accessToken string
	expiresAt   time.Time
}

// tokenResponse is the OAuth2 token endpoint response.
type tokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int    `json:"expires_in"`
}

const tokenRefreshMargin = 5 * time.Minute

func newTokenManager(domain string, saKey *ServiceAccountKey, privKey *rsa.PrivateKey) *tokenManager {
	return &tokenManager{
		domain:  domain,
		saKey:   saKey,
		privKey: privKey,
	}
}

// Token returns a valid access token, refreshing if expired or near-expiry.
func (tm *tokenManager) Token(ctx context.Context) (string, error) {
	tm.mu.RLock()
	if tm.accessToken != "" && timeNow().Add(tokenRefreshMargin).Before(tm.expiresAt) {
		tok := tm.accessToken
		tm.mu.RUnlock()
		return tok, nil
	}
	tm.mu.RUnlock()

	tm.mu.Lock()
	defer tm.mu.Unlock()

	// Double-check after acquiring write lock.
	if tm.accessToken != "" && timeNow().Add(tokenRefreshMargin).Before(tm.expiresAt) {
		return tm.accessToken, nil
	}

	if err := tm.refresh(ctx); err != nil {
		return "", err
	}
	return tm.accessToken, nil
}

// ForceRefresh clears the cached token so the next Token() call fetches a new one.
func (tm *tokenManager) ForceRefresh() {
	tm.mu.Lock()
	tm.accessToken = ""
	tm.expiresAt = time.Time{}
	tm.mu.Unlock()
}

func (tm *tokenManager) refresh(ctx context.Context) error {
	now := timeNow()

	assertion, err := tm.buildAssertion(now)
	if err != nil {
		return fmt.Errorf("build jwt assertion: %w", err)
	}

	tokenURL := fmt.Sprintf("https://%s/oauth/v2/token", tm.domain)
	form := url.Values{
		"grant_type": {"urn:ietf:params:oauth:grant-type:jwt-bearer"},
		"assertion":  {assertion},
		"scope":      {"openid urn:zitadel:iam:org:project:id:zitadel:aud"},
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return fmt.Errorf("build token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := httpDo(tokenHTTPClient, req)
	if err != nil {
		return fmt.Errorf("token exchange: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
	if err != nil {
		return fmt.Errorf("read token response: %w", err)
	}

	// Typed, because a non-200 here means Zitadel ANSWERED. This is where a
	// revoked or expired machine key actually fails — before any Management API
	// call — so a plain error made the commonest credential failure in the
	// system indistinguishable from an unreachable host to everything that
	// classifies on the way out. `doRequest` wraps this with %w, so callers
	// reach it with errors.As.
	//
	// The body is truncated rather than embedded whole. It is a bounded
	// diagnostic in a durable error that gets logged, and the token endpoint is
	// the one endpoint whose request carried a signed assertion: an upstream
	// that echoes it back should not be able to make Syndra copy it into a log.
	if resp.StatusCode != http.StatusOK {
		return &StatusError{Code: resp.StatusCode, Message: truncateDetail(string(body))}
	}

	var tokResp tokenResponse
	if err := json.Unmarshal(body, &tokResp); err != nil {
		return fmt.Errorf("decode token response: %w", err)
	}

	if tokResp.AccessToken == "" {
		return fmt.Errorf("token response missing access_token")
	}

	tm.accessToken = tokResp.AccessToken
	tm.expiresAt = now.Add(time.Duration(tokResp.ExpiresIn) * time.Second)
	return nil
}

func (tm *tokenManager) buildAssertion(now time.Time) (string, error) {
	claims := jwt.RegisteredClaims{
		Issuer:    tm.saKey.UserID,
		Subject:   tm.saKey.UserID,
		Audience:  jwt.ClaimStrings{fmt.Sprintf("https://%s", tm.domain)},
		IssuedAt:  jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(now.Add(time.Hour)),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	token.Header["kid"] = tm.saKey.KeyID

	return token.SignedString(tm.privKey)
}

// MintM2MToken mints a one-shot Zitadel M2M access token via the JWT profile
// grant from the service-account key at keyPath. Intended for CLI use
// (`backend/cmd/syndra-token`) — no caching, no ambient singleton state.
// Each call performs a fresh LoadServiceAccountKey + token exchange.
//
// This is the exported entry point the shell scripts use for the
// ZITADEL_MACHINE_KEY_PATH flow; the backend server itself reaches the same
// endpoint through the cached tokenManager in InitClient().
func MintM2MToken(ctx context.Context, domain, keyPath string) (string, error) {
	if domain == "" {
		return "", fmt.Errorf("domain is required")
	}
	if keyPath == "" {
		return "", fmt.Errorf("keyPath is required")
	}
	saKey, privKey, err := LoadServiceAccountKey(keyPath)
	if err != nil {
		return "", fmt.Errorf("load service account key: %w", err)
	}
	tm := newTokenManager(domain, saKey, privKey)
	return tm.Token(ctx)
}

// detailLimit bounds server-provided detail carried into a StatusError.
const detailLimit = 512

// truncateDetail keeps an error readable in a log line. The limit is generous
// enough for any real OAuth error body and short enough that a hostile or
// broken upstream cannot dictate how much of a log Syndra writes.
func truncateDetail(s string) string {
	s = strings.TrimSpace(s)
	if len(s) <= detailLimit {
		return s
	}
	return s[:detailLimit] + "… (truncated)"
}
