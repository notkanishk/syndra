package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"syndra/internal/addons"
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
	var converged []string
	orig, origTx, origResolve, origRecord :=
		dbResolveBindingConflict, svcInTxLockingAccess, svcResolveEntitlementsFor, dbRecordSystemConvergence
	t.Cleanup(func() {
		dbResolveBindingConflict, svcInTxLockingAccess, svcResolveEntitlementsFor, dbRecordSystemConvergence =
			orig, origTx, origResolve, origRecord
	})
	svcInTxLockingAccess = func(ctx context.Context, fn func(context.Context) error) error { return fn(ctx) }
	dbResolveBindingConflict = func(_ context.Context, id, owner, actor, note string) (db.BindingConflict, error) {
		got.id, got.owner, got.actor, got.note = id, owner, actor, note
		return db.BindingConflict{
			Target: "truenas", Username: "ada",
			ConvergedSubjectID: "subject-b", BoundSubjectID: "subject-a",
		}, nil
	}
	svcResolveEntitlementsFor = func(context.Context, string, string) (map[string]json.RawMessage, error) {
		return map[string]json.RawMessage{"enabled": json.RawMessage(`true`)}, nil
	}
	dbRecordSystemConvergence = func(_ context.Context, c db.SystemConvergence) (string, string, error) {
		converged = append(converged, c.SubjectID)
		return "plan_1", "outbox_1", nil
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
	// Both claimants are converged, in the same transaction as the rebind.
	//
	// Agreeing the records does not touch the target, and the target is wrong
	// both ways: the loser's groups were overwritten by the change that caused
	// the conflict, and the winner's account carries the OTHER person's
	// resolved set — access they were never granted, under a finding somebody
	// has just marked resolved.
	if len(converged) != 2 {
		t.Fatalf("both people need re-converging, got %v", converged)
	}
	// And the outcome says queued rather than done, naming what is still true
	// of the account until it drains.
	if !strings.Contains(rr.Body.String(), "convergence is queued") {
		t.Errorf("the outcome must say the work is queued: %s", rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "still holds whatever") {
		t.Errorf("and what the account holds until then: %s", rr.Body.String())
	}
}

// A convergence that cannot be queued fails the whole resolution.
//
// The rebind and the convergences are one decision: records agreeing while the
// target keeps one person's entitlements on another person's account is the
// state this endpoint exists to leave behind, not to create.
func TestAResolutionThatCannotQueueIsRolledBack(t *testing.T) {
	origTx, origResolve, origRecord, origConflict :=
		svcInTxLockingAccess, svcResolveEntitlementsFor, dbRecordSystemConvergence, dbResolveBindingConflict
	t.Cleanup(func() {
		svcInTxLockingAccess, svcResolveEntitlementsFor, dbRecordSystemConvergence, dbResolveBindingConflict =
			origTx, origResolve, origRecord, origConflict
	})
	var rolledBack bool
	svcInTxLockingAccess = func(ctx context.Context, fn func(context.Context) error) error {
		err := fn(ctx)
		rolledBack = err != nil
		return err
	}
	dbResolveBindingConflict = func(context.Context, string, string, string, string) (db.BindingConflict, error) {
		return db.BindingConflict{Target: "truenas", Username: "ada",
			ConvergedSubjectID: "subject-b", BoundSubjectID: "subject-a"}, nil
	}
	svcResolveEntitlementsFor = func(context.Context, string, string) (map[string]json.RawMessage, error) {
		return map[string]json.RawMessage{}, nil
	}
	dbRecordSystemConvergence = func(context.Context, db.SystemConvergence) (string, string, error) {
		return "", "", errors.New("the outbox is unavailable")
	}

	rr := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/api/v1/targets/truenas/binding-conflicts/c1/resolve",
		strings.NewReader(`{"owner":"subject-a","note":"checked with her","confirmed":true}`))
	r.SetPathValue("target", "truenas")
	r.SetPathValue("id", "c1")
	handleResolveBindingConflict(rr, r)

	if rr.Code == http.StatusOK {
		t.Fatalf("a resolution whose convergence could not be queued must not report success: %s", rr.Body.String())
	}
	if !rolledBack {
		t.Error("and the rebind must go back with it — records agreeing over a target nobody will fix is the state this prevents")
	}
}

// Naming somebody the finding does not is a validation failure, not a refusal
// to act: it is a different decision with no rehearsal behind it.
func TestResolvingToAThirdPartyIsRefused(t *testing.T) {
	orig, origTx := dbResolveBindingConflict, svcInTxLockingAccess
	t.Cleanup(func() { dbResolveBindingConflict, svcInTxLockingAccess = orig, origTx })
	svcInTxLockingAccess = func(ctx context.Context, fn func(context.Context) error) error { return fn(ctx) }
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

// A registered target whose transport secret stopped loading says so on the
// roster, before the next call fails at the handshake.
//
// The states are deliberately not merged. "Registered" is a deployment fact
// read once at start-up; this is a live read of the same material, and the gap
// between them is exactly the window an operator needs told about — a mount
// that was unmounted, a rotation that half-finished, a `_FILE` path that
// stopped resolving. The failure it precedes names three causes it cannot tell
// apart, so arriving first is the whole of this field's value.
func TestTheRosterReportsATransportSecretThatStoppedLoading(t *testing.T) {
	origReg, origTC := addonsRegistered, addonsTransportCredentials
	t.Cleanup(func() { addonsRegistered, addonsTransportCredentials = origReg, origTC })

	addonsRegistered = func() []addons.Registration {
		return []addons.Registration{
			{Target: "truenas", BaseURL: "https://truenas-addon:8443", Secret: []byte("configured")},
			{Target: "unifi", BaseURL: "https://unifi-addon:8443", Secret: []byte("configured")},
		}
	}
	addonsTransportCredentials = func() []addons.TransportCredential {
		return []addons.TransportCredential{
			{Target: "truenas", AuthMode: "derived", Status: "error",
				Error: "read ADDON_TRUENAS_SECRET_FILE (/run/secrets/addon/truenas.key): no such file or directory"},
			{Target: "unifi", AuthMode: "derived", Status: "ok"},
		}
	}

	rec := httptest.NewRecorder()
	handleListTargets(rec, httptest.NewRequest(http.MethodGet, "/api/v1/targets", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}

	var body struct {
		Targets []struct {
			Target          string `json:"target"`
			Registered      bool   `json:"registered"`
			TransportStatus string `json:"transport_status"`
			TransportError  string `json:"transport_error"`
		} `json:"targets"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	byTarget := map[string]struct {
		Target          string `json:"target"`
		Registered      bool   `json:"registered"`
		TransportStatus string `json:"transport_status"`
		TransportError  string `json:"transport_error"`
	}{}
	for _, row := range body.Targets {
		byTarget[row.Target] = row
	}

	broken, ok := byTarget["truenas"]
	if !ok {
		t.Fatal("the broken target vanished from the roster; it is still deployed")
	}
	// Still registered. Withdrawing the row would make a mount that fell off
	// look like a target that was never deployed, and those are different
	// sentences with different fixes.
	if !broken.Registered {
		t.Error("a target with an unloadable secret is still registered")
	}
	if broken.TransportStatus != "error" || broken.TransportError == "" {
		t.Errorf("the unloadable secret is not reported: %+v", broken)
	}
	// The path, because "no secret configured" and "the mount is missing" are
	// the same symptom and different fixes.
	if !strings.Contains(broken.TransportError, "/run/secrets/addon/truenas.key") {
		t.Errorf("the error must carry the path it was given, got %q", broken.TransportError)
	}
	// And a healthy target beside it is unaffected — the per-row read must not
	// smear one target's failure across the roster.
	if byTarget["unifi"].TransportStatus != "ok" {
		t.Errorf("the healthy target reports %q", byTarget["unifi"].TransportStatus)
	}
}
