package db

import (
	"context"
	"testing"
)

// The record and its lock against the real schema (migration 000077).
func TestActionKeyRotations_Live(t *testing.T) {
	ctx := liveDB(t)
	clean := func() {
		if _, err := PG.Exec(context.Background(), `TRUNCATE action_key_rotations`); err != nil {
			t.Fatal(err)
		}
	}
	clean()
	t.Cleanup(clean)

	// Single flight: the second taker is refused until the first releases.
	release, ok, err := TryActionKeyRotationLock(ctx)
	if err != nil || !ok {
		t.Fatalf("first lock: %v %v", ok, err)
	}
	if _, ok2, err := TryActionKeyRotationLock(ctx); err != nil || ok2 {
		t.Fatalf("second lock while held: %v %v", ok2, err)
	}
	release()
	release2, ok, err := TryActionKeyRotationLock(ctx)
	if err != nil || !ok {
		t.Fatalf("lock after release: %v %v", ok, err)
	}
	release2()

	st, err := ActionKeyRotationNow(ctx, "syndra-claim-injector")
	if err != nil || st.Last != nil || st.LastReplacedAt != nil || st.LastAutoAt != nil {
		t.Fatalf("empty: %+v %v", st, err)
	}

	// An attempt interrupted after Zitadel replaced the claims key.
	id, err := StartActionKeyRotation(ctx, "auto", "system", []string{"syndra-claim-injector", "syndra-event-listener"})
	if err != nil {
		t.Fatal(err)
	}
	if err := MarkActionKeyReplaced(ctx, id, "syndra-claim-injector"); err != nil {
		t.Fatal(err)
	}
	if err := MarkActionKeyReplaced(ctx, id, "syndra-claim-injector"); err != nil {
		t.Fatal(err)
	}
	st, _ = ActionKeyRotationNow(ctx, "syndra-claim-injector")
	if st.Last == nil || st.Last.Outcome != "in_flight" || len(st.Last.Replaced) != 1 || st.LastReplacedAt == nil || st.LastAutoAt == nil {
		t.Fatalf("in flight: %+v", st.Last)
	}
	if err := AbandonActionKeyRotations(ctx); err != nil {
		t.Fatal(err)
	}
	st, _ = ActionKeyRotationNow(ctx, "syndra-claim-injector")
	if st.Last.Outcome != "abandoned" || st.Last.FinishedAt == nil || st.Last.Error == "" || st.LastReplacedAt == nil {
		t.Fatalf("abandoned: %+v", st.Last)
	}

	// A finished, unverified one, then confirmed.
	id2, _ := StartActionKeyRotation(ctx, "button", "admin-1", []string{"syndra-claim-injector"})
	_ = MarkActionKeyReplaced(ctx, id2, "syndra-claim-injector")
	if err := FinishActionKeyRotation(ctx, id2, "unverified", map[string]any{"pass": false, "class": "claims"}, "no claims"); err != nil {
		t.Fatal(err)
	}
	st, _ = ActionKeyRotationNow(ctx, "syndra-claim-injector")
	if st.Last.ID != id2 || st.Last.Outcome != "unverified" || st.Last.Error != "no claims" || len(st.Last.Probe) == 0 {
		t.Fatalf("unverified: %+v", st.Last)
	}
	if err := FinishActionKeyRotation(ctx, id2, "verified", nil, ""); err != nil {
		t.Fatal(err)
	}
	st, _ = ActionKeyRotationNow(ctx, "syndra-claim-injector")
	if st.Last.Outcome != "verified" || st.Last.Error != "" || len(st.Last.Probe) == 0 {
		t.Fatalf("verified later keeps the probe: %+v", st.Last)
	}

	// The schema refuses what the record must never hold.
	if _, err := StartActionKeyRotation(ctx, "cron", "system", []string{"x"}); err == nil {
		t.Error("an unknown trigger was stored")
	}
	if _, err := StartActionKeyRotation(ctx, "auto", "system", []string{}); err == nil {
		t.Error("an attempt with no targets was stored")
	}
}
