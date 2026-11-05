package db

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"
)

// W1d (one-path): a requeued outbox row waits out a backoff before any batch
// claim may take it again. Before 000080 RequeuePropagation made the row
// claimable at once, so a Zitadel blip could spend a whole retry budget in
// back-to-back passes.

// The schedule handed to SQL is extcall.Backoff for each spent-try count: it
// grows, it never collapses below half its step, and it stops at the cap.
func TestBackoffSchedule_GrowsAndStopsAtTheCap(t *testing.T) {
	s := backoffSchedule()
	base, maxWait := outboxBackoff.Base.Seconds(), outboxBackoff.Max.Seconds()
	for n, got := range s {
		step := base
		for i := 0; i < n && step < maxWait; i++ {
			step *= 2
		}
		if step > maxWait {
			step = maxWait
		}
		if got < step/2 || got > step {
			t.Fatalf("wait after %d tries = %.1fs, want within [%.1f, %.1f]", n, got, step/2, step)
		}
	}
	if s[len(s)-1] > maxWait {
		t.Fatalf("the last wait %.1fs passes the %.0fs cap", s[len(s)-1], maxWait)
	}
}

// Both batch claims carry the gate; the by-id claim deliberately does not.
func TestBatchClaimsRespectNotBefore(t *testing.T) {
	src := readDBSource(t, "propagations.go")
	for _, fn := range []string{"ClaimPendingPropagations", "ClaimPendingRevocations"} {
		if !strings.Contains(funcBody(t, src, fn), "notBeforeDue") {
			t.Errorf("%s must skip a row whose backoff has not passed", fn)
		}
	}
	if strings.Contains(funcBody(t, src, "ClaimPropagationByID"), "notBeforeDue") {
		t.Error("ClaimPropagationByID is an operator's explicit apply of one row; it is not gated")
	}
	if !strings.Contains(funcBody(t, src, "RequeuePropagation"), "not_before = NOW() +") {
		t.Error("RequeuePropagation must set not_before; a requeue without it is claimable at once")
	}
}

func TestOutboxNotBeforeMigration_DownUndoesUp(t *testing.T) {
	up, err := os.ReadFile("../../db/migrations/000080_outbox_not_before.up.sql")
	if err != nil {
		t.Fatalf("read up: %v", err)
	}
	down, err := os.ReadFile("../../db/migrations/000080_outbox_not_before.down.sql")
	if err != nil {
		t.Fatalf("read down: %v", err)
	}
	if !strings.Contains(string(up), "ADD COLUMN IF NOT EXISTS not_before TIMESTAMPTZ") {
		t.Error("000080 up must add a nullable not_before; NULL is claimable now, so every existing row stays claimable")
	}
	if !strings.Contains(string(down), "DROP COLUMN IF EXISTS not_before") {
		t.Error("000080 down must drop the column up added")
	}
}

// Live: the down migration really removes the column and up restores it. Run
// inside a transaction that is rolled back, so the shared test database is
// left exactly as it was.
func TestOutboxNotBeforeMigration_DownAndUpLive(t *testing.T) {
	ctx := liveDB(t)
	up, _ := os.ReadFile("../../db/migrations/000080_outbox_not_before.up.sql")
	down, _ := os.ReadFile("../../db/migrations/000080_outbox_not_before.down.sql")

	tx, err := PG.Begin(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer tx.Rollback(ctx)

	has := func() bool {
		var n int
		if err := tx.QueryRow(ctx, `SELECT COUNT(*) FROM information_schema.columns
			WHERE table_name='propagation_outbox' AND column_name='not_before'`).Scan(&n); err != nil {
			t.Fatalf("read columns: %v", err)
		}
		return n == 1
	}
	if !has() {
		t.Fatal("not_before is missing after migrating up")
	}
	if _, err := tx.Exec(ctx, string(down)); err != nil {
		t.Fatalf("down: %v", err)
	}
	if has() {
		t.Fatal("down left not_before in place")
	}
	if _, err := tx.Exec(ctx, string(up)); err != nil {
		t.Fatalf("up again: %v", err)
	}
	if !has() {
		t.Fatal("up after down did not restore not_before")
	}
}

func revokeRow(t *testing.T, ctx context.Context) string {
	t.Helper()
	key, err := newOutboxIdempotencyKey()
	if err != nil {
		t.Fatalf("idempotency key: %v", err)
	}
	var id string
	if err := PG.QueryRow(ctx, `
		INSERT INTO propagation_outbox
			(op_type, user_id, project_id, role_keys, payload_json, idempotency_key,
			 initiated_by, source, target)
		VALUES ('revoke','u-backoff','p1',ARRAY['community'],'{}',$1,'tester','direct','zitadel')
		RETURNING id`, key).Scan(&id); err != nil {
		t.Fatalf("seed revoke row: %v", err)
	}
	return id
}

// Live: a requeued row is skipped by both batch claims until not_before
// passes, the spacing is at least half the base wait, the by-id claim is not
// gated, and a row whose budget is spent lands in failed, where the
// unfinished-revocations surface lists it as spent.
func TestRequeuedRowWaitsOutItsBackoffThenFails(t *testing.T) {
	ctx := liveDB(t)
	id := revokeRow(t, ctx)
	const budget = 5 // OUTBOX_MAX_RETRIES default

	claimBoth := func() int {
		a, err := ClaimPendingRevocations(ctx, TargetZitadel, 10)
		if err != nil {
			t.Fatalf("claim revocations: %v", err)
		}
		if len(a) > 0 {
			return len(a)
		}
		b, err := ClaimPendingPropagations(ctx, TargetZitadel, 10)
		if err != nil {
			t.Fatalf("claim pending: %v", err)
		}
		return len(b)
	}

	if claimBoth() != 1 {
		t.Fatal("a fresh row (not_before NULL) must be claimable")
	}
	for try := 1; try <= budget; try++ {
		attempts, err := RequeuePropagation(ctx, id, "zitadel api 503")
		if err != nil {
			t.Fatalf("requeue %d: %v", try, err)
		}
		if attempts != try {
			t.Fatalf("attempts = %d, want %d", attempts, try)
		}
		var wait time.Duration
		var secs float64
		if err := PG.QueryRow(ctx, `SELECT EXTRACT(EPOCH FROM not_before - NOW()) FROM propagation_outbox WHERE id=$1`, id).Scan(&secs); err != nil {
			t.Fatalf("read not_before: %v", err)
		}
		wait = time.Duration(secs * float64(time.Second))
		if wait < outboxBackoff.Base/2-time.Second {
			t.Fatalf("try %d: row waits %s, want at least %s", try, wait, outboxBackoff.Base/2)
		}
		if wait > outboxBackoff.Max+time.Second {
			t.Fatalf("try %d: row waits %s, past the %s cap", try, wait, outboxBackoff.Max)
		}
		if n := claimBoth(); n != 0 {
			t.Fatalf("try %d: a batch claim took a row inside its backoff", try)
		}
		// The operator's explicit apply of this one row is not gated.
		if _, found, err := ClaimPropagationByID(ctx, TargetZitadel, id); err != nil || !found {
			t.Fatalf("try %d: by-id claim must ignore the backoff (found=%v err=%v)", try, found, err)
		}
		if _, err := PG.Exec(ctx, `UPDATE propagation_outbox SET status='pending', not_before = NOW() - interval '1 second' WHERE id=$1`, id); err != nil {
			t.Fatalf("pass the backoff: %v", err)
		}
		if claimBoth() != 1 {
			t.Fatalf("try %d: a row past its backoff must be claimable", try)
		}
	}

	// The drain's exhausted branch (drain_test: ExhaustedRowGoesTerminal) does this.
	if err := MarkPropagationFailed(ctx, id, "out of retries after 5 attempts: zitadel api 503"); err != nil {
		t.Fatalf("mark failed: %v", err)
	}
	rows, err := ListUnconfirmedRevocations(ctx, 10)
	if err != nil {
		t.Fatalf("list unconfirmed: %v", err)
	}
	if len(rows) != 1 || rows[0].ID != id || !rows[0].Spent {
		t.Fatalf("an exhausted revoke must be listed as spent on the unfinished-revocations surface; got %+v", rows)
	}
}
