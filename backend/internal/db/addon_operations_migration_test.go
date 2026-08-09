package db

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// Migration-coherence guards for addon_operations (change `addon-platform`,
// task 2.12). No live-DB harness in this package — see
// propagations_migration_test.go — so these assert the migration text and the
// Go source agree.

const addonOpsMigrationBase = "000027_addon_operations"

func addonOpsMigrationSQL(t *testing.T) (up, down string) {
	t.Helper()
	dir := findMigrationsDir(t)
	u, err := os.ReadFile(filepath.Join(dir, addonOpsMigrationBase+".up.sql"))
	if err != nil {
		t.Fatalf("read up migration: %v", err)
	}
	d, err := os.ReadFile(filepath.Join(dir, addonOpsMigrationBase+".down.sql"))
	if err != nil {
		t.Fatalf("read down migration: %v", err)
	}
	return string(u), string(d)
}

// addonOpsColumns pulls the column names out of the CREATE TABLE body, ignoring
// table-level constraints.
func addonOpsColumns(t *testing.T) []string {
	t.Helper()
	up, _ := addonOpsMigrationSQL(t)
	body := stripSQLComments(createTableBody(t, up, "addon_operations"))

	var cols []string
	depth := 0
	var cur strings.Builder
	flush := func() {
		line := strings.TrimSpace(cur.String())
		cur.Reset()
		if line == "" {
			return
		}
		first := strings.Fields(line)
		if len(first) == 0 {
			return
		}
		switch strings.ToUpper(first[0]) {
		case "CONSTRAINT", "CHECK", "UNIQUE", "PRIMARY", "FOREIGN":
			return
		}
		cols = append(cols, first[0])
	}
	for _, r := range body {
		switch r {
		case '(':
			depth++
		case ')':
			depth--
		case ',':
			if depth == 0 {
				flush()
				continue
			}
		}
		cur.WriteRune(r)
	}
	flush()
	sort.Strings(cols)
	return cols
}

// 2.11, 2.12 — the record table exists, resolves its target against the
// registry, and carries the columns the protocol needs.
func TestAddonOperationsRecordTable(t *testing.T) {
	up, _ := addonOpsMigrationSQL(t)
	body := stripSQLComments(createTableBody(t, up, "addon_operations"))

	if !strings.Contains(body, "REFERENCES targets(target)") {
		t.Error("addon_operations.target must resolve against the targets registry, " +
			"or a record could name a target the deployment never registered")
	}
	for _, want := range []string{"actor_id", "subject_id", "operation", "status", "created_at", "settled_at"} {
		if !strings.Contains(body, want) {
			t.Errorf("addon_operations is missing %q", want)
		}
	}
}

// 2.11, 2.12 — the load-bearing guard. There is no column able to hold a secret
// parameter value, and the way that is guaranteed is that the column list is
// closed rather than merely conventional.
//
// A free-text `failure_detail` or `response_body` is exactly where a future
// maintainer would put an add-on's error payload, and an add-on's error payload
// is the most likely place for a submitted password to be echoed back. This test
// fails on the column being added, not on the password arriving.
func TestAddonOperationsHasNoColumnThatCouldHoldASecret(t *testing.T) {
	got := addonOpsColumns(t)
	want := []string{"actor_id", "claimed_at", "created_at", "id", "operation", "settled_at", "status", "subject_id", "target"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("the addon_operations column set changed.\n got: %v\nwant: %v\n\n"+
			"Every column here lives beside a subject's identity forever and the table is written on the "+
			"path that carries a password. Adding one is a decision, not a detail: say why it cannot hold "+
			"a parameter value, then update this list.", got, want)
	}

	up, _ := addonOpsMigrationSQL(t)
	body := strings.ToLower(stripSQLComments(createTableBody(t, up, "addon_operations")))
	for _, banned := range []string{"json", "jsonb", "payload", "params", "parameter", "secret", "detail", "body", "response"} {
		if strings.Contains(body, banned) {
			t.Errorf("addon_operations mentions %q — a column shaped to hold arbitrary content is "+
				"where a secret ends up, whatever it is named", banned)
		}
	}
}

// 2.11 — the statuses Go writes and the statuses the database accepts are one
// list. A mismatch surfaces as a constraint violation on a dispatch that has
// already happened, which is the worst moment to discover a typo.
func TestAddonOperationStatusesMatchTheCheck(t *testing.T) {
	up, _ := addonOpsMigrationSQL(t)
	body := stripSQLComments(up)

	re := regexp.MustCompile(`(?is)status\s+TEXT\s+NOT\s+NULL\s+DEFAULT\s+'([a-z]+)'\s*CHECK\s*\(\s*status\s+IN\s*\(([^)]*)\)`)
	m := re.FindStringSubmatch(body)
	if m == nil {
		t.Fatal("could not find the status column's CHECK; the guard cannot verify what it cannot read")
	}
	if m[1] != AddonOpDispatching {
		t.Errorf("the column defaults to %q; a record must be born non-terminal, as %q", m[1], AddonOpDispatching)
	}

	var inCheck []string
	for _, part := range strings.Split(m[2], ",") {
		inCheck = append(inCheck, strings.Trim(strings.TrimSpace(part), "'"))
	}
	sort.Strings(inCheck)

	fromGo := append([]string(nil), AddonOperationStatuses...)
	sort.Strings(fromGo)

	if strings.Join(inCheck, ",") != strings.Join(fromGo, ",") {
		t.Fatalf("status vocabulary drifted.\nCHECK: %v\n   Go: %v", inCheck, fromGo)
	}
}

// 2.11 — terminal and settled are the same fact, in both directions. A terminal
// row with no settlement time cannot be aged; a `dispatching` row carrying one
// was settled and then reopened, which nothing may do.
func TestAddonOperationsTiesSettlementToTerminality(t *testing.T) {
	up, _ := addonOpsMigrationSQL(t)
	body := stripSQLComments(up)
	if !strings.Contains(body, "(status = 'dispatching') = (settled_at IS NULL)") {
		t.Error("addon_operations must constrain settled_at to be present exactly when the status is terminal")
	}
}

// 2.15 — the unresolved surface reads a predicate, and the predicate has an
// index. Without one the surface degrades into a full scan of every operation
// ever performed, which is the surface an operator opens during an incident.
func TestUnresolvedOperationsArePredicateIndexed(t *testing.T) {
	up, _ := addonOpsMigrationSQL(t)
	body := stripSQLComments(up)
	if !regexp.MustCompile(`(?is)CREATE\s+INDEX[^;]*ON\s+addon_operations[^;]*WHERE\s+status\s+IN\s*\(\s*'dispatching'\s*,\s*'indeterminate'\s*\)`).MatchString(body) {
		t.Error("the unresolved predicate must be indexed")
	}

	// And the Go list that names those states agrees with it.
	if strings.Join(AddonUnresolvedStatuses, ",") != AddonOpDispatching+","+AddonOpIndeterminate {
		t.Errorf("AddonUnresolvedStatuses = %v, which no longer matches the indexed predicate", AddonUnresolvedStatuses)
	}
}

// 2.16 — the summary query splits three ways and never folds unresolved into
// either of the others.
func TestCountsExcludeUnresolvedFromBothTotals(t *testing.T) {
	src, err := os.ReadFile("addon_operations.go")
	if err != nil {
		t.Fatalf("read addon_operations.go: %v", err)
	}
	q := funcBody(t, string(src), "CountAddonOperations")

	if !strings.Contains(q, "FILTER (WHERE status = 'succeeded')") {
		t.Error("the succeeded count must name exactly that status")
	}
	if !strings.Contains(q, "FILTER (WHERE status IN ('rejected', 'unreached'))") {
		t.Error("the failure count must be the two deterministic non-successes and nothing else")
	}
	if !strings.Contains(q, "FILTER (WHERE status IN ('dispatching', 'indeterminate'))") {
		t.Error("unresolved must be counted separately")
	}
	// The specific mistake worth naming: an unresolved row counted as a failure
	// tells a member to try again on a target that may already hold their new
	// credential, and counted as a success claims what nobody knows.
	for _, wrong := range []string{
		"status IN ('rejected', 'unreached', 'indeterminate')",
		"status <> 'succeeded'",
	} {
		if strings.Contains(q, wrong) {
			t.Errorf("the failure count absorbs unresolved rows via %q", wrong)
		}
	}
}

// 2.13 — the record is written before the call, and it is written with an audit
// row in the same transaction. Structural: both writes live in one function
// that takes a tx, so there is no arrangement of callers that commits one
// without the other.
func TestRecordAndAuditRowShareOneTransaction(t *testing.T) {
	src, err := os.ReadFile("addon_operations.go")
	if err != nil {
		t.Fatalf("read addon_operations.go: %v", err)
	}
	body := funcBody(t, string(src), "insertAddonOperation")

	if !strings.Contains(body, "INSERT INTO addon_operations") {
		t.Error("insertAddonOperation must write the record")
	}
	if !regexp.MustCompile(`(?i)INSERT\s+INTO\s+audit_logs`).MatchString(body) {
		t.Error("insertAddonOperation must write the audit row alongside it — a mutation with no trace " +
			"is the thing the pre-dispatch record exists to prevent")
	}
	if strings.Contains(body, "PG.Exec") || strings.Contains(body, "PG.QueryRow") {
		t.Error("both writes must go through the passed transaction, or one can commit without the other")
	}
}

// 2.13 — a terminal status may be written once, and only over a non-terminal
// one. Without the status predicate a duplicated settle could overwrite
// `indeterminate` with `succeeded`, resolving on no evidence the exact question
// the unresolved surface exists to raise.
func TestSettleOnlyMovesARowOffDispatching(t *testing.T) {
	src, err := os.ReadFile("addon_operations.go")
	if err != nil {
		t.Fatalf("read addon_operations.go: %v", err)
	}
	body := funcBody(t, string(src), "SettleAddonOperation")

	if !strings.Contains(body, "AND status = 'dispatching'") {
		t.Fatal("the settle must be conditional on the row still awaiting an outcome")
	}
	if !strings.Contains(body, "RowsAffected() == 0") {
		t.Error("a settle that matched no row must be reported, not silently succeed")
	}
}

// 2.12 — the down migration refuses while any operation is unresolved. Those
// rows are the only surviving evidence that a secret-bearing call may have
// applied and nobody knows whether it did; the parameters were never retained,
// so nothing can reconstruct them.
func TestDownMigrationRefusesToDropUnresolvedOperations(t *testing.T) {
	_, down := addonOpsMigrationSQL(t)
	body := stripSQLComments(down)

	if !strings.Contains(body, "RAISE EXCEPTION") {
		t.Fatal("the down migration must refuse rather than silently drop unresolved operations")
	}
	if !regexp.MustCompile(`(?is)WHERE\s+status\s+IN\s*\(\s*'dispatching'\s*,\s*'indeterminate'\s*\)`).MatchString(body) {
		t.Error("the refusal must be scoped to the unresolved statuses; settled rows are history and may go")
	}
	if !strings.Contains(body, "DROP TABLE IF EXISTS addon_operations") {
		t.Error("the down migration must still drop the table once nothing is open")
	}
}

// P1 — the record that authorises a dispatch is CLAIMED, not read. A read can
// be repeated: two callers could obtain the same record concurrently, and a
// caller could re-read a settled record and dispatch under it again. A single
// conditional UPDATE has exactly one winner.
//
// The call's identity is in the predicate rather than compared afterwards, so a
// mismatched attempt consumes nothing and the legitimate dispatch behind it is
// unharmed.
func TestTheDispatchRecordIsClaimedAtomically(t *testing.T) {
	src, err := os.ReadFile("addon_operations.go")
	if err != nil {
		t.Fatalf("read addon_operations.go: %v", err)
	}
	body := funcBody(t, string(src), "ClaimAddonOperation")

	if !strings.Contains(body, "UPDATE addon_operations") || !strings.Contains(body, "SET claimed_at = NOW()") {
		t.Fatal("the record must be claimed by an UPDATE; a SELECT can be repeated, and a capability that " +
			"can be obtained twice authorises two dispatches under one record")
	}
	if strings.Contains(body, "SELECT id, target") {
		t.Error("the claim must not be a read")
	}
	for _, guard := range []string{
		"AND status = 'dispatching'",
		"AND claimed_at IS NULL",
		"AND target = $2",
		"AND operation = $3",
		"AND subject_id = $4",
	} {
		if !strings.Contains(body, guard) {
			t.Errorf("the claim predicate is missing %q — without it a record could authorise a call it does not describe, "+
				"or authorise one twice", guard)
		}
	}
	if !strings.Contains(body, "ErrAddonOperationNotOpen") {
		t.Error("a record that cannot be claimed must be reported as its own condition, not as a generic scan error")
	}
}

// P1 — the claim is what separates two failures that `dispatching` alone
// conflates. A claimed row that never settled may have applied to the target; an
// unclaimed one definitely did not, because nothing was sent. The unresolved
// surface ages from the claim where there is one, since that is when the call
// actually started.
func TestUnresolvedAgesFromTheClaimWhereThereIsOne(t *testing.T) {
	src, err := os.ReadFile("addon_operations.go")
	if err != nil {
		t.Fatalf("read addon_operations.go: %v", err)
	}
	body := funcBody(t, string(src), "ListUnresolvedAddonOperations")
	if !strings.Contains(body, "COALESCE(claimed_at, created_at)") {
		t.Error("a dispatch that waited in the process before being sent would otherwise be aged from " +
			"when its record was written rather than from when the call began")
	}
}

// P1 — no migration may add a column to addon_operations outside the closed
// list, including a later one. Without this, the column guard reads only the
// CREATE TABLE and a future ALTER could add a free-text column it never sees.
func TestNoLaterMigrationAltersTheRecordTable(t *testing.T) {
	all := stripSQLComments(allUpMigrationsSQL(t))
	re := regexp.MustCompile(`(?is)ALTER\s+TABLE\s+(?:IF\s+EXISTS\s+)?addon_operations`)
	if re.MatchString(all) {
		t.Fatal("a migration alters addon_operations. Every column on this table lives beside a subject's " +
			"identity forever and the table is written on the path that carries a password: add it to the " +
			"CREATE TABLE and to the closed list in TestAddonOperationsHasNoColumnThatCouldHoldASecret, " +
			"so the guard can see it.")
	}
}
