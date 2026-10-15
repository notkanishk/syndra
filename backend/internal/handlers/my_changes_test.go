package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"syndra/internal/auth"
	"syndra/internal/db"
)

func stubMyChanges(t *testing.T, rows []db.MyChangeRow) (gotUser *string, gotDays *int) {
	t.Helper()
	orig := dbListMyChanges
	t.Cleanup(func() { dbListMyChanges = orig })
	var user string
	var days int
	dbListMyChanges = func(_ context.Context, u string, d int) ([]db.MyChangeRow, error) {
		user, days = u, d
		return rows, nil
	}
	return &user, &days
}

func myChangesRequest(subject, query string) *http.Request {
	r := httptest.NewRequest(http.MethodGet, "/api/v1/me/changes"+query, nil)
	if subject != "" {
		r = r.WithContext(withPrincipal(r.Context(), &auth.Principal{Subject: subject}))
	}
	return r
}

// The feed is the caller's own, always: the subject comes from the token and
// there is no way to name somebody else.
func TestMyChanges_ReadsOnlyTheCallersOwn(t *testing.T) {
	user, days := stubMyChanges(t, nil)
	w := httptest.NewRecorder()
	handleMyChanges(w, myChangesRequest("priya", "?user_id=someone-else"))
	if w.Code != http.StatusOK {
		t.Fatalf("status %d: %s", w.Code, w.Body)
	}
	if *user != "priya" || *days != 30 {
		t.Fatalf("read %q over %d days, want priya over 30", *user, *days)
	}
	if !strings.Contains(w.Body.String(), `"items":[]`) {
		t.Fatalf("an empty month is an empty list, not null: %s", w.Body)
	}
}

func TestMyChanges_RefusesAnonymousAndOddWindows(t *testing.T) {
	stubMyChanges(t, nil)
	w := httptest.NewRecorder()
	handleMyChanges(w, myChangesRequest("", ""))
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("no subject: got %d", w.Code)
	}
	for _, q := range []string{"?days=0", "?days=91", "?days=week"} {
		w := httptest.NewRecorder()
		handleMyChanges(w, myChangesRequest("priya", q))
		if w.Code != http.StatusBadRequest {
			t.Fatalf("%s: got %d", q, w.Code)
		}
	}
	_, days := stubMyChanges(t, nil)
	handleMyChanges(httptest.NewRecorder(), myChangesRequest("priya", "?days=7"))
	if *days != 7 {
		t.Fatalf("days=7 read %d", *days)
	}
}

// What a member is told, from Syndra's records: who decided (never "system"
// or a rule), which bundle or rule it came through, and for an approval still
// queued, that it waits and why. Nothing else about the queued row.
func TestShapeMyChanges(t *testing.T) {
	at := time.Date(2026, 10, 2, 9, 0, 0, 0, time.UTC)
	until := at.Add(18 * 24 * time.Hour)
	got := shapeMyChanges([]db.MyChangeRow{
		{Kind: "request_approved", At: at, ProjectID: "p", RoleKey: "sup", Actor: "kanishk", Note: " Book the evening slots. ", Until: &until, Pending: true, WaitReason: "role_waits"},
		{Kind: "request_approved", At: at, ProjectID: "p", RoleKey: "op", Actor: "kanishk", Pending: true, WaitReason: "halted:zitadel_offline"},
		{Kind: "request_approved", At: at, ProjectID: "p", RoleKey: "sent", Actor: "kanishk"},
		{Kind: "request_declined", At: at, ProjectID: "p", RoleKey: "m", Actor: "system", Note: "n"},
		{Kind: "request_withdrawn", At: at, ProjectID: "p", RoleKey: "w"},
		{Kind: "given", At: at, ProjectID: "p", RoleKey: "b", Actor: "kanishk", Source: "bundle", SourceRef: "b1"},
		{Kind: "given", At: at, ProjectID: "p", RoleKey: "r", Actor: "kanishk", Source: "rule", SourceRef: "r1"},
		{Kind: "taken_away", At: at, ProjectID: "p", RoleKey: "t", Actor: "system:expiry", Source: "direct", Pending: true},
		{Kind: "taken_away", At: at, ProjectID: "p", RoleKey: "l", Actor: "kanishk", Source: "lifecycle_cascade"},
	})
	if len(got) != 9 {
		t.Fatalf("got %d items", len(got))
	}
	if c := got[0]; !c.Waiting || c.WaitReason != "role_waits" || c.By != "kanishk" || c.Note != "Book the evening slots." || c.Until == nil || c.Via != nil {
		t.Fatalf("a waiting approval: %+v", c)
	}
	if c := got[1]; !c.Waiting || c.WaitReason != "halted:zitadel_offline" {
		t.Fatalf("a halted approval says so: %+v", c)
	}
	if got[2].Waiting {
		t.Fatal("a sent approval doesn't wait")
	}
	if got[3].By != "" || got[4].By != "" {
		t.Fatalf("decided by Syndra, or withdrawn by the member, names nobody: %+v %+v", got[3], got[4])
	}
	if c := got[5]; c.Via == nil || c.Via.BundleID != "b1" || c.By != "kanishk" {
		t.Fatalf("via a bundle: %+v", c)
	}
	if c := got[6]; c.Via == nil || c.Via.RuleID != "r1" || c.By != "" {
		t.Fatalf("via a rule is automatic: %+v", c)
	}
	if c := got[7]; c.By != "" || c.Waiting {
		t.Fatalf("an expiry is automatic, and only approvals wait: %+v", c)
	}
	if got[8].By != "" {
		t.Fatalf("leaving the makerspace is automatic: %+v", got[8])
	}

	body, _ := json.Marshal(got[4])
	if !strings.Contains(string(body), `"via":null`) || strings.Contains(string(body), `"by"`) || strings.Contains(string(body), "waiting") {
		t.Fatalf("via is null, by and waiting absent: %s", body)
	}
}
