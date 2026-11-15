package db

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/golang-migrate/migrate/v4"
	pgx5 "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
)

// 7t-3: the binding_conflict finding (000096) and the in-flight read the drift
// sweep asks before concluding an account deleted.

// Guard: 000096 widens the outcome check, keeping every 000042 outcome, and
// its down clears binding_conflict rows before restoring exactly 000042's.
func TestMigration96_AllowsBindingConflictAndDownClearsIt(t *testing.T) {
	up, down := readMigrationPair(t, "000096_merge_finding_binding_conflict")
	added := up[strings.LastIndex(up, "ADD CONSTRAINT"):]
	for _, keep := range []string{"'theirs_only'", "'conflict'", "'deleted_upstream'", "'binding_conflict'"} {
		if !strings.Contains(added, keep) {
			t.Errorf("000096 up must allow %s", keep)
		}
	}
	if !strings.Contains(down, "DELETE FROM target_merge_findings WHERE outcome = 'binding_conflict'") {
		t.Error("000096 down must clear binding_conflict rows before restoring the old check")
	}
	restored := down[strings.LastIndex(down, "ADD CONSTRAINT"):]
	if strings.Contains(restored, "binding_conflict") || !strings.Contains(restored, "'deleted_upstream'") {
		t.Error("000096 down must restore exactly the 000042 check")
	}
}

func cleanMergeFindings(t *testing.T, ctx context.Context) {
	t.Helper()
	clean := func() {
		_, _ = PG.Exec(context.Background(), `DELETE FROM target_merge_findings WHERE target = 'truenas'`)
	}
	clean()
	t.Cleanup(clean)
}

func openFindings(t *testing.T, ctx context.Context, subject string) []MergeFinding {
	t.Helper()
	all, err := StandingMergeFindings(ctx, "truenas")
	if err != nil {
		t.Fatal(err)
	}
	var out []MergeFinding
	for _, f := range all {
		if f.SubjectID == subject {
			out = append(out, f)
		}
	}
	return out
}

// Raise, refresh, close, on the real table. A binding_conflict takes the
// account-level slot: it replaces a standing deleted_upstream for the same
// subject (the old wrong finding) rather than standing beside it, a second
// raise refreshes the one row with the holder as it is now, and the sweep's
// account-slot clear closes it `agreed`.
func TestLiveABindingConflictTakesTheAccountSlotAndClosesAgreed(t *testing.T) {
	ctx := liveDB(t)
	seedAddonTarget(t, ctx)
	cleanMergeFindings(t, ctx)

	if err := RecordMergeFinding(ctx, MergeFinding{Target: "truenas", SubjectID: "sub-1", Outcome: "deleted_upstream"}); err != nil {
		t.Fatal(err)
	}
	raise := func(holder string) {
		t.Helper()
		theirs, _ := json.Marshal(map[string]string{"subject": "sub-1", "reason": "number_held", "username": holder})
		if err := RecordMergeFinding(ctx, MergeFinding{Target: "truenas", SubjectID: "sub-1", Outcome: "binding_conflict", Theirs: theirs}); err != nil {
			t.Fatalf("the check must accept binding_conflict: %v", err)
		}
	}
	raise("kofi_kingston")
	raise("big_e")
	open := openFindings(t, ctx, "sub-1")
	if len(open) != 1 || open[0].Outcome != "binding_conflict" || open[0].Field != "" ||
		!strings.Contains(string(open[0].Theirs), "big_e") {
		t.Fatalf("want one binding_conflict in the account slot, refreshed to the current holder: %+v", open)
	}

	if err := ClearMergeFinding(ctx, "truenas", "sub-1", "", "system:reconcile"); err != nil {
		t.Fatal(err)
	}
	if open := openFindings(t, ctx, "sub-1"); len(open) != 0 {
		t.Fatalf("the clear must close it: %+v", open)
	}
	var resolution string
	if err := PG.QueryRow(ctx, `SELECT resolution FROM target_merge_findings
		WHERE target = 'truenas' AND subject_id = 'sub-1' AND outcome = 'binding_conflict'`).Scan(&resolution); err != nil || resolution != ResolutionAgreed {
		t.Fatalf("closed as %q (%v), want agreed", resolution, err)
	}
}

// What the sweep asks before concluding an account deleted, on the database's
// clock: applies settled since the read began, and applies claimed and not
// settled that started within the window a claimed apply's calls can run. Not
// one settled before the read, not one only queued, not another target's, not
// a Zitadel-shaped row, and not an in_flight row older than the window (a
// drain that crashed mid-row): that one can no longer land, and must not hide
// a real deletion for ever.
func TestLiveAppliesInFlightSince(t *testing.T) {
	ctx := liveDB(t)
	seedAddonTarget(t, ctx)
	since, err := DatabaseNow(ctx)
	if err != nil {
		t.Fatal(err)
	}
	const window = time.Minute
	seed := func(op, user, target, status string, started, completed *time.Time) {
		t.Helper()
		key, err := newOutboxIdempotencyKey()
		if err != nil {
			t.Fatal(err)
		}
		if _, err := PG.Exec(ctx, `
			INSERT INTO propagation_outbox
				(op_type, user_id, project_id, role_keys, payload_json, idempotency_key,
				 initiated_by, source, target, status, started_at, completed_at)
			VALUES ($1, $2, CASE WHEN $1 = 'apply' THEN NULL ELSE 'p1' END,
			        CASE WHEN $1 = 'apply' THEN NULL ELSE ARRAY['community'] END,
			        '{}', $3, 'tester', 'direct', $4, $5, $6, $7)`,
			op, user, key, target, status, started, completed); err != nil {
			t.Fatalf("seed %s %s: %v", user, status, err)
		}
	}
	at := func(d time.Duration) *time.Time { v := since.Add(d); return &v }
	seed("apply", "claimed", "truenas", "in_flight", at(-30*time.Second), nil)
	seed("apply", "orphaned", "truenas", "in_flight", at(-time.Hour), nil)
	seed("apply", "settled-during", "truenas", "applied", at(-time.Second), at(time.Second))
	seed("apply", "settled-before", "truenas", "applied", at(-2*time.Hour), at(-time.Hour))
	seed("apply", "queued", "truenas", "pending", nil, nil)
	seed("add", "zitadel-shaped", "zitadel", "in_flight", at(-time.Second), nil)

	got, err := AppliesInFlightSince(ctx, "truenas", since, window)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || !got["claimed"] || !got["settled-during"] {
		t.Fatalf("want claimed and settled-during, got %v", got)
	}
}

// A new outcome in a standing finding's slot is a new question (7t-3): a
// deleted_upstream decided "re-provision" and then replaced by a
// binding_conflict is undecided, detected anew, and unbound resolves it. The
// same outcome raised again keeps its decision and its age.
func TestLiveANewOutcomeInTheSlotDropsTheOldDecision(t *testing.T) {
	ctx := liveDB(t)
	seedAddonTarget(t, ctx)
	cleanMergeFindings(t, ctx)

	if err := RecordMergeFinding(ctx, MergeFinding{Target: "truenas", SubjectID: "sub-1", Outcome: "deleted_upstream"}); err != nil {
		t.Fatal(err)
	}
	first := openFindings(t, ctx, "sub-1")[0]
	if _, err := RecordMergeDecision(ctx, first.ID, "op_1", ResolutionReprovisioned, "they still need it"); err != nil {
		t.Fatal(err)
	}
	// Raised again as itself: the decision and the age stand.
	if err := RecordMergeFinding(ctx, MergeFinding{Target: "truenas", SubjectID: "sub-1", Outcome: "deleted_upstream"}); err != nil {
		t.Fatal(err)
	}
	if again := openFindings(t, ctx, "sub-1")[0]; again.Decision != ResolutionReprovisioned || !again.DetectedAt.Equal(first.DetectedAt) {
		t.Fatalf("the same question keeps its answer and its age: %+v", again)
	}

	if err := RecordMergeFinding(ctx, MergeFinding{Target: "truenas", SubjectID: "sub-1", Outcome: "binding_conflict",
		Theirs: []byte(`{"subject":"sub-1","reason":"number_held"}`)}); err != nil {
		t.Fatal(err)
	}
	now := openFindings(t, ctx, "sub-1")
	if len(now) != 1 || now[0].ID != first.ID || now[0].Outcome != "binding_conflict" {
		t.Fatalf("want the one slot, now a binding_conflict: %+v", now)
	}
	if now[0].Decision != "" || now[0].DecidedBy != "" || now[0].DecidedAt != nil || now[0].DecisionReason != "" ||
		!now[0].DetectedAt.After(first.DetectedAt) {
		t.Fatalf("a new outcome is undecided and detected now: %+v", now[0])
	}
	if _, err := RecordMergeDecision(ctx, now[0].ID, "op_2", ResolutionUnbound, "not theirs"); err != nil {
		t.Fatalf("unbound must be decidable on it: %v", err)
	}
	if _, err := ResolveMergeFinding(ctx, now[0].ID, "op_2", ResolutionUnbound); err != nil {
		t.Fatalf("unbound must resolve it: %v", err)
	}
	if open := openFindings(t, ctx, "sub-1"); len(open) != 0 {
		t.Fatalf("resolved: %+v", open)
	}
}

// The sweep's close for conflicts the add-on stopped listing: every standing
// binding_conflict on the target outside the kept subjects closes (agreed, or
// what was decided, by whoever decided it); the kept ones, other outcomes and
// other targets stand. An empty keep closes them all.
func TestLiveClearMergeFindingsOutside(t *testing.T) {
	ctx := liveDB(t)
	seedAddonTarget(t, ctx)
	cleanMergeFindings(t, ctx)
	if _, err := PG.Exec(ctx, `INSERT INTO targets (target) VALUES ('test_7t3_other') ON CONFLICT DO NOTHING`); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = PG.Exec(context.Background(), `DELETE FROM target_merge_findings WHERE target = 'test_7t3_other'`)
		_, _ = PG.Exec(context.Background(), `DELETE FROM targets WHERE target = 'test_7t3_other'`)
	})
	conflict := func(target, subject string) {
		t.Helper()
		if err := RecordMergeFinding(ctx, MergeFinding{Target: target, SubjectID: subject, Outcome: "binding_conflict",
			Theirs: []byte(`{"reason":"name_held"}`)}); err != nil {
			t.Fatal(err)
		}
	}
	conflict("truenas", "still")
	conflict("truenas", "ended")
	conflict("truenas", "decided")
	conflict("test_7t3_other", "elsewhere")
	if err := RecordMergeFinding(ctx, MergeFinding{Target: "truenas", SubjectID: "gone", Outcome: "deleted_upstream"}); err != nil {
		t.Fatal(err)
	}
	decided := openFindings(t, ctx, "decided")[0]
	if _, err := RecordMergeDecision(ctx, decided.ID, "op_1", ResolutionUnbound, "not theirs"); err != nil {
		t.Fatal(err)
	}

	n, err := ClearMergeFindingsOutside(ctx, "truenas", "binding_conflict", []string{"still"}, "system:reconcile")
	if err != nil || n != 2 {
		t.Fatalf("want ended and decided closed, got %d (%v)", n, err)
	}
	for subject, open := range map[string]bool{"still": true, "ended": false, "decided": false, "gone": true} {
		if got := len(openFindings(t, ctx, subject)) == 1; got != open {
			t.Errorf("%s: open %t, want %t", subject, got, open)
		}
	}
	var resolution, by string
	if err := PG.QueryRow(ctx, `SELECT resolution, resolved_by FROM target_merge_findings
		WHERE target = 'truenas' AND subject_id = 'ended'`).Scan(&resolution, &by); err != nil || resolution != ResolutionAgreed || by != "system:reconcile" {
		t.Errorf("ended closed as %q by %q (%v), want agreed by the sweep", resolution, by, err)
	}
	if err := PG.QueryRow(ctx, `SELECT resolution, resolved_by FROM target_merge_findings
		WHERE target = 'truenas' AND subject_id = 'decided'`).Scan(&resolution, &by); err != nil || resolution != ResolutionUnbound || by != "op_1" {
		t.Errorf("decided closed as %q by %q (%v), want what op_1 decided", resolution, by, err)
	}
	if n, err := ClearMergeFindingsOutside(ctx, "truenas", "binding_conflict", nil, "system:reconcile"); err != nil || n != 1 {
		t.Fatalf("an empty keep closes every one left, got %d (%v)", n, err)
	}
	var elsewhere int
	if err := PG.QueryRow(ctx, `SELECT COUNT(*) FROM target_merge_findings WHERE target = 'test_7t3_other' AND resolved_at IS NULL`).Scan(&elsewhere); err != nil || elsewhere != 1 {
		t.Errorf("another target's conflict must stand: %d (%v)", elsewhere, err)
	}
}

// 000096 runs down and up on the real engine: down removes binding_conflict
// rows and the check refuses the outcome again, up accepts it. Left where it
// began.
func TestLiveMigration96RunsDownAndUp(t *testing.T) {
	ctx := liveDB(t)
	var version int64
	var dirty bool
	if err := PG.QueryRow(ctx, `SELECT version, dirty FROM schema_migrations`).Scan(&version, &dirty); err != nil {
		t.Fatal(err)
	}
	if version < 96 || dirty {
		t.Skipf("the test database is at %d (dirty %t), before this branch's 96", version, dirty)
	}
	config, err := pgxpool.ParseConfig(os.Getenv("SYNDRA_TEST_DATABASE_URL"))
	if err != nil {
		t.Fatal(err)
	}
	driver, err := pgx5.WithInstance(stdlib.OpenDB(*config.ConnConfig), &pgx5.Config{})
	if err != nil {
		t.Fatal(err)
	}
	m, err := migrate.NewWithDatabaseInstance("file://../../db/migrations", "postgres", driver)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if v, _, _ := m.Version(); v != uint(version) {
			_ = m.Migrate(uint(version))
		}
	})
	seedAddonTarget(t, ctx)
	cleanMergeFindings(t, ctx)
	conflict := MergeFinding{Target: "truenas", SubjectID: "sub-1", Outcome: "binding_conflict", Theirs: []byte(`{"reason":"name_held"}`)}
	if err := RecordMergeFinding(ctx, conflict); err != nil {
		t.Fatal(err)
	}
	if err := RecordMergeFinding(ctx, MergeFinding{Target: "truenas", SubjectID: "sub-2", Outcome: "deleted_upstream"}); err != nil {
		t.Fatal(err)
	}

	if err := m.Migrate(95); err != nil {
		t.Fatalf("000096 down: %v", err)
	}
	if open := openFindings(t, ctx, "sub-1"); len(open) != 0 {
		t.Fatalf("down must remove binding_conflict rows: %+v", open)
	}
	if open := openFindings(t, ctx, "sub-2"); len(open) != 1 {
		t.Fatalf("down must keep every other finding: %+v", open)
	}
	if err := RecordMergeFinding(ctx, conflict); err == nil {
		t.Fatal("after down, the 000042 check must refuse binding_conflict")
	}

	if err := m.Migrate(96); err != nil {
		t.Fatalf("000096 up: %v", err)
	}
	if err := RecordMergeFinding(ctx, conflict); err != nil {
		t.Fatalf("after up, binding_conflict must be accepted: %v", err)
	}
}
