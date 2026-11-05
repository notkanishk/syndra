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
