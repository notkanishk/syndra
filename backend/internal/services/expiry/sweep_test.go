package expiry

import (
	"context"
	"errors"
	"sort"
	"sync"
	"testing"
	"time"

	"syndra/internal/models"
)

// resetSweepDeps saves and restores all injectable function vars so tests
// are order-independent.
func resetSweepDeps(t *testing.T) {
	t.Helper()
	origGet := svcGetExpiredDirectGrants
	origDel := svcDeleteExpiredDirectGrantsByIDs
	origEmit := svcEmitIntentFromScheduler
	origAudit := svcInsertAuditLog
	origCache := cacheInvalidateUser
	origZit := zitadelRevokeMappingRules
	t.Cleanup(func() {
		svcGetExpiredDirectGrants = origGet
		svcDeleteExpiredDirectGrantsByIDs = origDel
		svcEmitIntentFromScheduler = origEmit
		svcInsertAuditLog = origAudit
		cacheInvalidateUser = origCache
		zitadelRevokeMappingRules = origZit
	})
}

// recorder captures what sweep invoked, for assertions.
type recorder struct {
	mu             sync.Mutex
	emitted        []string // grantIDs in order of emit call
	emitActions    map[string]string
	deletedUser    string
	deletedIDs     []string
	audited        []string // grantIDs
	invalidatedFor []string // userIDs in order
	zitadelCalls   []string // "userID|project|role"
}

func newRecorder() *recorder {
	return &recorder{emitActions: map[string]string{}}
}

// installDefaultDelete makes the guarded delete return every input ID as
// successfully deleted. Tests that need a different renewal model override
// svcDeleteExpiredDirectGrantsByIDs after calling install().
func (r *recorder) install() {
	svcEmitIntentFromScheduler = func(_ context.Context, uid, action, proj, role, grantID string) error {
		r.mu.Lock()
		defer r.mu.Unlock()
		r.emitted = append(r.emitted, grantID)
		r.emitActions[grantID] = action
		return nil
	}
	svcDeleteExpiredDirectGrantsByIDs = func(_ context.Context, uid string, ids []string) ([]models.DirectGrant, error) {
		r.mu.Lock()
		defer r.mu.Unlock()
		r.deletedUser = uid
		r.deletedIDs = append(r.deletedIDs, ids...)
		out := make([]models.DirectGrant, len(ids))
		for i, id := range ids {
			out[i] = mkGrant(id, uid, "proj-a", "role-x")
		}
		return out, nil
	}
	svcInsertAuditLog = func(_ context.Context, _, _, _, resourceID string) error {
		r.mu.Lock()
		defer r.mu.Unlock()
		r.audited = append(r.audited, resourceID)
		return nil
	}
	cacheInvalidateUser = func(_ context.Context, uid string) error {
		r.mu.Lock()
		defer r.mu.Unlock()
		r.invalidatedFor = append(r.invalidatedFor, uid)
		return nil
	}
	zitadelRevokeMappingRules = func(_ context.Context, uid, proj, role string) error {
		r.mu.Lock()
		defer r.mu.Unlock()
		r.zitadelCalls = append(r.zitadelCalls, uid+"|"+proj+"|"+role)
		return nil
	}
}

func mkGrant(id, user, project, role string) models.DirectGrant {
	now := time.Now()
	expired := now.Add(-1 * time.Hour)
	return models.DirectGrant{
		ID: id, UserID: user, ProjectID: project, RoleKey: role,
		GrantedBy: "admin", ExpiresAt: &expired, CreatedAt: now, UpdatedAt: now,
	}
}

func TestSweep_NoExpired_NoOp(t *testing.T) {
	resetSweepDeps(t)
	r := newRecorder()
	r.install()
	svcGetExpiredDirectGrants = func(_ context.Context, _ int) ([]models.DirectGrant, error) {
		return nil, nil
	}

	Sweep(context.Background(), 10)

	if len(r.emitted)+len(r.deletedIDs)+len(r.audited)+len(r.invalidatedFor)+len(r.zitadelCalls) != 0 {
		t.Fatalf("expected no side effects, got emitted=%d deleted=%d audited=%d inv=%d zit=%d",
			len(r.emitted), len(r.deletedIDs), len(r.audited), len(r.invalidatedFor), len(r.zitadelCalls))
	}
}

func TestSweep_SingleExpired_FullFlow(t *testing.T) {
	resetSweepDeps(t)
	r := newRecorder()
	r.install()
	g := mkGrant("g1", "user-1", "proj-a", "role-x")
	svcGetExpiredDirectGrants = func(_ context.Context, _ int) ([]models.DirectGrant, error) {
		return []models.DirectGrant{g}, nil
	}
	svcDeleteExpiredDirectGrantsByIDs = func(_ context.Context, uid string, ids []string) ([]models.DirectGrant, error) {
		return []models.DirectGrant{g}, nil
	}

	Sweep(context.Background(), 10)

	if len(r.audited) != 1 || r.audited[0] != "g1" {
		t.Fatalf("expected audit for g1, got %v", r.audited)
	}
	if len(r.emitted) != 1 || r.emitted[0] != "g1" {
		t.Fatalf("expected one emit for g1, got %v", r.emitted)
	}
	if r.emitActions["g1"] != "remove" {
		t.Fatalf("expected action=remove, got %q", r.emitActions["g1"])
	}
	if len(r.invalidatedFor) != 1 || r.invalidatedFor[0] != "user-1" {
		t.Fatalf("expected one cache invalidate for user-1, got %v", r.invalidatedFor)
	}
	if len(r.zitadelCalls) != 1 || r.zitadelCalls[0] != "user-1|proj-a|role-x" {
		t.Fatalf("expected one zitadel call, got %v", r.zitadelCalls)
	}
}

// P1 regression: the critical race is a renewal landing between fetch and
// delete. UpsertDirectGrant reuses the row ID via ON CONFLICT DO UPDATE, so
// the pre-fetch snapshot cannot be trusted. DeleteExpiredDirectGrantsByIDs
// re-validates expires_at <= NOW() atomically; the sweep must drive every
// downstream side-effect off the RETURNING set only.
func TestSweep_GrantRenewedMidSweep_NotRevoked(t *testing.T) {
	resetSweepDeps(t)
	r := newRecorder()
	r.install()
	g := mkGrant("g1", "user-1", "proj-a", "role-x")
	svcGetExpiredDirectGrants = func(_ context.Context, _ int) ([]models.DirectGrant, error) {
		return []models.DirectGrant{g}, nil
	}
	// Simulate: the row was concurrently renewed after fetch; the guarded
	// DELETE matches zero rows and returns an empty slice.
	svcDeleteExpiredDirectGrantsByIDs = func(_ context.Context, _ string, _ []string) ([]models.DirectGrant, error) {
		return nil, nil
	}

	Sweep(context.Background(), 10)

	if len(r.audited) != 0 {
		t.Fatalf("audit MUST NOT be written for a renewed grant, got %v", r.audited)
	}
	if len(r.emitted) != 0 {
		t.Fatalf("intent MUST NOT be emitted for a renewed grant, got %v", r.emitted)
	}
	if len(r.invalidatedFor) != 0 {
		t.Fatalf("cache MUST NOT be invalidated for a no-op sweep, got %v", r.invalidatedFor)
	}
	if len(r.zitadelCalls) != 0 {
		t.Fatalf("Zitadel cascade MUST NOT fire for a renewed grant, got %v", r.zitadelCalls)
	}
}

// Partial renewal: of N candidates for one user, a subset was renewed and
// only the still-expired subset is returned from the guarded delete. Only
// those must flow through the post-delete pipeline.
func TestSweep_PartialRenewal_OnlyActuallyDeletedProgressDownstream(t *testing.T) {
	resetSweepDeps(t)
	r := newRecorder()
	r.install()
	g1 := mkGrant("g1", "user-1", "proj-a", "role-x")
	g2Renewed := mkGrant("g2", "user-1", "proj-a", "role-y")
	svcGetExpiredDirectGrants = func(_ context.Context, _ int) ([]models.DirectGrant, error) {
		return []models.DirectGrant{g1, g2Renewed}, nil
	}
	// DB says only g1 was still expired at delete time.
	svcDeleteExpiredDirectGrantsByIDs = func(_ context.Context, _ string, _ []string) ([]models.DirectGrant, error) {
		return []models.DirectGrant{g1}, nil
	}

	Sweep(context.Background(), 10)

	if len(r.audited) != 1 || r.audited[0] != "g1" {
		t.Fatalf("expected audit for g1 only, got %v", r.audited)
	}
	if len(r.emitted) != 1 || r.emitted[0] != "g1" {
		t.Fatalf("expected intent for g1 only, got %v", r.emitted)
	}
	if len(r.zitadelCalls) != 1 || r.zitadelCalls[0] != "user-1|proj-a|role-x" {
		t.Fatalf("expected one zitadel call for g1's (project, role), got %v", r.zitadelCalls)
	}
}

func TestSweep_MultiUser_OneInvalidateEach(t *testing.T) {
	resetSweepDeps(t)
	r := newRecorder()
	r.install()
	grants := []models.DirectGrant{
		mkGrant("g1", "user-1", "proj-a", "role-x"),
		mkGrant("g2", "user-1", "proj-a", "role-y"),
		mkGrant("g3", "user-2", "proj-b", "role-z"),
	}
	svcGetExpiredDirectGrants = func(_ context.Context, _ int) ([]models.DirectGrant, error) {
		return grants, nil
	}
	// Delete returns the full set per user (no renewals).
	svcDeleteExpiredDirectGrantsByIDs = func(_ context.Context, uid string, ids []string) ([]models.DirectGrant, error) {
		out := make([]models.DirectGrant, 0, len(ids))
		for _, id := range ids {
			for _, g := range grants {
				if g.ID == id {
					out = append(out, g)
				}
			}
		}
		return out, nil
	}

	Sweep(context.Background(), 10)

	if len(r.invalidatedFor) != 2 {
		t.Fatalf("expected 2 invalidates (one per user), got %d: %v", len(r.invalidatedFor), r.invalidatedFor)
	}
	if len(r.audited) != 3 {
		t.Fatalf("expected 3 audit rows, got %d", len(r.audited))
	}
	if len(r.emitted) != 3 {
		t.Fatalf("expected 3 emits, got %d", len(r.emitted))
	}
	if len(r.zitadelCalls) != 3 {
		t.Fatalf("expected 3 zitadel calls, got %d: %v", len(r.zitadelCalls), r.zitadelCalls)
	}
}

func TestSweep_DeleteFails_NoSideEffects(t *testing.T) {
	resetSweepDeps(t)
	r := newRecorder()
	r.install()
	g := mkGrant("g1", "user-1", "proj-a", "role-x")
	svcGetExpiredDirectGrants = func(_ context.Context, _ int) ([]models.DirectGrant, error) {
		return []models.DirectGrant{g}, nil
	}
	svcDeleteExpiredDirectGrantsByIDs = func(_ context.Context, _ string, _ []string) ([]models.DirectGrant, error) {
		return nil, errors.New("db down")
	}

	Sweep(context.Background(), 10)

	if len(r.audited) != 0 || len(r.emitted) != 0 || len(r.invalidatedFor) != 0 || len(r.zitadelCalls) != 0 {
		t.Fatalf("no side effects must happen when delete fails; got audited=%d emitted=%d inv=%d zit=%d",
			len(r.audited), len(r.emitted), len(r.invalidatedFor), len(r.zitadelCalls))
	}
}

// After the delete commits, an intent-emission failure must not roll back
// the authoritative deletion, the audit row, or the cache invalidation.
// This is the log-and-continue compromise the Phase-5 deferred "Partial
// Failure Rollback" item acknowledges: LLDAP orphans are preferable to a
// rollback we cannot execute atomically anyway, and the future reconciler
// will reap them.
func TestSweep_IntentFailsAfterDelete_AuditAndCacheStillLand(t *testing.T) {
	resetSweepDeps(t)
	r := newRecorder()
	r.install()
	g := mkGrant("g1", "user-1", "proj-a", "role-x")
	svcGetExpiredDirectGrants = func(_ context.Context, _ int) ([]models.DirectGrant, error) {
		return []models.DirectGrant{g}, nil
	}
	svcDeleteExpiredDirectGrantsByIDs = func(_ context.Context, _ string, _ []string) ([]models.DirectGrant, error) {
		return []models.DirectGrant{g}, nil
	}
	svcEmitIntentFromScheduler = func(_ context.Context, _, _, _, _, _ string) error {
		return errors.New("lldap down")
	}

	Sweep(context.Background(), 10)

	if len(r.audited) != 1 {
		t.Fatalf("audit MUST land after successful delete even if intent emit later fails, got %d", len(r.audited))
	}
	if len(r.invalidatedFor) != 1 {
		t.Fatalf("cache invalidate MUST still run, got %d", len(r.invalidatedFor))
	}
	if len(r.zitadelCalls) != 1 {
		t.Fatalf("zitadel cascade MUST still run, got %d", len(r.zitadelCalls))
	}
}

func TestSweep_ZitadelFails_OtherStepsSucceed(t *testing.T) {
	resetSweepDeps(t)
	r := newRecorder()
	r.install()
	g := mkGrant("g1", "user-1", "proj-a", "role-x")
	svcGetExpiredDirectGrants = func(_ context.Context, _ int) ([]models.DirectGrant, error) {
		return []models.DirectGrant{g}, nil
	}
	svcDeleteExpiredDirectGrantsByIDs = func(_ context.Context, _ string, _ []string) ([]models.DirectGrant, error) {
		return []models.DirectGrant{g}, nil
	}
	zitadelRevokeMappingRules = func(_ context.Context, _, _, _ string) error {
		return errors.New("zitadel unreachable")
	}

	Sweep(context.Background(), 10)

	if len(r.audited) != 1 || len(r.emitted) != 1 || len(r.invalidatedFor) != 1 {
		t.Fatalf("zitadel failure must not roll back earlier steps; got audited=%d emitted=%d inv=%d",
			len(r.audited), len(r.emitted), len(r.invalidatedFor))
	}
}

func TestSweep_BatchSizeRespected(t *testing.T) {
	resetSweepDeps(t)
	r := newRecorder()
	r.install()

	var capturedLimit int
	grants := make([]models.DirectGrant, 500)
	for i := range grants {
		grants[i] = mkGrant("g", "user-x", "proj", "role")
	}
	svcGetExpiredDirectGrants = func(_ context.Context, limit int) ([]models.DirectGrant, error) {
		capturedLimit = limit
		return grants[:limit], nil
	}
	svcDeleteExpiredDirectGrantsByIDs = func(_ context.Context, _ string, ids []string) ([]models.DirectGrant, error) {
		out := make([]models.DirectGrant, len(ids))
		for i := range out {
			out[i] = mkGrant(ids[i], "user-x", "proj", "role")
		}
		return out, nil
	}

	Sweep(context.Background(), 500)

	if capturedLimit != 500 {
		t.Fatalf("expected limit=500 forwarded to DB, got %d", capturedLimit)
	}
	if len(r.emitted) != 500 {
		t.Fatalf("expected 500 emits, got %d", len(r.emitted))
	}
}

// Protects Bug Fix #3 (idempotency-key discriminator). With the P1 fix,
// each successful sweep produces a fresh grant ID; re-grants get new IDs
// so their scheduler-origin idempotency keys differ.
func TestSweep_IntentIdempotencyAcrossReGrants(t *testing.T) {
	resetSweepDeps(t)
	r := newRecorder()
	r.install()

	var seenGrantIDs []string
	svcEmitIntentFromScheduler = func(_ context.Context, _, _, _, _, grantID string) error {
		seenGrantIDs = append(seenGrantIDs, grantID)
		return nil
	}

	g1 := mkGrant("grant-v1", "user-1", "proj-a", "role-x")
	svcGetExpiredDirectGrants = func(_ context.Context, _ int) ([]models.DirectGrant, error) {
		return []models.DirectGrant{g1}, nil
	}
	svcDeleteExpiredDirectGrantsByIDs = func(_ context.Context, _ string, _ []string) ([]models.DirectGrant, error) {
		return []models.DirectGrant{g1}, nil
	}
	Sweep(context.Background(), 10)

	g2 := mkGrant("grant-v2", "user-1", "proj-a", "role-x")
	svcGetExpiredDirectGrants = func(_ context.Context, _ int) ([]models.DirectGrant, error) {
		return []models.DirectGrant{g2}, nil
	}
	svcDeleteExpiredDirectGrantsByIDs = func(_ context.Context, _ string, _ []string) ([]models.DirectGrant, error) {
		return []models.DirectGrant{g2}, nil
	}
	Sweep(context.Background(), 10)

	if len(seenGrantIDs) != 2 {
		t.Fatalf("expected 2 emit calls, got %d", len(seenGrantIDs))
	}
	if seenGrantIDs[0] == seenGrantIDs[1] {
		t.Fatalf("grant IDs must differ so idempotency keys differ; both were %q", seenGrantIDs[0])
	}
}

func TestSweep_UserScopedDelete_NoCrossUserBleed(t *testing.T) {
	resetSweepDeps(t)
	r := newRecorder()
	r.install()

	type deleteCall struct {
		user string
		ids  []string
	}
	var deletes []deleteCall
	svcDeleteExpiredDirectGrantsByIDs = func(_ context.Context, uid string, ids []string) ([]models.DirectGrant, error) {
		deletes = append(deletes, deleteCall{user: uid, ids: append([]string{}, ids...)})
		out := make([]models.DirectGrant, len(ids))
		for i, id := range ids {
			out[i] = mkGrant(id, uid, "proj", "role")
		}
		return out, nil
	}
	svcGetExpiredDirectGrants = func(_ context.Context, _ int) ([]models.DirectGrant, error) {
		return []models.DirectGrant{
			mkGrant("g1", "user-1", "proj-a", "role-x"),
			mkGrant("g2", "user-2", "proj-b", "role-y"),
		}, nil
	}

	Sweep(context.Background(), 10)

	if len(deletes) != 2 {
		t.Fatalf("expected 2 user-scoped delete calls, got %d", len(deletes))
	}
	for _, d := range deletes {
		for _, id := range d.ids {
			if d.user == "user-1" && id != "g1" {
				t.Fatalf("user-1 delete contained %q; cross-user bleed", id)
			}
			if d.user == "user-2" && id != "g2" {
				t.Fatalf("user-2 delete contained %q; cross-user bleed", id)
			}
		}
	}
}

func TestSweep_ZitadelDedupPerProjectRole(t *testing.T) {
	resetSweepDeps(t)
	r := newRecorder()
	r.install()

	// Two grants for the same user with the same (project, role) — defense
	// in depth against a future relaxation of the unique constraint.
	grants := []models.DirectGrant{
		mkGrant("g1", "user-1", "proj-a", "role-x"),
		mkGrant("g2", "user-1", "proj-a", "role-x"),
		mkGrant("g3", "user-1", "proj-a", "role-y"),
	}
	svcGetExpiredDirectGrants = func(_ context.Context, _ int) ([]models.DirectGrant, error) {
		return grants, nil
	}
	svcDeleteExpiredDirectGrantsByIDs = func(_ context.Context, _ string, _ []string) ([]models.DirectGrant, error) {
		return grants, nil
	}

	Sweep(context.Background(), 10)

	if len(r.zitadelCalls) != 2 {
		t.Fatalf("expected 2 zitadel calls (dedup'd by project|role), got %d: %v",
			len(r.zitadelCalls), r.zitadelCalls)
	}
	sort.Strings(r.zitadelCalls)
	wantPrefix := []string{"user-1|proj-a|role-x", "user-1|proj-a|role-y"}
	for i, w := range wantPrefix {
		if r.zitadelCalls[i] != w {
			t.Fatalf("call %d = %q, want %q", i, r.zitadelCalls[i], w)
		}
	}
}

// Sweep clamps a misconfigured batch size before it reaches the fetch query
// (the clamp moved here from the deleted per-package scheduler).
func TestSweep_ClampsBatchSize(t *testing.T) {
	resetSweepDeps(t)
	var got []int
	svcGetExpiredDirectGrants = func(_ context.Context, limit int) ([]models.DirectGrant, error) {
		got = append(got, limit)
		return nil, nil
	}
	Sweep(context.Background(), 0)
	Sweep(context.Background(), 100000)
	if len(got) != 2 || got[0] != 1 || got[1] != 10000 {
		t.Fatalf("expected clamped batch sizes [1 10000]; got %v", got)
	}
}
