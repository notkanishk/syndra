package db

import (
	"context"
	"sort"
	"testing"
)

// The background retry claim and the operator's Send, against a real
// database (owner, 5 Nov 2026):
//   - a give Zitadel didn't take is claimed by the background drain once its
//     backoff has passed, and not before;
//   - a row that waits for the operator is never claimed by it, by its stamp
//     or by its role being marked to wait;
//   - Send takes every row, backoff included.

func outboxRow(t *testing.T, ctx context.Context, user, role, waitReason string, attempts int, notBefore string) string {
	t.Helper()
	key, err := newOutboxIdempotencyKey()
	if err != nil {
		t.Fatalf("idempotency key: %v", err)
	}
	var nb any
	if notBefore != "" {
		nb = notBefore
	}
	var wr any
	if waitReason != "" {
		wr = waitReason
	}
	var id string
	err = PG.QueryRow(ctx, `
		INSERT INTO propagation_outbox
			(op_type, user_id, project_id, role_keys, payload_json, idempotency_key,
			 initiated_by, source, target, status, attempts, wait_reason, not_before)
		VALUES ('add',$1,'p1',ARRAY[$2],'{}',$3,'tester','direct','zitadel','pending',$4,$5,
		        NOW() + $6::interval)
		RETURNING id`, user, role, key, attempts, wr, nb).Scan(&id)
	if err != nil {
		t.Fatalf("seed outbox row: %v", err)
	}
	return id // NOW() + NULL is NULL: an empty notBefore is a row never tried
}

func TestRetryClaims_BackgroundTakesDueRetriesOnlyAndSendTakesAll(t *testing.T) {
	ctx := liveDB(t)
	if _, err := PG.Exec(ctx, `DELETE FROM role_waits WHERE project_id = 'p1'`); err != nil {
		t.Fatalf("clear role_waits: %v", err)
	}
	t.Cleanup(func() { _, _ = PG.Exec(ctx, `DELETE FROM role_waits WHERE project_id = 'p1'`) })

	due := outboxRow(t, ctx, "u1", "community", "", 1, "-1 minute")
	backoff := outboxRow(t, ctx, "u2", "community", "", 1, "10 minutes")
	neverSent := outboxRow(t, ctx, "u3", "community", "", 0, "")
	waitsStamp := outboxRow(t, ctx, "u4", "community", WaitReasonRoleWaits, 1, "-1 minute")
	waitsRole := outboxRow(t, ctx, "u5", "laser", "", 1, "-1 minute")
	if _, err := PG.Exec(ctx, `INSERT INTO role_waits (project_id, role_key, marked_by) VALUES ('p1','laser','tester')`); err != nil {
		t.Fatalf("mark laser to wait: %v", err)
	}

	// Pending changes says the due and the backing-off rows are retrying.
	retrying, _, err := CountRetries(ctx)
	if err != nil {
		t.Fatalf("CountRetries: %v", err)
	}
	if retrying < 2 {
		t.Errorf("CountRetries = %d, want at least the due and the backing-off row", retrying)
	}
	pending, err := GetPendingPropagations(ctx)
	if err != nil {
		t.Fatalf("GetPendingPropagations: %v", err)
	}
	for _, p := range pending {
		if p.ID == backoff && (p.NotBefore == nil || p.Attempts != 1) {
			t.Errorf("GetPendingPropagations must carry not_before and attempts, got %+v", p)
		}
	}

	got, err := ClaimDueRetries(ctx, TargetZitadel, 100)
	if err != nil {
		t.Fatalf("ClaimDueRetries: %v", err)
	}
	if len(got) != 1 || got[0].ID != due {
		ids := []string{}
		for _, r := range got {
			ids = append(ids, r.ID)
		}
		t.Fatalf("background claimed %v, want only the due retry %s (not backoff %s, never-sent %s, waits %s/%s)",
			ids, due, backoff, neverSent, waitsStamp, waitsRole)
	}

	// Send: everything still pending, the row in backoff included.
	sent, err := ClaimPendingPropagations(ctx, TargetZitadel, 100)
	if err != nil {
		t.Fatalf("ClaimPendingPropagations: %v", err)
	}
	ids := []string{}
	for _, r := range sent {
		ids = append(ids, r.ID)
	}
	sort.Strings(ids)
	want := []string{due, backoff, neverSent, waitsStamp, waitsRole}
	sort.Strings(want)
	if len(ids) != len(want) {
		t.Fatalf("Send claimed %v, want every row %v (the in_flight due row is reclaimed too)", ids, want)
	}
}

// A change Syndra gave up on is counted until an operator marks the give-ups
// seen, then not; a later give-up is counted again.
func TestCountRetries_AGiveUpStaysUntilSeen(t *testing.T) {
	ctx := liveDB(t)
	_, _ = PG.Exec(ctx, `DELETE FROM config_settings WHERE key = $1`, ConfigKeyGaveUpSeenAt)
	t.Cleanup(func() { _, _ = PG.Exec(ctx, `DELETE FROM config_settings WHERE key = $1`, ConfigKeyGaveUpSeenAt) })
	gaveUp := func() string {
		id := outboxRow(t, ctx, "u9", "community", "", 6, "")
		if _, err := PG.Exec(ctx, `UPDATE propagation_outbox SET status = 'failed',
			last_error = 'out of retries: 503', completed_at = NOW() WHERE id = $1`, id); err != nil {
			t.Fatalf("give up: %v", err)
		}
		return id
	}
	count := func() int {
		_, n, err := CountRetries(ctx)
		if err != nil {
			t.Fatalf("CountRetries: %v", err)
		}
		return n
	}
	before := count()
	gaveUp()
	if got := count(); got != before+1 {
		t.Fatalf("an unseen give-up is counted: %d, want %d", got, before+1)
	}
	if err := MarkGaveUpSeen(ctx, "tester"); err != nil {
		t.Fatalf("MarkGaveUpSeen: %v", err)
	}
	if got := count(); got != 0 {
		t.Fatalf("after seen, nothing counts: %d", got)
	}
	gaveUp()
	if got := count(); got != 1 {
		t.Fatalf("a later give-up counts again: %d", got)
	}
}

// A straight-away send that halted before anything went (Zitadel unreachable)
// is scheduled like a 503 after a send: try 1 counted, a next try, the
// halted:retrying stamp, so the background claim takes it once due. A row that
// waits, or one a 503 already scheduled, is left alone.
func TestScheduleRetry_AHaltedSendGetsANextTry(t *testing.T) {
	ctx := liveDB(t)
	halted := outboxRow(t, ctx, "u6", "community", "", 0, "")
	waits := outboxRow(t, ctx, "u7", "community", WaitReasonRoleWaits, 0, "")
	already := outboxRow(t, ctx, "u8", "community", "", 2, "5 minutes")

	got, err := ScheduleRetry(ctx, []string{halted, waits, already}, "halted: zitadel_offline")
	if err != nil {
		t.Fatalf("ScheduleRetry: %v", err)
	}
	if len(got) != 1 || got[0] != halted {
		t.Fatalf("scheduled %v, want only %s", got, halted)
	}
	var attempts int
	var reason, lastErr string
	var inAMinute bool // the first backoff: a minute, jittered to 30-60s (extcall.Backoff)
	if err := PG.QueryRow(ctx, `SELECT attempts, wait_reason, last_error,
		not_before BETWEEN NOW() + interval '25 seconds' AND NOW() + interval '65 seconds'
		FROM propagation_outbox WHERE id = $1`, halted).Scan(&attempts, &reason, &lastErr, &inAMinute); err != nil {
		t.Fatalf("read back: %v", err)
	}
	if attempts != 1 || reason != WaitReasonHalted(HaltRetrying) || lastErr != "halted: zitadel_offline" || !inAMinute {
		t.Fatalf("row = attempts %d, reason %q, last_error %q, next try in a minute %v", attempts, reason, lastErr, inAMinute)
	}
	for id, want := range map[string]int{waits: 0, already: 2} {
		var n int
		if err := PG.QueryRow(ctx, `SELECT attempts FROM propagation_outbox WHERE id = $1`, id).Scan(&n); err != nil || n != want {
			t.Fatalf("row %s left alone: attempts %d (want %d) err %v", id, n, want, err)
		}
	}
}
