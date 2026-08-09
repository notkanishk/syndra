package zitadel

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// --- test helpers ---

func testRSAKey(t *testing.T) *rsa.PrivateKey {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate test RSA key: %v", err)
	}
	return key
}

func resetDeps(t *testing.T) {
	t.Helper()
	origHttpDo := httpDo
	origTimeNow := timeNow
	origTokenHTTPClient := tokenHTTPClient
	origMgmtClient := MgmtClient
	t.Cleanup(func() {
		httpDo = origHttpDo
		timeNow = origTimeNow
		tokenHTTPClient = origTokenHTTPClient
		MgmtClient = origMgmtClient
	})
}

// mockTransport returns a round-tripper that calls fn for every request.
type mockTransport struct {
	fn func(*http.Request) (*http.Response, error)
}

func (m *mockTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	return m.fn(req)
}

func jsonResp(status int, body any) *http.Response {
	b, _ := json.Marshal(body)
	return &http.Response{
		StatusCode: status,
		Body:       io.NopCloser(strings.NewReader(string(b))),
		Header:     http.Header{"Content-Type": []string{"application/json"}},
	}
}

// --- keyfile tests ---

func TestLoadServiceAccountKey_Valid(t *testing.T) {
	key := testRSAKey(t)
	keyFile := writeTestKeyFile(t, key, "serviceaccount", "key-1", "user-1")

	sa, privKey, err := LoadServiceAccountKey(keyFile)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sa.KeyID != "key-1" || sa.UserID != "user-1" || sa.Type != "serviceaccount" {
		t.Errorf("unexpected key metadata: %+v", sa)
	}
	if privKey == nil {
		t.Fatal("private key is nil")
	}
	if privKey.N.Cmp(key.N) != 0 {
		t.Error("private key modulus mismatch")
	}
}

func TestLoadServiceAccountKey_MissingFields(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "key.json")
	os.WriteFile(path, []byte(`{"type":"serviceaccount","keyId":"","key":"","userId":""}`), 0644)

	_, _, err := LoadServiceAccountKey(path)
	if err == nil {
		t.Fatal("expected error for missing fields")
	}
	if !strings.Contains(err.Error(), "missing required fields") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestLoadServiceAccountKey_InvalidPEM(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "key.json")
	data := `{"type":"serviceaccount","keyId":"k1","key":"not-pem-data","userId":"u1"}`
	os.WriteFile(path, []byte(data), 0644)

	_, _, err := LoadServiceAccountKey(path)
	if err == nil {
		t.Fatal("expected error for invalid PEM")
	}
	if !strings.Contains(err.Error(), "invalid PEM") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestLoadServiceAccountKey_WrongType(t *testing.T) {
	key := testRSAKey(t)
	dir := t.TempDir()
	path := filepath.Join(dir, "key.json")
	pemBytes := pemEncodeKey(key)
	data, _ := json.Marshal(map[string]string{
		"type": "application", "keyId": "k1", "key": string(pemBytes), "userId": "u1",
	})
	os.WriteFile(path, data, 0644)

	_, _, err := LoadServiceAccountKey(path)
	if err == nil || !strings.Contains(err.Error(), "unexpected key type") {
		t.Errorf("expected type error, got: %v", err)
	}
}

func TestLoadServiceAccountKey_FileNotFound(t *testing.T) {
	_, _, err := LoadServiceAccountKey("/nonexistent/path/key.json")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

// TestLoadServiceAccountKey_EmptyFile guards against the silent-empty-mount
// failure mode: if the bind mount resolves to /dev/null or a zero-byte file,
// the error must identify the cause explicitly rather than surface an opaque
// JSON decode error.
func TestLoadServiceAccountKey_EmptyFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "empty.json")
	os.WriteFile(path, []byte{}, 0644)

	_, _, err := LoadServiceAccountKey(path)
	if err == nil {
		t.Fatal("expected error for empty file")
	}
	if !strings.Contains(err.Error(), "empty") {
		t.Errorf("error should name the empty-file condition, got: %v", err)
	}
	if !strings.Contains(err.Error(), path) {
		t.Errorf("error should include the file path, got: %v", err)
	}
}

// --- token manager tests ---

func TestTokenManager_CachedToken(t *testing.T) {
	resetDeps(t)

	key := testRSAKey(t)
	sa := &ServiceAccountKey{KeyID: "kid-1", UserID: "sa-user", Type: "serviceaccount"}
	tm := newTokenManager("test.zitadel.cloud", sa, key)

	// Pre-populate cache
	tm.accessToken = "cached-token"
	tm.expiresAt = time.Now().Add(time.Hour)

	tok, err := tm.Token(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tok != "cached-token" {
		t.Errorf("expected cached token, got %q", tok)
	}
}

func TestTokenManager_ExpiredRefreshes(t *testing.T) {
	resetDeps(t)

	key := testRSAKey(t)
	sa := &ServiceAccountKey{KeyID: "kid-1", UserID: "sa-user", Type: "serviceaccount"}
	tm := newTokenManager("test.zitadel.cloud", sa, key)

	// Expired token
	tm.accessToken = "old-token"
	tm.expiresAt = time.Now().Add(-time.Minute)

	// Mock token endpoint
	tokenHTTPClient = &http.Client{Transport: &mockTransport{fn: func(req *http.Request) (*http.Response, error) {
		return jsonResp(200, tokenResponse{
			AccessToken: "fresh-token",
			ExpiresIn:   3600,
		}), nil
	}}}

	tok, err := tm.Token(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tok != "fresh-token" {
		t.Errorf("expected fresh-token, got %q", tok)
	}
}

func TestTokenManager_RefreshFailure(t *testing.T) {
	resetDeps(t)

	key := testRSAKey(t)
	sa := &ServiceAccountKey{KeyID: "kid-1", UserID: "sa-user", Type: "serviceaccount"}
	tm := newTokenManager("test.zitadel.cloud", sa, key)

	tokenHTTPClient = &http.Client{Transport: &mockTransport{fn: func(req *http.Request) (*http.Response, error) {
		return jsonResp(401, map[string]string{"error": "invalid_grant"}), nil
	}}}

	_, err := tm.Token(context.Background())
	if err == nil {
		t.Fatal("expected error on token refresh failure")
	}
	if !strings.Contains(err.Error(), "401") {
		t.Errorf("expected 401 in error, got: %v", err)
	}
}

func TestTokenManager_JWTAssertionFormat(t *testing.T) {
	resetDeps(t)

	key := testRSAKey(t)
	sa := &ServiceAccountKey{KeyID: "test-kid", UserID: "sa-123", Type: "serviceaccount"}
	tm := newTokenManager("auth.example.com", sa, key)

	var capturedAssertion string
	tokenHTTPClient = &http.Client{Transport: &mockTransport{fn: func(req *http.Request) (*http.Response, error) {
		body, _ := io.ReadAll(req.Body)
		// Parse form-encoded body
		for _, pair := range strings.Split(string(body), "&") {
			kv := strings.SplitN(pair, "=", 2)
			if len(kv) == 2 && kv[0] == "assertion" {
				capturedAssertion = kv[1]
			}
		}
		return jsonResp(200, tokenResponse{AccessToken: "tok", ExpiresIn: 3600}), nil
	}}}

	_, err := tm.Token(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if capturedAssertion == "" {
		t.Fatal("no assertion captured from token request")
	}

	// URL-decode the assertion (form-encoded % escapes)
	decoded := strings.ReplaceAll(capturedAssertion, "%2B", "+")
	decoded = strings.ReplaceAll(decoded, "%2F", "/")
	decoded = strings.ReplaceAll(decoded, "%3D", "=")

	// Parse and validate the JWT assertion (without signature verification here — we trust our own signing)
	parser := jwt.NewParser(jwt.WithoutClaimsValidation())
	token, _, err := parser.ParseUnverified(decoded, &jwt.RegisteredClaims{})
	if err != nil {
		t.Fatalf("parse assertion JWT: %v", err)
	}

	claims := token.Claims.(*jwt.RegisteredClaims)
	if claims.Issuer != "sa-123" {
		t.Errorf("expected issuer sa-123, got %q", claims.Issuer)
	}
	if claims.Subject != "sa-123" {
		t.Errorf("expected subject sa-123, got %q", claims.Subject)
	}
	aud, _ := claims.GetAudience()
	if len(aud) != 1 || aud[0] != "https://auth.example.com" {
		t.Errorf("unexpected audience: %v", aud)
	}
	if kid, ok := token.Header["kid"].(string); !ok || kid != "test-kid" {
		t.Errorf("expected kid test-kid, got %v", token.Header["kid"])
	}
}

// --- management client tests ---

func TestAddUserGrant_Success(t *testing.T) {
	resetDeps(t)
	client, _ := testClient(t)

	var capturedPath, capturedMethod string
	var capturedBody map[string]any
	httpDo = func(_ *http.Client, req *http.Request) (*http.Response, error) {
		capturedPath = req.URL.Path
		capturedMethod = req.Method
		body, _ := io.ReadAll(req.Body)
		json.Unmarshal(body, &capturedBody)
		return jsonResp(200, map[string]string{"grantId": "g-1"}), nil
	}

	err := client.AddUserGrant(context.Background(), "user-1", "proj-1", []string{"editor", "viewer"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if capturedMethod != "POST" {
		t.Errorf("expected POST, got %s", capturedMethod)
	}
	if capturedPath != "/management/v1/users/user-1/grants" {
		t.Errorf("unexpected path: %s", capturedPath)
	}
	if capturedBody["projectId"] != "proj-1" {
		t.Errorf("unexpected projectId: %v", capturedBody["projectId"])
	}
	roles, ok := capturedBody["roleKeys"].([]any)
	if !ok || len(roles) != 2 {
		t.Errorf("unexpected roleKeys: %v", capturedBody["roleKeys"])
	}
}

func TestAddUserGrant_401_RefreshesToken(t *testing.T) {
	resetDeps(t)
	client, tm := testClient(t)
	tm.accessToken = "stale-token"
	tm.expiresAt = time.Now().Add(time.Hour) // looks valid but is actually rejected

	attempt := 0
	httpDo = func(_ *http.Client, req *http.Request) (*http.Response, error) {
		attempt++
		if attempt == 1 {
			// First call (with stale token) → 401
			return jsonResp(401, map[string]string{"error": "unauthenticated"}), nil
		}
		if attempt == 2 {
			// Token refresh call
			return jsonResp(200, tokenResponse{AccessToken: "fresh-token", ExpiresIn: 3600}), nil
		}
		// Retry with fresh token → success
		if auth := req.Header.Get("Authorization"); auth != "Bearer fresh-token" {
			t.Errorf("expected fresh token, got %q", auth)
		}
		return jsonResp(200, map[string]string{"ok": "true"}), nil
	}

	err := client.AddUserGrant(context.Background(), "u1", "p1", []string{"role"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if attempt < 3 {
		t.Errorf("expected at least 3 HTTP calls (request, refresh, retry), got %d", attempt)
	}
}

func TestRemoveUserGrant_Success(t *testing.T) {
	resetDeps(t)
	client, _ := testClient(t)

	var capturedPath, capturedMethod string
	httpDo = func(_ *http.Client, req *http.Request) (*http.Response, error) {
		capturedPath = req.URL.Path
		capturedMethod = req.Method
		return jsonResp(200, map[string]string{}), nil
	}

	err := client.RemoveUserGrant(context.Background(), "user-1", "grant-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if capturedMethod != "DELETE" {
		t.Errorf("expected DELETE, got %s", capturedMethod)
	}
	if capturedPath != "/management/v1/users/user-1/grants/grant-1" {
		t.Errorf("unexpected path: %s", capturedPath)
	}
}

func TestListUserGrants_Success(t *testing.T) {
	resetDeps(t)
	client, _ := testClient(t)

	var capturedPath string
	var capturedBody map[string]any
	httpDo = func(_ *http.Client, req *http.Request) (*http.Response, error) {
		capturedPath = req.URL.Path
		body, _ := io.ReadAll(req.Body)
		json.Unmarshal(body, &capturedBody)
		return jsonResp(200, map[string]any{
			"result": []UserGrant{
				{ID: "g1", UserID: "u1", ProjectID: "p1", RoleKeys: []string{"admin"}},
				{ID: "g2", UserID: "u1", ProjectID: "p2", RoleKeys: []string{"viewer"}},
			},
		}), nil
	}

	result, err := client.ListUserGrants(context.Background(), "u1", SearchParams{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify correct endpoint (not user-scoped).
	if capturedPath != "/management/v1/users/grants/_search" {
		t.Errorf("unexpected path: %s", capturedPath)
	}

	// Verify user ID is passed as a query filter in the body.
	queries, ok := capturedBody["queries"].([]any)
	if !ok || len(queries) == 0 {
		t.Fatalf("expected queries in body, got: %v", capturedBody)
	}
	q := queries[0].(map[string]any)
	uidQuery, ok := q["userIdQuery"].(map[string]any)
	if !ok || uidQuery["userId"] != "u1" {
		t.Errorf("expected userIdQuery with userId=u1, got: %v", q)
	}

	if len(result.Items) != 2 {
		t.Fatalf("expected 2 grants, got %d", len(result.Items))
	}
	if result.Items[0].ID != "g1" || result.Items[1].ProjectID != "p2" {
		t.Errorf("unexpected grant data: %+v", result.Items)
	}
}

func TestGetUser_Success(t *testing.T) {
	resetDeps(t)
	client, _ := testClient(t)

	httpDo = func(_ *http.Client, req *http.Request) (*http.Response, error) {
		if req.URL.Path != "/management/v1/users/u42" {
			t.Errorf("unexpected path: %s", req.URL.Path)
		}
		// Mirror real Zitadel v1 response shape with nested human profile.
		return jsonResp(200, map[string]any{
			"user": map[string]any{
				"id":       "u42",
				"userName": "jane",
				"state":    "USER_STATE_ACTIVE",
				"human": map[string]any{
					"profile": map[string]any{
						"displayName": "Jane Doe",
						"firstName":   "Jane",
						"lastName":    "Doe",
					},
					"email": map[string]any{
						"email":           "jane@example.com",
						"isEmailVerified": true,
					},
				},
			},
		}), nil
	}

	user, err := client.GetUser(context.Background(), "u42")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if user.ID != "u42" {
		t.Errorf("expected ID u42, got %q", user.ID)
	}
	if user.Username != "jane" {
		t.Errorf("expected username jane, got %q", user.Username)
	}
	if user.DisplayName != "Jane Doe" {
		t.Errorf("expected displayName 'Jane Doe', got %q", user.DisplayName)
	}
	if user.Email != "jane@example.com" {
		t.Errorf("expected email jane@example.com, got %q", user.Email)
	}
	if user.State != "USER_STATE_ACTIVE" {
		t.Errorf("expected state USER_STATE_ACTIVE, got %q", user.State)
	}
}

func TestDoRequest_Retries429(t *testing.T) {
	resetDeps(t)
	client, _ := testClient(t)

	attempt := 0
	httpDo = func(_ *http.Client, req *http.Request) (*http.Response, error) {
		attempt++
		if attempt <= 2 {
			return jsonResp(429, map[string]string{"message": "rate limited"}), nil
		}
		return jsonResp(200, map[string]string{"ok": "true"}), nil
	}

	err := client.AddUserGrant(context.Background(), "u1", "p1", []string{"r1"})
	if err != nil {
		t.Fatalf("unexpected error after retries: %v", err)
	}
	if attempt != 3 {
		t.Errorf("expected 3 attempts, got %d", attempt)
	}
}

func TestDoRequest_NoRetryOn400(t *testing.T) {
	resetDeps(t)
	client, _ := testClient(t)

	attempt := 0
	httpDo = func(_ *http.Client, req *http.Request) (*http.Response, error) {
		attempt++
		return jsonResp(400, apiError{Code: 400, Message: "invalid request"}), nil
	}

	err := client.AddUserGrant(context.Background(), "u1", "p1", []string{"r1"})
	if err == nil {
		t.Fatal("expected error on 400")
	}
	if attempt != 1 {
		t.Errorf("expected 1 attempt (no retry), got %d", attempt)
	}
	if !strings.Contains(err.Error(), "invalid request") {
		t.Errorf("expected error message, got: %v", err)
	}
}

// --- test utilities ---

func testClient(t *testing.T) (*managementClient, *tokenManager) {
	t.Helper()
	key := testRSAKey(t)
	sa := &ServiceAccountKey{KeyID: "kid-1", UserID: "sa-user", Type: "serviceaccount"}
	tm := newTokenManager("test.zitadel.cloud", sa, key)
	tm.accessToken = "test-token"
	tm.expiresAt = time.Now().Add(time.Hour)

	c := &managementClient{
		domain: "test.zitadel.cloud",
		tokens: tm,
		http:   &http.Client{Timeout: 5 * time.Second},
	}
	return c, tm
}

func writeTestKeyFile(t *testing.T, key *rsa.PrivateKey, keyType, keyID, userID string) string {
	t.Helper()
	pemBytes := pemEncodeKey(key)
	data, _ := json.Marshal(map[string]string{
		"type": keyType, "keyId": keyID, "key": string(pemBytes), "userId": userID,
	})
	dir := t.TempDir()
	path := filepath.Join(dir, "sa-key.json")
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatalf("write test key file: %v", err)
	}
	return path
}

func pemEncodeKey(key *rsa.PrivateKey) []byte {
	return pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(key),
	})
}

// A revoked or expired machine key fails HERE — at the token exchange, before
// any Management API call is attempted. Returned as a plain error it was
// indistinguishable, to everything downstream, from a host that never answered:
// the drift sweep recorded an unreachable target and the operator waited out a
// credential that was never going to fix itself.
func TestTokenExchange_AnsweredFailureCarriesItsStatus(t *testing.T) {
	resetDeps(t)
	for _, tc := range []struct {
		name string
		code int
		body string
	}{
		{"revoked key", http.StatusUnauthorized, `{"error":"invalid_client","error_description":"key not found"}`},
		{"disabled service account", http.StatusForbidden, `{"error":"access_denied"}`},
		{"zitadel failing on its own side", http.StatusInternalServerError, `upstream error`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			httpDo = func(_ *http.Client, req *http.Request) (*http.Response, error) {
				if !strings.HasSuffix(req.URL.Path, "/oauth/v2/token") {
					t.Fatalf("expected the token endpoint, got %s", req.URL.Path)
				}
				return &http.Response{
					StatusCode: tc.code,
					Body:       io.NopCloser(strings.NewReader(tc.body)),
					Header:     http.Header{},
				}, nil
			}
			tm := newTokenManager("test.zitadel.cloud", &ServiceAccountKey{KeyID: "kid-1", UserID: "sa-user"}, testRSAKey(t))

			_, err := tm.Token(context.Background())
			var status *StatusError
			if !errors.As(err, &status) {
				t.Fatalf("an answered token failure must carry its status, got %T: %v", err, err)
			}
			if status.Code != tc.code {
				t.Fatalf("status = %d, want %d", status.Code, tc.code)
			}
		})
	}
}

// The same failure reached through the Management API path a caller actually
// uses. doRequest wraps the token error, so this asserts errors.As traverses
// the wrapping rather than that the sentinel is returned bare.
func TestManagementCall_SurfacesAnAnsweredTokenFailureAsStatus(t *testing.T) {
	resetDeps(t)
	c, tm := testClient(t)
	tm.ForceRefresh() // force the next call through the token exchange

	httpDo = func(_ *http.Client, req *http.Request) (*http.Response, error) {
		if !strings.HasSuffix(req.URL.Path, "/oauth/v2/token") {
			t.Fatalf("the token exchange must fail before any management call; got %s", req.URL.Path)
		}
		return &http.Response{
			StatusCode: http.StatusUnauthorized,
			Body:       io.NopCloser(strings.NewReader(`{"error":"invalid_client"}`)),
			Header:     http.Header{},
		}, nil
	}

	_, err := c.ListAllGrants(context.Background(), SearchParams{Limit: 10})
	var status *StatusError
	if !errors.As(err, &status) || status.Code != http.StatusUnauthorized {
		t.Fatalf("a caller must be able to classify this as answered-401, got %T: %v", err, err)
	}
}

// A transport failure is not an answered failure. Nothing came back, so nothing
// may claim a status.
func TestTokenExchange_ATransportFailureCarriesNoStatus(t *testing.T) {
	resetDeps(t)
	httpDo = func(*http.Client, *http.Request) (*http.Response, error) {
		return nil, errors.New("dial tcp: connection refused")
	}
	tm := newTokenManager("test.zitadel.cloud", &ServiceAccountKey{KeyID: "kid-1", UserID: "sa-user"}, testRSAKey(t))

	_, err := tm.Token(context.Background())
	var status *StatusError
	if errors.As(err, &status) {
		t.Fatalf("a host that never answered must not be reported with a status: %v", err)
	}
	if err == nil {
		t.Fatal("a transport failure must still be an error")
	}
}

// The detail is a diagnostic, not a transcript. A broken or hostile upstream
// must not dictate how much of a log Syndra writes — and the token request is
// the one that carried a signed assertion.
func TestTokenExchange_BoundsTheDetailItCarries(t *testing.T) {
	resetDeps(t)
	httpDo = func(*http.Client, *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusBadRequest,
			Body:       io.NopCloser(strings.NewReader(strings.Repeat("x", 40_000))),
			Header:     http.Header{},
		}, nil
	}
	tm := newTokenManager("test.zitadel.cloud", &ServiceAccountKey{KeyID: "kid-1", UserID: "sa-user"}, testRSAKey(t))

	_, err := tm.Token(context.Background())
	var status *StatusError
	if !errors.As(err, &status) {
		t.Fatalf("want a StatusError, got %T", err)
	}
	if len(status.Message) > detailLimit+len("… (truncated)") {
		t.Fatalf("detail must be bounded, got %d bytes", len(status.Message))
	}
}
