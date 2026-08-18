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
	"syndra/internal/services/addonop"
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
	decided    []string
	forgotten  []string
	allowErr   error
	// The add-on's half of a release. Recorded so a test can assert that the
	// backend never forgets its own record without the add-on confirming it let
	// go — the split-store failure `account.release` exists to close.
	dispatched  []addonop.Request
	dispatch    addons.Outcome
	dispatchErr error
}

func stubFindings(t *testing.T, h *findingHarness) {
	t.Helper()
	get, resolve := dbGetStandingMergeFinding, dbResolveMergeFinding
	converge, allow := dbRecordSystemConvergence, dbCreateAllowance
	decide := dbRecordMergeDecision
	t.Cleanup(func() { dbRecordMergeDecision = decide })
	resolveSet, inTx := svcResolveEntitlementsFor, svcInTxLockingAccess
	forget, forgetBase := dbForgetTargetBinding, dbForgetMergeBase
	dispatch := svcDispatchOperation
	t.Cleanup(func() {
		dbGetStandingMergeFinding, dbResolveMergeFinding = get, resolve
		dbRecordSystemConvergence, dbCreateAllowance = converge, allow
		svcResolveEntitlementsFor, svcInTxLockingAccess = resolveSet, inTx
		dbForgetTargetBinding, dbForgetMergeBase = forget, forgetBase
		svcDispatchOperation = dispatch
	})

	// Runs the body inline. The transaction itself is the db package's problem;
	// what these tests are about is the ORDER of the writes inside it.
	svcInTxLockingAccess = func(ctx context.Context, fn func(context.Context) error) error { return fn(ctx) }
	svcDispatchOperation = func(_ context.Context, req addonop.Request) (addonop.Result, error) {
		h.dispatched = append(h.dispatched, req)
		if h.dispatchErr != nil {
			return addonop.Result{}, h.dispatchErr
		}
		outcome := h.dispatch
		if outcome == "" {
			outcome = addons.OutcomeSucceeded
		}
		return addonop.Result{OperationID: "op_1", Outcome: outcome}, nil
	}
	dbGetStandingMergeFinding = func(context.Context, string) (db.MergeFinding, error) {
		return h.finding, h.getErr
	}
	dbResolveMergeFinding = func(_ context.Context, id, actor, resolution string) (db.MergeFinding, error) {
		h.resolved = append(h.resolved, resolution)
		return h.finding, nil
	}
	dbRecordMergeDecision = func(_ context.Context, id, actor, decision string) (db.MergeFinding, error) {
		h.decided = append(h.decided, decision)
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
func TestKeepingOursQueuesAConvergenceAndRecordsTheDecision(t *testing.T) {
	h := &findingHarness{finding: theirsOnly("enabled", `false`)}
	stubFindings(t, h)

	rr := resolveFinding(t, `{"resolution":"keep_ours","reason":"the suspension was a mistake"}`)
	if rr.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", rr.Code, rr.Body.String())
	}
	if len(h.converged) != 1 || h.converged[0].SubjectID != "sub-1" {
		t.Fatalf("keeping ours must queue the convergence that applies it: %+v", h.converged)
	}
	// Decided, NOT closed. The convergence is queued and the difference is still
	// there; closing now would let the next sweep raise a second finding about
	// the same field, so one decision would refill the queue every six hours
	// until the drain caught up.
	if len(h.decided) != 1 || h.decided[0] != db.ResolutionKeepOurs {
		t.Fatalf("the decision must be recorded: %v", h.decided)
	}
	if len(h.resolved) != 0 {
		t.Fatalf("queued work is not a settled finding: %v", h.resolved)
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
	if len(h.decided) != 1 || len(h.resolved) != 0 {
		t.Fatalf("an adoption is decided and settles when a pass agrees: decided=%v resolved=%v", h.decided, h.resolved)
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
	if len(h.allowances) != 0 || len(h.resolved) != 0 || len(h.decided) != 0 {
		t.Fatal("a refused resolution must write nothing, decide nothing and close nothing")
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
	if len(h.resolved) != 0 || len(h.decided) != 0 {
		t.Fatal("the finding must stay standing and undecided when its resolution did not land")
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
	if len(h.dispatched) != 1 || h.dispatched[0].Operation != "account.release" {
		t.Fatalf("the ADD-ON must be told to let go first: %+v", h.dispatched)
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

// Both stores or neither.
//
// The add-on's binding is what an apply consults; the backend's row is what the
// inventory and the reconciliation read. Forgetting only this side leaves the
// account managed by half the system — still planned and applied by an add-on
// that binds it, while every surface here calls it unmanaged. `account.release`
// exists to close exactly that split, and resolving a finding was quietly
// recreating it.
func TestUnbindingIsRefusedWhenTheAddOnDoesNotConfirm(t *testing.T) {
	h := &findingHarness{
		finding: db.MergeFinding{
			ID: "f1", Target: "truenas", SubjectID: "sub-1", Outcome: "deleted_upstream",
		},
		dispatch: addons.OutcomeIndeterminate,
	}
	stubFindings(t, h)

	rr := resolveFinding(t, `{"resolution":"unbound","reason":"the account was removed on purpose"}`)
	if rr.Code != http.StatusBadGateway {
		t.Fatalf("want 502, got %d: %s", rr.Code, rr.Body.String())
	}
	if len(h.forgotten) != 0 {
		t.Fatalf("this side must not forget what the add-on still holds: %v", h.forgotten)
	}
	if len(h.resolved) != 0 {
		t.Fatal("a release nobody confirmed must leave the finding standing")
	}
}

// Which buttons a surface renders is not validation. `unbound` against a value
// disagreement would stop Syndra managing an account that is sitting right
// there, and the API accepted it from any caller.
func TestAResolutionThatDoesNotFitTheFindingIsRefused(t *testing.T) {
	h := &findingHarness{finding: theirsOnly("enabled", `false`)}
	stubFindings(t, h)

	rr := resolveFinding(t, `{"resolution":"unbound","reason":"tidying up"}`)
	if rr.Code != http.StatusUnprocessableEntity {
		t.Fatalf("want 422, got %d: %s", rr.Code, rr.Body.String())
	}
	if len(h.dispatched) != 0 || len(h.forgotten) != 0 || len(h.resolved) != 0 || len(h.decided) != 0 {
		t.Fatal("a refused resolution must do nothing at all")
	}
}

// And the mirror: applying a value to an account that is gone. The plan for one
// says create, which is the decision `reprovisioned` exists to make explicitly.
func TestKeepingOursIsRefusedForAnAccountThatIsGone(t *testing.T) {
	h := &findingHarness{finding: db.MergeFinding{
		ID: "f1", Target: "truenas", SubjectID: "sub-1", Outcome: "deleted_upstream",
	}}
	stubFindings(t, h)

	rr := resolveFinding(t, `{"resolution":"keep_ours","reason":"put it back"}`)
	if rr.Code != http.StatusUnprocessableEntity {
		t.Fatalf("want 422, got %d: %s", rr.Code, rr.Body.String())
	}
	if len(h.converged) != 0 {
		t.Fatal("nothing may be queued for a resolution that does not fit")
	}
}

// What the surface has to render, computed here.
//
// Which fields have a per-subject home is a fact about the entitlement model —
// mappings are per role, lifecycle fields are derived — and a copy of that rule
// in a component is a second definition that disagrees the first time the model
// grows a field. So the list says whether each value can be adopted, and when
// it cannot, which policy would have to change and how far that reaches.

func stubFindingsList(t *testing.T, findings []db.MergeFinding, mappings []db.RoleMapping, holders []string) {
	t.Helper()
	list, listMappings, count := dbStandingMergeFindings, dbListRoleMappings, dbMappingHolders
	t.Cleanup(func() {
		dbStandingMergeFindings, dbListRoleMappings, dbMappingHolders = list, listMappings, count
	})
	dbStandingMergeFindings = func(context.Context, string) ([]db.MergeFinding, error) { return findings, nil }
	dbListRoleMappings = func(context.Context, string) ([]db.RoleMapping, error) { return mappings, nil }
	dbMappingHolders = func(context.Context, string, string) ([]string, error) { return holders, nil }
}

func listFindings(t *testing.T) map[string]any {
	t.Helper()
	r := httptest.NewRequest(http.MethodGet, "/api/v1/targets/truenas/merge-findings", nil)
	r.SetPathValue("target", "truenas")
	rr := httptest.NewRecorder()
	handleMergeFindings(rr, r)
	if rr.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	return body
}

func firstFinding(t *testing.T, body map[string]any) map[string]any {
	t.Helper()
	rows, ok := body["findings"].([]any)
	if !ok || len(rows) == 0 {
		t.Fatalf("no findings in %v", body)
	}
	return rows[0].(map[string]any)
}

// A group value has no per-person home, so the surface must not offer to adopt
// it — and the sentence it shows instead has to name the mapping and how many
// people editing it would reach.
func TestAGroupFindingIsNotAdoptableAndNamesItsPolicy(t *testing.T) {
	stubFindingsList(t,
		[]db.MergeFinding{theirsOnly("group", `["electronics"]`)},
		[]db.RoleMapping{{ID: "m1", Target: "truenas", ProjectID: "p1", RoleKey: "lab_tech", Field: "group", Value: "lab_makers"}},
		[]string{"u1", "u2", "u3"})

	row := firstFinding(t, listFindings(t))
	if row["adoptable"] == true {
		t.Fatal("a group value must not be offered for per-person adoption")
	}
	why, _ := row["why_not"].(string)
	if !strings.Contains(why, "every holder of that role") {
		t.Fatalf("the sentence must name the blast radius: %q", why)
	}
	policy, ok := row["policy"].([]any)
	if !ok || len(policy) != 1 {
		t.Fatalf("the governing mapping must be named: %v", row["policy"])
	}
	p := policy[0].(map[string]any)
	if p["role_key"] != "lab_tech" || p["holders"].(float64) != 3 {
		t.Fatalf("want the mapping and its holder count: %v", p)
	}
}

// A lifecycle field the target turned OFF is adoptable: it becomes a deny
// allowance, which is the mechanism that layer was built for.
func TestASuspensionIsAdoptable(t *testing.T) {
	stubFindingsList(t, []db.MergeFinding{theirsOnly("enabled", `false`)}, nil, nil)

	row := firstFinding(t, listFindings(t))
	if row["adoptable"] != true {
		t.Fatalf("a suspension adopted as a deny allowance is expressible: %v", row)
	}
	if row["why_not"] != nil {
		t.Fatalf("an adoptable value needs no explanation: %v", row["why_not"])
	}
}

// And the same field turned ON is not: it is produced by holding a mapped role,
// so there is nothing per-person to write.
func TestAPermissiveLifecycleValueIsNotAdoptable(t *testing.T) {
	stubFindingsList(t, []db.MergeFinding{theirsOnly("smb_enabled", `true`)}, nil, nil)

	row := firstFinding(t, listFindings(t))
	if row["adoptable"] == true {
		t.Fatal("there is no per-person way to switch a lifecycle field on")
	}
	if why, _ := row["why_not"].(string); !strings.Contains(why, "grant them a role") {
		t.Fatalf("the sentence must name the action that works: %q", why)
	}
}

// An account that is gone is neither adoptable nor a policy question. Its two
// answers are re-provision or stop managing it.
func TestADeletedAccountIsNeitherAdoptableNorAPolicyQuestion(t *testing.T) {
	stubFindingsList(t, []db.MergeFinding{{
		ID: "f1", Target: "truenas", SubjectID: "sub-1", Outcome: "deleted_upstream",
	}}, nil, nil)

	row := firstFinding(t, listFindings(t))
	if row["adoptable"] == true || row["why_not"] != nil {
		t.Fatalf("a missing account is a different question: %v", row)
	}
}

// Unbinding is the one decision that settles immediately, because it leaves
// nothing to observe: the binding is gone, so no pass will classify that
// subject again and a row waiting for agreement would stand forever.
func TestUnbindingSettlesImmediately(t *testing.T) {
	h := &findingHarness{finding: db.MergeFinding{
		ID: "f1", Target: "truenas", SubjectID: "sub-1", Outcome: "deleted_upstream",
	}}
	stubFindings(t, h)

	rr := resolveFinding(t, `{"resolution":"unbound","reason":"removed on purpose"}`)
	if rr.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", rr.Code, rr.Body.String())
	}
	if len(h.resolved) != 1 || len(h.decided) != 0 {
		t.Fatalf("unbinding settles rather than waits: resolved=%v decided=%v", h.resolved, h.decided)
	}
	if !strings.Contains(rr.Body.String(), `"resolved":true`) {
		t.Fatalf("the answer must say it is settled: %s", rr.Body.String())
	}
}

// And every other decision says plainly that it is waiting. Reporting a queued
// convergence as resolved is what made the queue refill itself, and to an
// operator it reads as the surface being broken.
func TestADecisionSaysItIsWaitingRatherThanDone(t *testing.T) {
	h := &findingHarness{finding: theirsOnly("enabled", `false`)}
	stubFindings(t, h)

	rr := resolveFinding(t, `{"resolution":"keep_ours","reason":"the suspension was a mistake"}`)
	body := rr.Body.String()
	if !strings.Contains(body, `"resolved":false`) || !strings.Contains(body, `"decided":true`) {
		t.Fatalf("a queued decision is decided, not resolved: %s", body)
	}
	if !strings.Contains(body, "until a reconciliation sees the target agree") {
		t.Fatalf("the answer must say what it is waiting for: %s", body)
	}
}
