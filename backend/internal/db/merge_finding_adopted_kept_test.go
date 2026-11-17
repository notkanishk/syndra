package db

import (
	"strings"
	"testing"
)

// 000100 (addon-contract 7t-7): 'adopted' and 'kept' answer a binding_conflict, and a row closes
// carrying its decision (COALESCE(decision, 'agreed')), so the resolution check allows each
// decision the surface can record. Down withdraws a standing one and reads a settled one as
// 'agreed', its decision column keeping what was chosen.

func TestMigration100_AllowsEveryDecisionAndDownMapsTheNewOnes(t *testing.T) {
	up, down := readMigrationPair(t, "000100_merge_finding_adopted_kept")
	added := up[strings.LastIndex(up, "ADD CONSTRAINT"):]
	for _, v := range []string{ResolutionKeepOurs, ResolutionTakeTheirs, ResolutionReprovisioned, ResolutionUnbound,
		ResolutionAgreed, ResolutionAdopted, ResolutionKept} {
		if !isMergeResolution(v) && v != ResolutionAgreed {
			t.Errorf("%s is not a decision RecordMergeDecision takes", v)
		}
		if !strings.Contains(added, "'"+v+"'") {
			t.Errorf("000100 up must allow %s", v)
		}
	}
	for _, step := range []string{
		"SET decision = NULL, decided_by = NULL, decided_at = NULL, decision_reason = NULL\nWHERE resolved_at IS NULL AND decision IN ('adopted', 'kept')",
		"SET resolution = 'agreed' WHERE resolution IN ('adopted', 'kept')",
	} {
		if !strings.Contains(down, step) {
			t.Errorf("000100 down must %q before restoring the old check", step)
		}
	}
	restored := down[strings.LastIndex(down, "ADD CONSTRAINT"):]
	if strings.Contains(restored, "adopted") || strings.Contains(restored, "kept") || !strings.Contains(restored, "'agreed'") {
		t.Error("000100 down must restore exactly the 000042 check")
	}
}

// Live: a standing 'kept' and a settled 'adopted' go down and up; nothing else is touched, and the
// database is left at the version it began at (atLeast).
func TestLiveMigration100RunsDownAndUp(t *testing.T) {
	ctx := liveDB(t)
	m := atLeast(t, ctx, 100)
	seedAddonTarget(t, ctx)
	cleanMergeFindings(t, ctx)
	for _, s := range []string{"kept", "adopted", "other"} {
		if err := RecordMergeFinding(ctx, MergeFinding{Target: "truenas", SubjectID: s, Outcome: "binding_conflict",
			Theirs: []byte(`{"reason":"claimed_twice"}`)}); err != nil {
			t.Fatal(err)
		}
	}
	id := func(subject string) string {
		t.Helper()
		open := openFindings(t, ctx, subject)
		if len(open) != 1 {
			t.Fatalf("%s: %d open findings", subject, len(open))
		}
		return open[0].ID
	}
	keptID, adoptedID := id("kept"), id("adopted")
	if _, err := RecordMergeDecision(ctx, keptID, "op-bayley", ResolutionKept, "bob is theirs"); err != nil {
		t.Fatal(err)
	}
	if _, err := RecordMergeDecision(ctx, adoptedID, "op-bayley", ResolutionAdopted, "eve is theirs"); err != nil {
		t.Fatal(err)
	}
	// The sweep's close: the add-on stopped reporting it, so it closes carrying the decision.
	if err := ClearMergeFinding(ctx, "truenas", "adopted", "", "system:reconcile"); err != nil {
		t.Fatalf("up: a row closes carrying 'adopted': %v", err)
	}
	read := func(rowID string) (decision, resolution string) {
		t.Helper()
		if err := PG.QueryRow(ctx, `SELECT COALESCE(decision, ''), COALESCE(resolution, '') FROM target_merge_findings WHERE id = $1::uuid`,
			rowID).Scan(&decision, &resolution); err != nil {
			t.Fatal(err)
		}
		return
	}
	if d, r := read(adoptedID); d != ResolutionAdopted || r != ResolutionAdopted {
		t.Fatalf("closed carrying the decision: %s / %s", d, r)
	}

	if err := m.Migrate(99); err != nil {
		t.Fatalf("000100 down: %v", err)
	}
	if d, r := read(keptID); d != "" || r != "" {
		t.Fatalf("down withdraws a standing kept: %q / %q", d, r)
	}
	if d, r := read(adoptedID); d != ResolutionAdopted || r != ResolutionAgreed {
		t.Fatalf("down reads a settled adopted as agreed and keeps the decision: %q / %q", d, r)
	}
	if _, err := PG.Exec(ctx, `UPDATE target_merge_findings SET resolution = 'kept' WHERE id = $1::uuid`, adoptedID); err == nil {
		t.Fatal("after down the check must refuse kept")
	}
	if err := m.Migrate(100); err != nil {
		t.Fatalf("000100 up: %v", err)
	}
	if _, err := PG.Exec(ctx, `UPDATE target_merge_findings SET resolution = 'kept' WHERE id = $1::uuid`, adoptedID); err != nil {
		t.Fatalf("after up the check must allow kept: %v", err)
	}
}
