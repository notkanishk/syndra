package handlers

import (
	"context"
	"errors"
	"net/http"
	"slices"
	"strings"
	"testing"

	"syndra/internal/access"
	"syndra/internal/db"
	"syndra/internal/extcall"
	"syndra/internal/models"
	"syndra/internal/services"
	"syndra/internal/services/propagation"
	"syndra/internal/zitadel"
)

// Re-audit R17: a Zitadel › Users change that fails part-way says exactly
// what the server wrote. A fault after a role was recorded is PARTLY_RECORDED
// with the roles named (never "Nothing was changed"); one before any write is
// DB_ERROR; and a grant that couldn't be read is GRANT_NOT_READ, a failure
// before anything was written (never "may have gone through").
func TestZitadelGrantChange_FailureSaysExactlyWhatWasWritten(t *testing.T) {
	p := newGrantPipe(t)
	calls := 0
	dbEnqueueDirectGrantPropagation = func(_ context.Context, ep db.EnqueueParams) (db.EnqueueResult, error) {
		calls++
		if calls == 2 {
			return db.EnqueueResult{}, errors.New("connection reset")
		}
		p.enqueued = append(p.enqueued, ep)
		return db.EnqueueResult{OutboxID: "ob-" + ep.RoleKeys[0]}, nil
	}
	w, _ := serveGrant(t, handleAssignZitadelGrant, http.MethodPost, "/api/v1/zitadel/users/u1/grants",
		`{"projectId":"p1","roleKeys":["r1","r2"]}`)
	if w.Code != http.StatusInternalServerError || !strings.Contains(w.Body.String(), partlyRecordedCode) ||
		!strings.Contains(w.Body.String(), `"recorded":"p1/r1"`) {
		t.Fatalf("a fault after r1 was written is partly recorded, naming r1: %d %s", w.Code, w.Body)
	}

	calls = 1 // the very first write fails
	w, _ = serveGrant(t, handleAssignZitadelGrant, http.MethodPost, "/api/v1/zitadel/users/u1/grants",
		`{"projectId":"p1","roleKeys":["r1","r2"]}`)
	if w.Code != http.StatusInternalServerError || !strings.Contains(w.Body.String(), `"DB_ERROR"`) {
		t.Fatalf("a fault before any write is a plain DB_ERROR: %d %s", w.Code, w.Body)
	}

	dbListUserGrantsLive = func(context.Context, string, string) (zitadel.UserGrant, error) {
		return zitadel.UserGrant{}, errors.New("zitadel: unavailable")
	}
	w, _ = serveGrant(t, handleRemoveZitadelGrant, http.MethodDelete, "/api/v1/zitadel/users/u1/grants/g1", "")
	if w.Code != http.StatusBadGateway || !strings.Contains(w.Body.String(), grantNotReadCode) {
		t.Fatalf("an unread grant is refused before any write: %d %s", w.Code, w.Body)
	}
}

// Re-audit R18: the revoke dialog's count is the server's own rule. The
// preview walks the removal's own code, writes nothing, and answers the same
// revoked and kept roles the removal then does: a direct grant by its delta,
// a role a bundle or a rule gives kept, an untraced role revoked.
func TestRemoveZitadelGrantPreview_IsTheRemovalsOwnAnswer(t *testing.T) {
	p := newGrantPipe(t)
	origPreview := svcPreviewDirectGrantRemoval
	t.Cleanup(func() { svcPreviewDirectGrantRemoval = origPreview })
	dbGetGrantIndex = func(context.Context, string) (db.ZitadelGrantIndex, error) {
		return db.ZitadelGrantIndex{ProjectID: "p1", RoleKeys: []string{"direct", "bundled", "untraced"}}, nil
	}
	svcUserDirectGrants = func(context.Context, string) ([]models.DirectGrant, error) {
		return []models.DirectGrant{{ID: "dg1", ProjectID: "p1", RoleKey: "direct"}}, nil
	}
	svcUserExpectsRole = func(_ context.Context, _, _, role string) (bool, error) { return role == "bundled", nil }
	answer := services.DirectGrantRemoval{Revoked: []string{"p1/direct", "p2/derived"}, Retained: []string{}}
	svcDeleteDirectGrant = func(context.Context, string, string, string) (services.DirectGrantRemoval, error) {
		out := answer
		out.OutboxIDs = []string{"ob-del"}
		return out, nil
	}
	svcPreviewDirectGrantRemoval = func(context.Context, string, string, string) (services.DirectGrantRemoval, error) {
		return answer, nil
	}

	w, preview := serveGrant(t, handleRemoveZitadelGrantPreview, http.MethodGet, "/api/v1/zitadel/users/u1/grants/g1/removal", "")
	if w.Code != http.StatusOK {
		t.Fatalf("preview: %d %s", w.Code, w.Body)
	}
	if len(p.enqueued) != 0 {
		t.Fatalf("a preview writes nothing, got %v", p.enqueued)
	}
	_, removal := serveGrant(t, handleRemoveZitadelGrant, http.MethodDelete, "/api/v1/zitadel/users/u1/grants/g1", "")
	if !slices.Equal(preview.Revoked, removal.Revoked) || !slices.Equal(preview.Retained, removal.Retained) {
		t.Fatalf("preview %v/%v must equal the removal %v/%v", preview.Revoked, preview.Retained, removal.Revoked, removal.Retained)
	}
	if !slices.Equal(preview.Revoked, []string{"p1/direct", "p2/derived", "p1/untraced"}) || !slices.Equal(preview.Retained, []string{"p1/bundled"}) {
		t.Fatalf("revoked %v kept %v", preview.Revoked, preview.Retained)
	}
}

// Re-audit R3: a give Zitadel didn't take carries its next try, so the screen
// can say Syndra sends it again on its own and when.
func TestZitadelGrantChange_CarriesTheRetry(t *testing.T) {
	newGrantPipe(t)
	origDeliver, origStatus := svcDeliver, dbGetPropagationStatus
	t.Cleanup(func() { svcDeliver, dbGetPropagationStatus = origDeliver, origStatus })
	next := &extcall.RetryState{State: extcall.RetryRetrying, Try: 2, Of: 6}
	svcDeliver = func(_ context.Context, ids []string, _ bool) services.Delivery {
		return services.Delivery{Sent: ids, Drain: propagation.DrainResult{Requeued: 1, Retry: next}}
	}
	dbGetPropagationStatus = func(context.Context, string) (string, error) { return "pending", nil }
	_, res := serveGrant(t, handleAssignZitadelGrant, http.MethodPost, "/api/v1/zitadel/users/u1/grants",
		`{"projectId":"p1","roleKeys":["r1"]}`)
	if res.WaitReason != "halted:retrying" || res.Retry == nil || res.Retry.Try != 2 {
		t.Fatalf("a retrying give says when it goes again: %+v", res)
	}
}

// Re-audit R12: one delivery carries the act's own row and its rules' rows.
// The act's own reason is its own; a rule-given role that waits is counted
// apart, never blamed on the act and never left unsaid.
func TestOwnWait_SplitsTheActFromItsRules(t *testing.T) {
	d := services.Delivery{
		Sent:    []string{"own"},
		Waiting: []string{"rule"},
		Reason:  map[string]string{"rule": db.WaitReasonRoleWaits},
		Drain:   propagation.DrainResult{Applied: 1},
	}
	reason, retry, ruleWaiting := ownWait(d, "own", "applied")
	if reason != "" || retry != nil || ruleWaiting != 1 {
		t.Fatalf("own applied, rule waits: got %q %v %d", reason, retry, ruleWaiting)
	}
	// The own row waits for its role; no rule row.
	d = services.Delivery{Waiting: []string{"own"}, Reason: map[string]string{"own": db.WaitReasonRoleWaits}}
	if reason, _, n := ownWait(d, "own", ""); reason != db.WaitReasonRoleWaits || n != 0 {
		t.Fatalf("own waits: got %q %d", reason, n)
	}
	// The own row Zitadel didn't take: retrying, with its next try.
	next := &extcall.RetryState{State: extcall.RetryRetrying, Try: 2, Of: 6}
	d = services.Delivery{Sent: []string{"own"}, Drain: propagation.DrainResult{Requeued: 1, Retry: next}}
	if reason, retry, _ := ownWait(d, "own", "pending"); reason != "halted:retrying" || retry != next {
		t.Fatalf("own retrying: got %q %v", reason, retry)
	}
}

// Re-audit C2: the bulk-approve plan names what the automatic rules give with
// each approval, as a single approval does. R2: an approval whose role waits
// is queued in the result, with its reason, not "applied" regardless.
func TestBulkApprove_NamesRuleGiftsAndSaysWhatWaits(t *testing.T) {
	stubRequestLookup(t, map[string]models.AccessRequest{"r1": pendingRequest("r1", "u1", "trained")})
	origOffered := checkRoleOffered
	t.Cleanup(func() { checkRoleOffered = origOffered })
	checkRoleOffered = func(context.Context, string, string) error { return nil }
	svcRulesFollowing = func(_ context.Context, uid string, roles []access.Role) ([]services.RuleRow, error) {
		return []services.RuleRow{{ProjectID: "p_shop", RoleKey: "bench", RuleID: "rule1",
			Sentence: "Also gives Bench in Workshop, by the automatic rule from Trained."}}, nil
	}
	plan := rehearseDecisionBatch(context.Background(), []string{"r1"}, "approved")
	if !strings.Contains(plan.Outcomes[0].Consequence, "Also gives Bench in Workshop") {
		t.Fatalf("the plan must name the rule gift: %+v", plan.Outcomes[0])
	}

	origDeliver := svcDeliver
	t.Cleanup(func() { svcDeliver = origDeliver })
	svcDeliver = func(_ context.Context, ids []string, _ bool) services.Delivery {
		return services.Delivery{Waiting: ids, Reason: map[string]string{ids[0]: db.WaitReasonRoleWaits}}
	}
	plan.Outcomes[0].Effect = services.EffectApply
	applyDecisionPlan(bulkDecisionReq(t, `{"ids":["r1"],"status":"approved"}`, true), &plan, "approved", "")
	if o := plan.Outcomes[0]; o.Effect != services.EffectQueued || o.WaitReason != db.WaitReasonRoleWaits ||
		!strings.Contains(o.Detail, "always waits for you") {
		t.Fatalf("an approval whose role waits is queued and says why: %+v", o)
	}
}
