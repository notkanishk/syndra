package zitadel

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

func TestListApplications_ParsesOIDCAndAPITypes(t *testing.T) {
	// Verify that the derived Type field is inferred from which *Config block
	// the response carries. This is the discriminator Zitadel actually uses
	// — app "type" is not a top-level field in the v1 response.
	resetDeps(t)

	httpDo = func(_ *http.Client, req *http.Request) (*http.Response, error) {
		if strings.Contains(req.URL.Path, "/oauth/") {
			return jsonResp(200, map[string]any{
				"access_token": "test-token",
				"token_type":   "Bearer",
				"expires_in":   3600,
			}), nil
		}

		return jsonResp(200, map[string]any{
			"details": map[string]string{"totalResult": "3"},
			"result": []map[string]any{
				{"id": "app-1", "name": "Web Client", "state": "ACTIVE", "oidcConfig": map[string]any{"clientId": "c1"}},
				{"id": "app-2", "name": "Ingest API", "state": "ACTIVE", "apiConfig": map[string]any{"clientId": "c2"}},
				{"id": "app-3", "name": "SAML SP", "state": "ACTIVE", "samlConfig": map[string]any{"entityId": "e1"}},
			},
		}), nil
	}

	key := testRSAKey(t)
	tm := newTokenManager("test.zitadel.dev", &ServiceAccountKey{
		Type: "serviceaccount", KeyID: "key-1", UserID: "user-1",
	}, key)

	client := &managementClient{
		domain: "test.zitadel.dev", tokens: tm, http: &http.Client{},
	}

	result, err := client.ListApplications(context.Background(), "proj-1", SearchParams{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Items) != 3 || result.Total != 3 {
		t.Fatalf("expected 3 apps total=3, got %d items total=%d", len(result.Items), result.Total)
	}
	if result.Items[0].Type != "OIDC" || result.Items[0].Name != "Web Client" {
		t.Errorf("app-1 mapping wrong: %+v", result.Items[0])
	}
	if result.Items[1].Type != "API" {
		t.Errorf("app-2 should be API, got %q", result.Items[1].Type)
	}
	if result.Items[2].Type != "SAML" {
		t.Errorf("app-3 should be SAML, got %q", result.Items[2].Type)
	}
}

// Check 1's facts come from two GETs and nothing else; Zitadel omits the
// Bearer enum (its zero value), and an app with no OIDC config has no type.
func TestTokenSettings_ReadsAppAndProject(t *testing.T) {
	resetDeps(t)
	var calls []string
	tokenType := ""
	httpDo = func(_ *http.Client, req *http.Request) (*http.Response, error) {
		if strings.Contains(req.URL.Path, "/oauth/") {
			return jsonResp(200, map[string]any{"access_token": "t", "token_type": "Bearer", "expires_in": 3600}), nil
		}
		calls = append(calls, req.Method+" "+req.URL.Path)
		if strings.Contains(req.URL.Path, "/apps/") {
			oidc := map[string]any{"idTokenUserinfoAssertion": true}
			if tokenType != "" {
				oidc["accessTokenType"] = tokenType
			}
			return jsonResp(200, map[string]any{"app": map[string]any{"oidcConfig": oidc}}), nil
		}
		return jsonResp(200, map[string]any{"project": map[string]any{"projectRoleAssertion": true}}), nil
	}
	key := testRSAKey(t)
	tm := newTokenManager("test.zitadel.dev", &ServiceAccountKey{Type: "serviceaccount", KeyID: "k", UserID: "u"}, key)
	client := &managementClient{domain: "test.zitadel.dev", tokens: tm, http: &http.Client{}}

	s, err := client.TokenSettings(context.Background(), "p1", "a1")
	if err != nil {
		t.Fatal(err)
	}
	if s.AccessTokenType != "BEARER" || !s.IDTokenUserinfoAssertion || !s.ProjectRoleAssertion || s.ProjectRoleCheck {
		t.Fatalf("settings: %+v", s)
	}
	tokenType = "OIDC_TOKEN_TYPE_JWT"
	if s, _ := client.TokenSettings(context.Background(), "p1", "a1"); s.AccessTokenType != "JWT" {
		t.Fatalf("jwt: %+v", s)
	}
	want := []string{"GET /management/v1/projects/p1/apps/a1", "GET /management/v1/projects/p1"}
	if len(calls) != 4 || calls[0] != want[0] || calls[1] != want[1] {
		t.Fatalf("read-only calls: %v", calls)
	}
}

func TestListApplications_HitsCorrectEndpoint(t *testing.T) {
	resetDeps(t)

	var capturedPath string
	httpDo = func(_ *http.Client, req *http.Request) (*http.Response, error) {
		if strings.Contains(req.URL.Path, "/oauth/") {
			return jsonResp(200, map[string]any{
				"access_token": "test-token", "token_type": "Bearer", "expires_in": 3600,
			}), nil
		}
		capturedPath = req.URL.Path
		return jsonResp(200, map[string]any{"details": map[string]string{"totalResult": "0"}, "result": []any{}}), nil
	}

	key := testRSAKey(t)
	tm := newTokenManager("test.zitadel.dev", &ServiceAccountKey{
		Type: "serviceaccount", KeyID: "key-1", UserID: "user-1",
	}, key)
	client := &managementClient{domain: "test.zitadel.dev", tokens: tm, http: &http.Client{}}

	if _, err := client.ListApplications(context.Background(), "proj-42", SearchParams{}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if capturedPath != "/management/v1/projects/proj-42/apps/_search" {
		t.Errorf("unexpected path: %s", capturedPath)
	}
}

func TestListUserMetadata_DecodesBase64Values(t *testing.T) {
	// Zitadel encodes metadata values as base64 in the response. Round-trip a
	// known plaintext through the client and verify the caller sees the
	// decoded value.
	resetDeps(t)

	want := "Makerspace Director"
	encoded := base64.StdEncoding.EncodeToString([]byte(want))

	httpDo = func(_ *http.Client, req *http.Request) (*http.Response, error) {
		if strings.Contains(req.URL.Path, "/oauth/") {
			return jsonResp(200, map[string]any{
				"access_token": "test-token", "token_type": "Bearer", "expires_in": 3600,
			}), nil
		}
		return jsonResp(200, map[string]any{
			"details": map[string]string{"totalResult": "1"},
			"result":  []map[string]string{{"key": "title", "value": encoded}},
		}), nil
	}

	key := testRSAKey(t)
	tm := newTokenManager("test.zitadel.dev", &ServiceAccountKey{
		Type: "serviceaccount", KeyID: "key-1", UserID: "user-1",
	}, key)
	client := &managementClient{domain: "test.zitadel.dev", tokens: tm, http: &http.Client{}}

	result, err := client.ListUserMetadata(context.Background(), "user-123", SearchParams{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Items) != 1 {
		t.Fatalf("expected 1 metadata entry, got %d", len(result.Items))
	}
	if result.Items[0].Key != "title" || result.Items[0].Value != want {
		t.Errorf("got %+v, want key=title value=%q", result.Items[0], want)
	}
}

func TestHumanizeAppType_KnownAndUnknown(t *testing.T) {
	cases := []struct{ in, want string }{
		{"OIDC", "OIDC Client"},
		{"API", "API"},
		{"SAML", "SAML SP"},
		{"", ""},
		{"UNKNOWN", ""},
	}
	for _, c := range cases {
		if got := HumanizeAppType(c.in); got != c.want {
			t.Errorf("HumanizeAppType(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestClientIDOf_ReadsOIDCAndAPIBlocksAndNeverFails(t *testing.T) {
	raw := func(s string) json.RawMessage { return json.RawMessage(s) }
	cases := []struct {
		name   string
		blocks []json.RawMessage
		want   string
	}{
		{"oidc", []json.RawMessage{raw(`{"clientId":"c1@p"}`), nil, nil}, "c1@p"},
		{"api", []json.RawMessage{nil, raw(`{"clientId":"c2"}`), nil}, "c2"},
		{"saml has none", []json.RawMessage{nil, nil, raw(`{"entityId":"e"}`)}, ""},
		{"malformed", []json.RawMessage{raw(`[1,`)}, ""},
		{"empty", nil, ""},
	}
	for _, c := range cases {
		blocks := c.blocks
		if len(blocks) > 2 {
			blocks = blocks[:2] // clientIDOf is only given the OIDC and API blocks
		}
		if got := clientIDOf(blocks...); got != c.want {
			t.Errorf("%s: %q, want %q", c.name, got, c.want)
		}
	}
}
