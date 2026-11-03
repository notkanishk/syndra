package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"syndra/internal/auth"
	"syndra/internal/db"
	"syndra/internal/models"
	"syndra/internal/zitadel"
)

// Sign-ins per app (A-10 Part 2; owner, 3 Nov).

func receivingSignIns(t *testing.T, receiving bool) {
	t.Helper()
	stubUsage(t)
	dbUsageLatestByKind = func(context.Context, time.Time) (map[string]time.Time, error) {
		if !receiving {
			return map[string]time.Time{}, nil
		}
		return map[string]time.Time{db.UsageSignIn: time.Now()}, nil
	}
}

func TestUserUsage_PerAppWithWhetherSignInsArrive(t *testing.T) {
	receivingSignIns(t, true)
	at := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	since := time.Date(2026, 9, 3, 0, 0, 0, 0, time.UTC)
	var asked string
	dbGetUserUsage = func(_ context.Context, id string) ([]db.UsageRow, error) {
		asked = id
		return []db.UsageRow{{ProjectID: "p", AppID: "app-1", Kind: "sign_in", LastAt: at}}, nil
	}
	dbSignInsSince = func(context.Context) (*time.Time, error) { return &since, nil }

	req := httptest.NewRequest(http.MethodGet, "/api/v1/users/u7/usage", nil)
	req.SetPathValue("id", "u7")
	rr := httptest.NewRecorder()
	handleGetUserUsage(rr, req)
	var got struct {
		Usage          []map[string]any `json:"usage"`
		Receiving      bool             `json:"receiving"`
		ReceivingSince *time.Time       `json:"receiving_since"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil || rr.Code != 200 || asked != "u7" {
		t.Fatalf("status %d asked %q body %s", rr.Code, asked, rr.Body.String())
	}
	if len(got.Usage) != 1 || got.Usage[0]["app_id"] != "app-1" || got.Usage[0]["project_id"] != "p" ||
		!got.Receiving || got.ReceivingSince == nil || !got.ReceivingSince.Equal(since) {
		t.Fatalf("body %s", rr.Body.String())
	}

	// Sign-ins not arriving: said, and no "since" to build a sentence on.
	receivingSignIns(t, false)
	dbGetUserUsage = func(context.Context, string) ([]db.UsageRow, error) { return []db.UsageRow{}, nil }
	rr = httptest.NewRecorder()
	handleGetUserUsage(rr, req)
	if rr.Code != 200 || !strings.Contains(rr.Body.String(), `"receiving":false`) || strings.Contains(rr.Body.String(), "receiving_since") {
		t.Fatalf("not receiving: %d %s", rr.Code, rr.Body.String())
	}

	dbGetUserUsage = func(context.Context, string) ([]db.UsageRow, error) { return nil, errors.New("boom") }
	rr = httptest.NewRecorder()
	handleGetUserUsage(rr, req)
	if rr.Code != http.StatusInternalServerError {
		t.Errorf("status %d", rr.Code)
	}
}

func TestAppUsage_NamesNobodyAndSaysNothingWhenSignInsDontArrive(t *testing.T) {
	prev := dbGetAppUsage
	t.Cleanup(func() { dbGetAppUsage = prev })
	last := time.Now().Add(-10 * time.Minute).UTC().Truncate(time.Second)
	var asked string
	var recent, window time.Time
	dbGetAppUsage = func(_ context.Context, id string, r, w time.Time) (db.AppUsage, error) {
		asked, recent, window = id, r, w
		return db.AppUsage{RecentPeople: 9, WindowPeople: 23, LastAt: &last,
			Days: []db.SignInDay{{Day: "2026-11-02", Count: 5}}}, nil
	}
	get := func() *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/applications/app-1/usage", nil)
		req.SetPathValue("id", "app-1")
		rr := httptest.NewRecorder()
		handleGetApplicationUsage(rr, req)
		return rr
	}

	receivingSignIns(t, false)
	rr := get()
	if rr.Code != 200 || strings.TrimSpace(rr.Body.String()) != `{"receiving":false}` || asked != "" {
		t.Fatalf("not receiving must say only that: %d %s (asked %q)", rr.Code, rr.Body.String(), asked)
	}

	receivingSignIns(t, true)
	rr = get()
	var got map[string]any
	json.Unmarshal(rr.Body.Bytes(), &got)
	if rr.Code != 200 || asked != "app-1" || got["people_7d"] != float64(9) || got["people_30d"] != float64(23) ||
		got["last_at"] == nil || len(got["days"].([]any)) != 1 {
		t.Fatalf("%d %s", rr.Code, rr.Body.String())
	}
	if d := time.Since(recent); d < 7*24*time.Hour-time.Minute || d > 7*24*time.Hour+time.Minute {
		t.Errorf("people window starts %v ago, want 7 days", d)
	}
	if d := time.Since(window); d < 30*24*time.Hour-time.Minute || d > 30*24*time.Hour+time.Minute {
		t.Errorf("strip starts %v ago, want 30 days", d)
	}
	if strings.Contains(rr.Body.String(), "user") {
		t.Errorf("an app's totals name nobody: %s", rr.Body.String())
	}
}

// ── Look up sign-ins ──

type lookupHarness struct {
	audits   []map[string]any
	auditErr error
	searched []zitadel.EventSearch
	events   []zitadel.LoggedEvent
	truncate bool
	zErr     error
	appsErr  error
}

func stubLookup(t *testing.T) *lookupHarness {
	t.Helper()
	h := &lookupHarness{}
	audit, search, apps := dbInsertAuditLogDetails, zitadelSearchEvents, directoryApplications
	t.Cleanup(func() { dbInsertAuditLogDetails, zitadelSearchEvents, directoryApplications = audit, search, apps })
	dbInsertAuditLogDetails = func(_ context.Context, actor, target, action, resource string, details any) error {
		if h.auditErr != nil {
			return h.auditErr
		}
		h.audits = append(h.audits, map[string]any{"actor": actor, "target": target, "action": action, "resource": resource, "details": details})
		return nil
	}
	zitadelSearchEvents = func(_ context.Context, q zitadel.EventSearch) ([]zitadel.LoggedEvent, bool, error) {
		h.searched = append(h.searched, q)
		return h.events, h.truncate, h.zErr
	}
	directoryApplications = func(context.Context) ([]models.ApplicationCatalog, error) {
		if h.appsErr != nil {
			return nil, h.appsErr
		}
		return []models.ApplicationCatalog{
			{ID: "app-1", ProjectID: "proj-A", ClientID: "111@portal"},
			{ID: "app-2", ProjectID: "proj-B", ClientID: "222@wiki"},
		}, nil
	}
	return h
}

func lookUp(body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/api/v1/sign-ins/lookup", strings.NewReader(body))
	req = req.WithContext(withPrincipal(req.Context(), &auth.Principal{Subject: "op-1"}))
	rr := httptest.NewRecorder()
	handleLookUpSignIns(rr, req)
	return rr
}

func at(s string) time.Time { t, _ := time.Parse(time.RFC3339, s); return t }

func TestLookUpSignIns_RefusesWithoutAReasonARangeOrSomeone(t *testing.T) {
	h := stubLookup(t)
	for name, body := range map[string]string{
		"no reason":     `{"user_id":"u1","days":30,"reason":"  "}`,
		"no one":        `{"days":30,"reason":"a complaint"}`,
		"odd range":     `{"user_id":"u1","days":45,"reason":"a complaint"}`,
		"long reason":   `{"user_id":"u1","days":30,"reason":"` + strings.Repeat("x", 501) + `"}`,
		"unknown field": `{"user_id":"u1","days":30,"reason":"a complaint","everyone":true}`,
	} {
		if rr := lookUp(body); rr.Code != http.StatusBadRequest {
			t.Errorf("%s: status %d", name, rr.Code)
		}
	}
	if len(h.audits) != 0 || len(h.searched) != 0 {
		t.Fatalf("a refused lookup is neither audited nor made: audits=%v searched=%v", h.audits, h.searched)
	}
}

func TestLookUpSignIns_NotAuditedIsNotMade(t *testing.T) {
	h := stubLookup(t)
	h.auditErr = errors.New("db down")
	rr := lookUp(`{"user_id":"u1","days":30,"reason":"a complaint"}`)
	if rr.Code != http.StatusInternalServerError || len(h.searched) != 0 {
		t.Fatalf("status %d searched %v; an unaudited lookup must not reach Zitadel", rr.Code, h.searched)
	}
}

func TestLookUpSignIns_AuditsFirstThenReadsZitadelAndFiltersHere(t *testing.T) {
	h := stubLookup(t)
	h.truncate = true
	h.events = []zitadel.LoggedEvent{
		// u1 to app-1 by its app id, and to app-2 by client id in an OIDC session.
		{Type: "user.token.added", AggregateID: "u1", AggregateType: "user", At: at("2026-11-02T09:00:00Z"),
			Payload: json.RawMessage(`{"applicationId":"app-1"}`)},
		{Type: "oidc_session.added", AggregateID: "sess-1", AggregateType: "oidc_session", At: at("2026-11-02T10:00:00Z"),
			Payload: json.RawMessage(`{"userID":"u1","clientID":"222@wiki"}`)},
		// Somebody else, and an event naming nobody.
		{Type: "user.token.added", AggregateID: "u2", AggregateType: "user", At: at("2026-11-01T09:00:00Z"),
			Payload: json.RawMessage(`{"applicationId":"app-1"}`)},
		{Type: "oidc_session.added", AggregateID: "sess-2", AggregateType: "oidc_session", At: at("2026-11-01T08:00:00Z"),
			Payload: json.RawMessage(`{"clientID":"111@portal"}`)},
	}

	rr := lookUp(`{"user_id":"u1","days":30,"reason":"They say they never used it"}`)
	if rr.Code != 200 {
		t.Fatalf("status %d: %s", rr.Code, rr.Body.String())
	}
	if len(h.audits) != 1 || len(h.searched) != 1 {
		t.Fatalf("audits=%v searched=%v", h.audits, h.searched)
	}
	a := h.audits[0]
	details, _ := a["details"].(map[string]any)
	if a["actor"] != "op-1" || a["target"] != "u1" || a["action"] != "sign_ins.looked_up" ||
		details["reason"] != "They say they never used it" || details["days"] != 30 || details["since"] == nil {
		t.Errorf("audit = %v", a)
	}
	q := h.searched[0]
	if got := q.Until.Sub(q.Since); got != 30*24*time.Hour || q.Limit != lookupReadLimit ||
		strings.Join(q.Types, ",") != "user.token.added,oidc_session.added" {
		t.Errorf("search = %+v", q)
	}

	var got struct {
		SignIns   []lookedUpSignIn `json:"sign_ins"`
		Truncated bool             `json:"truncated"`
	}
	json.Unmarshal(rr.Body.Bytes(), &got)
	if !got.Truncated || len(got.SignIns) != 2 {
		t.Fatalf("got %s", rr.Body.String())
	}
	if s := got.SignIns[0]; s.UserID != "u1" || s.AppID != "app-2" || s.ProjectID != "proj-B" {
		t.Errorf("newest first, resolved through the client id: %+v", s)
	}
	if s := got.SignIns[1]; s.AppID != "app-1" {
		t.Errorf("second = %+v", s)
	}

	// One app, everyone: names come back, filtered to that app.
	rr = lookUp(`{"app_id":"app-1","days":7,"reason":"Was the laser booked out of hours?"}`)
	json.Unmarshal(rr.Body.Bytes(), &got)
	if rr.Code != 200 || len(got.SignIns) != 2 || got.SignIns[0].UserID != "u1" || got.SignIns[1].UserID != "u2" {
		t.Fatalf("app lookup: %s", rr.Body.String())
	}
	if h.audits[1]["resource"] != "app-1" || h.audits[1]["target"] != "" {
		t.Errorf("app audit = %v", h.audits[1])
	}
}

func TestLookUpSignIns_ZitadelOrDirectoryFailureSaysSo(t *testing.T) {
	h := stubLookup(t)
	h.zErr = errors.New("timeout")
	if rr := lookUp(`{"user_id":"u1","days":7,"reason":"r"}`); rr.Code != http.StatusBadGateway || len(h.audits) != 1 {
		t.Fatalf("status %d audits %d; the attempt stays audited", rr.Code, len(h.audits))
	}
	h.zErr = errNoClient
	if rr := lookUp(`{"user_id":"u1","days":7,"reason":"r"}`); rr.Code != http.StatusServiceUnavailable {
		t.Errorf("no client: status %d", rr.Code)
	}

	// Apps unreadable: a person's lookup still answers ("app unknown"); an
	// app's lookup can't tell which sign-ins were its own and refuses.
	h.zErr, h.appsErr = nil, errors.New("down")
	h.events = []zitadel.LoggedEvent{{Type: "user.token.added", AggregateID: "u1", AggregateType: "user",
		At: at("2026-11-02T09:00:00Z"), Payload: json.RawMessage(`{"applicationId":"app-1"}`)}}
	rr := lookUp(`{"user_id":"u1","days":7,"reason":"r"}`)
	if rr.Code != 200 || !strings.Contains(rr.Body.String(), `"app_id":""`) {
		t.Errorf("person, apps down: %d %s", rr.Code, rr.Body.String())
	}
	if rr := lookUp(`{"app_id":"app-1","days":7,"reason":"r"}`); rr.Code != http.StatusBadGateway {
		t.Errorf("app, apps down: status %d", rr.Code)
	}
}

// Through the REAL router: the lookup, the app totals and a person's storage
// are an operator's.
func TestRouter_SignInAndPersonStorageRoutesAreOperatorOnly(t *testing.T) {
	stubLookup(t)
	receivingSignIns(t, false)
	t.Setenv("ZITADEL_DOMAIN", "example.zitadel.cloud")
	t.Setenv("ZITADEL_AUDIENCE", "test-aud")
	t.Setenv("ZITADEL_ADMIN_ROLE_KEY", "admin")
	origValidate := jwtValidate
	t.Cleanup(func() { jwtValidate = origValidate })
	jwtValidate = func(context.Context, string, string, string) (*auth.Principal, error) {
		return &auth.Principal{Subject: "u", ProjectRoles: map[string]struct{}{"member": {}}}, nil
	}
	router := NewRouter()
	for _, c := range []struct{ method, path, body string }{
		{http.MethodPost, "/api/v1/sign-ins/lookup", `{"user_id":"u1","days":7,"reason":"r"}`},
		{http.MethodGet, "/api/v1/applications/app-1/usage", ""},
		{http.MethodGet, "/api/v1/users/u1/targets", ""},
	} {
		req := httptest.NewRequest(c.method, c.path, strings.NewReader(c.body))
		req.Header.Set("Authorization", "Bearer token")
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)
		if rr.Code != http.StatusForbidden {
			t.Errorf("%s %s as a member: status %d", c.method, c.path, rr.Code)
		}
	}
}
