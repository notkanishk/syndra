package db

import "testing"

// A row the target refused keeps its plain cause, so Pending changes and Home
// can say why Syndra stopped sending it; a row that ran out of tries keeps
// none (it says "didn't answer N times" instead).
func TestListGaveUp_RefusedRowCarriesItsCause(t *testing.T) {
	ctx := liveDB(t)
	refused := outboxRow(t, ctx, "refused@example.org", "community", "", 0, "")
	spent := outboxRow(t, ctx, "spent@example.org", "community", "", 5, "")
	ids := []string{refused, spent}
	if _, err := PG.Exec(ctx, `UPDATE propagation_outbox SET status='in_flight' WHERE id::text = ANY($1)`, ids); err != nil {
		t.Fatalf("claim rows: %v", err)
	}
	t.Cleanup(func() { _, _ = PG.Exec(ctx, `DELETE FROM propagation_outbox WHERE id::text = ANY($1)`, ids) })

	if err := MarkPropagationRefused(ctx, refused, "zitadel api 403: no", RefusalNotAllowed); err != nil {
		t.Fatalf("refuse: %v", err)
	}
	if err := MarkPropagationFailed(ctx, spent, "out of retries"); err != nil {
		t.Fatalf("fail: %v", err)
	}

	rows, err := ListGaveUp(ctx, "tester@example.org")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	got := map[string]string{}
	for _, r := range rows {
		got[r.ID] = r.WaitReason
	}
	if w, ok := got[refused]; !ok || w != WaitReasonRefused(RefusalNotAllowed) {
		t.Errorf("refused row wait_reason = %q (listed %v), want %q", w, ok, WaitReasonRefused(RefusalNotAllowed))
	}
	if w, ok := got[spent]; !ok || w != "" {
		t.Errorf("out-of-tries row wait_reason = %q (listed %v), want empty", w, ok)
	}
}
