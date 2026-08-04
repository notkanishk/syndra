package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"syndra/internal/db"
	"syndra/internal/models"
)

func resetAckDeps(t *testing.T) {
	t.Helper()
	origAck := dbAcknowledgeGrantExpiry
	origClear := dbClearGrantExpiryAcknowledgement
	origRead := dbGetExpiringWithAcks
	origAudit := dbInsertAuditLog
	t.Cleanup(func() {
		dbAcknowledgeGrantExpiry = origAck
		dbClearGrantExpiryAcknowledgement = origClear
		dbGetExpiringWithAcks = origRead
		dbInsertAuditLog = origAudit
	})
	dbInsertAuditLog = func(context.Context, string, string, string, string) error { return nil }
}

func ackRequest(t *testing.T, grantID string, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost,
		"/api/v1/review/expiring-grants/"+grantID+"/acknowledge", strings.NewReader(body))
	req.SetPathValue("grantId", grantID)
	rr := httptest.NewRecorder()
	handleAcknowledgeGrantExpiry(rr, req)
	return rr
}

// The reopen rule's user-facing half. An operator whose page was loaded before somebody extended
// the grant is acknowledging a date that no longer exists — and the write is refused rather than
// stored, because a stored one would be accepted and then never apply.
func TestAcknowledgeGrantExpiry_MovedDateIsRefusedWithAnExplanation(t *testing.T) {
	resetAckDeps(t)
	dbAcknowledgeGrantExpiry = func(context.Context, string, time.Time, string, string) (string, error) {
		return "", db.ErrGrantExpiryMoved
	}

	rr := ackRequest(t, "g1", `{"expires_at":"2026-09-01T12:00:00Z"}`)

	if rr.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d: %s", rr.Code, rr.Body.String())
	}
	// The message has to say what to do next, not just that something failed.
	if !strings.Contains(rr.Body.String(), "Reload") {
		t.Errorf("the conflict must tell the operator to reload before deciding: %s", rr.Body.String())
	}
}

// The date is the decision. Without it there is nothing to compare against later, so the
// acknowledgement would be permanent — which is the rule we deliberately did not choose.
func TestAcknowledgeGrantExpiry_RequiresTheDate(t *testing.T) {
	resetAckDeps(t)
	called := false
	dbAcknowledgeGrantExpiry = func(context.Context, string, time.Time, string, string) (string, error) {
		called = true
		return "u1", nil
	}

	rr := ackRequest(t, "g1", `{"note":"letting this go"}`)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}
	if called {
		t.Error("nothing should be written without the date the decision is about")
	}
}

// Passed through verbatim: the note is what the NEXT operator reads, and the date is what the
// reopen rule is measured against.
func TestAcknowledgeGrantExpiry_PassesTheDateAndNoteThrough(t *testing.T) {
	resetAckDeps(t)
	var gotID, gotActor, gotNote string
	var gotAt time.Time
	dbAcknowledgeGrantExpiry = func(_ context.Context, id string, at time.Time, actor, note string) (string, error) {
		gotID, gotAt, gotActor, gotNote = id, at, actor, note
		return "u1", nil
	}

	var auditAction, auditTarget string
	dbInsertAuditLog = func(_ context.Context, _, target, action, _ string) error {
		auditAction, auditTarget = action, target
		return nil
	}

	rr := ackRequest(t, "g1", `{"expires_at":"2026-09-01T12:00:00Z","note":"  Cohort ends  "}`)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	if gotID != "g1" || gotActor == "" {
		t.Errorf("expected the grant id and an actor, got %q / %q", gotID, gotActor)
	}
	if !gotAt.Equal(time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)) {
		t.Errorf("expected the submitted date, got %v", gotAt)
	}
	if gotNote != "Cohort ends" {
		t.Errorf("expected a trimmed note, got %q", gotNote)
	}
	// Audited against the person whose access it is, not against the grant id alone — their
	// Activity tab is where somebody asks "why did this lapse".
	if auditAction != "grant_expiry.acknowledged" || auditTarget != "u1" {
		t.Errorf("expected the acknowledgement audited against u1, got %q / %q", auditAction, auditTarget)
	}
}

// The response must not let an operator believe they have kept the access alive.
func TestAcknowledgeGrantExpiry_SaysTheGrantStillLapses(t *testing.T) {
	resetAckDeps(t)
	dbAcknowledgeGrantExpiry = func(context.Context, string, time.Time, string, string) (string, error) {
		return "u1", nil
	}

	rr := ackRequest(t, "g1", `{"expires_at":"2026-09-01T12:00:00Z"}`)

	if !strings.Contains(rr.Body.String(), "still lapses") {
		t.Errorf("acknowledging is not extending, and the response must say so: %s", rr.Body.String())
	}
}

func TestAcknowledgeGrantExpiry_UnknownGrantIs404(t *testing.T) {
	resetAckDeps(t)
	dbAcknowledgeGrantExpiry = func(context.Context, string, time.Time, string, string) (string, error) {
		return "", db.ErrGrantNotFound
	}

	rr := ackRequest(t, "gone", `{"expires_at":"2026-09-01T12:00:00Z"}`)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", rr.Code, rr.Body.String())
	}
}

// Unknown fields are rejected on every mutation endpoint; this one is no exception, and the
// console must not be able to invent a field that quietly does nothing.
func TestAcknowledgeGrantExpiry_RejectsUnknownFields(t *testing.T) {
	resetAckDeps(t)
	dbAcknowledgeGrantExpiry = func(context.Context, string, time.Time, string, string) (string, error) {
		t.Fatal("must not reach the write")
		return "", nil
	}

	rr := ackRequest(t, "g1", `{"expires_at":"2026-09-01T12:00:00Z","until":"forever"}`)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestClearAcknowledgement_NothingToTakeBackIs404(t *testing.T) {
	resetAckDeps(t)
	dbClearGrantExpiryAcknowledgement = func(context.Context, string) (string, error) {
		return "", db.ErrAcknowledgementNotFound
	}

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/review/expiring-grants/g1/acknowledge", nil)
	req.SetPathValue("grantId", "g1")
	rr := httptest.NewRecorder()
	handleClearGrantExpiryAcknowledgement(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestClearAcknowledgement_AuditsAgainstThePerson(t *testing.T) {
	resetAckDeps(t)
	dbClearGrantExpiryAcknowledgement = func(context.Context, string) (string, error) { return "u7", nil }
	var action, target string
	dbInsertAuditLog = func(_ context.Context, _, tgt, act, _ string) error {
		action, target = act, tgt
		return nil
	}

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/review/expiring-grants/g1/acknowledge", nil)
	req.SetPathValue("grantId", "g1")
	rr := httptest.NewRecorder()
	handleClearGrantExpiryAcknowledgement(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	if action != "grant_expiry.acknowledgement_cleared" || target != "u7" {
		t.Errorf("expected the clearing audited against u7, got %q / %q", action, target)
	}
}

// An org that has acknowledged nothing must serialise as [], not null — the console renders a
// null list as a failure, which would report "couldn't load" for an empty queue.
func TestGetExpiringGrants_EmptyIsAnArray(t *testing.T) {
	resetAckDeps(t)
	dbGetExpiringWithAcks = func(context.Context, time.Duration) ([]models.ExpiringGrant, error) {
		return nil, nil
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/review/expiring-grants", nil)
	rr := httptest.NewRecorder()
	handleGetExpiringGrants(rr, req)

	if !strings.Contains(rr.Body.String(), `"grants":[]`) {
		t.Errorf("expected an empty array, got %s", rr.Body.String())
	}
}

// The acknowledgement travels on the row it belongs to, flattened alongside the grant's own
// fields — the console reads one shape, not a grant plus a lookup table.
func TestGetExpiringGrants_CarriesTheAcknowledgement(t *testing.T) {
	resetAckDeps(t)
	at := time.Date(2026, 8, 4, 9, 0, 0, 0, time.UTC)
	dbGetExpiringWithAcks = func(context.Context, time.Duration) ([]models.ExpiringGrant, error) {
		return []models.ExpiringGrant{{
			DirectGrant:  models.DirectGrant{ID: "g1", UserID: "u1", RoleKey: "trained"},
			Acknowledged: &models.GrantExpiryAcknowledgement{By: "priya", At: at, Note: "Cohort ends"},
		}}, nil
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/review/expiring-grants", nil)
	rr := httptest.NewRecorder()
	handleGetExpiringGrants(rr, req)

	var body struct {
		Grants []struct {
			ID           string `json:"id"`
			RoleKey      string `json:"role_key"`
			Acknowledged *struct {
				By   string `json:"by"`
				Note string `json:"note"`
			} `json:"acknowledged"`
		} `json:"grants"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v — %s", err, rr.Body.String())
	}
	if len(body.Grants) != 1 || body.Grants[0].RoleKey != "trained" {
		t.Fatalf("expected the grant's own fields flattened, got %s", rr.Body.String())
	}
	if body.Grants[0].Acknowledged == nil || body.Grants[0].Acknowledged.By != "priya" {
		t.Fatalf("expected the acknowledgement on the row, got %s", rr.Body.String())
	}
}
