package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"syndra/internal/db"
)

// Resolving a merge finding, and the two refusals that make the surface honest.
//
// "Take theirs" and "edit" were written into the design as though the desired
// state for one subject were a thing one could set. It is not: `group` comes
// from role mappings that have no subject column, and the lifecycle fields are
// derived and refused as mapping targets at three layers. A resolution the model
// cannot express is worse than one it does not offer — it fails after the
// operator believes they have decided.

type findingHarness struct {
	finding    db.MergeFinding
	getErr     error
	converged  []db.SystemConvergence
	allowances []db.Allowance
	resolved   []string
	forgotten  []string
	allowErr   error
}

func stubFindings(t *testing.T, h *findingHarness) {
	t.Helper()
	get, resolve := dbGetStandingMergeFinding, dbResolveMergeFinding
	converge, allow := dbRecordSystemConvergence, dbCreateAllowance
	resolveSet, inTx := svcResolveEntitlementsFor, svcInTxLockingAccess
	forget, forgetBase := dbForgetTargetBinding, dbForgetMergeBase
	t.Cleanup(func() {
		dbGetStandingMergeFinding, dbResolveMergeFinding = get, resolve
		dbRecordSystemConvergence, dbCreateAllowance = converge, allow
		svcResolveEntitlementsFor, svcInTxLockingAccess = resolveSet, inTx
		dbForgetTargetBinding, dbForgetMergeBase = forget, forgetBase
	})

	// Runs the body inline. The transaction itself is the db package's problem;
	// what these tests are about is the ORDER of the writes inside it.
	svcInTxLockingAccess = func(ctx context.Context, fn func(context.Context) error) error { return fn(ctx) }
	dbGetStandingMergeFinding = func(context.Context, string) (db.MergeFinding, error) {
		return h.finding, h.getErr
	}
	dbResolveMergeFinding = func(_ context.Context, id, actor, resolution string) (db.MergeFinding, error) {
		h.resolved = append(h.resolved, resolution)
		return h.finding, nil
	}
	dbRecordSystemConvergence = func(_ context.Context, c db.SystemConvergence) (string, string, error) {
		h.converged = append(h.converged, c)
		return "plan_1", "outbox_1", nil
	}
	dbCreateAllowance = func(_ context.Context, a db.Allowance) (db.Allowance, error) {
		if h.allowErr != nil {
			return db.Allowance{}, h.allowErr
		}
		h.allowances = append(h.allowances, a)
		return a, nil
	}
	svcResolveEntitlementsFor = func(context.Context, string, string) (map[string]json.RawMessage, error) {
		return map[string]json.RawMessage{"enabled": json.RawMessage(`true`)}, nil
	}
	dbForgetTargetBinding = func(_ context.Context, _, subject string) error {
		h.forgotten = append(h.forgotten, subject)
		return nil
	}
	dbForgetMergeBase = func(context.Context, string, string) error { return nil }
}

func resolveFinding(t *testing.T, body string) *httptest.ResponseRecorder {
	t.Helper()
	r := httptest.NewRequest(http.MethodPost,
		"/api/v1/targets/truenas/merge-findings/f1/resolve", strings.NewReader(body))
	r.SetPathValue("target", "truenas")
	r.SetPathValue("id", "f1")
	rr := httptest.NewRecorder()
	handleResolveMergeFinding(rr, r)
	return rr
}

func theirsOnly(field, theirs string) db.MergeFinding {
	return db.MergeFinding{
		ID: "f1", Target: "truenas", SubjectID: "sub-1", Field: field,
		Outcome: "theirs_only",
		Base:    json.RawMessage(`true`), Ours: json.RawMessage(`true`),
		Theirs: json.RawMessage(theirs),
	}
}

// Keeping Syndra's state queues a convergence rather than writing to the target
// here. The apply path is the only thing that talks to a target, and its
// read-back is what records the new base — which is what stops the finding
// returning on the next pass.
func TestKeepingOursQueuesAConvergenceAndThenCloses(t *testing.T) {
	h := &findingHarness{finding: theirsOnly("enabled", `false`)}
	stubFindings(t, h)

	rr := resolveFinding(t, `{"resolution":"keep_ours","reason":"the suspension was a mistake"}`)
	if rr.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", rr.Code, rr.Body.String())
	}
	if len(h.converged) != 1 || h.converged[0].SubjectID != "sub-1" {
		t.Fatalf("keeping ours must queue the convergence that applies it: %+v", h.converged)
	}
	if len(h.resolved) != 1 || h.resolved[0] != db.ResolutionKeepOurs {
		t.Fatalf("the finding must be closed with what was chosen: %v", h.resolved)
	}
}

// Adopting a restrictive lifecycle value writes the decision where one can
// live: a deny allowance, with an author, a reason and a bound in time. The
// hand edit it came from had none of the three.
func TestAdoptingASuspensionWritesItAsADenyAllowance(t *testing.T) {
	h := &findingHarness{finding: theirsOnly("enabled", `false`)}
	stubFindings(t, h)

	rr := resolveFinding(t, `{"resolution":"take_theirs","reason":"disabled pending an incident review"}`)
	if rr.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", rr.Code, rr.Body.String())
	}
	if len(h.allowances) != 1 {
		t.Fatalf("want one allowance: %+v", h.allowances)
	}
	a := h.allowances[0]
	if a.Direction != db.AllowanceDeny || a.Field != "enabled" || a.Value != "true" {
		t.Fatalf("the denial must remove the value the resolver would compute: %+v", a)
	}
	if a.Reason == "" || a.ActorID == "" {
		t.Fatalf("an adopted value is policy, and policy carries who and why: %+v", a)
	}
	if len(h.converged) != 0 {
		t.Fatal("adopting the target's value must not write to the target")
	}
}

// The refusal that keeps the surface honest. `group` belongs to a role mapping
// that reaches every holder of that role — there is nothing that can hold one
// person's group, and inventing a per-subject grant so this dialog has a button
// would put an entitlement where no access review looks.
func TestAdoptingAGroupValueIsRefusedAndNamesThePolicy(t *testing.T) {
	h := &findingHarness{finding: theirsOnly("group", `["electronics"]`)}
	stubFindings(t, h)

	rr := resolveFinding(t, `{"resolution":"take_theirs","reason":"they asked for it"}`)
	if rr.Code != http.StatusUnprocessableEntity {
		t.Fatalf("want 422, got %d: %s", rr.Code, rr.Body.String())
	}
	body := rr.Body.String()
	if !strings.Contains(body, "role mappings") || !strings.Contains(body, "every holder") {
		t.Fatalf("the refusal must name the policy and its blast radius: %s", body)
	}
	if len(h.allowances) != 0 || len(h.resolved) != 0 {
		t.Fatal("a refused resolution must write nothing and close nothing")
	}
}

// And the other half of the same refusal: a lifecycle field the target has
// turned ON is produced by holding a mapped role. There is no per-person switch
// for it either, and saying so is more useful than a button that fails.
func TestAdoptingAPermissiveLifecycleValueIsRefused(t *testing.T) {
	h := &findingHarness{finding: theirsOnly("smb_enabled", `true`)}
	stubFindings(t, h)

	rr := resolveFinding(t, `{"resolution":"take_theirs","reason":"they need shares"}`)
	if rr.Code != http.StatusUnprocessableEntity {
		t.Fatalf("want 422, got %d: %s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "grant them a role") {
		t.Fatalf("the refusal must name the action that works: %s", rr.Body.String())
	}
	if len(h.resolved) != 0 {
		t.Fatal("nothing was decided, so nothing may be closed")
	}
}

// An adopted value is policy for that person. Policy with no stated reason is
// what the allowance layer exists to replace, so the reason is required here
// and not only by the schema underneath.
func TestAdoptingWithoutAReasonIsRefused(t *testing.T) {
	h := &findingHarness{finding: theirsOnly("enabled", `false`)}
	stubFindings(t, h)

	rr := resolveFinding(t, `{"resolution":"take_theirs","reason":"  "}`)
	if rr.Code != http.StatusUnprocessableEntity && rr.Code != http.StatusBadRequest {
		t.Fatalf("want a refusal, got %d: %s", rr.Code, rr.Body.String())
	}
	if len(h.allowances) != 0 || len(h.resolved) != 0 {
		t.Fatal("nothing may be written without a reason")
	}
}

// The order that makes the table worth having. A finding closed by an action
// that then failed is a difference nothing will raise again until it changes a
// second time.
func TestAFailedResolutionLeavesTheFindingStanding(t *testing.T) {
	h := &findingHarness{
		finding:  theirsOnly("enabled", `false`),
		allowErr: errors.New("the database went away"),
	}
	stubFindings(t, h)

	rr := resolveFinding(t, `{"resolution":"take_theirs","reason":"pending review"}`)
	if rr.Code == http.StatusOK {
		t.Fatalf("a failed resolution must not answer 200: %s", rr.Body.String())
	}
	if len(h.resolved) != 0 {
		t.Fatal("the finding must stay standing when its resolution did not land")
	}
}

// Unbinding is the deleted-upstream answer that does not recreate anything, and
// it takes the observation with it: a base outliving its binding is compared
// against whatever that subject is bound to next.
func TestUnbindingForgetsTheBindingAndTheObservation(t *testing.T) {
	h := &findingHarness{finding: db.MergeFinding{
		ID: "f1", Target: "truenas", SubjectID: "sub-1", Outcome: "deleted_upstream",
	}}
	stubFindings(t, h)

	rr := resolveFinding(t, `{"resolution":"unbound","reason":"the account was removed on purpose"}`)
	if rr.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", rr.Code, rr.Body.String())
	}
	if len(h.forgotten) != 1 {
		t.Fatalf("the binding must go: %v", h.forgotten)
	}
	if len(h.converged) != 0 {
		t.Fatal("unbinding must not queue a convergence — that would recreate the account")
	}
}

// A finding somebody else already resolved is refused rather than resolved
// twice: the second operator would believe they made a decision that was
// already made differently.
func TestResolvingAFindingThatIsNotStandingIsRefused(t *testing.T) {
	h := &findingHarness{getErr: db.ErrNoSuchMergeFinding}
	stubFindings(t, h)

	rr := resolveFinding(t, `{"resolution":"keep_ours","reason":"x"}`)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("want 404, got %d: %s", rr.Code, rr.Body.String())
	}
}
