package planapply

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"

	"syndra/internal/db"
)

// harness records the order the seams were called in, because the ordering is
// the whole content of this gate and no per-call assertion can see one.
type harness struct {
	steps     []string
	committed bool

	state      string
	stateErr   error
	plan       db.Plan
	subjects   []db.PlanSubject
	claimErr   error
	citation   db.PlanCitation
	enqueued   []db.EntitlementApply
	enqueueErr map[string]error
	rolledBack bool
	txOpened   bool
}

// subjectOf resolves an approval to its subject the way the enqueue's own
// statement does — from the plan subject row, never from a caller's field.
func (h *harness) subjectOf(planSubjectID string) string {
	for _, s := range h.subjects {
		if s.ID == planSubjectID {
			return s.SubjectID
		}
	}
	return "unknown:" + planSubjectID
}

func install(t *testing.T, h *harness) {
	t.Helper()
	savedTx, savedState, savedClaim, savedEnqueue := inTx, targetState, claimPlan, enqueue
	t.Cleanup(func() { inTx, targetState, claimPlan, enqueue = savedTx, savedState, savedClaim, savedEnqueue })

	inTx = func(ctx context.Context, fn func(pgx.Tx) error) error {
		h.txOpened = true
		// A nil transaction is safe precisely because every seam that would use
		// one is faked below. If a future step reaches the database directly,
		// this panics rather than passing.
		if err := fn(nil); err != nil {
			h.rolledBack = true
			return err
		}
		h.committed = true
		h.steps = append(h.steps, "commit")
		return nil
	}
	targetState = func(_ context.Context, _ pgx.Tx, target string) (string, error) {
		h.steps = append(h.steps, "target:"+target)
		return h.state, h.stateErr
	}
	claimPlan = func(_ context.Context, _ pgx.Tx, c db.PlanCitation) (db.Plan, []db.PlanSubject, error) {
		h.steps = append(h.steps, "claim:"+c.PlanID)
		h.citation = c
		return h.plan, h.subjects, h.claimErr
	}
	enqueue = func(_ context.Context, _ pgx.Tx, p db.EntitlementApply) (string, error) {
		// The approval is all the gate passes, so the fake resolves the subject
		// from it exactly as the real INSERT ... SELECT does.
		subject := h.subjectOf(p.PlanSubjectID)
		h.steps = append(h.steps, "enqueue:"+subject)
		h.enqueued = append(h.enqueued, p)
		if err := h.enqueueErr[subject]; err != nil {
			return "", err
		}
		return "outbox-" + subject, nil
	}
}

func working() *harness {
	return &harness{
		state: db.TargetActive,
		plan:  db.Plan{ID: "plan-1", Target: "truenas", Surface: "entitlements", CreatedBy: "operator-1"},
		subjects: []db.PlanSubject{
			{ID: "ps-1", SubjectID: "u1", Fingerprint: "sha256:a", Outcome: db.PlanOutcome{Effect: db.PlanEffectApply}},
			{ID: "ps-2", SubjectID: "u2", Fingerprint: "sha256:b", Outcome: db.PlanOutcome{Effect: db.PlanEffectApply}},
		},
		enqueueErr: map[string]error{},
	}
}

func request() Request {
	return Request{PlanID: "plan-1", Target: "truenas", Surface: "entitlements", Actor: "operator-1"}
}

// 2.19 — an apply that cites no plan is refused, and refused before anything
// opens. This is the rule the whole mechanism rests on: without it the apply
// falls back to recomputing from a re-submitted request, which is the gap the
// plan store exists to close.
func TestAnApplyCitingNoPlanIsRefusedBeforeAnythingOpens(t *testing.T) {
	for _, id := range []string{"", "   ", "\t\n"} {
		h := working()
		install(t, h)

		_, err := Apply(context.Background(), Request{PlanID: id, Target: "truenas", Surface: "entitlements", Actor: "operator-1"})
		if !errors.Is(err, ErrPlanRequired) {
			t.Errorf("Apply(%q) = %v, want ErrPlanRequired", id, err)
		}
		if h.txOpened {
			t.Errorf("Apply(%q) opened a transaction for a request it was always going to refuse", id)
		}
		if len(h.steps) != 0 {
			t.Errorf("Apply(%q) ran %v", id, h.steps)
		}
	}
}

// 2.9, 2.19 — the ordering. The plan is claimed before any work is queued, and
// the whole thing commits once.
func TestThePlanIsClaimedBeforeAnyWorkIsQueued(t *testing.T) {
	h := working()
	install(t, h)

	res, err := Apply(context.Background(), request())
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}

	want := []string{"target:truenas", "claim:plan-1", "enqueue:u1", "enqueue:u2", "commit"}
	if strings.Join(h.steps, ",") != strings.Join(want, ",") {
		t.Errorf("steps = %v, want %v", h.steps, want)
	}
	if len(res.Queued) != 2 || res.Queued[0].OutboxID != "outbox-u1" {
		t.Errorf("queued = %+v", res.Queued)
	}
	// Each row cites the approval that authorises it. Without that reference
	// the drain has no fingerprint to re-verify and no way to know what was
	// approved for this subject.
	for i, e := range h.enqueued {
		if e.PlanSubjectID != h.subjects[i].ID {
			t.Errorf("queued row %d cites %q, want the plan subject %q", i, e.PlanSubjectID, h.subjects[i].ID)
		}
	}
}

// 2.9 — the gate passes the approval and nothing else. Every other field on a
// queued row is derived from that approval by the write itself, so there is no
// second value here that could disagree with it.
func TestTheGatePassesOnlyTheApproval(t *testing.T) {
	h := working()
	install(t, h)

	if _, err := Apply(context.Background(), request()); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	for i, e := range h.enqueued {
		if (e != db.EntitlementApply{PlanSubjectID: h.subjects[i].ID}) {
			t.Errorf("queued row %d carries caller-supplied fields beside the approval: %+v", i, e)
		}
	}
}

// 2.19 — the citation reaches the claim unchanged. Four identifiers, all
// strings: two transposed at this call site would compare a surface against a
// target and either refuse everything or, symmetrically, compare nothing.
func TestTheCitationReachesTheClaimUnchanged(t *testing.T) {
	h := working()
	install(t, h)

	req := Request{PlanID: "plan-1", Target: "truenas", Surface: "drift.triage", Actor: "operator-7"}
	if _, err := Apply(context.Background(), req); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	want := db.PlanCitation{PlanID: "plan-1", Target: "truenas", Surface: "drift.triage", Actor: "operator-7"}
	if h.citation != want {
		t.Errorf("citation = %+v, want %+v", h.citation, want)
	}
}

// 2.19 — a refused claim queues nothing. Expired, spent, wrong surface, someone
// else's approval: the gate does not distinguish them here, because the claim
// already did and its refusal is what the operator needs to read.
func TestARefusedClaimQueuesNothingAndDoesNotCommit(t *testing.T) {
	for _, refusal := range []error{db.ErrPlanNotFound, db.ErrPlanExpired, db.ErrPlanAlreadyApplied, db.ErrPlanNotCitableHere, db.ErrPlanNotYours} {
		h := working()
		h.claimErr = refusal
		install(t, h)

		res, err := Apply(context.Background(), request())
		if !errors.Is(err, refusal) {
			t.Errorf("Apply = %v, want %v", err, refusal)
		}
		if len(h.enqueued) != 0 {
			t.Errorf("%v queued %d rows", refusal, len(h.enqueued))
		}
		if h.committed {
			t.Errorf("%v committed", refusal)
		}
		if len(res.Queued) != 0 {
			t.Errorf("%v returned queued rows: %+v", refusal, res.Queued)
		}
	}
}

// 2.9 — a target the deployment dropped takes no work, and the plan is not
// spent finding that out. Nothing would ever drain those rows, and a row that
// never drains is counted as queued, which reads as "recorded" everywhere.
func TestADisabledTargetQueuesNothingAndLeavesThePlanUnspent(t *testing.T) {
	h := working()
	h.state = db.TargetDisabled
	install(t, h)

	_, err := Apply(context.Background(), request())
	if !errors.Is(err, db.ErrTargetNotActive) {
		t.Fatalf("Apply = %v, want ErrTargetNotActive", err)
	}
	if !strings.Contains(err.Error(), db.TargetDisabled) {
		t.Errorf("the refusal does not say what state the target is in: %v", err)
	}
	for _, step := range h.steps {
		if strings.HasPrefix(step, "claim:") {
			t.Error("the plan was claimed for a target that can take no work — the approval is spent and the operator cannot re-apply it after re-enabling the target")
		}
	}
	if h.committed {
		t.Error("committed")
	}
}

// 2.10 — a failure partway through leaves nothing behind. The claim and the
// queued rows share one transaction precisely so that this rollback undoes the
// approval's spending too.
func TestAFailedEnqueueStopsAndUndoesTheClaim(t *testing.T) {
	h := working()
	h.enqueueErr["u2"] = errors.New("outbox unique violation")
	install(t, h)

	res, err := Apply(context.Background(), request())
	if err == nil {
		t.Fatal("Apply succeeded despite a failed enqueue")
	}
	if !strings.Contains(err.Error(), "u2") {
		t.Errorf("the failure does not name the subject it stopped on: %v", err)
	}
	if len(h.enqueued) != 2 {
		t.Errorf("attempted %d enqueues, want the first two and no more", len(h.enqueued))
	}
	if h.committed {
		t.Error("committed after a failed enqueue — the claim would be spent on work that was never queued")
	}
	if !h.rolledBack {
		t.Error("the transaction was not rolled back")
	}
	// And the caller learns nothing about the row that briefly succeeded.
	if len(res.Queued) != 0 {
		t.Errorf("a failed apply returned queued rows: %+v", res.Queued)
	}
}

// 2.23, 2.24 — a provisional plan applies, and says so. "Recorded and waiting"
// and "applied" are the distinction the queued accounting exists to protect.
func TestAProvisionalApplyIsReportedAsProvisional(t *testing.T) {
	h := working()
	h.plan.Provisional = true
	install(t, h)

	res, err := Apply(context.Background(), request())
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if !res.Provisional {
		t.Error("a provisional plan applied as though its fingerprints were live")
	}
	if len(res.Queued) != 2 {
		t.Errorf("a provisional apply queued %d rows, want 2 — the change is recorded, not refused", len(res.Queued))
	}
}

// 2.10, 2.19 — the gate dispatches nothing. Verification happens at dispatch
// and so does the call; an apply that reached a target here would be doing it
// before the fingerprint check that guards it.
func TestTheGateReachesNoTarget(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("read package dir: %v", err)
	}
	forbidden := []string{"internal/addons", "internal/zitadel", "net/http"}
	var checked int
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".go") || strings.HasSuffix(e.Name(), "_test.go") {
			continue
		}
		src, err := os.ReadFile(filepath.Clean(e.Name()))
		if err != nil {
			t.Fatalf("read %s: %v", e.Name(), err)
		}
		checked++
		for _, imp := range forbidden {
			if strings.Contains(string(src), `"syndra/`+strings.TrimPrefix(imp, "internal/")+`"`) ||
				strings.Contains(string(src), `"`+imp+`"`) ||
				strings.Contains(string(src), `"syndra/`+imp+`"`) {
				t.Errorf("%s imports %s — the apply gate must queue work, not perform it", e.Name(), imp)
			}
		}
	}
	if checked == 0 {
		t.Fatal("no source files were examined, so this guard proved nothing")
	}
}

// The rehearsal records `blocked` and `no_change` on purpose. Queueing them
// dispatches convergence for subjects the plan said would not change — or
// refused outright — which is the gate approving work nobody reviewed.
func TestOnlySubjectsThePlanSaidWouldChangeAreQueued(t *testing.T) {
	h := working()
	h.subjects = []db.PlanSubject{
		{ID: "ps-1", SubjectID: "u1", Outcome: db.PlanOutcome{Effect: db.PlanEffectApply}},
		{ID: "ps-2", SubjectID: "u2", Outcome: db.PlanOutcome{Effect: db.PlanEffectNoChange}},
		{ID: "ps-3", SubjectID: "u3", Outcome: db.PlanOutcome{Effect: db.PlanEffectBlocked}},
	}
	install(t, h)

	res, err := Apply(context.Background(), request())
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Queued) != 1 || res.Queued[0].SubjectID != "u1" {
		t.Fatalf("only the changing subject may be queued: %+v", res.Queued)
	}
	if len(h.enqueued) != 1 {
		t.Fatalf("nothing may be enqueued for a subject the rehearsal refused: %+v", h.enqueued)
	}
	// The approval is still spent on the whole plan: the operator approved this
	// plan, and the rows it said would do nothing do nothing.
	if !h.committed {
		t.Fatal("the claim must still commit")
	}
}

// The claim predicates on the request fingerprint, so a gate that always sent
// "" could only ever claim plans stored with no fingerprint — every plan bound
// to a request would refuse its own apply.
func TestTheCitationCarriesTheRequestFingerprint(t *testing.T) {
	h := working()
	install(t, h)

	req := request()
	req.RequestFingerprint = "sha256:body"
	if _, err := Apply(context.Background(), req); err != nil {
		t.Fatal(err)
	}
	if h.citation.RequestFingerprint != "sha256:body" {
		t.Fatalf("the citation must carry it: %+v", h.citation)
	}
}

// 2.30/2.31 — the accounting an add-on target makes load-bearing.
//
// Nothing this endpoint does reaches a target: the rows are written to the
// outbox and an add-on row waits for an operator to resume the drain. So the
// summary has a `queued` column and a `succeeded` column, and the second one is
// always zero — present precisely so a client cannot default it into existence.
func TestAnAcceptedApplyCountsQueuedAndNeverSucceeded(t *testing.T) {
	h := working()
	h.subjects = []db.PlanSubject{
		{ID: "ps-1", SubjectID: "u1", Outcome: db.PlanOutcome{Effect: db.PlanEffectApply}},
		{ID: "ps-2", SubjectID: "u2", Outcome: db.PlanOutcome{Effect: db.PlanEffectApply}},
		{ID: "ps-3", SubjectID: "u3", Outcome: db.PlanOutcome{Effect: db.PlanEffectNoChange}},
		{ID: "ps-4", SubjectID: "u4", Outcome: db.PlanOutcome{Effect: db.PlanEffectBlocked}},
	}
	install(t, h)

	res, err := Apply(context.Background(), request())
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	want := ApplySummary{Total: 4, Queued: 2, NoChange: 1, Blocked: 1}
	if res.Summary != want {
		t.Fatalf("summary = %+v, want %+v", res.Summary, want)
	}
	if res.Summary.Succeeded != 0 {
		t.Error("nothing has reached the target; a non-zero succeeded count is a change reported that has not left the database")
	}
	if res.Summary.Total != res.Summary.Queued+res.Summary.NoChange+res.Summary.Blocked {
		t.Error("the counts must add up, or an operator is left wondering where the difference went")
	}
}

// 9.18/9.19 — the response says whether these rows reach the target on their
// own. An operator who assumes a queued change applies itself finds out when
// somebody complains.
func TestTheResponseSaysWhetherItDrainsOnItsOwn(t *testing.T) {
	for _, tc := range []struct {
		name        string
		queued      int
		provisional bool
		wants       string
		forbids     string
	}{
		{
			name: "a grant waits", queued: 2,
			wants:   "resumes the drain",
			forbids: "applied",
		},
		{
			name: "a provisional plan says what it is waiting for", queued: 1, provisional: true,
			wants:   "re-checked when the target comes back",
			forbids: "resumes the drain",
		},
		{
			name: "an empty apply says so rather than implying work", queued: 0,
			wants:   "Nothing was queued",
			forbids: "resumes the drain",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := disclose(tc.queued, tc.provisional)
			if !strings.Contains(got, tc.wants) {
				t.Errorf("disclosure = %q, want it to mention %q", got, tc.wants)
			}
			if strings.Contains(got, tc.forbids) {
				t.Errorf("disclosure = %q, must not mention %q", got, tc.forbids)
			}
		})
	}
}
