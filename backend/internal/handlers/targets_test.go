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

// §29's surface — the operator action, and the two things that make it safe.
func TestResolvingABindingConflict(t *testing.T) {
	var got struct{ id, owner, actor, note string }
	orig := dbResolveBindingConflict
	t.Cleanup(func() { dbResolveBindingConflict = orig })
	dbResolveBindingConflict = func(_ context.Context, id, owner, actor, note string) (db.BindingConflict, error) {
		got.id, got.owner, got.actor, got.note = id, owner, actor, note
		return db.BindingConflict{Target: "truenas", Username: "ada"}, nil
	}

	resolve := func(body string) *httptest.ResponseRecorder {
		rr := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodPost,
			"/api/v1/targets/truenas/binding-conflicts/c1/resolve", strings.NewReader(body))
		r.SetPathValue("target", "truenas")
		r.SetPathValue("id", "c1")
		handleResolveBindingConflict(rr, r)
		return rr
	}

	// The confirmation names the person who LOSES the account — the half an
	// operator can get wrong and the half nobody is notified about.
	rr := resolve(`{"owner":"subject-a","note":"payroll confirmed it is hers"}`)
	if rr.Code != http.StatusUnprocessableEntity {
		t.Fatalf("want a confirmation gate, got %d", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "stops holding it here") {
		t.Errorf("the confirmation must name what the other subject loses: %s", rr.Body.String())
	}

	if rr := resolve(`{"owner":"subject-a","note":"  ","confirmed":true}`); rr.Code != http.StatusBadRequest {
		t.Errorf("a resolution with no explanation must be refused, got %d", rr.Code)
	}

	rr = resolve(`{"owner":"subject-a","note":"payroll confirmed it is hers","confirmed":true}`)
	if rr.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", rr.Code, rr.Body.String())
	}
	if got.id != "c1" || got.owner != "subject-a" || got.note == "" {
		t.Errorf("the citation, the owner and the reason must all reach the store: %+v", got)
	}
	// And it says the TARGET was not touched. Syndra's records agreeing is not
	// the same as the account being right, and an operator who reads this as
	// "done" would not converge.
	if !strings.Contains(rr.Body.String(), "Nothing on the target changed") {
		t.Errorf("the outcome must not imply the target was converged: %s", rr.Body.String())
	}
}

// Naming somebody the finding does not is a validation failure, not a refusal
// to act: it is a different decision with no rehearsal behind it.
func TestResolvingToAThirdPartyIsRefused(t *testing.T) {
	orig := dbResolveBindingConflict
	t.Cleanup(func() { dbResolveBindingConflict = orig })
	dbResolveBindingConflict = func(context.Context, string, string, string, string) (db.BindingConflict, error) {
		return db.BindingConflict{}, db.ErrInvalidTargetBinding
	}

	rr := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/api/v1/targets/truenas/binding-conflicts/c1/resolve",
		strings.NewReader(`{"owner":"somebody-else","note":"hunch","confirmed":true}`))
	r.SetPathValue("target", "truenas")
	r.SetPathValue("id", "c1")
	handleResolveBindingConflict(rr, r)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("want a validation failure, got %d: %s", rr.Code, rr.Body.String())
	}
}
