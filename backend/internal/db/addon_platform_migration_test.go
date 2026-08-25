package db

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// Migration-coherence guards for the add-on platform's target dimension
// (change `addon-platform`, tasks 1.8 and 1.9). The db package has no live-DB
// harness — see propagations_migration_test.go — so these assert the migration
// text and the Go source agree. A CHECK, a foreign key, or a unique index that
// drifts from what the repository writes fails on an operator's screen at
// runtime, which is the failure these guards exist to move into CI.

const addonMigrationBase = "000026_addon_platform_target_dimension"

func addonMigrationSQL(t *testing.T) (up, down string) {
	t.Helper()
	dir := findMigrationsDir(t)
	u, err := os.ReadFile(filepath.Join(dir, addonMigrationBase+".up.sql"))
	if err != nil {
		t.Fatalf("read up migration: %v", err)
	}
	d, err := os.ReadFile(filepath.Join(dir, addonMigrationBase+".down.sql"))
	if err != nil {
		t.Fatalf("read down migration: %v", err)
	}
	return string(u), string(d)
}

// allUpMigrationsSQL concatenates every up migration in order. Assertions about
// what the schema IS (rather than what one migration did) must read all of
// them: a constraint does not stay in the migration that introduced it, and
// pinning to one file asserts what the schema used to be while passing against
// a running database that disagrees.
func allUpMigrationsSQL(t *testing.T) string {
	t.Helper()
	ups, err := filepath.Glob(filepath.Join(findMigrationsDir(t), "*.up.sql"))
	if err != nil || len(ups) == 0 {
		t.Fatalf("glob migrations: %v (found %d)", err, len(ups))
	}
	sort.Strings(ups)
	var b strings.Builder
	for _, p := range ups {
		c, err := os.ReadFile(p)
		if err != nil {
			t.Fatalf("read %s: %v", filepath.Base(p), err)
		}
		b.Write(c)
		b.WriteString("\n")
	}
	return b.String()
}

// stripSQLComments removes `--` line comments. Assertions about what the schema
// forbids have to read the statements, not the prose: this migration's comments
// name the very constructs it declines to use.
func stripSQLComments(sql string) string {
	return regexp.MustCompile(`(?m)--.*$`).ReplaceAllString(sql, "")
}

// createTableBody returns the parenthesised body of `CREATE TABLE [IF NOT
// EXISTS] name (...)`, so a column assertion cannot be satisfied by a mention
// of the same word elsewhere in the migration.
func createTableBody(t *testing.T, sql, table string) string {
	t.Helper()
	re := regexp.MustCompile(`(?is)CREATE TABLE\s+(?:IF NOT EXISTS\s+)?` + regexp.QuoteMeta(table) + `\s*\((.*?)\n\);`)
	m := re.FindStringSubmatch(sql)
	if m == nil {
		t.Fatalf("could not isolate CREATE TABLE %s in the migration", table)
	}
	return m[1]
}

// 1.1 — the registry exists, carries state, and is seeded with the target that
// already had history pointing at it.
func TestTargetsRegistry(t *testing.T) {
	up, down := addonMigrationSQL(t)

	body := createTableBody(t, up, "targets")
	if !strings.Contains(body, "target        TEXT PRIMARY KEY") && !regexp.MustCompile(`target\s+TEXT\s+PRIMARY KEY`).MatchString(body) {
		t.Error("targets.target must be the primary key — it is the value every other table's foreign key resolves against")
	}
	for _, st := range []string{"'active'", "'disabled'"} {
		if !strings.Contains(body, st) {
			t.Errorf("targets.state must permit %s", st)
		}
	}
	// Unregistering is disabling, never deleting: propagation and drift history
	// keeps pointing here (design §3). A registry with no state column would
	// force a delete, and the foreign keys would correctly refuse it.
	if !regexp.MustCompile(`state\s+TEXT\s+NOT NULL`).MatchString(body) {
		t.Error("targets.state must be NOT NULL — an unknown lifecycle state is not a state the drain can check")
	}
	if !regexp.MustCompile(`(?is)INSERT INTO targets[^;]*'zitadel'`).MatchString(up) {
		t.Error("the migration must seed 'zitadel' — every pre-existing row defaults to it and its foreign key must resolve")
	}
	if !strings.Contains(down, "DROP TABLE IF EXISTS targets") {
		t.Error("down migration must drop the registry")
	}
}

// 1.1 / 1.8 — every table carrying a target must resolve it against the
// registry, and none may re-enumerate the permitted values in a CHECK. A CHECK
// would make registering a later add-on a schema migration, which is the whole
// reason the registry is a table (design §3).
func TestTargetColumnsReferenceTheRegistry(t *testing.T) {
	up, _ := addonMigrationSQL(t)

	for _, table := range []string{"propagation_outbox", "drift_items", "external_grant_exclusions"} {
		re := regexp.MustCompile(`(?is)ALTER TABLE\s+` + table + `\s+ADD COLUMN IF NOT EXISTS target\s+TEXT\s+NOT NULL\s+DEFAULT\s+'zitadel'\s+REFERENCES targets\(target\)`)
		if !re.MatchString(up) {
			t.Errorf("%s.target must be NOT NULL DEFAULT 'zitadel' REFERENCES targets(target): the default is what makes every pre-existing row read back as zitadel, and the reference is what refuses a row naming an unregistered target", table)
		}
	}
	for _, table := range []string{"desired_state_snapshots", "plans"} {
		body := createTableBody(t, up, table)
		if !regexp.MustCompile(`target\s+TEXT\s+NOT NULL REFERENCES targets\(target\)`).MatchString(body) {
			t.Errorf("%s.target must reference the registry", table)
		}
	}
	// A CHECK enumerating targets would defeat the registry: config and schema
	// would have to move together, and a config-only deployment could write
	// rows the database refuses.
	if regexp.MustCompile(`(?i)CHECK\s*\(\s*target\s+IN\s*\(`).MatchString(up) {
		t.Error("target must not be constrained by a CHECK enumerating values — registration is a data fact, not a schema constant (design §3)")
	}
}

// 1.2 — the rename is complete: the table, its indexes, and its constraints.
// Postgres renames none of the latter with the table, so a partial rename
// leaves the old name in the schema while the code says otherwise.
func TestOutboxRenameIsComplete(t *testing.T) {
	up, down := addonMigrationSQL(t)

	if !strings.Contains(up, "ALTER TABLE IF EXISTS pending_zitadel_propagations RENAME TO propagation_outbox") {
		t.Fatal("up migration must rename the outbox table")
	}
	for _, idx := range []string{
		"idx_pending_zitadel_propagations_status",
		"idx_pending_zitadel_propagations_source",
		"idx_pending_zitadel_propagations_cascade",
		"pending_zitadel_propagations_pkey",
		"pending_zitadel_propagations_idempotency_key_key",
	} {
		if !regexp.MustCompile(`(?is)ALTER INDEX IF EXISTS\s+` + regexp.QuoteMeta(idx) + `\s+RENAME TO`).MatchString(up) {
			t.Errorf("index %s still carries the old table's name after the rename", idx)
		}
	}
	for _, c := range []string{
		"pending_zitadel_propagations_source_check",
		"pending_zitadel_propagations_status_check",
		"pending_zitadel_propagations_op_type_check",
	} {
		if !strings.Contains(up, "DROP CONSTRAINT IF EXISTS "+c) {
			t.Errorf("constraint %s must be dropped and re-added under the new table's name", c)
		}
	}
	if !strings.Contains(down, "ALTER TABLE IF EXISTS propagation_outbox RENAME TO pending_zitadel_propagations") {
		t.Error("down migration must rename the outbox back")
	}
}

// 1.2 — and no live query may still name the old table. A missed one is a
// runtime "relation does not exist" on whichever operator path reaches it
// first. Migration files are excluded: their text is history, not a statement
// about the current schema.
func TestNoLiveQueryNamesTheOldOutbox(t *testing.T) {
	roots := []string{"..", filepath.Join("..", "..", "cmd")}
	var offenders []string
	// Counted, because this asserts an ABSENCE across a walk: a root that stops
	// resolving makes every assertion below vacuously true and the guard reports
	// success having read nothing. Same failure mode as the deploy check that
	// passed with a dead route on the target.
	examined := 0
	for _, root := range roots {
		err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() || !strings.HasSuffix(path, ".go") {
				return err
			}
			// Guard tests assert against historical migration text on purpose.
			if strings.HasSuffix(path, "_test.go") {
				return nil
			}
			src, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			examined++
			if strings.Contains(string(src), "pending_zitadel_propagations") {
				offenders = append(offenders, path)
			}
			return nil
		})
		if err != nil {
			t.Fatalf("walk %s: %v", root, err)
		}
	}
	if examined < 50 {
		t.Fatalf("only %d source files walked; the roots %v no longer resolve and this guard is watching nothing",
			examined, roots)
	}
	if len(offenders) > 0 {
		t.Errorf("these files still name the pre-rename outbox table: %v", offenders)
	}
}

// 1.3 — the Zitadel-shaped columns relax so an add-on row can exist, and a
// CHECK keeps them mandatory for the target that actually has them. Without
// that second half, relaxation is a licence to write a half-formed Zitadel row.
func TestOutboxRelaxationKeepsZitadelRowsWellFormed(t *testing.T) {
	up, down := addonMigrationSQL(t)

	if !regexp.MustCompile(`(?is)ALTER TABLE propagation_outbox\s+ALTER COLUMN project_id DROP NOT NULL,\s*ALTER COLUMN role_keys\s+DROP NOT NULL`).MatchString(up) {
		t.Error("propagation_outbox.project_id and .role_keys must lose their NOT NULL — a TrueNAS apply has no project and no role keys")
	}
	shape := regexp.MustCompile(`(?is)ADD CONSTRAINT propagation_outbox_zitadel_shape_check\s+CHECK \(target <> 'zitadel' OR \(project_id IS NOT NULL AND role_keys IS NOT NULL\)\)`)
	if !shape.MatchString(up) {
		t.Error("relaxing the NOT NULLs must be paired with a CHECK keeping project_id and role_keys mandatory for target='zitadel'")
	}
	if !strings.Contains(down, "ALTER COLUMN project_id SET NOT NULL") {
		t.Error("down migration must restore the outbox NOT NULLs")
	}
}

// 1.3 — op_type widens for the add-on convergence, and status widens for the
// version-rejection terminal state. Both are installed now so the drain needs
// no second ALTER, the same reasoning 000015 used for the 5-value source enum.
func TestOutboxEnumsAreWidened(t *testing.T) {
	up, _ := addonMigrationSQL(t)

	opCheck := regexp.MustCompile(`(?is)ADD CONSTRAINT propagation_outbox_op_type_check\s+CHECK \(op_type IN \(([^)]*)\)\)`).FindStringSubmatch(up)
	if opCheck == nil {
		t.Fatal("could not isolate the op_type CHECK")
	}
	for _, op := range []string{"'add'", "'revoke'", "'replace'", "'apply'"} {
		if !strings.Contains(opCheck[1], op) {
			t.Errorf("op_type %s missing from the widened CHECK", op)
		}
	}

	stCheck := regexp.MustCompile(`(?is)ADD CONSTRAINT propagation_outbox_status_check\s+CHECK \(status IN \(([^)]*)\)\)`).FindStringSubmatch(up)
	if stCheck == nil {
		t.Fatal("could not isolate the status CHECK")
	}
	for _, st := range []string{"'pending'", "'in_flight'", "'applied'", "'failed'", "'superseded'"} {
		if !strings.Contains(stCheck[1], st) {
			t.Errorf("status %s missing from the widened CHECK", st)
		}
	}
}

// 1.4 — snapshots are immutable audit records with a monotonic version per
// (subject, target).
func TestDesiredStateSnapshotsAreImmutableAndVersioned(t *testing.T) {
	up, down := addonMigrationSQL(t)

	body := createTableBody(t, up, "desired_state_snapshots")
	if !strings.Contains(body, "UNIQUE (subject_id, target, version)") {
		t.Error("desired_state_snapshots must be unique per (subject_id, target, version) — the database backstop for monotonic versioning")
	}
	// CHECK (version > 0) survives the trigger rather than being replaced by it:
	// it is the assertion that holds if the trigger is ever dropped, and it is
	// what turns the DEFAULT 0 sentinel into an error rather than a stored row
	// should an allocation ever fail to happen.
	if !regexp.MustCompile(`version\s+BIGINT NOT NULL DEFAULT 0 CHECK \(version > 0\)`).MatchString(body) {
		t.Error("version must be a positive BIGINT defaulting to the 0 sentinel the trigger replaces")
	}

	// Immutability is enforced, not merely intended. Without the trigger, "an
	// operator-initiated change applies what was approved" is a convention any
	// later UPDATE can break silently.
	if !regexp.MustCompile(`(?is)CREATE TRIGGER desired_state_snapshots_immutable\s+BEFORE UPDATE OR DELETE ON desired_state_snapshots`).MatchString(up) {
		t.Error("desired_state_snapshots needs a BEFORE UPDATE OR DELETE trigger — the snapshot an outbox row cites must not be editable after approval")
	}

	// UNIQUE forbids two rows sharing a version; it does NOT make versions
	// monotonic. Version 2 then version 1 satisfies it, and the stale-version
	// check ("older than the last version applied") is then comparing against a
	// number that went backwards.
	if !regexp.MustCompile(`(?is)CREATE TRIGGER desired_state_snapshots_version_monotonic\s+BEFORE INSERT ON desired_state_snapshots`).MatchString(up) {
		t.Error("desired_state_snapshots needs a BEFORE INSERT trigger allocating the version — UNIQUE(subject_id, target, version) permits 2 then 1")
	}
	// Comments stripped: a commented-out lock still contains the word, and a
	// guard satisfied by prose guards nothing.
	fn := stripSQLComments(regexp.MustCompile(`(?is)CREATE OR REPLACE FUNCTION enforce_desired_state_snapshot_version\(\).*?\n\$\$;`).FindString(up))
	if strings.TrimSpace(fn) == "" {
		t.Fatal("could not isolate the version-allocation trigger function")
	}
	for _, want := range []string{
		"pg_advisory_xact_lock",                                     // serialize the pair, so MAX+1 is still true at insert
		"NEW.subject_id || ':' || NEW.target",                       // scoped to the pair, not to the whole table
		"COALESCE(MAX(version), 0)",                                 // the predecessor, with an empty history meaning 0
		"WHERE subject_id = NEW.subject_id AND target = NEW.target", // read the same pair the lock covers
		"NEW.version := last_version + 1",                           // ALLOCATE. Validating would put a retry loop in every writer
	} {
		if !strings.Contains(fn, want) {
			t.Errorf("the version trigger must contain %q; body:\n%s", want, fn)
		}
	}
	// The lock has to precede the read it protects, or it protects nothing.
	if strings.Index(fn, "pg_advisory_xact_lock") > strings.Index(fn, "COALESCE(MAX(version), 0)") {
		t.Error("the advisory lock must be taken before MAX(version) is read")
	}
	// A writer cannot know the next version for a pair, so it must be able to
	// omit the column entirely.
	if !regexp.MustCompile(`version\s+BIGINT NOT NULL DEFAULT 0`).MatchString(body) {
		t.Error("version must default, so an INSERT can omit what the trigger allocates")
	}

	if !strings.Contains(up, "RAISE EXCEPTION") {
		t.Error("the immutability trigger must raise, not silently swallow the write")
	}
	for _, want := range []string{
		"DROP TRIGGER IF EXISTS desired_state_snapshots_immutable",
		"DROP TRIGGER IF EXISTS desired_state_snapshots_version_monotonic",
		"DROP FUNCTION IF EXISTS reject_desired_state_snapshot_mutation",
		"DROP FUNCTION IF EXISTS enforce_desired_state_snapshot_version",
	} {
		if !strings.Contains(down, want) {
			t.Errorf("down migration missing %q — a leftover trigger or function is a rollback that did not roll back", want)
		}
	}
}

// 1.7 — a primary key gaining a column is a change to every ON CONFLICT arbiter
// that named it. Postgres cannot infer a constraint from a partial column list,
// so a missed arbiter is not a subtle mismatch: the statement errors and the
// whole triage transaction rolls back, which is Mark external failing outright.
func TestExclusionWritesMatchTheWidenedPrimaryKey(t *testing.T) {
	src, err := os.ReadFile("drift.go")
	if err != nil {
		t.Fatalf("read drift.go: %v", err)
	}
	ins := regexp.MustCompile(`(?is)INSERT INTO external_grant_exclusions \(([^)]*)\).*?ON CONFLICT \(([^)]*)\)`).FindStringSubmatch(string(src))
	if ins == nil {
		t.Fatal("could not isolate the external_grant_exclusions INSERT and its conflict target")
	}
	for _, col := range []string{"target", "user_id", "project_id", "role_key"} {
		if !strings.Contains(ins[2], col) {
			t.Errorf("ON CONFLICT must name every primary-key column; %q missing from (%s)", col, ins[2])
		}
		if !strings.Contains(ins[1], col) {
			t.Errorf("the INSERT must supply %q — relying on the column default would write every target's exclusion as zitadel", col)
		}
	}

	// And the target written must be the drift row's own, not a literal: an
	// exclusion is a statement about the target that drifted.
	if !regexp.MustCompile(`(?is)target, err := claimDriftTx\(`).MatchString(string(src)) {
		t.Error("MarkDriftExternalTx must take the target from the drift row it claims, so the exclusion cannot outlive its target's identity")
	}

	// The read side of the same key. An unscoped read lets one target's
	// exclusion suppress another target's drift — the suppression the target
	// column entered the primary key to prevent.
	exc, err := os.ReadFile("exclusions.go")
	if err != nil {
		t.Fatalf("read exclusions.go: %v", err)
	}
	if !regexp.MustCompile(`(?is)FROM external_grant_exclusions WHERE target\s*=`).MatchString(string(exc)) {
		t.Error("GetExclusions must scope its read by target — an unscoped read lets an exclusion recorded on one target silence the identical triple on another")
	}
}

// 1.5 / 1.6 — one approval, one durable object: the outbox references the
// per-subject plan row, which holds both the snapshot and the fingerprint of
// the state that was reviewed.
func TestPlanStorageCarriesOneApprovalObject(t *testing.T) {
	up, down := addonMigrationSQL(t)

	subjects := createTableBody(t, up, "plan_subjects")
	for _, col := range []string{"plan_id", "subject_id", "snapshot_id", "fingerprint"} {
		if !strings.Contains(subjects, col) {
			t.Errorf("plan_subjects must carry %s", col)
		}
	}
	if !strings.Contains(subjects, "UNIQUE (plan_id, subject_id)") {
		t.Error("a plan must hold at most one row per subject — two would be two records of one decision, free to disagree")
	}
	if !strings.Contains(subjects, "snapshot_id  UUID REFERENCES desired_state_snapshots(id)") {
		t.Error("plan_subjects.snapshot_id must reference the snapshot, so the desired state and the fingerprint verifying it come from one row")
	}
	if !regexp.MustCompile(`(?is)ALTER TABLE propagation_outbox\s+ADD COLUMN IF NOT EXISTS plan_subject_id UUID REFERENCES plan_subjects\(id\)`).MatchString(up) {
		t.Error("the outbox must reference plan_subjects, not desired_state_snapshots directly (design §8)")
	}

	plans := createTableBody(t, up, "plans")
	if !strings.Contains(plans, "surface") {
		t.Error("plans.surface must record which rehearsal issued the plan — otherwise a drift-triage plan can be cited on the bulk-grant apply endpoint")
	}
	// A provisional plan is gated by re-fingerprinting on the target's return,
	// never by a clock: applying the ordinary lifetime to it would silently
	// discard an approved change whenever an outage outlasted it (design §8).
	lifetime := regexp.MustCompile(`(?is)CONSTRAINT plans_lifetime_check CHECK \((.*?)\n    \)`).FindStringSubmatch(plans)
	if lifetime == nil {
		t.Fatal("plans must constrain the relationship between provisional and expires_at")
	}
	for _, frag := range []string{"provisional AND expires_at IS NULL", "state_read_at IS NOT NULL", "NOT provisional AND expires_at IS NOT NULL"} {
		if !strings.Contains(lifetime[1], frag) {
			t.Errorf("plans_lifetime_check must assert %q", frag)
		}
	}

	// 1.6: plan expiry must never be able to reach a snapshot. With no ON
	// DELETE clause anywhere in the chain, deleting a plan whose subject rows
	// an outbox row still cites is refused, and a snapshot can never be dragged
	// along behind a plan.
	if stmts := stripSQLComments(up); strings.Contains(stmts, "ON DELETE CASCADE") || strings.Contains(stmts, "ON DELETE SET NULL") {
		t.Error("no foreign key in the plan chain may cascade or null out: snapshots are audit records that outlive the plan that produced them (task 1.6)")
	}

	if !strings.Contains(down, "DROP TABLE IF EXISTS plan_subjects") || !strings.Contains(down, "DROP TABLE IF EXISTS plans") {
		t.Error("down migration must drop both plan tables")
	}
	if !strings.Contains(down, "DROP COLUMN IF EXISTS plan_subject_id") {
		t.Error("down migration must drop the outbox's plan reference before dropping the table it points at")
	}
}

// 1.6 — no column on the plan or snapshot tables can hold a declared secret.
// Plans persist intent: who, on whom, against what state. A `secret_params`
// value rides the apply request and is discarded with it (design §5).
func TestPlanAndSnapshotTablesHoldNoSecretColumn(t *testing.T) {
	up, _ := addonMigrationSQL(t)

	forbidden := regexp.MustCompile(`(?i)\b\w*(password|passwd|secret|credential|plaintext|api_key|token)\w*\b`)
	for _, table := range []string{"plans", "plan_subjects", "desired_state_snapshots"} {
		body := createTableBody(t, up, table)
		if m := forbidden.FindString(body); m != "" {
			t.Errorf("%s declares column %q — a durable place a declared secret must never reach", table, m)
		}
	}
}

// 1.7 — the drift type stops naming its target inside its value, and the
// pending-dedupe index stops letting two targets suppress each other.
func TestDriftGainsTheTargetDimension(t *testing.T) {
	up, down := addonMigrationSQL(t)

	// The value rename, in the order 000025 established: the constraint comes
	// off before the UPDATE, because the old constraint forbids the new value
	// and the new one forbids the old rows.
	dropIdx := strings.Index(up, "ALTER TABLE drift_items DROP CONSTRAINT IF EXISTS drift_items_drift_type_check")
	updIdx := strings.Index(up, "UPDATE drift_items SET drift_type = 'target_only'")
	addIdx := strings.Index(up, "CHECK (drift_type IN ('target_only', 'syndra_only'))")
	if dropIdx < 0 || updIdx < 0 || addIdx < 0 {
		t.Fatal("the drift_type rename must drop the CHECK, move the rows, and re-add the CHECK")
	}
	if !(dropIdx < updIdx && updIdx < addIdx) {
		t.Error("drop CHECK -> UPDATE rows -> add CHECK; any other order fails against real data")
	}
	if regexp.MustCompile(`(?i)CHECK \(drift_type IN \([^)]*'zitadel_only'`).MatchString(up) {
		t.Error("'zitadel_only' names the target inside the value and is a false statement on any target that is not Zitadel")
	}

	// The dedupe index. NULLS NOT DISTINCT (Postgres 15) is load-bearing: an
	// add-on row's project_id and role_keys are NULL, and under the default
	// every re-detection would insert a fresh row and flood triage.
	idx := regexp.MustCompile(`(?is)CREATE UNIQUE INDEX idx_drift_items_pending_unique\s+ON drift_items \(([^)]*)\) NULLS NOT DISTINCT\s+WHERE status = 'pending_triage'`).FindStringSubmatch(up)
	if idx == nil {
		t.Fatal("the pending-dedupe index must be rebuilt with NULLS NOT DISTINCT so add-on rows dedupe")
	}
	for _, col := range []string{"target", "user_id", "project_id", "drift_type", "role_keys"} {
		if !strings.Contains(idx[1], col) {
			t.Errorf("dedupe index missing %s — two targets drifting on one user would silently suppress each other", col)
		}
	}

	// The repository's ON CONFLICT arbiter must name exactly the index columns,
	// or the upsert fails at runtime with "no unique or exclusion constraint
	// matching the ON CONFLICT specification".
	src, err := os.ReadFile("drift.go")
	if err != nil {
		t.Fatalf("read drift.go: %v", err)
	}
	if !strings.Contains(string(src), "ON CONFLICT (target, user_id, project_id, drift_type, role_keys) WHERE (status = 'pending_triage')") {
		t.Error("UpsertDriftItemWithEvidence's ON CONFLICT arbiter must match the rebuilt dedupe index")
	}

	// Same relaxation, same pairing, as the outbox.
	if !regexp.MustCompile(`(?is)ADD CONSTRAINT drift_items_zitadel_shape_check\s+CHECK \(target <> 'zitadel' OR \(project_id IS NOT NULL AND role_keys IS NOT NULL\)\)`).MatchString(up) {
		t.Error("drift_items' relaxed NOT NULLs must be paired with a CHECK keeping them mandatory for target='zitadel'")
	}

	// An exclusion is a statement about one target, so two targets holding the
	// same (user, project, role) must be excludable independently.
	if !strings.Contains(up, "ADD PRIMARY KEY (target, user_id, project_id, role_key)") {
		t.Error("external_grant_exclusions' primary key must include target")
	}

	if !strings.Contains(down, "UPDATE drift_items SET drift_type = 'zitadel_only' WHERE drift_type = 'target_only'") {
		t.Error("down migration must move the drift rows back")
	}
	if !strings.Contains(down, "ADD PRIMARY KEY (user_id, project_id, role_key)") {
		t.Error("down migration must restore the exclusions primary key")
	}
}

// 1.8 — the down migration refuses rather than reinterprets. Dropping `target`
// while a second target has rows turns a TrueNAS drift row into a Zitadel drift
// row that never happened, sitting in an operator's triage queue. Rolling back
// after a target is registered is disabling that target, not this migration
// (design §Rollback).
func TestDownMigrationRefusesToReinterpretForeignRows(t *testing.T) {
	_, down := addonMigrationSQL(t)

	if !strings.Contains(down, "RAISE EXCEPTION") {
		t.Fatal("the down migration must refuse when rows name a target other than zitadel")
	}
	for _, table := range []string{"propagation_outbox", "drift_items", "external_grant_exclusions", "desired_state_snapshots"} {
		if !regexp.MustCompile(`(?is)FROM ` + table + ` WHERE target <> 'zitadel'`).MatchString(down) {
			t.Errorf("the down-migration guard must count %s rows naming a non-zitadel target", table)
		}
	}
}

// 1.9 — direct_role_grants gains NO target column, and nothing reads or writes
// a non-zitadel direct grant. Direct grants are intents against Zitadel
// user_grants; add-on entitlements come from mappings and allowances, which
// have their own tables. A column no code path can populate is a column someone
// will later assume means something (design §3).
func TestDirectRoleGrantsHasNoTargetColumn(t *testing.T) {
	all := allUpMigrationsSQL(t)

	// Any ALTER TABLE direct_role_grants ... target, in any migration.
	for _, stmt := range regexp.MustCompile(`(?is)ALTER TABLE\s+direct_role_grants(.*?);`).FindAllStringSubmatch(all, -1) {
		if regexp.MustCompile(`(?i)\btarget\b`).MatchString(stmt[1]) {
			t.Errorf("direct_role_grants must not gain a target column; found: ALTER TABLE direct_role_grants%s", stmt[1])
		}
	}
	body := regexp.MustCompile(`(?is)CREATE TABLE\s+(?:IF NOT EXISTS\s+)?direct_role_grants\s*\((.*?)\n\);`).FindStringSubmatch(all)
	if body == nil {
		t.Fatal("could not isolate CREATE TABLE direct_role_grants")
	}
	if regexp.MustCompile(`(?im)^\s*target\b`).MatchString(body[1]) {
		t.Error("direct_role_grants must not declare a target column")
	}
}
