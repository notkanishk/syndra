package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"syndra/internal/db"
)

// §23 — the surface has always said "this stays until somebody resolves it".
//
// Nothing could, so a legitimate volume replacement pinned a target as
// compromised permanently and that sentence named an action that did not exist.
// The three properties that make the action safe are the ones tested here.
func TestResolvingALogFinding(t *testing.T) {
	var got struct {
		target, actor, note, head string
	}
	orig := dbResolveLogViolation
	t.Cleanup(func() { dbResolveLogViolation = orig })
	dbResolveLogViolation = func(_ context.Context, target, actor, note, head string) (db.LogAnchor, error) {
		got.target, got.actor, got.note, got.head = target, actor, note, head
		return db.LogAnchor{Target: target, Head: head}, nil
	}

	resolve := func(body string) *httptest.ResponseRecorder {
		rr := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodPost, "/api/v1/targets/truenas/log-anchor/resolve",
			strings.NewReader(body))
		r.SetPathValue("target", "truenas")
		handleResolveLogFinding(rr, r)
		return rr
	}

	// Unconfirmed, and the copy says what is GIVEN UP rather than what is done.
	// It is the only action in the product that discards evidence.
	rr := resolve(`{"head":"abc","note":"we replaced the volume"}`)
	if rr.Code != http.StatusUnprocessableEntity {
		t.Fatalf("want a confirmation gate, got %d", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "stops being able to tell you") {
		t.Errorf("the confirmation must name what is lost: %s", rr.Body.String())
	}

	// An explanation is required. "We replaced the volume" and "we do not know"
	// are the same anchor state and completely different facts.
	if rr := resolve(`{"head":"abc","note":"  ","confirmed":true}`); rr.Code != http.StatusBadRequest {
		t.Errorf("a resolution with no explanation must be refused, got %d", rr.Code)
	}

	if rr := resolve(`{"head":"abc","note":"replaced the volume","confirmed":true}`); rr.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", rr.Code, rr.Body.String())
	}
	// The cited head travels, which is what makes this an adoption of something
	// the operator saw rather than of whatever the target says now.
	if got.head != "abc" || got.note != "replaced the volume" || got.target != "truenas" {
		t.Errorf("the citation and the reason must reach the store: %+v", got)
	}
}

// And a finding that moved is refused, not re-baselined onto a head nobody read.
func TestResolvingAFindingThatMovedIsRefused(t *testing.T) {
	orig := dbResolveLogViolation
	t.Cleanup(func() { dbResolveLogViolation = orig })
	dbResolveLogViolation = func(context.Context, string, string, string, string) (db.LogAnchor, error) {
		return db.LogAnchor{}, db.ErrAnchorMoved
	}

	rr := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/api/v1/targets/truenas/log-anchor/resolve",
		strings.NewReader(`{"head":"stale","note":"replaced the volume","confirmed":true}`))
	r.SetPathValue("target", "truenas")
	handleResolveLogFinding(rr, r)

	if rr.Code != http.StatusConflict {
		t.Fatalf("want 409, got %d: %s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "changed again") {
		t.Errorf("the refusal must say the log moved, which is the event this mechanism is for: %s", rr.Body.String())
	}
}
