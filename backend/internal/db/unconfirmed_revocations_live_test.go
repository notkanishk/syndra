package db

import (
	"testing"
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
