package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"reflect"
	"strings"
	"testing"
	"time"

	"syndra/internal/db"
	"syndra/internal/models"
)

// Sign-in events (I6): translation, bounds, and that the grant path is never
// touched.

func TestTranslate_OIDCSessionAdded_UserComesFromPayloadNotAggregate(t *testing.T) {
	body := realZitadelBody("session-77", "oidc_session", "oidc_session.added", "someone",
		`{"userID":"user-5","sessionID":"s","clientID":"cid-1@portal","audience":["cid-1@portal","proj-9"],"scope":["openid"]}`)
	got, ok, err := translateZitadelEvent(body)
	if err != nil || !ok {
		t.Fatalf("ok=%v err=%v", ok, err)
	}
	if got.EventType != "user_signed_in" || got.UserID != "user-5" {
		t.Fatalf("got %+v; the session id must not become the user", got)
	}
	if want := []string{"cid-1@portal", "cid-1@portal", "proj-9"}; !reflect.DeepEqual(got.ClientRefs, want) {
		t.Errorf("refs = %v, want %v (explicit client id first, then audience)", got.ClientRefs, want)
	}
	if got.EventCreatedAt == nil {
		t.Error("the event's creation time must be carried")
	}
}

func TestTranslate_UserTokenAdded_AggregateIsTheUser_BothNames(t *testing.T) {
	for _, name := range []string{"user.token.added"} {
		body := realZitadelBody("user-9", "user", name, "user-9", `{"applicationId":"app-3","audience":"app-3"}`)
		got, ok, err := translateZitadelEvent(body)
		if err != nil || !ok {
			t.Fatalf("%s: ok=%v err=%v", name, ok, err)
		}
		if got.EventType != "user_signed_in" || got.UserID != "user-9" {
			t.Errorf("%s: got %+v", name, got)
		}
		if want := []string{"app-3", "app-3"}; !reflect.DeepEqual(got.ClientRefs, want) { // a bare-string audience is accepted
			t.Errorf("%s: refs = %v, want %v", name, got.ClientRefs, want)
		}
	}
}

func TestTranslate_SignIn_PayloadFieldNames(t *testing.T) {
	for payload, want := range map[string][]string{
		`{"applicationId":"a"}`:  {"a"},
		`{"clientId":"c"}`:       {"c"},
		`{"clientID":"c"}`:       {"c"},
		`{"client_id":"c"}`:      {"c"},
		`{"audience":["x","y"]}`: {"x", "y"},
		`{}`:                     nil,
	} {
		got, _, _ := translateZitadelEvent(realZitadelBody("user-1", "user", "user.token.added", "e", payload))
		if !reflect.DeepEqual(got.ClientRefs, want) {
			t.Errorf("%s: refs = %v, want %v", payload, got.ClientRefs, want)
		}
	}
}

func TestTranslate_SignIn_Bounds(t *testing.T) {
	long := strings.Repeat("a", maxSignInRefLen+1)
	// Refs: oversize, control characters and invalid UTF-8 are dropped; the
	// audience is read for at most 8 entries.
	aud := make([]string, 20)
	for i := range aud {
		aud[i] = fmt.Sprintf("aud-%d", i)
	}
	payload := fmt.Sprintf(`{"applicationId":%q,"clientId":"bad\u0007id","client_id":"ok id","audience":[%s]}`,
		long, `"`+strings.Join(aud, `","`)+`"`)
	got, _, _ := translateZitadelEvent(realZitadelBody("user-1", "user", "user.token.added", "e", payload))
	if len(got.ClientRefs) != 1+maxSignInAudience || got.ClientRefs[0] != "ok id" {
		t.Errorf("refs = %v", got.ClientRefs)
	}

	// A user id that is empty, too long, or not visible ASCII is not an event.
	for _, bad := range []string{"", strings.Repeat("9", maxSignInUserIDLen+1), "a b", "us\x00er", "üser"} {
		enc, _ := json.Marshal(bad)
		body := realZitadelBody("session-1", "oidc_session", "oidc_session.added", "e", fmt.Sprintf(`{"userID":%s}`, enc))
		if got, ok, err := translateZitadelEvent(body); err != nil || !ok || got.EventType != "" {
			t.Errorf("userID %q: got %+v ok=%v err=%v, want ignored", bad, got, ok, err)
		}
	}
	// A user.token event on the wrong aggregate type is ignored too.
	got, _, _ = translateZitadelEvent(realZitadelBody("x-1", "oidc_session", "user.token.added", "e", `{}`))
	if got.EventType != "" {
		t.Errorf("wrong aggregate type accepted: %+v", got)
	}
	// And a payload that is not JSON at all yields no refs, not a failure.
	got, _, err := translateZitadelEvent(realZitadelBody("user-1", "user", "user.token.added", "e", `"nope"`))
	if err != nil || got.EventType != "user_signed_in" || len(got.ClientRefs) != 0 {
		t.Errorf("got %+v err=%v", got, err)
	}
}

func TestTranslate_UnknownSessionEventsStayIgnored(t *testing.T) {
	for _, name := range []string{"session.added", "oidc_session.access_token.added", "user.human.password.check.succeeded"} {
		got, ok, err := translateZitadelEvent(realZitadelBody("a", "session", name, "e", `{"userID":"u"}`))
		if err != nil || !ok || got.EventType != "" {
			t.Errorf("%s: got %+v", name, got)
		}
	}
}

type usageCall struct {
	user, project, kind string
	at                  time.Time
}

// setupSignInDeps makes every grant-path seam fail the test if it is touched,
// and records usage instead of writing it.
func setupSignInDeps(t *testing.T) *[]usageCall {
	t.Helper()
	setupNoopWebhookDeps(t)
	prevUsage, prevApps := dbRecordUsage, directoryApplications
	t.Cleanup(func() { dbRecordUsage, directoryApplications = prevUsage, prevApps })

	trip := func(what string) { t.Errorf("a sign-in must not reach %s", what) }
	dbInsertWebhookEvent = func(context.Context, string, string, string, string, string) (string, bool, error) {
		trip("webhook_events")
		return "", true, nil
	}
	webhookEnforceMappingRules = func(context.Context, string, string, string) error { trip("mapping rules"); return nil }
	webhookRevokeMappingRules = func(context.Context, string, string, string) error { trip("mapping rules"); return nil }
	webhookTriggerOnboarding = func(context.Context, string, string, string) error { trip("onboarding"); return nil }
	cacheRebuildUser = func(context.Context, string, []string) { trip("the cache") }
	observeUser = func(context.Context, string) (db.Observation, error) {
		trip("observation")
		return db.Observation{}, nil
	}
	svcUserExpectsRole = func(context.Context, string, string, string) (bool, error) { trip("drift"); return false, nil }
	dbUpsertDriftItemWithEvidence = func(context.Context, string, string, string, []string, string, string, string, db.DriftEvidence) (string, bool, error) {
		trip("drift")
		return "", false, nil
	}

	var calls []usageCall
	dbRecordUsage = func(_ context.Context, user, project, kind string, at time.Time) error {
		calls = append(calls, usageCall{user, project, kind, at})
		return nil
	}
	directoryApplications = func(context.Context) ([]models.ApplicationCatalog, error) {
		return []models.ApplicationCatalog{
			{ID: "app-1", ProjectID: "proj-A", ClientID: "111@portal"},
			{ID: "app-2", ProjectID: "proj-B", ClientID: "222@wiki"},
			{ID: "orphan", ProjectID: ""},
		}, nil
	}
	return &calls
}

func TestWebhook_SignIn_RecordsUsageAndTouchesNothingElse(t *testing.T) {
	calls := setupSignInDeps(t)
	created := time.Now().Add(-time.Hour).UTC().Truncate(time.Second)
	body := fmt.Appendf(nil,
		`{"aggregateID":"sess-1","aggregateType":"oidc_session","resourceOwner":"o","instanceID":"i","version":"v1","sequence":1,"event_type":"oidc_session.added","created_at":%q,"userID":"u1","event_payload":{"userID":"u1","clientID":"222@wiki"}}`,
		created.Format(time.RFC3339))
	rr := postWebhook(t, body)
	if rr.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rr.Code, rr.Body.String())
	}
	if len(*calls) != 1 {
		t.Fatalf("usage calls = %v", *calls)
	}
	c := (*calls)[0]
	if c.user != "u1" || c.project != "proj-B" || c.kind != "sign_in" || !c.at.Equal(created) {
		t.Errorf("recorded %+v; want u1/proj-B/sign_in at %v (client id resolved to its project)", c, created)
	}
}

func TestWebhook_SignIn_ResolvesByAppIDAndFallsBackToAudience(t *testing.T) {
	calls := setupSignInDeps(t)
	postWebhook(t, realZitadelBody("u1", "user", "user.token.added", "u1", `{"applicationId":"app-1"}`))
	postWebhook(t, realZitadelBody("u2", "user", "user.token.added", "u2", `{"applicationId":"unknown","audience":["nope","111@portal"]}`))
	if len(*calls) != 2 || (*calls)[0].project != "proj-A" || (*calls)[1].project != "proj-A" {
		t.Errorf("calls = %v", *calls)
	}
}

func TestWebhook_SignIn_UnresolvableAppRecordsEmptyProject(t *testing.T) {
	calls := setupSignInDeps(t)
	postWebhook(t, realZitadelBody("u1", "user", "user.token.added", "u1", `{"applicationId":"who"}`))
	postWebhook(t, realZitadelBody("u1", "user", "user.token.added", "u1", `{"applicationId":"orphan"}`)) // app with no project
	postWebhook(t, realZitadelBody("u1", "user", "user.token.added", "u1", `{}`))
	directoryApplications = func(context.Context) ([]models.ApplicationCatalog, error) { return nil, errors.New("zitadel down") }
	postWebhook(t, realZitadelBody("u1", "user", "user.token.added", "u1", `{"applicationId":"app-1"}`))
	if len(*calls) != 4 {
		t.Fatalf("calls = %v", *calls)
	}
	for _, c := range *calls {
		if c.project != "" {
			t.Errorf("recorded project %q, want ''", c.project)
		}
	}
}

func TestWebhook_SignIn_TimeIsCappedAtNow(t *testing.T) {
	calls := setupSignInDeps(t)
	future := time.Now().Add(48 * time.Hour).UTC().Format(time.RFC3339)
	body := fmt.Appendf(nil, `{"aggregateID":"u1","aggregateType":"user","sequence":1,"event_type":"user.token.added","created_at":%q,"userID":"u1","event_payload":{}}`, future)
	before := time.Now()
	postWebhook(t, body)
	if len(*calls) != 1 || (*calls)[0].at.Before(before) || (*calls)[0].at.After(time.Now()) {
		t.Errorf("calls = %v; a future event time must be replaced by now", *calls)
	}
	// No created_at at all also means now.
	*calls = nil
	postWebhook(t, []byte(`{"aggregateID":"u1","aggregateType":"user","sequence":2,"event_type":"user.token.added","userID":"u1","event_payload":{}}`))
	if len(*calls) != 1 || (*calls)[0].at.Before(before) {
		t.Errorf("calls = %v", *calls)
	}
}

func TestWebhook_SignIn_UnusableEventsAreAcknowledgedAndNotRecorded(t *testing.T) {
	calls := setupSignInDeps(t)
	rr := postWebhook(t, realZitadelBody("sess", "oidc_session", "oidc_session.added", "e", `{"clientID":"x"}`)) // no userID
	if rr.Code != http.StatusOK || len(*calls) != 0 {
		t.Errorf("status %d calls %v", rr.Code, *calls)
	}
}

func TestWebhook_SignIn_StoreFailureIs500SoZitadelRedelivers(t *testing.T) {
	setupSignInDeps(t)
	dbRecordUsage = func(context.Context, string, string, string, time.Time) error { return errors.New("db down") }
	rr := postWebhook(t, realZitadelBody("u1", "user", "user.token.added", "u1", `{}`))
	if rr.Code != http.StatusInternalServerError {
		t.Errorf("status %d", rr.Code)
	}
}

func TestWebhook_SignIn_SelfMutationGuardStillApplies(t *testing.T) {
	calls := setupSignInDeps(t)
	t.Setenv("ZITADEL_M2M_USER_ID", "m2m-1")
	rr := postWebhook(t, realZitadelBody("m2m-1", "user", "user.token.added", "m2m-1", `{}`))
	if rr.Code != http.StatusOK || len(*calls) != 0 {
		t.Errorf("status %d calls %v", rr.Code, *calls)
	}
}

func TestWebhook_SignIn_InternalShapeNeedsAUserAndRecordsNoProject(t *testing.T) {
	calls := setupSignInDeps(t)
	if rr := postWebhook(t, []byte(`{"event_type":"user_signed_in"}`)); rr.Code != http.StatusBadRequest {
		t.Errorf("no user_id: status %d", rr.Code)
	}
	if rr := postWebhook(t, []byte(`{"event_type":"user_signed_in","user_id":"u1","source_project":"p"}`)); rr.Code != http.StatusOK {
		t.Errorf("status %d", rr.Code)
	}
	if len(*calls) != 1 || (*calls)[0].project != "" {
		t.Errorf("calls = %v", *calls)
	}
}

// The grant path is unchanged: a grant event still goes through dedupe and
// dispatch exactly as before, and an unknown event is still a no-op.
func TestWebhook_GrantPathUnchangedBySignInRouting(t *testing.T) {
	setupNoopWebhookDeps(t)
	var inserted, enforced int
	dbInsertWebhookEvent = func(context.Context, string, string, string, string, string) (string, bool, error) {
		inserted++
		return "evt", true, nil
	}
	webhookEnforceMappingRules = func(context.Context, string, string, string) error { enforced++; return nil }
	rr := postWebhook(t, realZitadelBody("g1", "user_grant", "user.grant.added", "e", `{"userId":"u1","projectId":"p1","roleKeys":["maker"]}`))
	if rr.Code != http.StatusOK || inserted != 1 || enforced != 1 {
		t.Errorf("status %d inserted %d enforced %d", rr.Code, inserted, enforced)
	}
	rr = postWebhook(t, realZitadelBody("x", "user", "user.something.else", "e", `{}`))
	if rr.Code != http.StatusOK || inserted != 1 {
		t.Errorf("unknown event: status %d inserted %d", rr.Code, inserted)
	}
}
