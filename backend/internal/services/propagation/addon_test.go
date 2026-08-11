package propagation

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"
	"testing"

	"syndra/internal/addons"
	"syndra/internal/db"
	"syndra/internal/models"
)

// 1.10's deferred add-on dispatcher, and 1.22's first read path.
//
// The property under everything here: an operator-initiated change is an
// approval, and the diff that was approved is the diff that lands.

func addonRow(id string) models.PendingPropagation {
	return models.PendingPropagation{ID: id, Target: "truenas", OpType: "apply", UserID: "sub-1"}
}

type addonHarness struct {
	reachable  bool
	dispatched []addons.ApplyRequest
	resp       addons.ApplyResponse
	intent     db.EntitlementIntent
	intentErr  error
	applied    []string
	failed     map[string]string
	requeued   []string
	released   []string
	attempts   int
}

func stubAddonDrain(t *testing.T, rows ...models.PendingPropagation) *addonHarness {
	t.Helper()
	stubDrainDeps(t)
	h := &addonHarness{
		reachable: true,
		failed:    map[string]string{},
		resp:      addons.ApplyResponse{Outcome: addons.OutcomeSucceeded},
		intent: db.EntitlementIntent{
			OutboxID: "o1", Target: "truenas", SubjectID: "sub-1",
			Fingerprint: "fp-1", Version: 3,
			DesiredJSON: []byte(`{"group":["lab_makers"],"enabled":true}`),
		},
	}
	t.Cleanup(swap(&claimPending, func(_ context.Context, target string, _ int) ([]models.PendingPropagation, error) {
		if target != "truenas" {
			t.Fatalf("the claim must be scoped to the dispatched target, got %q", target)
		}
		return rows, nil
	}))
	t.Cleanup(swap(&addonReachable, func(context.Context, string) bool { return h.reachable }))
	// The conflict path's reads, defaulted so a test about anything else does
	// not reach the real database. "Nobody else holds it" is the ordinary case
	// and makes the finding unrecordable, which is the right default: a test
	// that wants a conflict recorded says so.
	t.Cleanup(swap(&bindingHolder, func(context.Context, string, string, int64) (string, bool, error) {
		return "", false, nil
	}))
	t.Cleanup(swap(&saveConflict, func(context.Context, db.BindingConflict) error { return nil }))
	t.Cleanup(swap(&readIntent, func(context.Context, string) (db.EntitlementIntent, error) {
		return h.intent, h.intentErr
	}))
	t.Cleanup(swap(&applyEntitlement, func(_ context.Context, req addons.ApplyRequest) addons.ApplyResponse {
		h.dispatched = append(h.dispatched, req)
		return h.resp
	}))
	markApplied = func(_ context.Context, id string) error { h.applied = append(h.applied, id); return nil }
	markFailed = func(_ context.Context, id, msg string) error { h.failed[id] = msg; return nil }
	requeue = func(_ context.Context, id, _ string) (int, error) {
		h.requeued = append(h.requeued, id)
		h.attempts++
		return h.attempts, nil
	}
	t.Cleanup(swap(&release, func(_ context.Context, id, _ string) error {
		h.released = append(h.released, id)
		return nil
	}))
	return h
}

// The approved snapshot is what is dispatched. Re-resolving at drain time would
// send whatever policy says now, which is a different decision wearing the
// approval's plan id.
func TestTheDispatcherSendsTheApprovedSnapshot(t *testing.T) {
	h := stubAddonDrain(t, addonRow("o1"))

	res, err := DrainAddon(context.Background(), "truenas")
	if err != nil {
		t.Fatal(err)
	}
	if res.Applied != 1 || len(h.applied) != 1 {
		t.Fatalf("want one applied row: %+v", res)
	}
	if len(h.dispatched) != 1 {
		t.Fatalf("want one dispatch, got %d", len(h.dispatched))
	}
	sent := h.dispatched[0]
	if sent.Subject != "sub-1" || sent.Fingerprint != "fp-1" {
		t.Errorf("the subject and the reviewed fingerprint must travel: %+v", sent)
	}
	// The outbox row's id is the dedup token, so a re-drive returns the
	// original outcome rather than converging twice.
	if sent.CallID != "o1" {
		t.Errorf("the call id must be the outbox row: %q", sent.CallID)
	}
	var desired map[string]json.RawMessage
	if err := json.Unmarshal([]byte(`{"group":["lab_makers"],"enabled":true}`), &desired); err != nil {
		t.Fatal(err)
	}
	if len(sent.Desired) != len(desired) || string(sent.Desired["group"]) != string(desired["group"]) {
		t.Errorf("the approved set must be sent verbatim: %v", sent.Desired)
	}
}

// A row citing no approved snapshot must not be dispatched. Sending an empty
// desired state would converge the subject to nothing, which is the most
// destructive possible reading of a missing citation.
func TestARowWithNoApprovedIntentIsFailedRatherThanDispatched(t *testing.T) {
	h := stubAddonDrain(t, addonRow("o1"))
	h.intentErr = db.ErrNoApprovedIntent

	res, err := DrainAddon(context.Background(), "truenas")
	if err != nil {
		t.Fatal(err)
	}
	if len(h.dispatched) != 0 {
		t.Fatal("nothing may be sent without an approved desired state")
	}
	if res.Failed != 1 || !strings.Contains(h.failed["o1"], "no approved desired state") {
		t.Fatalf("want a named failure: %+v %v", res, h.failed)
	}
}

// A Zitadel-shaped row on an add-on target names a project and roles this
// dispatcher cannot send. Terminal, because no number of retries changes that.
func TestAZitadelShapedRowIsFailedRatherThanRetriedForever(t *testing.T) {
	row := addonRow("o1")
	row.OpType = "add"
	h := stubAddonDrain(t, row)

	res, _ := DrainAddon(context.Background(), "truenas")
	if len(h.dispatched) != 0 || len(h.requeued) != 0 {
		t.Fatal("it must not be dispatched or requeued")
	}
	if res.Failed != 1 || !strings.Contains(h.failed["o1"], `"add"`) {
		t.Fatalf("the failure must name the shape it could not send: %v", h.failed)
	}
}

// A lifecycle refusal accounts as queued, never failed, and does not spend the
// retry budget: the budget exists for real failures, and a maintenance window
// converting every pending change into a failed row is the false finality the
// queued accounting exists to prevent.
func TestALifecycleRefusalRequeuesWithoutFailing(t *testing.T) {
	h := stubAddonDrain(t, addonRow("o1"))
	h.resp = addons.ApplyResponse{Outcome: addons.OutcomeUnreached, LifecycleRefusal: true}

	// The budget is already at its limit, so anything that SPENT one would halt.
	h.attempts = maxRetries

	res, _ := DrainAddon(context.Background(), "truenas")
	if res.Failed != 0 {
		t.Fatalf("a maintenance window is not a failure: %+v", res)
	}
	if res.Requeued != 1 || len(h.released) != 1 {
		t.Fatalf("it must be released back to pending: %+v released=%v", res, h.released)
	}
	if len(h.requeued) != 0 {
		t.Fatal("a maintenance window must not spend a retry: nothing was attempted")
	}
	if res.Halted {
		t.Error("and so it must not halt the pass — a long window would otherwise exhaust every queued row's budget")
	}
}

// The four outcomes are four outcomes. Rejected is terminal, unreached is
// retryable, and indeterminate is neither — retrying it duplicates a mutation
// the target may already hold, and counting it either way asserts what the
// backend does not know.
func TestEachDispatchOutcomeIsRecordedAsItself(t *testing.T) {
	for _, tc := range []struct {
		name           string
		resp           addons.ApplyResponse
		applied, faild int
		requeued       int
		errored        int
	}{
		{"succeeded", addons.ApplyResponse{Outcome: addons.OutcomeSucceeded}, 1, 0, 0, 0},
		{"rejected", addons.ApplyResponse{Outcome: addons.OutcomeRejected, Status: 400}, 0, 1, 0, 0},
		{"unreached", addons.ApplyResponse{Outcome: addons.OutcomeUnreached}, 0, 0, 1, 0},
		{"indeterminate", addons.ApplyResponse{Outcome: addons.OutcomeIndeterminate}, 0, 0, 0, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := stubAddonDrain(t, addonRow("o1"))
			h.resp = tc.resp

			res, _ := DrainAddon(context.Background(), "truenas")
			if res.Applied != tc.applied || res.Failed != tc.faild ||
				res.Requeued != tc.requeued || res.Errored != tc.errored {
				t.Fatalf("%s recorded as %+v", tc.name, res)
			}
		})
	}
}

// The add-on's own detail may reach `last_error`; its response body may not.
func TestARefusalRecordsTheAddonsAnswerAndNotItsBody(t *testing.T) {
	h := stubAddonDrain(t, addonRow("o1"))
	h.resp = addons.ApplyResponse{Outcome: addons.OutcomeRejected, Status: 422, Detail: "lab_makers does not exist"}

	DrainAddon(context.Background(), "truenas")
	if !strings.Contains(h.failed["o1"], "lab_makers does not exist") {
		t.Fatalf("the operator needs the reason: %q", h.failed["o1"])
	}
}

// A spent row goes terminal and the pass continues, as it does on the Zitadel
// side. A row left non-terminal at the head of the queue is one every later
// pass re-claims and halts on, which stops everything behind it in silence.
func TestTheAddonDrainTerminatesASpentRowAndKeepsGoing(t *testing.T) {
	spent := addonRow("o1")
	spent.Attempts = maxRetries
	h := stubAddonDrain(t, spent, addonRow("o2"))
	h.resp = addons.ApplyResponse{Outcome: addons.OutcomeUnreached}

	res, _ := DrainAddon(context.Background(), "truenas")
	if res.Halted {
		t.Fatalf("one spent row must not stop the queue: %+v", res)
	}
	if res.Failed != 1 || res.Exhausted != 1 {
		t.Fatalf("the spent row must be terminal and counted as such: %+v", res)
	}
	if !strings.Contains(h.failed["o1"], "out of retries") {
		t.Errorf("the reason must say nobody will try again: %q", h.failed["o1"])
	}
	if len(h.requeued) != 1 || h.requeued[0] != "o2" {
		t.Fatalf("the pass must reach the rows behind it: %v", h.requeued)
	}
}

// A drain holds one dispatcher. Reaching this one with the built-in target is a
// wrong model of which pass does what, and it is refused rather than absorbed.
func TestTheAddonDispatcherRefusesTheBuiltInTarget(t *testing.T) {
	stubAddonDrain(t)
	if _, err := DrainAddon(context.Background(), db.TargetZitadel); err == nil {
		t.Fatal("the built-in target has its own dispatcher")
	}
}

// It shares the one drain lock, so it cannot run beside the Zitadel pass or the
// revocation runner.
func TestTheAddonDrainYieldsToAConcurrentDrain(t *testing.T) {
	h := stubAddonDrain(t, addonRow("o1"))
	acquireDrainLock = func(context.Context) (func(), bool, error) { return nil, false, nil }

	res, err := DrainAddon(context.Background(), "truenas")
	if err != nil {
		t.Fatal(err)
	}
	if !res.Halted || res.Reason != "drain_in_progress" {
		t.Fatalf("want drain_in_progress, got %+v", res)
	}
	if len(h.dispatched) != 0 {
		t.Fatal("nothing may be dispatched without the lock")
	}
}

// An unreadable snapshot yields nothing to send rather than an empty set. An
// empty desired state removes every entitlement the subject has, and a JSON
// column that would not parse is not a reason to do that.
func TestAnUnreadableSnapshotSendsNothingRatherThanAnEmptySet(t *testing.T) {
	intent := db.EntitlementIntent{DesiredJSON: []byte(`{not json`)}
	if got := intent.Desired(); got != nil {
		t.Fatalf("want nil, got %v", got)
	}
	empty := db.EntitlementIntent{}
	if got := empty.Desired(); got != nil {
		t.Fatalf("an absent snapshot must not decode to an empty set, got %v", got)
	}
}

// The decode returning nil is only half the guard. Nothing read it: the
// dispatcher sent `intent.Desired()` straight out, and nil desired state is
// zero managed fields — which the add-on answers `no_change` and this drain
// records as applied. An approval marked converged having done nothing.
func TestAnUnreadableSnapshotIsRefusedBeforeItIsDispatched(t *testing.T) {
	h := stubAddonDrain(t, addonRow("o1"))
	h.intent.DesiredJSON = []byte(`{not json`)

	res, err := DrainAddon(context.Background(), "truenas")
	if err != nil {
		t.Fatal(err)
	}
	if len(h.dispatched) != 0 {
		t.Fatalf("nothing may be dispatched for a snapshot nobody can read: %+v", h.dispatched)
	}
	if res.Failed != 1 || h.failed["o1"] == "" {
		t.Fatalf("the row must be failed with a reason: %+v %v", res, h.failed)
	}
}

// A plan subject with no fingerprint verifies vacuously at the add-on. The row
// is refused here rather than spending a dispatch and a retry to learn it.
func TestARowWithNoFingerprintIsNotDispatched(t *testing.T) {
	h := stubAddonDrain(t, addonRow("o1"))
	h.intent.Fingerprint = ""

	res, err := DrainAddon(context.Background(), "truenas")
	if err != nil {
		t.Fatal(err)
	}
	if len(h.dispatched) != 0 {
		t.Fatal("an apply that could not verify anything must not be sent")
	}
	if res.Failed != 1 {
		t.Fatalf("want the row failed: %+v", res)
	}
}

// A read that failed is not a citation that is missing. `failed` has no way
// back, so a connection blip must not permanently fail an approved change.
func TestATransientIntentReadFailureDoesNotFailTheRowTerminally(t *testing.T) {
	h := stubAddonDrain(t, addonRow("o1"))
	h.intentErr = errors.New("connection reset by peer")

	res, err := DrainAddon(context.Background(), "truenas")
	if err != nil {
		t.Fatal(err)
	}
	if len(h.failed) != 0 {
		t.Fatalf("a transient read must not be terminal: %v", h.failed)
	}
	if res.Failed != 0 || res.Errored != 1 {
		t.Fatalf("it must be recorded as an error on this pass: %+v", res)
	}
}

// The add-on's mutation log promises who did what to whom, and the add-on knows
// only the whom.
func TestTheDispatchCarriesWhoDecidedIt(t *testing.T) {
	row := addonRow("o1")
	row.InitiatedBy = "op_7"
	h := stubAddonDrain(t, row)

	if _, err := DrainAddon(context.Background(), "truenas"); err != nil {
		t.Fatal(err)
	}
	if len(h.dispatched) != 1 || h.dispatched[0].Actor != "op_7" {
		t.Fatalf("the actor must travel with the apply: %+v", h.dispatched)
	}
}

// One probe before any row is claimed, like the Zitadel passes. Without it an
// outage spends one retry per row to learn what a single call establishes — and
// a spent budget is terminal, so the target being switched off would FAIL work
// an operator approved instead of leaving it queued for the target coming back.
func TestTheAddonDrainProbesBeforeItSpendsARetry(t *testing.T) {
	h := stubAddonDrain(t, addonRow("o1"), addonRow("o2"))
	h.reachable = false

	res, err := DrainAddon(context.Background(), "truenas")
	if err != nil {
		t.Fatal(err)
	}
	if !res.Halted || res.Reason != "target_unreachable" {
		t.Fatalf("want a clean halt, got %+v", res)
	}
	if len(h.dispatched) != 0 || len(h.requeued) != 0 || len(h.failed) != 0 {
		t.Fatalf("nothing may be claimed or spent: %+v %v %v", h.dispatched, h.requeued, h.failed)
	}
}

// 2.26/2.27 — a plan whose target moved underneath it is withheld, and the
// refusal says which of the two things happened.
//
// This is the resolution path for a provisional plan and for an ordinary one
// alike, and deliberately the same path: the add-on re-verifies the recorded
// fingerprint against live state immediately before writing, so "the target
// came back and nothing had changed" and "the target came back changed" are
// answered by the write itself rather than by a second protocol.
//
// Terminal, because a stale approval must not be retried into a world it does
// not describe. What makes it actionable rather than a phantom failure is that
// the reason names the re-plan — every other refusal means "fix it and retry".
func TestAStalePlanIsWithheldAndSaysSo(t *testing.T) {
	h := stubAddonDrain(t, addonRow("o1"))
	h.resp = addons.ApplyResponse{
		Outcome: addons.OutcomeRejected, Status: 409,
		Code:   addons.CodePlanStale,
		Detail: "ada moved on the target since the plan was approved",
	}

	res, err := DrainAddon(context.Background(), "truenas")
	if err != nil {
		t.Fatal(err)
	}
	if res.Failed != 1 {
		t.Fatalf("a stale plan must settle rather than sit in the queue: %+v", res)
	}
	if len(h.applied) != 0 {
		t.Fatal("nothing may be recorded as applied for a call the target refused")
	}
	if len(h.requeued) != 0 {
		t.Fatal("a stale approval must not be retried into a world it does not describe")
	}
	reason := h.failed["o1"]
	if !strings.Contains(reason, addons.CodePlanStale) {
		t.Errorf("the refusal must be distinguishable from an ordinary one: %q", reason)
	}
	if !strings.Contains(reason, "re-plan") {
		// Every other refusal means "fix it and retry". This one means "look at
		// what moved, then approve it again", and an operator reading the queue
		// has only this sentence to tell them apart.
		t.Errorf("the refusal must name the operator's next action: %q", reason)
	}
}

// The dispatched fingerprint is the one the approval recorded — not one read at
// dispatch time, which would verify a world against itself and pass always.
func TestTheDispatchCarriesTheApprovedFingerprintAndNotAFreshOne(t *testing.T) {
	h := stubAddonDrain(t, addonRow("o1"))
	h.intent.Fingerprint = "fp-approved"

	if _, err := DrainAddon(context.Background(), "truenas"); err != nil {
		t.Fatal(err)
	}
	if len(h.dispatched) != 1 {
		t.Fatalf("want one dispatch, got %d", len(h.dispatched))
	}
	if h.dispatched[0].Fingerprint != "fp-approved" {
		t.Errorf("fingerprint = %q, want the one the approval recorded", h.dispatched[0].Fingerprint)
	}
}

// The defect this pass was written for and did not have: nothing called it.
//
// `DrainAddon` was complete and tested, and no scheduler or route reached it —
// so an approved entitlement change queued an outbox row that no code path
// would ever dispatch, with no error anywhere to say so. Found by deploying and
// pressing the operator's own Resume button, which drained Zitadel and reported
// success while the NAS row sat pending.
//
// The property, stated so it stays: the operator's drain dispatches EVERY
// registered target, not only the built-in one.
func TestTheOperatorDrainDispatchesAddOnTargetsToo(t *testing.T) {
	h := stubAddonDrain(t, addonRow("o1"))
	claimed := map[string]bool{}
	t.Cleanup(swap(&claimPending, func(_ context.Context, target string, _ int) ([]models.PendingPropagation, error) {
		claimed[target] = true
		if target == "truenas" {
			return []models.PendingPropagation{addonRow("o1")}, nil
		}
		return nil, nil
	}))
	t.Cleanup(swap(&registeredAddons, func() []addons.Registration {
		return []addons.Registration{{Target: "truenas"}}
	}))

	res, err := Drain(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !claimed[db.TargetZitadel] || !claimed["truenas"] {
		t.Fatalf("both targets must have a pass, claimed=%v", claimed)
	}
	if len(h.dispatched) != 1 {
		t.Fatalf("the add-on row was never dispatched: %+v", res)
	}
	if res.Applied != 1 {
		t.Errorf("the combined summary must count the add-on row: %+v", res)
	}
	if len(res.Passes) != 2 {
		t.Errorf("each target's pass must be reported separately: %+v", res.Passes)
	}
}

// And a Zitadel outage does not hold a reachable NAS's approved work. They are
// separate deployments with separate outages; coupling them is the thing the
// target column was introduced to remove.
func TestAZitadelOutageDoesNotHoldAnAddOnTarget(t *testing.T) {
	h := stubAddonDrain(t, addonRow("o1"))
	t.Cleanup(swap(&zitadelReachable, func(context.Context) bool { return false }))
	t.Cleanup(swap(&claimPending, func(_ context.Context, target string, _ int) ([]models.PendingPropagation, error) {
		if target == db.TargetZitadel {
			t.Fatal("no row may be claimed for an unreachable Zitadel")
		}
		return []models.PendingPropagation{addonRow("o1")}, nil
	}))
	t.Cleanup(swap(&registeredAddons, func() []addons.Registration {
		return []addons.Registration{{Target: "truenas"}}
	}))

	res, err := Drain(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(h.dispatched) != 1 || res.Applied != 1 {
		t.Fatalf("the add-on pass must run regardless of Zitadel: %+v", res)
	}
	// And the operator is still told which half stopped, or "halted" is a
	// sentence about nothing.
	if !res.Halted || res.Reason != "zitadel_offline" || res.HaltedTarget != db.TargetZitadel {
		t.Errorf("want the halt attributed to zitadel: %+v", res)
	}
}

// The pre-flight probes the TARGET, not the add-on in front of it.
//
// The add-on is a separate container and stays up throughout a NAS outage, so a
// manifest read — which is how this was probed — answered "reachable" while
// nothing behind it was. A whole batch went through: twenty-one rows, twenty-one
// round trips, twenty-one outcomes nobody could confirm, all to learn what one
// call establishes.
func TestThePreflightProbesTheTargetRatherThanTheAddon(t *testing.T) {
	src, err := os.ReadFile("deps.go")
	if err != nil {
		t.Fatalf("read source: %v", err)
	}
	probe := regexp.MustCompile(`(?s)addonReachable = func\(ctx context\.Context, target string\) bool \{(.*?)\n\t\}`).
		FindStringSubmatch(string(src))
	if probe == nil {
		t.Fatal("the pre-flight is gone; if it moved, move this guard with it")
	}
	if !strings.Contains(probe[1], "Reachable") {
		t.Error("the pre-flight must read the target's reachability, not merely that the add-on answered")
	}
	if strings.Contains(probe[1], "addons.Refresh") {
		t.Error("a manifest read proves only that the add-on's own process is up")
	}
}

// §29 — a refused binding is a FINDING, not a failed write.
//
// `recordBinding` swallowed every error into one log line and the row settled
// `applied` regardless. The reasoning above it is right about the transient
// case — the add-on already persisted the decision, so failing to copy it does
// not un-apply anything — and wrong about this one, because a conflict is not a
// failure to write.
//
// What it means: the account the add-on just wrote to is one the backend
// attributes to a DIFFERENT subject. The two stores have diverged and this
// subject's entitlements have landed on somebody else's account. Marked
// applied, that surfaces nowhere — the other subject still shows as holding the
// account, this one shows as having none, and `convergeBound` iterates bindings
// so it is never revisited.
func TestARefusedBindingDoesNotSettleApplied(t *testing.T) {
	h := stubAddonDrain(t, addonRow("o1"))
	h.resp = addons.ApplyResponse{Outcome: addons.OutcomeSucceeded, Username: "ada", UID: 3001}
	t.Cleanup(swap(&saveBinding, func(context.Context, db.TargetBinding) error {
		return fmt.Errorf("%w: ada on truenas", db.ErrBindingConflict)
	}))

	res, err := DrainAddon(context.Background(), "truenas")
	if err != nil {
		t.Fatal(err)
	}
	if len(h.applied) != 0 || res.Applied != 0 {
		t.Fatalf("a convergence the backend cannot attribute must not be recorded as applied: %+v", res)
	}
	if res.Failed != 1 {
		t.Fatalf("it must settle terminally — a retry converges the same wrong account again: %+v", res)
	}

	reason := h.failed["o1"]
	// The mutation LANDED. Every other terminal failure on this path means it
	// did not, so an operator who reads this as "the change did not go through"
	// re-drives it onto the same account belonging to the same other person.
	if !strings.Contains(reason, "was applied") {
		t.Errorf("the reason must say the change landed: %q", reason)
	}
	if !strings.Contains(reason, "ada") || !strings.Contains(reason, "another subject") {
		t.Errorf("it must name the account and what is wrong with it: %q", reason)
	}
	if !strings.Contains(reason, "Do not retry") {
		t.Errorf("and that retrying is the wrong move: %q", reason)
	}
}

// A transient write failure keeps its old behaviour, which was correct. The
// binding is the backend's copy of a decision the add-on already persisted, so
// a connection blip must not convert a successful convergence into a terminal
// failure an operator has to triage.
func TestATransientBindingWriteFailureStillSettlesApplied(t *testing.T) {
	h := stubAddonDrain(t, addonRow("o1"))
	h.resp = addons.ApplyResponse{Outcome: addons.OutcomeSucceeded, Username: "ada", UID: 3001}
	t.Cleanup(swap(&saveBinding, func(context.Context, db.TargetBinding) error {
		return errors.New("connection reset by peer")
	}))

	res, err := DrainAddon(context.Background(), "truenas")
	if err != nil {
		t.Fatal(err)
	}
	if res.Applied != 1 || len(h.applied) != 1 {
		t.Fatalf("a blip writing the mirror must not un-apply a convergence that happened: %+v", res)
	}
	if res.Failed != 0 {
		t.Errorf("and it is not a finding: %+v", res)
	}
}

// The finding is persisted, and it names BOTH claimants.
//
// The failed row carries the account name and the reason, and retention prunes
// it — so an operator who was not watching that pass has nothing. The finding
// has to outlive the drain that produced it, like the log anchor's does.
func TestABindingConflictIsRecordedAsAStandingFinding(t *testing.T) {
	h := stubAddonDrain(t, addonRow("o1"))
	h.resp = addons.ApplyResponse{Outcome: addons.OutcomeSucceeded, Username: "ada", UID: 3001}
	t.Cleanup(swap(&saveBinding, func(context.Context, db.TargetBinding) error {
		return fmt.Errorf("%w: ada on truenas", db.ErrBindingConflict)
	}))
	// The other claimant is READ BACK rather than inferred: the conflict may
	// have been raised by the uid index on an account renamed out of band, and
	// guessing from the reported name would put the wrong person on a screen.
	t.Cleanup(swap(&bindingHolder, func(_ context.Context, target, username string, uid int64) (string, bool, error) {
		if target != "truenas" || username != "ada" || uid != 3001 {
			t.Errorf("the holder lookup must carry both keys: %s %s %d", target, username, uid)
		}
		return "subject-a", true, nil
	}))
	var recorded []db.BindingConflict
	t.Cleanup(swap(&saveConflict, func(_ context.Context, c db.BindingConflict) error {
		recorded = append(recorded, c)
		return nil
	}))

	if _, err := DrainAddon(context.Background(), "truenas"); err != nil {
		t.Fatal(err)
	}
	if len(recorded) != 1 {
		t.Fatalf("the disagreement must be persisted, got %d", len(recorded))
	}
	got := recorded[0]
	if got.ConvergedSubjectID != "sub-1" || got.BoundSubjectID != "subject-a" {
		t.Errorf("both claimants must be named: %+v", got)
	}
	if got.Username != "ada" || got.AccountUID == nil || *got.AccountUID != 3001 {
		t.Errorf("the account must be identified by name and uid: %+v", got)
	}
	if got.OutboxID == "" {
		t.Error("the finding must trace back to the change that caused it")
	}
}

// A finding that cannot name the other claimant is not recorded, and the row
// still settles terminally. A warning with no subject is worse than the failed
// row's reason, which already carries the account name.
func TestAConflictWithNoTraceableHolderStillSettlesTerminally(t *testing.T) {
	h := stubAddonDrain(t, addonRow("o1"))
	h.resp = addons.ApplyResponse{Outcome: addons.OutcomeSucceeded, Username: "ada", UID: 3001}
	t.Cleanup(swap(&saveBinding, func(context.Context, db.TargetBinding) error {
		return fmt.Errorf("%w: ada on truenas", db.ErrBindingConflict)
	}))
	t.Cleanup(swap(&bindingHolder, func(context.Context, string, string, int64) (string, bool, error) {
		return "", false, nil
	}))
	t.Cleanup(swap(&saveConflict, func(context.Context, db.BindingConflict) error {
		t.Error("a finding that cannot name the other claimant must not be recorded")
		return nil
	}))

	res, err := DrainAddon(context.Background(), "truenas")
	if err != nil {
		t.Fatal(err)
	}
	if res.Failed != 1 || res.Applied != 0 {
		t.Fatalf("the row still settles terminally whatever the finding did: %+v", res)
	}
}
