package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"syndra/internal/claims"
	"syndra/internal/services"
)

// Two apps on one project, each told only what its own shape says, picked by
// the request's application.client_id (ADR-003). NetBird's shape renames and
// leaves out; Inventory's nests and keeps row order. Any other client, or a
// payload with no client id, gets the project's default.
func TestActionV2_PerAppShapeByClientID(t *testing.T) {
	origRedis, origMode := redisGetClaims, dbGetClaimFailureMode
	resetActionDeps(t, origRedis, origMode)
	redisGetClaims = func(context.Context, string) (string, error) {
		b, _ := json.Marshal(claims.Facts{UserID: "ada", ProjectID: "pNet", Roles: []string{"admin", "guest", "staff"}, GivenName: "Ada"})
		return string(b), nil
	}
	netbird, _ := claims.FindTemplate("netbird")
	nb := netbird.Profile
	nb.Roles = []claims.RoleMap{{Key: "staff", As: "netbird-staff", Included: true}, {Key: "admin", As: "netbird-admins", Included: true}}
	stubProjectShapes(services.ProjectShapes{
		Default: claims.FromFlat("net.roles", claims.FormatArray, nil, nil),
		ByClient: map[string]services.AppShape{
			"cNB":  {ApplicationID: "aNB", Version: 3, Profile: nb},
			"cInv": {ApplicationID: "aInv", Version: 1, Profile: claims.ActionsV1()},
		},
	})

	ask := func(client, function string) string {
		t.Helper()
		body, _ := json.Marshal(map[string]any{
			"function":    function,
			"application": map[string]string{"client_id": client},
			"user":        map[string]string{"id": "ada"},
			"user_grants": []ActionV2UserGrantRef{{ProjectID: "pNet", Roles: rolesJSON("staff")}},
		})
		rr := httptest.NewRecorder()
		HandleActionInject(rr, httptest.NewRequest(http.MethodPost, "/api/action/inject", strings.NewReader(string(body))))
		if rr.Code != http.StatusOK {
			t.Fatalf("status %d", rr.Code)
		}
		return strings.TrimSpace(rr.Body.String())
	}

	cases := []struct{ client, function, want string }{
		{"cNB", claims.FunctionPreAccessToken, `{"append_claims":[{"key":"groups","value":["netbird-staff","netbird-admins"]}]}`},
		{"cInv", claims.FunctionPreUserinfo, `{"append_claims":[{"key":"groups","value":["admin","guest","staff"]},` +
			`{"key":"custom_claims","value":{"groups":["admin","guest","staff"],"given_name":"Ada"}}]}`},
		{"cElse", claims.FunctionPreAccessToken, `{"append_claims":[{"key":"net.roles","value":["admin","guest","staff"]}]}`},
		{"", claims.FunctionPreAccessToken, `{"append_claims":[{"key":"net.roles","value":["admin","guest","staff"]}]}`},
	}
	for _, c := range cases {
		if got := ask(c.client, c.function); got != c.want {
			t.Errorf("client %q %s:\n got %s\nwant %s", c.client, c.function, got, c.want)
		}
	}
}

// A shape answers only the function where it lands: Pre Access Token for the
// access token, Pre Userinfo for userinfo (and the ID token). The other
// function gets nothing, and no facts are read for it.
func TestActionV2_LandsGatesTheFunction(t *testing.T) {
	origRedis, origMode := redisGetClaims, dbGetClaimFailureMode
	resetActionDeps(t, origRedis, origMode)
	reads := 0
	redisGetClaims = func(context.Context, string) (string, error) {
		reads++
		return factsJSON("ada", "pNet", "staff"), nil
	}
	supabase, _ := claims.FindTemplate("supabase-oidc") // lands userinfo
	stubProjectShapes(services.ProjectShapes{Default: claims.Builtin(), ByClient: map[string]services.AppShape{"cSB": {Profile: supabase.Profile}}})

	for _, c := range []struct {
		function string
		want     int
	}{{claims.FunctionPreAccessToken, 0}, {claims.FunctionPreUserinfo, 1}} {
		body := fmt.Sprintf(`{"function":%q,"application":{"client_id":"cSB"},"user":{"id":"ada"},"user_grants":[{"projectId":"pNet","roles":["staff"]}]}`, c.function)
		rr := httptest.NewRecorder()
		HandleActionInject(rr, httptest.NewRequest(http.MethodPost, "/api/action/inject", strings.NewReader(body)))
		got := decodeActionResponse(t, rr)
		if len(got.AppendClaims) != c.want {
			t.Fatalf("%s: want %d claims, got %v", c.function, c.want, got.AppendClaims)
		}
	}
	if reads != 1 {
		t.Fatalf("facts read %d times, want 1 (only where it lands)", reads)
	}
}

// Debug mode captures exactly the one watched account on the watched client,
// after the answer, with the body kept as Zitadel sent it. Nobody else's
// sign-in is held.
func TestActionV2_DebugCapturesOnlyTheWatchedAccount(t *testing.T) {
	origRedis, origMode := redisGetClaims, dbGetClaimFailureMode
	resetActionDeps(t, origRedis, origMode)
	origExplain := svcExplainSignIn
	t.Cleanup(func() { svcExplainSignIn = origExplain; claimDebug.Stop("aDbg", "test") })
	svcExplainSignIn = func(context.Context, string, string, string) (services.Explanation, error) {
		return services.Explanation{Checks: []services.Check{{State: services.CheckPass}}}, nil
	}
	redisGetClaims = func(context.Context, string) (string, error) { return factsJSON("ada", "pNet", "staff"), nil }

	claimDebug.Start("aDbg", "cDbg", "ada", "kanishk")
	for _, user := range []string{"grace", "ada"} {
		body := fmt.Sprintf(`{"function":"function/preaccesstoken","application":{"client_id":"cDbg"},"user":{"id":%q},"user_grants":[{"projectId":"pNet","roles":["staff"]}]}`, user)
		HandleActionInject(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/api/action/inject", strings.NewReader(body)))
	}

	deadline := time.Now().Add(2 * time.Second)
	var got []string
	for time.Now().Before(deadline) {
		list := claimDebug.SignIns("aDbg", 0)
		if len(list) > 0 {
			for _, e := range list {
				got = append(got, e.UserID)
				if !strings.Contains(string(e.Request), `"client_id":"cDbg"`) || !strings.Contains(string(e.Response), `"append_claims"`) || e.Checks == nil {
					t.Fatalf("entry: %+v", e)
				}
			}
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if len(got) != 1 || got[0] != "ada" {
		t.Fatalf("want only Ada's sign-in, got %v", got)
	}
}

// canonicalV2Payload is the full shape Zitadel sends for a
// `function/preaccesstoken` trigger. Fields beyond user and user_grants are
// included to prove the lenient decoder ignores them without error, even as
// Zitadel extends the schema over time.
const canonicalV2Payload = `{
  "function": "function/preaccesstoken",
  "userinfo": {
    "sub": "250124622953808001",
    "name": "Test User",
    "email": "test@example.com",
    "email_verified": true
  },
  "user": {
    "id": "250124622953808001",
    "creationDate": "2024-02-01T10:00:00Z",
    "changeDate": "2024-04-01T12:00:00Z",
    "state": "USER_STATE_ACTIVE",
    "human": {
      "profile": {
        "firstName": "Test",
        "lastName": "User",
        "displayName": "Test User",
        "preferredLanguage": "en"
      },
      "email": {
        "email": "test@example.com",
        "isEmailVerified": true
      }
    }
  },
  "user_metadata": [
    {"key": "department", "value": "aGFyZHdhcmU="}
  ],
  "org": {
    "id": "250124592953808001",
    "name": "Ashoka Makerspace",
    "primary_domain": "makerspace.example.org"
  },
  "application": {"client_id": "printing-portal@example"},
  "user_grants": [
    {
      "id": "grant-1",
      "creation_date": "2026-09-01T10:00:00Z",
      "sequence": 4,
      "roles": ["operator"],
      "grant_id": "",
      "state": 1,
      "user_id": "250124622953808001",
      "project_id": "pPrinting",
      "project_name": "Printing Portal",
      "project_resource_owner": "250124592953808001"
    },
    {
      "id": "grant-2",
      "roles": ["viewer"],
      "user_id": "250124622953808001",
      "project_id": "pPrinting",
      "project_name": "Printing Portal"
    }
  ]
}`

// TestHandleActionInject_CanonicalV2Payload_Accepted proves the reshaped
// handler consumes the full Zitadel Actions v2 function payload (with every
// documented field populated) without rejecting it, and returns the v2
// envelope. This is the wire-level contract round-trip test.
func TestHandleActionInject_CanonicalV2Payload_Accepted(t *testing.T) {
	origRedis, origMode := redisGetClaims, dbGetClaimFailureMode
	resetActionDeps(t, origRedis, origMode)

	redisGetClaims = func(ctx context.Context, key string) (string, error) {
		if key == "mapping:250124622953808001:pPrinting" {
			return factsJSON("250124622953808001", "pPrinting", "operator"), nil
		}
		return "", fmt.Errorf("unexpected key: %s", key)
	}
	dbGetClaimFailureMode = func(ctx context.Context, projectID string) (string, map[string]interface{}, error) {
		t.Errorf("failure mode queried despite cache hits for project=%s", projectID)
		return "fail_closed", nil, nil
	}

	req := httptest.NewRequest(http.MethodPost, "/api/action/inject", strings.NewReader(canonicalV2Payload))
	rr := httptest.NewRecorder()
	HandleActionInject(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("canonical v2 payload rejected: %d (body=%s)", rr.Code, rr.Body.String())
	}

	got := decodeActionResponse(t, rr)
	// Zitadel lists only the token's own project; its two rows (one per role)
	// are one project, and with no configured profile it emits the built-in key.
	// Roles come from Syndra's compiled facts, not the payload's role list.
	if len(got.AppendClaims) != 1 {
		t.Fatalf("expected 1 append_claim, got %v", got.AppendClaims)
	}
	printing, ok := claimByKey(got.AppendClaims, "roles")
	if !ok || !rolesEqual(printing.Value, "operator") {
		t.Fatalf("expected roles=[operator], got %v", got.AppendClaims)
	}
}

// rolesEqual compares a decoded array-format roles claim against expected values.
func rolesEqual(value interface{}, want ...string) bool {
	list, ok := value.([]interface{})
	if !ok || len(list) != len(want) {
		return false
	}
	for i, w := range want {
		if list[i] != w {
			return false
		}
	}
	return true
}

// The grant rows are Zitadel's query.UserGrant, serialised with snake_case
// tags ("project_id", internal/query/user_grant.go in v3.4 and v4). Zitadel's
// docs show "projectId", and reading only that is the defect that left every
// v2 token without Syndra's claims. Both spellings must resolve the project.
func TestActionV2UserGrantRef_ReadsBothProjectSpellings(t *testing.T) {
	for _, body := range []string{
		`{"project_id":"pNet","roles":["staff"]}`,
		`{"projectId":"pNet","roles":["staff"]}`,
		`{"project_id":"pNet","projectId":"pOther","roles":["staff"]}`,
	} {
		var g ActionV2UserGrantRef
		if err := json.Unmarshal([]byte(body), &g); err != nil {
			t.Fatalf("%s: %v", body, err)
		}
		if g.ProjectID != "pNet" || string(g.Roles) != `["staff"]` {
			t.Fatalf("%s: got %+v", body, g)
		}
	}
}
