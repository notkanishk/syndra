package propagation

import (
	"context"
	"encoding/json"
	"errors"
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
