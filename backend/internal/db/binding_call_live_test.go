package db

import (
	"context"
	"errors"
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

func TestLiveARecordedBindingCallSettlesWithItsHalfOnce(t *testing.T) {
	ctx := bindingLive(t)
	id := "200001"
	adoption := func(ctx context.Context) error {
		return RecordTargetBinding(ctx, TargetBinding{Target: bindTarget, SubjectID: "u-becky", Username: "charlotte", BoundBy: "op-lita", AccountID: &id})
	}

	call := lostCall(t, ctx, "account.adopt", "u-becky", "charlotte")
	runs := 0
	half := func(ctx context.Context) error { runs++; return adoption(ctx) }
	if _, err := ResolveBindingCall(ctx, bindTarget, call, AddonUnresolvedGrace, half); err != nil {
		t.Fatal(err)
	}
	if res, by := resolution(t, ctx, call); res != AddonResolutionHappened || by != SyndraReader {
		t.Fatalf("want happened by Syndra's reading, got %q %q", res, by)
	}
	b, found := boundRow(t, ctx, "u-becky")
	if !found || b.Username != "charlotte" || b.AccountID == nil || *b.AccountID != id || b.BoundBy != "op-lita" {
		t.Fatalf("want the binding the adoption writes, got %+v %t", b, found)
	}

	// Twice: the row is answered, so the half does not run again.
	if _, err := ResolveBindingCall(ctx, bindTarget, call, AddonUnresolvedGrace, half); !errors.Is(err, ErrAddonOperationNotUnresolved) || runs != 1 {
		t.Fatalf("a second settle must change nothing: %v, half ran %d times", err, runs)
	}

	// A release recorded: the binding goes, with the row's answer.
	rel := lostCall(t, ctx, "account.release", "u-becky", "charlotte")
	forget := func(ctx context.Context) error {
		if err := ForgetMergeBase(ctx, bindTarget, "u-becky"); err != nil {
			return err
		}
		return ForgetTargetBinding(ctx, bindTarget, "u-becky")
	}
	if _, err := ResolveBindingCall(ctx, bindTarget, rel, AddonUnresolvedGrace, forget); err != nil {
		t.Fatal(err)
	}
	if _, found := boundRow(t, ctx, "u-becky"); found {
		t.Fatal("a settled release must forget the binding")
	}
	if res, _ := resolution(t, ctx, rel); res != AddonResolutionHappened {
		t.Fatalf("the release's row must settle, got %q", res)
	}
}

// One transaction: a half that fails leaves no binding and the row open, and
// a resolve that cannot happen takes the half's write back with it.
func TestLiveABindingHalfAndItsSettleCommitTogether(t *testing.T) {
	ctx := bindingLive(t)
	id := "200002"
	write := func(ctx context.Context) error {
		return RecordTargetBinding(ctx, TargetBinding{Target: bindTarget, SubjectID: "u-kofi", Username: "kofi", BoundBy: "op-lita", AccountID: &id})
	}

	call := lostCall(t, ctx, "account.adopt", "u-kofi", "kofi")
	boom := errors.New("half failed")
	if _, err := ResolveBindingCall(ctx, bindTarget, call, AddonUnresolvedGrace, func(ctx context.Context) error {
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
	young, err := BeginAddonOperation(ctx, AddonOperationParams{Target: bindTarget, Operation: "account.adopt", ActorID: "op-lita", SubjectID: "u-xavier", About: "xavier"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ClaimAddonOperation(ctx, young, bindTarget, "account.adopt", "u-xavier"); err != nil {
		t.Fatal(err)
	}
	if _, err := ResolveBindingCall(ctx, bindTarget, young, AddonUnresolvedGrace, write); !errors.Is(err, ErrAddonOperationNotUnresolved) {
		t.Fatalf("a call in flight must not settle: %v", err)
	}
	if _, found := boundRow(t, ctx, "u-kofi"); found {
		t.Fatal("the half's write outlived a resolve that did not happen")
	}
}

// A later adopt, release or purge of the subject means its binding was
// decided since: the older call's half would write over it.
func TestLiveABindingCallDecidedSinceStaysForAPerson(t *testing.T) {
	ctx := bindingLive(t)
	call := lostCall(t, ctx, "account.adopt", "u-becky", "charlotte")
	if _, err := BeginAddonOperation(ctx, AddonOperationParams{Target: bindTarget, Operation: "account.release", ActorID: "op-lita", SubjectID: "u-becky"}); err != nil {
		t.Fatal(err)
	}
	ran := false
	_, err := ResolveBindingCall(ctx, bindTarget, call, AddonUnresolvedGrace, func(context.Context) error { ran = true; return nil })
	if !errors.Is(err, ErrBindingDecidedSince) || ran {
		t.Fatalf("want ErrBindingDecidedSince and no half, got %v (ran %t)", err, ran)
	}
	if res, _ := resolution(t, ctx, call); res != "" {
		t.Fatalf("the row must stay open, got %q", res)
	}

	// Another subject's later call is not this binding's.
	other := lostCall(t, ctx, "account.adopt", "u-kofi", "kofi")
	if _, err := BeginAddonOperation(ctx, AddonOperationParams{Target: bindTarget, Operation: "account.purge", ActorID: "op-lita", SubjectID: "u-xavier"}); err != nil {
		t.Fatal(err)
	}
	if _, err := ResolveBindingCall(ctx, bindTarget, other, AddonUnresolvedGrace, func(context.Context) error { return nil }); err != nil {
		t.Fatalf("an unrelated subject's call must not block it: %v", err)
	}
}
