package services

import (
	"context"
	"errors"
	"testing"
	"time"

	"syndra/internal/db"
	"syndra/internal/models"
)

// Deciding a request (decision 5): the request read and its pending check, the
// approval through the planner with the ends an extension remembers, the
// decline's one read, and the audit row. The handler tests stub all of this
// away; here it runs, with the database stubbed at its edges.

type decisionWorld struct {
	request  models.AccessRequest
	holding  models.RequestHolding
	approved struct {
		own                    db.EnqueueParams
		endedBefore, endsAfter *time.Time
		calls                  int
	}
	declined struct {
		endedBefore *time.Time
		calls       int
	}
	audit []string
}

func stubDecisionWorld(t *testing.T, w *decisionWorld) {
	t.Helper()
	origReq, origHold, origApprove, origResolve, origAudit := svcGetAccessRequestByID, svcRequestHolding, svcApproveRequestAndEnqueue, svcResolveAccessRequest, svcInsertAuditLog
	origLock, origSnap, origFind, origCat, origTargets := svcInTxLockingAccess, svcReadAccessSnapshot, svcFindUser, PlannerCatalogue, dbTargetsMappedToRole
	t.Cleanup(func() {
		svcGetAccessRequestByID, svcRequestHolding, svcApproveRequestAndEnqueue, svcResolveAccessRequest, svcInsertAuditLog = origReq, origHold, origApprove, origResolve, origAudit
		svcInTxLockingAccess, svcReadAccessSnapshot, svcFindUser, PlannerCatalogue, dbTargetsMappedToRole = origLock, origSnap, origFind, origCat, origTargets
	})
	svcGetAccessRequestByID = func(context.Context, string) (models.AccessRequest, error) { return w.request, nil }
	svcRequestHolding = func(context.Context, string, string, string) (models.RequestHolding, error) { return w.holding, nil }
	svcApproveRequestAndEnqueue = func(_ context.Context, _, _, _ string, p db.EnqueueParams, before, after *time.Time, _ ...db.EnqueueParams) (db.EnqueueResult, error) {
		w.approved.own, w.approved.endedBefore, w.approved.endsAfter = p, before, after
		w.approved.calls++
		return db.EnqueueResult{OutboxID: "o1"}, nil
	}
	svcResolveAccessRequest = func(_ context.Context, _, _, _, _ string, before *time.Time) error {
		w.declined.endedBefore = before
		w.declined.calls++
		return nil
	}
	svcInsertAuditLog = func(_ context.Context, _, _, action, _ string) error {
		w.audit = append(w.audit, action)
		return nil
	}
	svcInTxLockingAccess = func(ctx context.Context, fn func(context.Context) error) error { return fn(ctx) }
	svcReadAccessSnapshot = func(context.Context, []string) (db.AccessSnapshot, error) {
		var direct []models.DirectGrant
		if w.holding.Kind != models.HoldingNone {
			direct = []models.DirectGrant{{ID: "g1", UserID: "u1", ProjectID: "pLab", RoleKey: "member", ExpiresAt: w.holding.Until}}
		}
		return db.AccessSnapshot{People: map[string]db.PersonAccess{"u1": {Direct: direct}}, RoleWaits: map[db.RoleRef]bool{}}, nil
	}
	svcFindUser = func(context.Context, string) (models.UserProfile, bool, error) {
		return models.UserProfile{ID: "u1", Name: "Charlotte Flair", Status: "active"}, true, nil
	}
	PlannerCatalogue = func(context.Context) ([]models.ProjectCatalog, error) {
		return []models.ProjectCatalog{{ID: "pLab", Name: "Laser Lab", Roles: []models.ProjectRole{{Key: "member", Label: "Member"}}}}, nil
	}
	dbTargetsMappedToRole = func(context.Context, string, string) ([]string, error) { return nil, nil }
}

func pendingRequest(kind string) models.AccessRequest {
	days := 30
	return models.AccessRequest{ID: "req1", Kind: kind, RequesterID: "u1", ProjectID: "pLab", RoleKey: "member", Status: "pending", DurationDays: &days}
}

func TestDecideRequest_ApprovalGoesThroughThePlanner(t *testing.T) {
	w := &decisionWorld{request: pendingRequest(RequestKindAccess), holding: models.RequestHolding{Kind: models.HoldingNone}}
	stubDecisionWorld(t, w)
	d, err := DecideRequest(context.Background(), "req1", "approved", "op1", "ok", nil)
	if err != nil {
		t.Fatal(err)
	}
	own := w.approved.own
	if w.approved.calls != 1 || own.SourceRef != "req1" || own.Source != "direct" || own.ExpiresAt == nil ||
		own.Reason != "Approved from access request" || own.GrantedBy != "op1" {
		t.Fatalf("the approval's own write: %+v", own)
	}
	if w.approved.endedBefore != nil || w.approved.endsAfter != nil {
		t.Error("an ordinary request records neither end")
	}
	if d.UserID != "u1" || d.Approval.Effect != EffectGive || d.Change.Writes[0].OutboxID != "o1" {
		t.Fatalf("decision = %+v", d)
	}
	if len(w.audit) != 1 || w.audit[0] != "access_request.approved" {
		t.Fatalf("audit = %v", w.audit)
	}
}

// An operator approving their own request is recorded as that (owner, 15 Oct).
func TestDecideRequest_SelfApprovalIsSaid(t *testing.T) {
	w := &decisionWorld{request: pendingRequest(RequestKindAccess), holding: models.RequestHolding{Kind: models.HoldingNone}}
	stubDecisionWorld(t, w)
	if _, err := DecideRequest(context.Background(), "req1", "approved", "u1", "", nil); err != nil {
		t.Fatal(err)
	}
	if len(w.audit) != 1 || w.audit[0] != "access_request.self_approved" {
		t.Fatalf("audit = %v", w.audit)
	}
}

func TestDecideRequest_AnExtensionRemembersBothEnds(t *testing.T) {
	until := time.Now().Add(72 * time.Hour)
	w := &decisionWorld{request: pendingRequest(RequestKindExtend), holding: models.RequestHolding{Kind: models.HoldingUntil, Until: &until}}
	stubDecisionWorld(t, w)
	d, err := DecideRequest(context.Background(), "req1", "approved", "op1", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	if d.Approval.Effect != EffectExtend || w.approved.endedBefore == nil || !w.approved.endedBefore.Equal(until) ||
		w.approved.endsAfter == nil || !w.approved.endsAfter.After(until) || w.approved.own.EndRule != db.EndSet {
		t.Fatalf("effect %s, ended before %v, ends after %v, end rule %s", d.Approval.Effect, w.approved.endedBefore, w.approved.endsAfter, w.approved.own.EndRule)
	}
}

func TestDecideRequest_DecliningAnExtensionRemembersTheEndItLeft(t *testing.T) {
	until := time.Now().Add(72 * time.Hour)
	w := &decisionWorld{request: pendingRequest(RequestKindExtend), holding: models.RequestHolding{Kind: models.HoldingUntil, Until: &until}}
	stubDecisionWorld(t, w)
	if _, err := DecideRequest(context.Background(), "req1", "rejected", "op1", "", nil); err != nil {
		t.Fatal(err)
	}
	if w.declined.calls != 1 || w.declined.endedBefore == nil || !w.declined.endedBefore.Equal(until) || w.approved.calls != 0 {
		t.Fatalf("declined %d with %v; approved %d", w.declined.calls, w.declined.endedBefore, w.approved.calls)
	}
	if len(w.audit) != 1 || w.audit[0] != "access_request.rejected" {
		t.Fatalf("audit = %v", w.audit)
	}

	// A failed read never stops a decline: it only leaves the end unsaid.
	svcRequestHolding = func(context.Context, string, string, string) (models.RequestHolding, error) {
		return models.RequestHolding{}, errors.New("db down")
	}
	if _, err := DecideRequest(context.Background(), "req1", "rejected", "op1", "", nil); err != nil || w.declined.endedBefore != nil {
		t.Fatalf("decline with an unread holding: %v, end %v", err, w.declined.endedBefore)
	}
}

func TestDecideRequest_RefusalsKeepTheirWords(t *testing.T) {
	for name, tc := range map[string]struct {
		status  string
		holding models.RequestHolding
		want    error
	}{
		"settled before (withdrawn)": {status: "withdrawn", holding: models.RequestHolding{Kind: models.HoldingNone}, want: ErrRequestAlreadyResolved},
		"held since asking":          {status: "pending", holding: models.RequestHolding{Kind: models.HoldingForGood}, want: ErrRequestAlreadyHeld},
	} {
		t.Run(name, func(t *testing.T) {
			r := pendingRequest(RequestKindAccess)
			r.Status = tc.status
			w := &decisionWorld{request: r, holding: tc.holding}
			stubDecisionWorld(t, w)
			if _, err := DecideRequest(context.Background(), "req1", "approved", "op1", "", nil); !errors.Is(err, tc.want) {
				t.Fatalf("err = %v, want %v", err, tc.want)
			}
			if w.approved.calls != 0 || len(w.audit) != 0 {
				t.Fatal("a refused decision writes nothing")
			}
		})
	}
}

// With what the reviewer was shown, an approval whose Change moved is stale
// and writes nothing (decision 3).
func TestDecideRequest_AStaleApprovalWritesNothing(t *testing.T) {
	w := &decisionWorld{request: pendingRequest(RequestKindAccess), holding: models.RequestHolding{Kind: models.HoldingNone}}
	stubDecisionWorld(t, w)
	_, err := DecideRequest(context.Background(), "req1", "approved", "op1", "", &Seen{Fingerprint: "an older plan"})
	var stale *StalePlan
	if !errors.As(err, &stale) || w.approved.calls != 0 || len(w.audit) != 0 {
		t.Fatalf("err = %v, approved %d, audit %v", err, w.approved.calls, w.audit)
	}
}
