package db

import (
	"testing"
	"time"

	"syndra/internal/claimtel"
)

// A finding counts on while open, settles after enough clean looks, starts
// again on a later failure, and a check that no longer sees its subject
// settles it.
func TestClaimFindings_Lifecycle(t *testing.T) {
	ctx := liveDB(t)
	wipe := func() {
		if _, err := PG.Exec(ctx, `TRUNCATE claim_findings`); err != nil {
			t.Fatalf("truncate: %v", err)
		}
	}
	wipe()
	t.Cleanup(wipe)

	at := time.Now().UTC().Add(-time.Hour).Truncate(time.Second)
	fail := func(n int64, clean int) claimtel.FindingUpdate {
		return claimtel.FindingUpdate{Kind: claimtel.KindNotCalled, Subject: "app-1", Fails: n, FirstFail: at, LastFail: at.Add(time.Minute),
			Sample: map[string]any{"user_id": "u1"}, TrailingClean: clean}
	}
	read := func() claimtel.FindingUpdate {
		t.Helper()
		fs, err := ClaimFindings(ctx, time.Now().Add(-24*time.Hour))
		if err != nil || len(fs) != 1 {
			t.Fatalf("findings = %+v, %v", fs, err)
		}
		f := fs[0]
		return claimtel.FindingUpdate{Kind: f.Kind, Subject: f.Subject, Fails: f.Count, FirstFail: f.FirstSeen, TrailingClean: f.CleanStreak,
			Sample: map[string]any{"resolved": f.ResolvedAt != nil}}
	}

	if err := ApplyClaimFindings(ctx, []claimtel.FindingUpdate{fail(2, 0)}, 3); err != nil {
		t.Fatal(err)
	}
	if err := ApplyClaimFindings(ctx, []claimtel.FindingUpdate{fail(1, 1)}, 3); err != nil {
		t.Fatal(err)
	}
	if f := read(); f.Fails != 3 || f.TrailingClean != 1 || f.Sample["resolved"] != false {
		t.Fatalf("open: %+v", f)
	}
	clean := claimtel.FindingUpdate{Kind: claimtel.KindNotCalled, Subject: "app-1", TrailingClean: 2}
	if err := ApplyClaimFindings(ctx, []claimtel.FindingUpdate{clean}, 3); err != nil {
		t.Fatal(err)
	}
	if f := read(); f.TrailingClean != 3 || f.Sample["resolved"] != true {
		t.Fatalf("three clean looks settle it: %+v", f)
	}
	// A clean look at a settled row changes nothing; a failure starts it again.
	_ = ApplyClaimFindings(ctx, []claimtel.FindingUpdate{clean}, 3)
	if err := ApplyClaimFindings(ctx, []claimtel.FindingUpdate{fail(1, 0)}, 3); err != nil {
		t.Fatal(err)
	}
	if f := read(); f.Fails != 1 || f.TrailingClean != 0 || f.Sample["resolved"] != false {
		t.Fatalf("reopened: %+v", f)
	}

	// Configuration: R1 ran and no longer saw R1:endpoint; R5 did not run.
	wipe()
	drift := func(subject string) claimtel.FindingUpdate {
		return claimtel.FindingUpdate{Kind: claimtel.KindConfigDrift, Subject: subject, Fails: 1, FirstFail: at, LastFail: at}
	}
	_ = ApplyClaimFindings(ctx, []claimtel.FindingUpdate{drift("R1:endpoint"), drift("R5:app-9")}, 1)
	if err := SettleUnseenClaimFindings(ctx, claimtel.KindConfigDrift, []string{"R1", "R2"}, []string{"R1:timeout"}); err != nil {
		t.Fatal(err)
	}
	fs, _ := ClaimFindings(ctx, time.Now().Add(-time.Hour))
	open := map[string]bool{}
	for _, f := range fs {
		open[f.Subject] = f.ResolvedAt == nil
	}
	if open["R1:endpoint"] || !open["R5:app-9"] {
		t.Errorf("open = %v: R1 ran without it, R5 did not run", open)
	}
}
