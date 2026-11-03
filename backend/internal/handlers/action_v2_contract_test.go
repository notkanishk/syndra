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
	"syndra/internal/claimserve"
	"syndra/internal/services"
)

// Two apps on one project, each told only what its own shape says, picked by
// the request's application.client_id (ADR-003). NetBird's shape renames and
// leaves out; Inventory's nests and keeps row order. Any other client, or a
// payload with no client id, gets the project's default.
func TestActionV2_PerAppShapeByClientID(t *testing.T) {
	netbird, _ := claims.FindTemplate("netbird")
	nb := netbird.Profile
	nb.Roles = []claims.RoleMap{{Key: "staff", As: "netbird-staff", Included: true}, {Key: "admin", As: "netbird-admins", Included: true}}
	r := testPlane(t, map[string]claimserve.Project{"pNet": project(services.ProjectShapes{
		Default: claims.FromFlat("net.roles", claims.FormatArray, nil, nil),
		ByClient: map[string]services.AppShape{
			"cNB":  {ApplicationID: "aNB", Version: 3, Profile: nb},
			"cInv": {ApplicationID: "aInv", Version: 1, Profile: claims.ActionsV1()},
		},
	}, failClosed)})
	b, _ := json.Marshal(claims.Facts{UserID: "ada", ProjectID: "pNet", Roles: []string{"staff"}, GivenName: "Ada"})
	setFacts(r, "ada", "pNet", string(b))

	ask := func(client, function string) string {
		t.Helper()
		// The roles are Zitadel's, across its rows; Syndra's facts give only
		// the name here.
		body, _ := json.Marshal(map[string]any{
			"function":    function,
			"application": map[string]string{"client_id": client},
			"user":        map[string]string{"id": "ada"},
			"user_grants": []ActionV2UserGrantRef{{ProjectID: "pNet", Roles: rolesJSON("staff", "guest")}, {ProjectID: "pNet", Roles: rolesJSON("admin")}},
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
	supabase, _ := claims.FindTemplate("supabase-oidc") // lands userinfo
	r := testPlane(t, map[string]claimserve.Project{"pNet": project(services.ProjectShapes{
		ByClient: map[string]services.AppShape{"cSB": {Profile: supabase.Profile}}}, failClosed)})
	setFacts(r, "ada", "pNet", factsJSON("ada", "pNet", "staff"))

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
	if reads := r.Count("HGET"); reads != 1 {
		t.Fatalf("facts read %d times, want 1 (only where it lands)", reads)
	}
}

// Debug mode captures exactly the one watched account on the watched client,
// after the answer, with the body kept as Zitadel sent it. Nobody else's
// sign-in is held.
func TestActionV2_DebugCapturesOnlyTheWatchedAccount(t *testing.T) {
	r := builtinPlane(t, "pNet")
	setFacts(r, "ada", "pNet", factsJSON("ada", "pNet", "staff"))
	origExplain := svcExplainSignIn
	t.Cleanup(func() { svcExplainSignIn = origExplain; claimDebug.Stop("aDbg", "test") })
	svcExplainSignIn = func(context.Context, string, string, string) (services.Explanation, error) {
		return services.Explanation{Checks: []services.Check{{State: services.CheckPass}}}, nil
	}

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
	r := builtinPlane(t, "pPrinting")
	setFacts(r, "250124622953808001", "pPrinting", factsJSON("250124622953808001", "pPrinting", "operator"))

	req := httptest.NewRequest(http.MethodPost, "/api/action/inject", strings.NewReader(canonicalV2Payload))
	rr := httptest.NewRecorder()
	HandleActionInject(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("canonical v2 payload rejected: %d (body=%s)", rr.Code, rr.Body.String())
	}

	got := decodeActionResponse(t, rr)
	// Zitadel lists only the token's own project; its two rows are one
	// project, and with no configured profile it emits the built-in key. The
	// roles are the payload's, both rows' (ADR-004): Syndra's facts give
	// operator alone, and viewer, given in Zitadel directly, is sent too.
	if len(got.AppendClaims) != 1 {
		t.Fatalf("expected 1 append_claim, got %v", got.AppendClaims)
	}
	printing, ok := claimByKey(got.AppendClaims, "roles")
	if !ok || !rolesEqual(printing.Value, "operator", "viewer") {
		t.Fatalf("expected roles=[operator viewer], got %v", got.AppendClaims)
	}
}

// The prod defect of 4 Nov 2026: the operator's admin role, given in Zitadel's
// console, was in the payload, but Syndra held no facts for him in the
// project, so his sign-in was answered degraded_no_facts and carried no
// roles. Zitadel's list is the token's roles: he gets them, counted
// served_roles_only, and the miss still asks for his facts to be built. A
// role of Syndra's that Zitadel did not list (queued, not yet sent) is never
// sent.
func TestActionV2_RolesComeFromZitadelsPayload(t *testing.T) {
	r := builtinPlane(t, "pPrinting")
	var repaired []string
	origMiss := claimDoor.OnMiss
	t.Cleanup(func() { claimDoor.OnMiss = origMiss })
	claimDoor.OnMiss = func(u string) { repaired = append(repaired, u) }

	rr := inject(t, canonicalV2Payload)
	got := decodeActionResponse(t, rr)
	if roles, ok := claimByKey(got.AppendClaims, "roles"); rr.Code != 200 || !ok || !rolesEqual(roles.Value, "operator", "viewer") {
		t.Fatalf("Zitadel-only roles, no facts: HTTP %d %v", rr.Code, got.AppendClaims)
	}
	if len(repaired) != 1 || repaired[0] != "250124622953808001" {
		t.Fatalf("a miss asks for a repair: %v", repaired)
	}

	setFacts(r, "250124622953808001", "pPrinting", factsJSON("250124622953808001", "pPrinting", "operator", "queued"))
	got = decodeActionResponse(t, inject(t, canonicalV2Payload))
	if roles, _ := claimByKey(got.AppendClaims, "roles"); !rolesEqual(roles.Value, "operator", "viewer") {
		t.Fatalf("a ledger role Zitadel did not list was sent: %v", got.AppendClaims)
	}
}

// The correlator's "does this person hold a role here?" is Zitadel's answer as
// Syndra last observed it, not Syndra's facts (which lack roles given in
// Zitadel directly). None is known only from a complete current sweep; an
// unknown client or a failed read is unknown, so no_grants keeps its stricter
// verdict.
func TestObservedHolds(t *testing.T) {
	testPlane(t, map[string]claimserve.Project{"pNet": project(services.ProjectShapes{
		ByClient: map[string]services.AppShape{"cNB": {ApplicationID: "aNB", Profile: claims.Builtin()}}}, failClosed)})
	orig := svcObservedRoles
	t.Cleanup(func() { svcObservedRoles = orig })
	for _, c := range []struct {
		name, client string
		roles        []string
		absent       bool
		err          error
		holds, known bool
	}{
		{"a role in Zitadel", "cNB", []string{"admin"}, false, nil, true, true},
		{"none, by a complete sweep", "cNB", nil, true, nil, false, true},
		{"none, but the sweep can't vouch", "cNB", nil, false, nil, false, false},
		{"the read failed", "cNB", nil, false, fmt.Errorf("down"), false, false},
		{"a client that is no app", "cElse", []string{"admin"}, false, nil, false, false},
	} {
		svcObservedRoles = func(_ context.Context, _, projectID string) ([]string, bool, error) {
			if projectID != "pNet" {
				t.Fatalf("asked about %q", projectID)
			}
			return c.roles, c.absent, c.err
		}
		if holds, known := observedHolds(context.Background(), "ada", c.client); holds != c.holds || known != c.known {
			t.Errorf("%s: holds=%v known=%v", c.name, holds, known)
		}
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
		if g.ProjectID != "pNet" || fmt.Sprint(g.Roles) != "[staff]" {
			t.Fatalf("%s: got %+v", body, g)
		}
	}
	// Roles are now what the token carries: absent or null is none, anything
	// but a list of strings breaks the contract loudly.
	for body, ok := range map[string]bool{
		`{"project_id":"pNet"}`: true, `{"project_id":"pNet","roles":null}`: true,
		`{"project_id":"pNet","roles":"staff"}`: false, `{"project_id":"pNet","roles":[{"key":"staff"}]}`: false,
	} {
		var g ActionV2UserGrantRef
		if err := json.Unmarshal([]byte(body), &g); (err == nil) != ok || len(g.Roles) != 0 {
			t.Fatalf("%s: err %v, roles %v", body, err, g.Roles)
		}
	}
}
