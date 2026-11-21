package db

import (
	"testing"
	"time"
)

// ListUnconfirmedRevocations carries each row's wait_reason (7t-4): Withdrawn
// access says why a given-up revocation stopped through it, so a connected
// system's own refusal is told from Zitadel's `refused:<cause>` and from a row
// that ran out of tries (no reason). The column is nullable, so a row without
// one must come back as an empty string, never fail the scan.
func TestLiveUnconfirmedRevocationsCarryTheirWaitReason(t *testing.T) {
	ctx := liveDB(t)
	seedAddonTarget(t, ctx)
	seed := func(user, target, status, waitReason string) {
		t.Helper()
		key, err := newOutboxIdempotencyKey()
		if err != nil {
			t.Fatal(err)
		}
		var reason any
		if waitReason != "" {
			reason = waitReason
		}
		if _, err := PG.Exec(ctx, `
			INSERT INTO propagation_outbox
				(op_type, user_id, project_id, role_keys, payload_json, idempotency_key,
				 initiated_by, source, target, status, wait_reason, last_error)
			VALUES ('revoke', $1, 'p1', ARRAY['community'], '{}', $2, 'tester', 'direct', $3, $4, $5, 'why')`,
			user, key, target, status, reason); err != nil {
			t.Fatalf("seed %s: %v", user, err)
		}
	}
	seed("refused-by-system", "truenas", "failed", WaitReasonTargetRefused)
	seed("refused-by-zitadel", "zitadel", "failed", WaitReasonRefused("not_found"))
	seed("out-of-tries", "zitadel", "failed", "")
	seed("still-pending", "truenas", "pending", "")

	got, err := ListUnconfirmedRevocations(ctx, 50)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{
		"refused-by-system":  WaitReasonTargetRefused,
		"refused-by-zitadel": WaitReasonRefused("not_found"),
		"out-of-tries":       "",
		"still-pending":      "",
	}
	if len(got) != len(want) {
		t.Fatalf("want %d rows, got %d: %+v", len(want), len(got), got)
	}
	for _, r := range got {
		if reason, ok := want[r.UserID]; !ok || r.WaitReason != reason {
			t.Errorf("%s: wait_reason %q, want %q", r.UserID, r.WaitReason, reason)
		}
		if r.Spent != (r.Status == "failed") {
			t.Errorf("%s: spent %v for status %s", r.UserID, r.Spent, r.Status)
		}
	}
}

// Owner 28.3: a queued revocation escalates on age unless its system is held
// unreachable right now. An add-on row whose add-on answers and fails the call
// (left in_flight, indeterminate) is not exempt: it ages like Zitadel's.
func TestLiveAnAddonRevocationEscalatesUnlessItsSystemIsHeld(t *testing.T) {
	ctx := liveDB(t)
	seedAddonTarget(t, ctx)
	key, err := newOutboxIdempotencyKey()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := PG.Exec(ctx, `
		INSERT INTO propagation_outbox
			(op_type, user_id, payload_json, idempotency_key, initiated_by, source, target, status, withdraws_only, created_at)
		VALUES ('apply', 'the-undertaker', '{}', $1, 'tester', 'direct', 'truenas', 'in_flight', true, NOW() - interval '2 days')`,
		key); err != nil {
		t.Fatal(err)
	}
	day := 24 * time.Hour

	answering, err := CountUnconfirmedRevocations(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !answering.Escalated(day) || answering.WaitingAge < day {
		t.Fatalf("an add-on that answers and fails escalates on age: %+v", answering)
	}
	held, err := CountUnconfirmedRevocations(ctx, []string{"truenas"})
	if err != nil {
		t.Fatal(err)
	}
	if held.Escalated(day) || held.Queued != 1 || held.OldestAge < day {
		t.Fatalf("a held system's revocation waits, listed with its age, unescalated: %+v", held)
	}
}

// A revocation that lands writes revocation.landed with both its times, in the
// statement that settles it; a confer-only apply that lands writes none.
func TestLiveALandedRevocationIsAuditedWithBothTimes(t *testing.T) {
	ctx := liveDB(t)
	seedAddonTarget(t, ctx)
	seed := func(user string, withdraws bool) string {
		t.Helper()
		key, err := newOutboxIdempotencyKey()
		if err != nil {
			t.Fatal(err)
		}
		var id string
		if err := PG.QueryRow(ctx, `
			INSERT INTO propagation_outbox
				(op_type, user_id, payload_json, idempotency_key, initiated_by, source, target, status, withdraws_only, created_at)
			VALUES ('apply', $1, '{}', $2, 'tester', 'direct', 'truenas', 'in_flight', $3, '2026-11-20T09:00:00Z')
			RETURNING id`, user, key, withdraws).Scan(&id); err != nil {
			t.Fatal(err)
		}
		return id
	}
	revoked, conferred := seed("kane", true), seed("mankind", false)
	for _, id := range []string{revoked, conferred} {
		if err := MarkPropagationApplied(ctx, id); err != nil {
			t.Fatal(err)
		}
	}
	rows, err := PG.Query(ctx, `SELECT resource_id, details->>'recorded_at', details->>'landed_at', details->>'target'
		FROM audit_logs WHERE action = 'revocation.landed' AND resource_id IN ($1, $2)`, revoked, conferred)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	n := 0
	for rows.Next() {
		var id, recorded, landed, target string
		if err := rows.Scan(&id, &recorded, &landed, &target); err != nil {
			t.Fatal(err)
		}
		n++
		if id != revoked || recorded != "2026-11-20T09:00:00Z" || landed == "" || target != "truenas" {
			t.Errorf("landed line = %s %s %s %s", id, recorded, landed, target)
		}
	}
	if n != 1 {
		t.Fatalf("want one revocation.landed, for the withdrawal only; got %d", n)
	}
}
