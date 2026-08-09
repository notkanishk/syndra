package expiry

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"syndra/internal/db"
	"syndra/internal/models"
	"syndra/internal/services"
)

// resetSweepDeps saves and restores all injectable function vars so tests
// are order-independent.
func resetSweepDeps(t *testing.T) {
	t.Helper()
	origGet := svcGetExpiredDirectGrants
	origExpire := svcExpireDirectGrant
	origEmit := svcEmitIntentFromScheduler
	origCache := cacheInvalidateUser
	t.Cleanup(func() {
		svcGetExpiredDirectGrants = origGet
		svcExpireDirectGrant = origExpire
		svcEmitIntentFromScheduler = origEmit
		cacheInvalidateUser = origCache
	})
}

// recorder captures what the sweep invoked, for assertions.
type recorder struct {
	mu sync.Mutex
	// expiredCalls records "user|grant|project|role" in call order — the whole
	// argument list, because a sweep that expires the right grant against the
	// wrong user's holdings is the bug this ordering exists to prevent.
	expiredCalls   []string
	emitted        []string // grantIDs in order of emit call
	emitActions    map[string]string
	invalidatedFor []string // userIDs in order
	// outcome decides, per grant id, what expiring it does.
	outcome map[string]error
}

func newRecorder() *recorder {
	return &recorder{emitActions: map[string]string{}, outcome: map[string]error{}}
}

func (r *recorder) install() {
	svcExpireDirectGrant = func(_ context.Context, userID, grantID, projectID, role, actor string) (services.ExpiredGrantRevocation, error) {
		r.mu.Lock()
		defer r.mu.Unlock()
		r.expiredCalls = append(r.expiredCalls, userID+"|"+grantID+"|"+projectID+"|"+role)
		if err := r.outcome[grantID]; err != nil {
			return services.ExpiredGrantRevocation{}, err
		}
		return services.ExpiredGrantRevocation{
			ProjectID: projectID, RoleKey: role,
			OutboxIDs: []string{"ob-" + grantID},
			Revoked:   []string{projectID + "/" + role},
		}, nil
	}
	svcEmitIntentFromScheduler = func(_ context.Context, uid, action, proj, role, grantID string) error {
		r.mu.Lock()
		defer r.mu.Unlock()
		r.emitted = append(r.emitted, grantID)
		r.emitActions[grantID] = action
		return nil
	}
	cacheInvalidateUser = func(_ context.Context, uid string) error {
		r.mu.Lock()
		defer r.mu.Unlock()
		r.invalidatedFor = append(r.invalidatedFor, uid)
		return nil
	}
}

func mkGrant(id, user, project, role string) models.DirectGrant {
	now := time.Now()
	expiredAt := now.Add(-1 * time.Hour)
	return models.DirectGrant{
		ID: id, UserID: user, ProjectID: project, RoleKey: role,
		GrantedBy: "admin", ExpiresAt: &expiredAt, CreatedAt: now, UpdatedAt: now,
	}
}

func fetchReturns(grants ...models.DirectGrant) {
	svcGetExpiredDirectGrants = func(context.Context, int) ([]models.DirectGrant, error) {
		return grants, nil
	}
}

func TestSweep_NoExpired_NoOp(t *testing.T) {
	resetSweepDeps(t)
	r := newRecorder()
	r.install()
	fetchReturns()

	Sweep(context.Background(), 100)

	if len(r.expiredCalls) != 0 || len(r.emitted) != 0 || len(r.invalidatedFor) != 0 {
		t.Fatalf("nothing expired; nothing may happen: %+v", r)
	}
}

func TestSweep_SingleExpired_FullFlow(t *testing.T) {
	resetSweepDeps(t)
	r := newRecorder()
	r.install()
	fetchReturns(mkGrant("g1", "u1", "proj-a", "role-x"))

	Sweep(context.Background(), 100)

	if len(r.expiredCalls) != 1 || r.expiredCalls[0] != "u1|g1|proj-a|role-x" {
		t.Fatalf("the grant must be expired with its own identifiers, got %v", r.expiredCalls)
	}
	if len(r.emitted) != 1 || r.emitted[0] != "g1" {
		t.Fatalf("the provisioning intent must name the grant, got %v", r.emitted)
	}
	if r.emitActions["g1"] != "remove" {
		t.Errorf("intent action = %q, want remove", r.emitActions["g1"])
	}
	if len(r.invalidatedFor) != 1 || r.invalidatedFor[0] != "u1" {
		t.Fatalf("the user's cache must be invalidated once, got %v", r.invalidatedFor)
	}
}

// The critical race: a renewal landing between the fetch and the write.
// UpsertDirectGrant reuses the row id via ON CONFLICT DO UPDATE, so the
// snapshot cannot be trusted; the delete's own predicate is what decides, and
// it reports a renewed grant as such. Nothing downstream may run for it.
func TestSweep_GrantRenewedMidSweep_NoDownstreamWork(t *testing.T) {
	resetSweepDeps(t)
	r := newRecorder()
	r.install()
	r.outcome["g1"] = db.ErrGrantRenewed
	fetchReturns(mkGrant("g1", "u1", "proj-a", "role-x"))

	Sweep(context.Background(), 100)

	if len(r.emitted) != 0 {
		t.Fatalf("a renewed grant must emit no removal intent, got %v", r.emitted)
	}
	// The cache is still invalidated: it is per-user and cheap, and a rebuild
	// that finds nothing changed costs nothing. What must not happen is a
	// removal.
	if len(r.expiredCalls) != 1 {
		t.Fatalf("the write is what decides, so it must still be attempted: %v", r.expiredCalls)
	}
}

// Of several candidates for one user, a subset was renewed. Only the rest
// progress, and one renewal does not stop the others.
func TestSweep_PartialRenewal_OnlyTheLapsedProgress(t *testing.T) {
	resetSweepDeps(t)
	r := newRecorder()
	r.install()
	r.outcome["g2"] = db.ErrGrantRenewed
	fetchReturns(
		mkGrant("g1", "u1", "proj-a", "role-x"),
		mkGrant("g2", "u1", "proj-a", "role-y"),
		mkGrant("g3", "u1", "proj-a", "role-z"),
	)

	Sweep(context.Background(), 100)

	if len(r.expiredCalls) != 3 {
		t.Fatalf("every candidate must be attempted, got %v", r.expiredCalls)
	}
	if len(r.emitted) != 2 || contains(r.emitted, "g2") {
		t.Fatalf("only the still-lapsed grants progress, got %v", r.emitted)
	}
}

// One grant's failure must not cost the next one its expiry.
func TestSweep_OneFailure_DoesNotStopTheRest(t *testing.T) {
	resetSweepDeps(t)
	r := newRecorder()
	r.install()
	r.outcome["g1"] = errors.New("database unreachable")
	fetchReturns(
		mkGrant("g1", "u1", "proj-a", "role-x"),
		mkGrant("g2", "u1", "proj-a", "role-y"),
	)

	Sweep(context.Background(), 100)

	if len(r.emitted) != 1 || r.emitted[0] != "g2" {
		t.Fatalf("the healthy grant must still expire, got %v", r.emitted)
	}
}

// A failed expiry queues nothing and removes nothing — the grant stands.
func TestSweep_ExpiryFails_NoSideEffects(t *testing.T) {
	resetSweepDeps(t)
	r := newRecorder()
	r.install()
	r.outcome["g1"] = errors.New("boom")
	fetchReturns(mkGrant("g1", "u1", "proj-a", "role-x"))

	Sweep(context.Background(), 100)

	if len(r.emitted) != 0 {
		t.Fatalf("a failed expiry must emit no intent, got %v", r.emitted)
	}
}

// The ledger delete, audit row and revocations commit together. The LLDAP
// intent is a different system's queue and cannot join that transaction, so its
// failure leaves a group membership behind — not an access decision half-made.
func TestSweep_IntentFailsAfterExpiry_CacheStillInvalidated(t *testing.T) {
	resetSweepDeps(t)
	r := newRecorder()
	r.install()
	svcEmitIntentFromScheduler = func(context.Context, string, string, string, string, string) error {
		return errors.New("sync service down")
	}
	fetchReturns(mkGrant("g1", "u1", "proj-a", "role-x"))

	Sweep(context.Background(), 100)

	if len(r.invalidatedFor) != 1 {
		t.Fatalf("the expiry committed; the cache must still be invalidated, got %v", r.invalidatedFor)
	}
}

func TestSweep_MultiUser_OneInvalidateEach(t *testing.T) {
	resetSweepDeps(t)
	r := newRecorder()
	r.install()
	fetchReturns(
		mkGrant("g1", "u1", "proj-a", "role-x"),
		mkGrant("g2", "u1", "proj-a", "role-y"),
		mkGrant("g3", "u2", "proj-b", "role-z"),
	)

	Sweep(context.Background(), 100)

	counts := map[string]int{}
	for _, u := range r.invalidatedFor {
		counts[u]++
	}
	if counts["u1"] != 1 || counts["u2"] != 1 {
		t.Fatalf("exactly one invalidation per user, got %v", counts)
	}
}

// Each grant is expired against its own user's holdings. A cross-user bleed
// here would revoke access from somebody whose grant never expired.
func TestSweep_NoCrossUserBleed(t *testing.T) {
	resetSweepDeps(t)
	r := newRecorder()
	r.install()
	fetchReturns(
		mkGrant("g1", "u1", "proj-a", "role-x"),
		mkGrant("g2", "u2", "proj-b", "role-y"),
	)

	Sweep(context.Background(), 100)

	for _, call := range r.expiredCalls {
		switch call {
		case "u1|g1|proj-a|role-x", "u2|g2|proj-b|role-y":
		default:
			t.Errorf("a grant was expired against the wrong user or role: %q", call)
		}
	}
}

// Each grant is expired sequentially so its delta is computed against the state
// the previous one left. Two grants deriving the same role would otherwise each
// see the other still covering it, and neither would revoke.
func TestSweep_ExpiresOneGrantAtATime(t *testing.T) {
	resetSweepDeps(t)
	r := newRecorder()
	r.install()

	inFlight := 0
	svcExpireDirectGrant = func(_ context.Context, userID, grantID, projectID, role, _ string) (services.ExpiredGrantRevocation, error) {
		r.mu.Lock()
		inFlight++
		if inFlight > 1 {
			r.mu.Unlock()
			t.Error("two expiries overlapped; each delta must see the previous one's result")
			return services.ExpiredGrantRevocation{}, nil
		}
		r.expiredCalls = append(r.expiredCalls, userID+"|"+grantID+"|"+projectID+"|"+role)
		inFlight--
		r.mu.Unlock()
		return services.ExpiredGrantRevocation{ProjectID: projectID, RoleKey: role}, nil
	}
	fetchReturns(
		mkGrant("g1", "u1", "proj-a", "role-x"),
		mkGrant("g2", "u1", "proj-a", "role-y"),
	)

	Sweep(context.Background(), 100)

	if len(r.expiredCalls) != 2 {
		t.Fatalf("both grants must be expired, got %v", r.expiredCalls)
	}
}

func TestSweep_BatchSizeRespected(t *testing.T) {
	resetSweepDeps(t)
	r := newRecorder()
	r.install()
	var got int
	svcGetExpiredDirectGrants = func(_ context.Context, limit int) ([]models.DirectGrant, error) {
		got = limit
		return nil, nil
	}

	Sweep(context.Background(), 42)

	if got != 42 {
		t.Fatalf("batch size = %d, want 42", got)
	}
}

// A misconfigured batch is clamped before it reaches the fetch: zero would
// sweep nothing forever, unbounded defeats the batching.
func TestSweep_ClampsBatchSize(t *testing.T) {
	resetSweepDeps(t)
	r := newRecorder()
	r.install()
	var got int
	svcGetExpiredDirectGrants = func(_ context.Context, limit int) ([]models.DirectGrant, error) {
		got = limit
		return nil, nil
	}

	for _, tc := range []struct{ in, want int }{{0, 1}, {-5, 1}, {99999, 10000}} {
		Sweep(context.Background(), tc.in)
		if got != tc.want {
			t.Errorf("batch %d clamped to %d, want %d", tc.in, got, tc.want)
		}
	}
}

// Re-granting after an expiry produces a fresh grant id, so the scheduler's
// grantID-discriminated idempotency key cannot collide with the earlier one.
func TestSweep_IntentIdempotencyAcrossReGrants(t *testing.T) {
	resetSweepDeps(t)
	r := newRecorder()
	r.install()

	fetchReturns(mkGrant("g1", "u1", "proj-a", "role-x"))
	Sweep(context.Background(), 100)
	fetchReturns(mkGrant("g2", "u1", "proj-a", "role-x")) // re-granted, new row
	Sweep(context.Background(), 100)

	if len(r.emitted) != 2 || r.emitted[0] == r.emitted[1] {
		t.Fatalf("each sweep must emit against its own grant id, got %v", r.emitted)
	}
}

// Cancellation midway through ONE user's grants stops there too. The outer
// loop's check only guards the boundary between users, so a user with a long
// list would keep expiring grants through a shutdown that had already been
// asked for — and each one commits.
func TestSweep_ContextCancelledMidUser_StopsThere(t *testing.T) {
	resetSweepDeps(t)
	r := newRecorder()
	r.install()
	ctx, cancel := context.WithCancel(context.Background())

	inner := svcExpireDirectGrant
	svcExpireDirectGrant = func(c context.Context, userID, grantID, projectID, role, actor string) (services.ExpiredGrantRevocation, error) {
		cancel() // the shutdown lands while this user still has grants queued
		return inner(c, userID, grantID, projectID, role, actor)
	}
	fetchReturns(
		mkGrant("g1", "u1", "proj-a", "role-x"),
		mkGrant("g2", "u1", "proj-a", "role-y"),
		mkGrant("g3", "u1", "proj-a", "role-z"),
	)

	Sweep(ctx, 100)

	if len(r.expiredCalls) != 1 {
		t.Fatalf("the sweep must stop within the user, not finish their list: %v", r.expiredCalls)
	}
}

// A cancelled context stops the sweep rather than working through the rest.
func TestSweep_ContextCancelled_StopsEarly(t *testing.T) {
	resetSweepDeps(t)
	r := newRecorder()
	r.install()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	fetchReturns(mkGrant("g1", "u1", "proj-a", "role-x"))

	Sweep(ctx, 100)

	if len(r.expiredCalls) != 0 {
		t.Fatalf("a cancelled sweep must expire nothing, got %v", r.expiredCalls)
	}
}

func contains(hay []string, needle string) bool {
	for _, h := range hay {
		if h == needle {
			return true
		}
	}
	return false
}
