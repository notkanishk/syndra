package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"syndra/internal/claims"
	"syndra/internal/claimserve"
	"syndra/internal/claimserve/claimservetest"
	"syndra/internal/services"
)

// testPlane installs a loaded data plane over an in-memory Redis with these
// projects published, as the write side would have left them. Every project a
// test names but does not pass answers as never published.
func testPlane(t *testing.T, projects map[string]claimserve.Project) *claimservetest.Redis {
	t.Helper()
	r := claimservetest.New()
	if _, err := claimserve.Publish(context.Background(), r, projects); err != nil {
		t.Fatal(err)
	}
	p := claimserve.New(r, nil)
	if err := p.Poll(context.Background()); err != nil {
		t.Fatal(err)
	}
	orig := claimPlane
	claimPlane = p
	t.Cleanup(func() { claimPlane = orig })
	return r
}

// project is one project's published config: shapes and a failure mode.
func project(ps services.ProjectShapes, mode claimserve.Mode) claimserve.Project {
	if ps.Default.Lands == "" {
		ps.Default = claims.Builtin()
	}
	return claimserve.Project{Shapes: ps, Mode: mode}
}

var failClosed = claimserve.Mode{Mode: claimserve.FailClosed}

// builtinPlane publishes p1 with the built-in shape, failing closed.
func builtinPlane(t *testing.T, projectIDs ...string) *claimservetest.Redis {
	t.Helper()
	if len(projectIDs) == 0 {
		projectIDs = []string{"p1"}
	}
	ps := map[string]claimserve.Project{}
	for _, id := range projectIDs {
		ps[id] = project(services.ProjectShapes{}, failClosed)
	}
	return testPlane(t, ps)
}

func setFacts(r *claimservetest.Redis, userID, projectID, factsJSON string) {
	r.SetField(claimserve.FactsKey(userID), projectID, factsJSON)
}

// factsJSON builds the cache body the compiler writes: facts, not claims.
func factsJSON(userID, projectID string, roles ...string) string {
	b, _ := json.Marshal(claims.Facts{
		Roles:      roles,
		UserID:     userID,
		ProjectID:  projectID,
		Email:      userID + "@example.edu",
		Name:       "Test " + userID,
		Team:       "Fabrication",
		CompiledAt: "2026-07-30T11:04:00Z",
	})
	return string(b)
}

func decodeActionResponse(t *testing.T, rr *httptest.ResponseRecorder) ActionV2Response {
	t.Helper()
	var got ActionV2Response
	if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode ActionV2Response: %v\nbody: %s", err, rr.Body.String())
	}
	return got
}

// claimByKey returns the first claim matching key, or the zero value.
func claimByKey(claims []ActionV2Claim, key string) (ActionV2Claim, bool) {
	for _, c := range claims {
		if c.Key == key {
			return c, true
		}
	}
	return ActionV2Claim{}, false
}

// rolesJSON is a grant row's roles as Zitadel sends them.
func rolesJSON(roles ...string) json.RawMessage {
	b, _ := json.Marshal(roles)
	return b
}

// v2Body builds a minimal v2 function-trigger request body for a single user
// and grant list.
func v2Body(userID string, grants ...ActionV2UserGrantRef) string {
	b, _ := json.Marshal(map[string]interface{}{
		"function":    "function/preaccesstoken",
		"user":        map[string]string{"id": userID},
		"user_grants": grants,
	})
	return string(b)
}

func inject(t *testing.T, body string) *httptest.ResponseRecorder {
	t.Helper()
	rr := httptest.NewRecorder()
	HandleActionInject(rr, httptest.NewRequest(http.MethodPost, "/api/action/inject", strings.NewReader(body)))
	return rr
}

func TestHandleActionInject_CacheMissFailClosed(t *testing.T) {
	builtinPlane(t)
	rr := inject(t, v2Body("u1", ActionV2UserGrantRef{ProjectID: "p1", Roles: rolesJSON("admin")}))
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	if got := decodeActionResponse(t, rr); len(got.AppendClaims) != 0 {
		t.Fatalf("fail_closed: expected empty append_claims, got %v", got.AppendClaims)
	}
}

func TestHandleActionInject_CacheMissMinimalSafe(t *testing.T) {
	testPlane(t, map[string]claimserve.Project{"p1": project(services.ProjectShapes{},
		claimserve.Mode{Mode: claimserve.MinimalSafe, MinimalSafeClaims: map[string]any{"role": "guest"}})})
	got := decodeActionResponse(t, inject(t, v2Body("u1", ActionV2UserGrantRef{ProjectID: "p1", Roles: rolesJSON("viewer")})))
	if claim, ok := claimByKey(got.AppendClaims, "role"); !ok || claim.Value != "guest" {
		t.Fatalf("minimal_safe: expected role=guest, got %v", got.AppendClaims)
	}
}

func TestHandleActionInject_MalformedCache_FailClosed(t *testing.T) {
	r := builtinPlane(t)
	setFacts(r, "u1", "p1", "not-valid-json{{")
	if got := decodeActionResponse(t, inject(t, v2Body("u1", ActionV2UserGrantRef{ProjectID: "p1"}))); len(got.AppendClaims) != 0 {
		t.Fatalf("malformed cache fail_closed: expected empty append_claims, got %v", got.AppendClaims)
	}
}

func TestHandleActionInject_CacheHit_UsesConfiguredClaimNameAndFormat(t *testing.T) {
	r := testPlane(t, map[string]claimserve.Project{"p1": project(services.ProjectShapes{
		Default: claims.FromFlat("x-groups", claims.FormatCSV, nil, nil)}, failClosed)})
	setFacts(r, "u1", "p1", factsJSON("u1", "p1", "admin", "lab"))

	got := decodeActionResponse(t, inject(t, v2Body("u1", ActionV2UserGrantRef{ProjectID: "p1", Roles: rolesJSON("admin")})))
	claim, ok := claimByKey(got.AppendClaims, "x-groups")
	if !ok || claim.Value != "admin,lab" {
		t.Fatalf("expected csv-formatted roles admin,lab under x-groups, got %v", got.AppendClaims)
	}
}

// The token an operator previews and the token Zitadel receives must be shaped
// by the same profile. This pins the half of that contract the data plane owns:
// an attribute claim and a static claim configured on the profile both reach
// the envelope, alongside the roles.
func TestHandleActionInject_EmitsAttributeAndStaticClaims(t *testing.T) {
	r := testPlane(t, map[string]claimserve.Project{"pLaser": project(services.ProjectShapes{
		Default: claims.FromFlat("syndra.laser.roles", claims.FormatArray,
			map[string]string{"syndra.laser.team": claims.AttrTeam}, map[string]any{"syndra.tenant": "makerspace"})}, failClosed)})
	setFacts(r, "u1", "pLaser", factsJSON("u1", "pLaser", "trained"))

	got := decodeActionResponse(t, inject(t, v2Body("u1", ActionV2UserGrantRef{ProjectID: "pLaser", Roles: rolesJSON("trained")})))
	roles, okRoles := claimByKey(got.AppendClaims, "syndra.laser.roles")
	team, okTeam := claimByKey(got.AppendClaims, "syndra.laser.team")
	tenant, okTenant := claimByKey(got.AppendClaims, "syndra.tenant")
	if !okRoles || !okTeam || !okTenant {
		t.Fatalf("expected roles, attribute and static claims, got %v", got.AppendClaims)
	}
	if team.Value != "Fabrication" {
		t.Errorf("expected team claim from cached facts, got %v", team.Value)
	}
	if tenant.Value != "makerspace" {
		t.Errorf("expected static claim verbatim, got %v", tenant.Value)
	}
	list, ok := roles.Value.([]interface{})
	if !ok || len(list) != 1 || list[0] != "trained" {
		t.Errorf("expected array-formatted roles, got %#v", roles.Value)
	}
}

// A project the snapshot does not hold is never shaped by a guess: a guess
// could show a role the app's own shape hides. It answers fail closed.
func TestHandleActionInject_UnpublishedProjectIsNeverGuessed(t *testing.T) {
	r := builtinPlane(t, "pOther")
	setFacts(r, "u1", "pLaser", factsJSON("u1", "pLaser", "trained", "admin"))
	got := decodeActionResponse(t, inject(t, v2Body("u1", ActionV2UserGrantRef{ProjectID: "pLaser"})))
	if len(got.AppendClaims) != 0 {
		t.Fatalf("a guessed shape sent claims: %v", got.AppendClaims)
	}
}

// A token whose grants span two projects is a cross-project request (an app
// asked for another project's audience). Syndra delivers one project per
// token, so it carries no Syndra claims at all, and nothing is read.
func TestHandleActionInject_CrossProjectTokenCarriesNoClaims(t *testing.T) {
	r := testPlane(t, map[string]claimserve.Project{
		"pPrint": project(services.ProjectShapes{}, claimserve.Mode{Mode: claimserve.MinimalSafe, MinimalSafeClaims: map[string]any{"leak": true}}),
		"pDoor":  project(services.ProjectShapes{}, failClosed),
	})
	rr := inject(t, v2Body("u1",
		ActionV2UserGrantRef{ProjectID: "pPrint", Roles: rolesJSON("operator")},
		ActionV2UserGrantRef{ProjectID: "pDoor", Roles: rolesJSON("keyholder")},
	))
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 so Zitadel still issues the token, got %d", rr.Code)
	}
	if got := decodeActionResponse(t, rr); len(got.AppendClaims) != 0 {
		t.Fatalf("cross-project token: expected no claims, got %v", got.AppendClaims)
	}
	if !strings.Contains(rr.Body.String(), `"append_claims":[]`) {
		t.Errorf("append_claims must be an empty list, not null: %s", rr.Body.String())
	}
	if n := r.Count("HGET"); n != 0 {
		t.Errorf("facts read %d times for a cross-project token", n)
	}
}

func TestHandleActionInject_NoGrants_EmptyClaims(t *testing.T) {
	r := builtinPlane(t)
	rr := inject(t, v2Body("u1"))
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	if got := decodeActionResponse(t, rr); len(got.AppendClaims) != 0 {
		t.Fatalf("no grants: expected empty append_claims, got %v", got.AppendClaims)
	}
	if n := r.Count("HGET"); n != 0 {
		t.Errorf("no grants must short-circuit; facts read %d times", n)
	}
}

func TestHandleActionInject_DuplicateProjectGrants_DedupedToFlatKeys(t *testing.T) {
	// Zitadel emits one grant row per role. Two rows with the same projectId
	// but different roles MUST dedupe to a single project in Syndra's view,
	// or the token would read as cross-project and carry nothing.
	r := builtinPlane(t)
	setFacts(r, "u1", "p1", factsJSON("u1", "p1", "admin"))
	got := decodeActionResponse(t, inject(t, v2Body("u1",
		ActionV2UserGrantRef{ProjectID: "p1", Roles: rolesJSON("admin")},
		ActionV2UserGrantRef{ProjectID: "p1", Roles: rolesJSON("operator")},
	)))
	if n := r.Count("HGET"); n != 1 {
		t.Fatalf("expected exactly one facts read after dedup, got %d", n)
	}
	claim, ok := claimByKey(got.AppendClaims, "roles")
	list, isList := claim.Value.([]interface{})
	if !ok || !isList || len(list) != 1 || list[0] != "admin" {
		t.Fatalf("expected one deduped roles claim, got %v", got.AppendClaims)
	}
}

func TestHandleActionInject_InvalidJSON_400(t *testing.T) {
	builtinPlane(t)
	if rr := inject(t, `not-json`); rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid JSON, got %d", rr.Code)
	}
}

func TestHandleActionInject_UnknownFields_Accepted(t *testing.T) {
	// The v2 payload is Zitadel-owned and may grow. Unknown fields MUST NOT
	// cause a 400 — they are silently ignored by the lenient decoder.
	r := builtinPlane(t)
	setFacts(r, "u1", "p1", factsJSON("u1", "p1", "admin"))
	body := `{
		"function":"function/preaccesstoken",
		"user":{"id":"u1","human":{"profile":{"displayName":"Test"}}},
		"user_grants":[{"projectId":"p1","roles":["admin"],"projectName":"Printing"}],
		"org":{"id":"org1","name":"Example"},
		"user_metadata":[{"key":"foo","value":"YmFy"}]
	}`
	rr := inject(t, body)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 with unknown fields accepted, got %d (body=%s)", rr.Code, rr.Body.String())
	}
	claim, ok := claimByKey(decodeActionResponse(t, rr).AppendClaims, "roles")
	list, isList := claim.Value.([]interface{})
	if !ok || !isList || len(list) != 1 || list[0] != "admin" {
		t.Fatalf("expected roles=[admin], got %s", rr.Body.String())
	}
}
