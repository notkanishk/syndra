package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"syndra/internal/db"
	"syndra/internal/models"
	"syndra/internal/services"
)

// refusedDelivery sends every row and has Zitadel refuse `refused` outright.
func refusedDelivery(t *testing.T, refused, cause string) {
	t.Helper()
	orig := svcDeliver
	t.Cleanup(func() { svcDeliver = orig })
	svcDeliver = func(_ context.Context, ids []string, _ bool) services.Delivery {
		d := services.Delivery{Sent: ids, Reason: map[string]string{}}
		for _, id := range ids {
			if id == refused {
				d.Reason[id] = db.WaitReasonRefused(cause)
			}
		}
		return d
	}
}

// N1: a give or approval whose own row Zitadel refused says refused, never
// "" (done); a rule row's refusal is not the act's.
func TestOwnWait_ARefusalIsTheActsOnlyWhenItsOwnRowWasRefused(t *testing.T) {
	own := services.Delivery{Sent: []string{"own", "rule"}, Reason: map[string]string{"own": db.WaitReasonRefused(db.RefusalNotFound)}}
	if reason, retry, _ := ownWait(own, "own", "failed"); reason != db.WaitReasonRefused(db.RefusalNotFound) || retry != nil {
		t.Fatalf("own refused: got %q %v", reason, retry)
	}
	rule := services.Delivery{Sent: []string{"own", "rule"}, Reason: map[string]string{"rule": db.WaitReasonRefused(db.RefusalOther)}}
	if reason, _, _ := ownWait(rule, "own", "applied"); reason != "" {
		t.Fatalf("a rule row's refusal must not be blamed on the act: got %q", reason)
	}
}

// N1: a bulk row Zitadel refused is failed, never queued for Send, and
// carries its cause in the wait reason; the screen says the refusal and its
// step from it (ui/src/lib/plan-groups.ts refusedSeat), under the plan's words.
func TestDelivered_ARefusalIsFailedWithItsCause(t *testing.T) {
	reason, retry := delivered(services.Delivery{Sent: []string{"o1"},
		Reason: map[string]string{"o1": db.WaitReasonRefused(db.RefusalNotAllowed)}})
	if reason != db.WaitReasonRefused(db.RefusalNotAllowed) || retry != nil {
		t.Fatalf("got %q %v", reason, retry)
	}
	out := services.BulkOutcome{Effect: services.EffectApply, Detail: "Gains Member in Laser Lab."}
	settleBulkRow(&out, reason, retry)
	if out.Effect != services.EffectFailed || out.WaitReason != reason || out.Detail != "Gains Member in Laser Lab." {
		t.Fatalf("a refused row is failed with its cause and keeps its words: %+v", out)
	}
}

// N1 + N4 for bulk approve: a refused approval is failed with its cause; an
// approval whose own row went is applied even when a rule's row waits, and
// the rule row is counted apart.
func TestBulkApprove_RefusedIsFailedAndARuleWaitIsNotTheApprovals(t *testing.T) {
	// The planner writes one row per approval, "own-<id>" (stubPlannedBulk);
	// the send is the plan's.
	stubRequestLookup(t, map[string]models.AccessRequest{
		"r1": pendingRequest("r1", "u1", "trained"),
		"r2": pendingRequest("r2", "u2", "trained"),
	})
	svcDeliverChange = func(_ context.Context, ch services.Change) services.Delivery {
		own := ch.Writes[0].OutboxID
		if own == "own-r1" { // r1: Zitadel refused the approved role itself
			return services.Delivery{Sent: []string{own}, Reason: map[string]string{own: db.WaitReasonRefused(db.RefusalInvalid)}}
		}
		// r2: the approved role went; a rule's role waits.
		return services.Delivery{Sent: []string{own}, Waiting: []string{"rule-" + own},
			Reason: map[string]string{"rule-" + own: db.WaitReasonRoleWaits}}
	}
	rr := httptest.NewRecorder()
	handleBulkDecideRequests(rr, bulkDecisionReq(t, `{"ids":["r1","r2"],"status":"approved"}`, true))
	plan := decodeBulkPlan(t, rr)

	byUser := map[string]services.BulkOutcome{}
	for _, o := range plan.Outcomes {
		byUser[o.UserID] = o
	}
	if o := byUser["r1"]; o.Effect != services.EffectFailed || !db.IsRefused(o.WaitReason) || strings.Contains(o.Detail, "Recorded in Syndra") {
		t.Fatalf("refused approval: %+v", o)
	}
	// How the rule's wait is worded is the screen's; the row counts it apart.
	if o := byUser["r2"]; o.Effect != services.EffectApplied || o.RuleWaiting != 1 || o.WaitReason != "" {
		t.Fatalf("an approval that went, with a rule's role waiting, stays applied: %+v", o)
	}
}

// N1: each role of a Zitadel › Users change carries its own refusal cause.
func TestZitadelGrantChange_ARefusedRoleSaysWhy(t *testing.T) {
	newGrantPipe(t)
	origStatus := dbGetPropagationStatus
	t.Cleanup(func() { dbGetPropagationStatus = origStatus })
	refusedDelivery(t, "ob-add-bad", db.RefusalNotFound)
	dbGetPropagationStatus = func(_ context.Context, id string) (string, error) {
		if id == "ob-add-bad" {
			return "failed", nil
		}
		return "applied", nil
	}
	_, res := serveGrant(t, handleAssignZitadelGrant, http.MethodPost, "/api/v1/zitadel/users/u1/grants?apply=true",
		`{"projectId":"p1","roleKeys":["good","bad"]}`)
	for _, rc := range res.Roles {
		switch rc.Role {
		case "p1/bad":
			if rc.Status != "failed" || rc.Reason != db.WaitReasonRefused(db.RefusalNotFound) {
				t.Fatalf("refused role: %+v", rc)
			}
		case "p1/good":
			if rc.Status != "applied" || rc.Reason != "" {
				t.Fatalf("given role: %+v", rc)
			}
		}
	}
	if res.Status != "failed" {
		t.Fatalf("the change is only as far as its least delivered role: %+v", res)
	}
}

// N6: what a part-recorded change left says what really happens to it.
func TestPartlyRecordedMessage_FollowsTheDrainSwitch(t *testing.T) {
	on := partlyRecordedMessage(2, true, true)
	if !strings.Contains(on, "a revocation there sends on its own") || !strings.Contains(on, "press Send") {
		t.Fatalf("drain on, with a revocation: %q", on)
	}
	for _, off := range []string{partlyRecordedMessage(2, true, false), partlyRecordedMessage(2, false, true)} {
		if strings.Contains(off, "on its own") || !strings.Contains(off, "until you press Send") {
			t.Fatalf("nothing goes on its own here: %q", off)
		}
	}
}
