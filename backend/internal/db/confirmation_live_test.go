package db

import (
	"context"
	"testing"
	"time"
)

// What "confirmed" means, against a real database.
//
// `applied` has always meant Zitadel returned 2xx — an acknowledgement of
// receipt, which this product read as evidence of state for long enough to
// deliver a grant that was never sent. `confirmed_at` is the separate fact: a
// read has since OBSERVED the change.
//
// The queries below are new, and untested SQL is what caused the failure these
// exist because of.

func appliedRow(t *testing.T, ctx context.Context, completedAgo time.Duration) string {
	t.Helper()
	key, err := newOutboxIdempotencyKey()
	if err != nil {
		t.Fatalf("idempotency key: %v", err)
	}
	var id string
	err = PG.QueryRow(ctx, `
		INSERT INTO propagation_outbox
			(op_type, user_id, project_id, role_keys, payload_json, idempotency_key,
			 initiated_by, source, target, status, completed_at)
		VALUES ('add','u1','p1',ARRAY['community'],'{}',$1,'tester','direct','zitadel','applied',
		        NOW() - $2::interval)
		RETURNING id`, key, completedAgo.String()).Scan(&id)
	if err != nil {
		t.Fatalf("seed applied row: %v", err)
	}
	return id
}

func TestAnObservedWriteStopsBeingUnobserved(t *testing.T) {
	ctx := liveDB(t)
	id := appliedRow(t, ctx, time.Hour)

	found, err := AppliedButUnobserved(ctx, 30*time.Minute)
	if err != nil {
		t.Fatalf("AppliedButUnobserved: %v", err)
	}
	if len(found) != 1 || found[0].ID != id {
		t.Fatalf("an hour-old unobserved write was not listed; got %+v", found)
	}

	if err := MarkPropagationConfirmed(ctx, id); err != nil {
		t.Fatalf("MarkPropagationConfirmed: %v", err)
	}
	found, err = AppliedButUnobserved(ctx, 30*time.Minute)
	if err != nil {
		t.Fatalf("AppliedButUnobserved: %v", err)
	}
	if len(found) != 0 {
		t.Fatalf("a confirmed write is still listed as unobserved: %+v", found)
	}
}

// Age is the whole reason one of these is on a screen. A write accepted a
// moment ago and not yet visible is Zitadel's read projection catching up, and
// putting it in front of somebody would train them to ignore the list.
func TestAWriteTooYoungToJudgeIsNotListed(t *testing.T) {
	ctx := liveDB(t)
	appliedRow(t, ctx, time.Minute)

	found, err := AppliedButUnobserved(ctx, 30*time.Minute)
	if err != nil {
		t.Fatalf("AppliedButUnobserved: %v", err)
	}
	if len(found) != 0 {
		t.Fatalf("a one-minute-old write was reported as unsubstantiated: %+v", found)
	}
}

// Confirming twice is not an error. Two reads observing one write must not
// produce a fault the second time.
func TestConfirmingAnAlreadyConfirmedWriteIsNotAFault(t *testing.T) {
	ctx := liveDB(t)
	id := appliedRow(t, ctx, time.Hour)

	for i := range 2 {
		if err := MarkPropagationConfirmed(ctx, id); err != nil {
			t.Fatalf("confirmation %d: %v", i+1, err)
		}
	}
}

// A confirmation may never resurrect a row that failed, was superseded, or was
// abandoned. `applied` is the only status it acts on.
func TestOnlyAnAppliedWriteCanBeConfirmed(t *testing.T) {
	ctx := liveDB(t)
	id := appliedRow(t, ctx, time.Hour)
	if _, err := PG.Exec(ctx,
		`UPDATE propagation_outbox SET status='superseded' WHERE id=$1`, id); err != nil {
		t.Fatalf("supersede: %v", err)
	}

	if err := MarkPropagationConfirmed(ctx, id); err != nil {
		t.Fatalf("MarkPropagationConfirmed: %v", err)
	}
	var confirmed *time.Time
	if err := PG.QueryRow(ctx,
		`SELECT confirmed_at FROM propagation_outbox WHERE id=$1`, id).Scan(&confirmed); err != nil {
		t.Fatalf("read back: %v", err)
	}
	if confirmed != nil {
		t.Fatal("a superseded row was marked as observed in Zitadel")
	}
}
