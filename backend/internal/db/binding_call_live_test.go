package db

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
)

// 6.3r against a real Postgres: a recorded adoption or release settles in one
// transaction with the backend's binding half, once, and never over a later
// decision about the same subject's binding.

const bindTarget = "truenas-6-3r-test"

func bindingLive(t *testing.T) context.Context {
	t.Helper()
	ctx := liveDB(t)
	if _, err := PG.Exec(ctx, `INSERT INTO targets (target) VALUES ($1) ON CONFLICT DO NOTHING`, bindTarget); err != nil {
		t.Fatal(err)
	}
	clear := func() {
		for _, q := range []string{
			`DELETE FROM addon_operations WHERE target = $1`,
			`DELETE FROM target_account_bindings WHERE target = $1`,
			`DELETE FROM audit_logs WHERE action LIKE 'addon.' || $1 || '.%'`,
		} {
			if _, err := PG.Exec(ctx, q, bindTarget); err != nil {
				t.Fatal(err)
			}
		}
	}
	clear()
	t.Cleanup(clear)
	return ctx
}

// lostCall is a sent adopt or release whose answer was lost an hour ago.
func lostCall(t *testing.T, ctx context.Context, op, subject, about string) string {
	t.Helper()
	id, err := BeginAddonOperation(ctx, AddonOperationParams{Target: bindTarget, Operation: op, ActorID: "op-lita", SubjectID: subject, About: about})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ClaimAddonOperation(ctx, id, bindTarget, op, subject); err != nil {
		t.Fatal(err)
	}
	if err := SettleAddonOperation(ctx, id, AddonOpIndeterminate); err != nil {
		t.Fatal(err)
	}
	if _, err := PG.Exec(ctx, `UPDATE addon_operations SET claimed_at = NOW() - interval '1 hour', created_at = NOW() - interval '1 hour' WHERE id = $1`, id); err != nil {
		t.Fatal(err)
	}
	return id
}

func resolution(t *testing.T, ctx context.Context, id string) (res, by string) {
	t.Helper()
	var r, b *string
	if err := PG.QueryRow(ctx, `SELECT resolution, resolved_by FROM addon_operations WHERE id = $1`, id).Scan(&r, &b); err != nil {
		t.Fatal(err)
	}
	if r != nil {
		res = *r
	}
	if b != nil {
		by = *b
	}
	return res, by
}

func boundRow(t *testing.T, ctx context.Context, subject string) (TargetBinding, bool) {
	t.Helper()
	b, found, err := GetTargetBinding(ctx, bindTarget, subject)
	if err != nil {
		t.Fatal(err)
	}
	return b, found
}

// aged makes a binding row last written two hours ago, before any lostCall.
func aged(t *testing.T, ctx context.Context, subject string) {
	t.Helper()
	if _, err := PG.Exec(ctx, `UPDATE target_account_bindings SET last_seen_at = NOW() - interval '2 hours', bound_at = NOW() - interval '2 hours'
		WHERE target = $1 AND subject_id = $2`, bindTarget, subject); err != nil {
		t.Fatal(err)
	}
}

func adoptionOf(subject, username, id string) func(context.Context) error {
	return func(ctx context.Context) error {
		return RecordTargetBinding(ctx, TargetBinding{Target: bindTarget, SubjectID: subject, Username: username, BoundBy: "op-lita", AccountID: &id})
	}
}

func releaseOf(subject string) func(context.Context) error {
	return func(ctx context.Context) error {
		if err := ForgetMergeBase(ctx, bindTarget, subject); err != nil {
			return err
		}
		return ForgetTargetBinding(ctx, bindTarget, subject)
	}
}

func TestLiveARecordedBindingCallSettlesWithItsHalfOnce(t *testing.T) {
	ctx := bindingLive(t)
	call := lostCall(t, ctx, "account.adopt", "u-becky", "charlotte")
	runs := 0
	half := func(ctx context.Context) error { runs++; return adoptionOf("u-becky", "charlotte", "200001")(ctx) }
	if _, err := ResolveBindingCall(ctx, bindTarget, call, "200001", AddonUnresolvedGrace, half); err != nil {
		t.Fatal(err)
	}
	if res, by := resolution(t, ctx, call); res != AddonResolutionHappened || by != SyndraReader {
		t.Fatalf("want happened by Syndra's reading, got %q %q", res, by)
	}
	b, found := boundRow(t, ctx, "u-becky")
	if !found || b.Username != "charlotte" || b.AccountID == nil || *b.AccountID != "200001" || b.BoundBy != "op-lita" {
		t.Fatalf("want the binding the adoption writes, got %+v %t", b, found)
	}

	// Twice: the row is answered, so the half does not run again.
	if _, err := ResolveBindingCall(ctx, bindTarget, call, "200001", AddonUnresolvedGrace, half); !errors.Is(err, ErrAddonOperationNotUnresolved) || runs != 1 {
		t.Fatalf("a second settle must change nothing: %v, half ran %d times", err, runs)
	}

	// A release recorded, the binding older than it: the binding goes, with
	// the row's answer.
	aged(t, ctx, "u-becky")
	rel := lostCall(t, ctx, "account.release", "u-becky", "charlotte")
	if _, err := ResolveBindingCall(ctx, bindTarget, rel, "", AddonUnresolvedGrace, releaseOf("u-becky")); err != nil {
		t.Fatal(err)
	}
	if _, found := boundRow(t, ctx, "u-becky"); found {
		t.Fatal("a settled release must forget the binding")
	}
	if res, _ := resolution(t, ctx, rel); res != AddonResolutionHappened {
		t.Fatalf("the release's row must settle, got %q", res)
	}
}

// Settles of one row at once run its half once (the advisory lock, then the
// row lock). Rounds, because a race lost once is luck.
func TestLiveConcurrentSettlesOfOneBindingCallRunItsHalfOnce(t *testing.T) {
	ctx := bindingLive(t)
	for round := range 5 {
		if _, err := PG.Exec(ctx, `DELETE FROM target_account_bindings WHERE target = $1`, bindTarget); err != nil {
			t.Fatal(err)
		}
		call := lostCall(t, ctx, "account.adopt", "u-becky", "charlotte")
		var runs atomic.Int32
		var wg sync.WaitGroup
		start := make(chan struct{})
		for range 8 {
			wg.Add(1)
			go func() {
				defer wg.Done()
				<-start
				_, err := ResolveBindingCall(ctx, bindTarget, call, "200001", AddonUnresolvedGrace, func(ctx context.Context) error {
					runs.Add(1)
					return adoptionOf("u-becky", "charlotte", "200001")(ctx)
				})
				if err != nil && !errors.Is(err, ErrAddonOperationNotUnresolved) {
					t.Error(err)
				}
			}()
		}
		close(start)
		wg.Wait()
		if runs.Load() != 1 {
			t.Fatalf("round %d: the half ran %d times", round, runs.Load())
		}
		if _, err := PG.Exec(ctx, `DELETE FROM addon_operations WHERE id = $1`, call); err != nil {
			t.Fatal(err)
		}
	}
}

// One transaction: a half that fails leaves no binding and the row open, and
// a resolve that cannot happen takes the half's write back with it.
func TestLiveABindingHalfAndItsSettleCommitTogether(t *testing.T) {
	ctx := bindingLive(t)
	write := adoptionOf("u-kofi", "kofi", "200002")

	call := lostCall(t, ctx, "account.adopt", "u-kofi", "kofi")
	boom := errors.New("half failed")
	if _, err := ResolveBindingCall(ctx, bindTarget, call, "200002", AddonUnresolvedGrace, func(ctx context.Context) error {
		if err := write(ctx); err != nil {
			return err
		}
		return boom
	}); !errors.Is(err, boom) {
		t.Fatalf("want the half's error, got %v", err)
	}
	if _, found := boundRow(t, ctx, "u-kofi"); found {
		t.Fatal("a failed settle left the binding written")
	}
	if res, _ := resolution(t, ctx, call); res != "" {
		t.Fatalf("a failed settle answered the row: %q", res)
	}

	// Still in flight (dispatching inside the grace): the resolve refuses, and
	// the half's write rolls back with it.
	young, err := BeginAddonOperation(ctx, AddonOperationParams{Target: bindTarget, Operation: "account.adopt", ActorID: "op-lita", SubjectID: "u-kofi", About: "kofi"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ClaimAddonOperation(ctx, young, bindTarget, "account.adopt", "u-kofi"); err != nil {
		t.Fatal(err)
	}
	if _, err := ResolveBindingCall(ctx, bindTarget, young, "200002", AddonUnresolvedGrace, write); !errors.Is(err, ErrAddonOperationNotUnresolved) {
		t.Fatalf("a call in flight must not settle: %v", err)
	}
	if _, found := boundRow(t, ctx, "u-kofi"); found {
		t.Fatal("the half's write outlived a resolve that did not happen")
	}
}

// The binding copy was decided since: the row settles as happened (the
// add-on recorded it), and the half does not run over the later write.
// Otherwise it runs, a stale copy included.
func TestLiveABindingCallDecidedSinceSettlesWithoutItsHalf(t *testing.T) {
	settles := func(t *testing.T, ctx context.Context, call, accountID string, wantHalf bool) {
		t.Helper()
		ran := false
		if _, err := ResolveBindingCall(ctx, bindTarget, call, accountID, AddonUnresolvedGrace, func(context.Context) error { ran = true; return nil }); err != nil {
			t.Fatal(err)
		}
		if ran != wantHalf {
			t.Fatalf("the half ran %t, want %t", ran, wantHalf)
		}
		if res, _ := resolution(t, ctx, call); res != AddonResolutionHappened {
			t.Fatalf("the row must settle as happened, got %q", res)
		}
	}
	laterCall := func(t *testing.T, ctx context.Context, op, subject, status string) {
		t.Helper()
		id, err := BeginAddonOperation(ctx, AddonOperationParams{Target: bindTarget, Operation: op, ActorID: "op-lita", SubjectID: subject})
		if err != nil {
			t.Fatal(err)
		}
		if status == AddonOpDispatching {
			return
		}
		if status != AddonOpUnreached {
			if _, err := ClaimAddonOperation(ctx, id, bindTarget, op, subject); err != nil {
				t.Fatal(err)
			}
		}
		if err := SettleAddonOperation(ctx, id, status); err != nil {
			t.Fatal(err)
		}
	}

	// Review round 3, item 2: only a later call that may have reached the
	// add-on decides the copy.
	for _, tc := range []struct {
		status string
		skips  bool
	}{
		{AddonOpDispatching, true}, {AddonOpSucceeded, true}, {AddonOpIndeterminate, true},
		{AddonOpUnreached, false}, {AddonOpRejected, false},
	} {
		t.Run("a later release, "+tc.status, func(t *testing.T) {
			ctx := bindingLive(t)
			call := lostCall(t, ctx, "account.adopt", "u-becky", "charlotte")
			laterCall(t, ctx, "account.release", "u-becky", tc.status)
			settles(t, ctx, call, "200001", !tc.skips)
		})
	}
	t.Run("another subject's later call is not this binding's", func(t *testing.T) {
		ctx := bindingLive(t)
		call := lostCall(t, ctx, "account.adopt", "u-kofi", "kofi")
		laterCall(t, ctx, "account.purge", "u-xavier", AddonOpDispatching)
		settles(t, ctx, call, "200002", true)
	})

	// Item 3, a release: the copy decides only when it names another account
	// than the one released; with no name to compare, by time.
	t.Run("a release, the drain since bound another account", func(t *testing.T) {
		ctx := bindingLive(t)
		call := lostCall(t, ctx, "account.release", "u-becky", "charlotte")
		if err := adoptionOf("u-becky", "lita", "200009")(ctx); err != nil {
			t.Fatal(err)
		}
		settles(t, ctx, call, "", false)
		if _, found := boundRow(t, ctx, "u-becky"); !found {
			t.Fatal("a stale release deleted a newer binding of another account")
		}
	})
	t.Run("a release of the account the copy names", func(t *testing.T) {
		ctx := bindingLive(t)
		call := lostCall(t, ctx, "account.release", "u-becky", "charlotte")
		if err := adoptionOf("u-becky", "charlotte", "200001")(ctx); err != nil {
			t.Fatal(err)
		}
		settles(t, ctx, call, "", true)
	})
	// Review round 4: the same name, a new account. A release of charlotte
	// (200001) is recorded; the drain then creates charlotte again (200002)
	// and saves its copy. The release's id, not the name, decides.
	t.Run("a release of the same name, a new account since", func(t *testing.T) {
		ctx := bindingLive(t)
		call := lostCall(t, ctx, "account.release", "u-becky", "charlotte")
		if err := adoptionOf("u-becky", "charlotte", "200002")(ctx); err != nil {
			t.Fatal(err)
		}
		settles(t, ctx, call, "200001", false)
		if b, found := boundRow(t, ctx, "u-becky"); !found || b.AccountID == nil || *b.AccountID != "200002" {
			t.Fatalf("a stale release forgot the new account's copy: %+v %t", b, found)
		}
	})
	t.Run("a release whose reply names another account", func(t *testing.T) {
		ctx := bindingLive(t)
		if err := adoptionOf("u-becky", "charlotte", "200001")(ctx); err != nil {
			t.Fatal(err)
		}
		aged(t, ctx, "u-becky")
		call := lostCall(t, ctx, "account.release", "u-becky", "charlotte")
		settles(t, ctx, call, "200009", false)
	})
	for _, tc := range []struct {
		name  string
		age   bool
		skips bool
	}{{"a release naming nothing, the copy newer", false, true}, {"a release naming nothing, the copy older", true, false}} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := bindingLive(t)
			call := lostCall(t, ctx, "account.release", "u-becky", "")
			if err := adoptionOf("u-becky", "charlotte", "200001")(ctx); err != nil {
				t.Fatal(err)
			}
			if tc.age {
				aged(t, ctx, "u-becky")
			}
			settles(t, ctx, call, "", !tc.skips)
		})
	}

	// Item 3, an adoption: only a copy newer than the call AND naming another
	// account decides it. An older one naming another account is stale.
	for _, tc := range []struct {
		name, copyID string
		age, skips   bool
	}{
		{"an adoption, a newer copy of another account", "200009", false, true},
		{"an adoption, an older copy of another account", "200009", true, false},
		{"an adoption, a newer copy of the same account", "200001", false, false},
		{"an adoption, an older copy of the same account", "200001", true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := bindingLive(t)
			call := lostCall(t, ctx, "account.adopt", "u-becky", "charlotte")
			if err := adoptionOf("u-becky", "charlotte", tc.copyID)(ctx); err != nil {
				t.Fatal(err)
			}
			if tc.age {
				aged(t, ctx, "u-becky")
			}
			settles(t, ctx, call, "200001", !tc.skips)
		})
	}
}
