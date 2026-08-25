package db

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"
)

// 1.12 — a finding has to say what it looked at, and the statement has to be
// what says it. The column default is gone (000026), so an INSERT that omits
// the target now fails; before that, it produced a plausible wrong row.
func TestDriftWritesNameTheirTarget(t *testing.T) {
	src := readDBSource(t, "drift.go")

	cols := splitList(balancedAfter(t, src, "INSERT INTO drift_items "))
	if len(cols) == 0 || cols[0] != "target" {
		t.Fatalf("the drift INSERT must name target first, matching its parameter order; got %v", cols)
	}
	vals := splitList(balancedAfter(t, src, "VALUES "))
	if len(vals) != len(cols) {
		t.Fatalf("column list (%d) and value list (%d) must have the same arity:\n  %v\n  %v", len(cols), len(vals), cols, vals)
	}
	// A literal here would be the same mistake as the default, only harder to
	// see: every detector would file against whichever target was written in.
	if vals[0] != "$1" {
		t.Errorf("target must be bound to a parameter, not written as a literal; got %q", vals[0])
	}
	if conflict := regexp.MustCompile(`(?is)ON CONFLICT \((.*?)\)`).FindStringSubmatch(src); conflict == nil ||
		!strings.Contains(conflict[1], "target") {
		t.Error("the pending-dedupe arbiter must name target, or two targets drifting on one user suppress each other")
	}
}

// The refusal comes before anything is opened: PG is nil in a unit test, so a
// guard that ran after the query would panic here rather than return.
func TestUpsertDriftItemRefusesAFindingWithNoTarget(t *testing.T) {
	_, _, err := UpsertDriftItem(context.Background(), "", "u1", "p1", []string{"viewer"}, "", "reconciliation_sweep", "target_only")
	if !errors.Is(err, ErrDriftTargetRequired) {
		t.Fatalf("an untargeted finding must be refused with ErrDriftTargetRequired, got %v", err)
	}
	_, _, err = UpsertDriftItemWithEvidence(context.Background(), "", "u1", "p1", nil, "", "webhook", "target_only", DriftEvidence{})
	if !errors.Is(err, ErrDriftTargetRequired) {
		t.Fatalf("the evidence-carrying form must refuse it too, got %v", err)
	}
}

// The whole reason the defaults come off: an INSERT that forgets the target is
// a statement nobody reviews as wrong. With no default it fails outright — but
// only against a live database, and this package has no live-DB harness, so the
// column list is asserted here instead.
func TestEveryOutboxWriterNamesItsTarget(t *testing.T) {
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	seen := 0
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		src := readDBSource(t, f)
		for _, m := range regexp.MustCompile(`(?is)INSERT INTO propagation_outbox\s*\((.*?)\)`).FindAllStringSubmatch(src, -1) {
			seen++
			if !contains(splitList(m[1]), "target") {
				t.Errorf("%s: an outbox INSERT omits target — with the default dropped this write fails, and with it present it would have queued add-on work down the Zitadel path\n  columns: %s", f, m[1])
			}
		}
	}
	if seen < 4 {
		t.Fatalf("expected to inspect every outbox writer; found only %d", seen)
	}
}

func TestTargetColumnsCarryNoDefault(t *testing.T) {
	up, _ := addonMigrationSQL(t)
	for _, table := range []string{"propagation_outbox", "drift_items", "external_grant_exclusions"} {
		add := regexp.MustCompile(`(?is)ALTER TABLE ` + table + `\s+ADD COLUMN IF NOT EXISTS target TEXT NOT NULL DEFAULT 'zitadel'`)
		drop := regexp.MustCompile(`(?is)ALTER TABLE ` + table + ` ALTER COLUMN target DROP DEFAULT`)
		if !add.MatchString(up) {
			t.Errorf("%s: the target column must be added with a default so existing rows backfill", table)
		}
		if !drop.MatchString(up) {
			t.Errorf("%s: the default must be dropped again — kept, it answers 'which target?' on behalf of every statement that forgot to say, and it answers zitadel", table)
		}
		if add.FindStringIndex(up)[0] > drop.FindStringIndex(up)[0] {
			t.Errorf("%s: the backfilling default must be added before it is dropped", table)
		}
	}
}

// Drift on one target must not appear under another. That is a property of the
// read as much as of the write: a listing that does not select the target
// cannot report it, and one that cannot filter on it returns every target's
// findings to a caller that asked about one.
func TestDriftReadsCarryAndFilterTheTarget(t *testing.T) {
	src := readDBSource(t, "drift.go")
	for _, fn := range []string{"GetDriftItems", "GetDriftItem"} {
		body := funcBody(t, src, fn)
		if !regexp.MustCompile(`(?is)SELECT id, target,`).MatchString(body) {
			t.Errorf("%s must select the target immediately after the id, matching the scan order", fn)
		}
	}
	if !regexp.MustCompile(`(?is)\$\d+ = '' OR target = \$\d+`).MatchString(funcBody(t, src, "GetDriftItems")) {
		t.Error("GetDriftItems must narrow by target when one is given")
	}
	if !regexp.MustCompile(`rows\.Scan\(&d\.ID, &d\.Target,`).MatchString(src) {
		t.Error("the scan must read the target in the position the SELECT returns it — a silent column shift here mislabels every row")
	}
}

// The handler picks the enriched queue by asking whether the filter narrows
// anything, so a field this misses is a field that narrows the query without
// reaching the branch: the caller asks about one target and is handed the whole
// queue, believing it was scoped.
func TestDriftFilterEmptyConsidersEveryField(t *testing.T) {
	if !(DriftFilter{}).Empty() {
		t.Fatal("a zero filter narrows nothing")
	}
	v := reflect.ValueOf(&DriftFilter{}).Elem()
	for i := 0; i < v.NumField(); i++ {
		f := v.Type().Field(i)
		if f.Type.Kind() != reflect.String {
			t.Fatalf("%s is not a string; Empty's whole-struct comparison needs re-reading, not just extending", f.Name)
		}
		probe := DriftFilter{}
		reflect.ValueOf(&probe).Elem().Field(i).SetString("x")
		if probe.Empty() {
			t.Errorf("a filter narrowed by %s must not report itself empty", f.Name)
		}
	}
}

// The scope has to survive the read as data, not just as a WHERE clause.
// services.IsExcluded matches on the target it is handed, so a row that comes
// back with an empty one matches nothing — every exclusion silently stops
// working and the triage queue fills with grants somebody already decided
// about. The predicate and the projection have to move together.
func TestExclusionRowsCarryTheTargetTheyScope(t *testing.T) {
	body := funcBody(t, readDBSource(t, "exclusions.go"), "GetExclusions")
	if !regexp.MustCompile(`(?is)SELECT target, user_id`).MatchString(body) {
		t.Error("GetExclusions must project the target, or IsExcluded's target match can never succeed")
	}
	if !regexp.MustCompile(`rows\.Scan\(&e\.Target, &e\.UserID`).MatchString(body) {
		t.Error("the scan must read the target in the position the SELECT returns it")
	}
	if !regexp.MustCompile(`(?is)WHERE target = \$1`).MatchString(body) {
		t.Error("GetExclusions must scope its read by target — an unscoped read lets one target's exclusion silence another's drift")
	}
}

// "Is something already queued that will fix this drift?" is a question about
// one target. Answered from another target's queue, the sweep suppresses a
// replay it should have made and reports itself satisfied.
func TestQueuedWorkLookupIsScopedToItsTarget(t *testing.T) {
	body := funcBody(t, readDBSource(t, "propagations.go"), "PendingOutboxAddExists")
	if !regexp.MustCompile(`(?is)WHERE op_type='add' AND target=\$1`).MatchString(body) {
		t.Error("PendingOutboxAddExists must narrow by target in the predicate, not in the caller's head")
	}
	if !regexp.MustCompile(`querier\(ctx\)\.QueryRow\(ctx, q, target, userID, projectID, roleKey\)`).MatchString(body) {
		t.Error("the target must be bound first, matching $1 in the predicate")
	}
}

func readDBSource(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile(name)
	if err != nil {
		t.Fatalf("read %s: %v", name, err)
	}
	return string(b)
}

// balancedAfter returns the contents of the parenthesised group that follows
// the first occurrence of prefix, matched by depth rather than by a lazy regex:
// NULLIF($5,”) closes a paren the regex would mistake for the end of the list,
// which is how a guard reads six values where the statement writes ten.
func balancedAfter(t *testing.T, src, prefix string) string {
	t.Helper()
	i := strings.Index(src, prefix)
	if i < 0 {
		t.Fatalf("could not find %q in source", prefix)
	}
	open := strings.Index(src[i:], "(")
	if open < 0 {
		t.Fatalf("no parenthesised list after %q", prefix)
	}
	open += i
	depth := 0
	for j := open; j < len(src); j++ {
		switch src[j] {
		case '(':
			depth++
		case ')':
			if depth--; depth == 0 {
				return src[open+1 : j]
			}
		}
	}
	t.Fatalf("unbalanced parentheses after %q", prefix)
	return ""
}

// splitList splits a SQL column or value list on its top-level commas only.
func splitList(s string) []string {
	out := []string{}
	depth, start := 0, 0
	parts := []string{}
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '(':
			depth++
		case ')':
			depth--
		case ',':
			if depth == 0 {
				parts = append(parts, s[start:i])
				start = i + 1
			}
		}
	}
	parts = append(parts, s[start:])
	for _, p := range parts {
		p = strings.TrimSpace(p)
		// Drop SQL comments and blank continuation lines the column list spans.
		if i := strings.Index(p, "--"); i >= 0 {
			p = strings.TrimSpace(p[:i])
		}
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func contains(hay []string, needle string) bool {
	for _, h := range hay {
		if h == needle {
			return true
		}
	}
	return false
}

// A resolution whose side effects are Zitadel-shaped — a direct_role_grants row
// keyed by zitadel_project_id, a revoke outbox row bound to the Zitadel
// dispatcher — must not be reachable from a finding on another target. It would
// mutate one system while marking the other's finding resolved, and the finding
// would be gone.
func TestZitadelOnlyResolutionsRefuseAnotherTargetsFinding(t *testing.T) {
	if err := unsupportedTarget(TargetZitadel, "truenas"); !errors.Is(err, ErrDriftTargetUnsupported) {
		t.Fatalf("a Zitadel-only resolution must refuse an add-on finding, got %v", err)
	}
	if err := unsupportedTarget(TargetZitadel, TargetZitadel); err != nil {
		t.Fatalf("it must still resolve its own target's findings, got %v", err)
	}
	// Mark external writes an exclusion carrying the drift row's own target, so
	// it says something true whichever target that is.
	if err := unsupportedTarget("", "truenas"); err != nil {
		t.Fatalf("a target-generic resolution must accept any target, got %v", err)
	}
	// Distinct from a lost race: those tell the operator opposite things.
	if errors.Is(unsupportedTarget(TargetZitadel, "truenas"), ErrDriftNotPending) {
		t.Error("an unsupported target must not be reported as a lost triage race — retrying would never work")
	}
}

// The requirement lives in the claim because both callers are exported, and an
// invariant a caller enforces is one the next caller can skip.
func TestEachResolutionDeclaresWhatItCanActOn(t *testing.T) {
	src := readDBSource(t, "drift.go")
	for _, want := range []struct{ fn, requires string }{
		{"AttributeDriftTx", "TargetZitadel"},
		{"RevokeDriftAndEnqueue", "TargetZitadel"},
		{"MarkDriftExternalTx", `""`},
	} {
		body := funcBody(t, src, want.fn)
		re := regexp.MustCompile(`claimDriftTx\(ctx, tx, driftID, ` + regexp.QuoteMeta(want.requires) + `,`)
		if !re.MatchString(body) {
			t.Errorf("%s must claim with requireTarget=%s, so the target it can act on is stated by the claim rather than assumed by the call site", want.fn, want.requires)
		}
	}
	if !regexp.MustCompile(`(?s)func claimDriftTx\(.*?unsupportedTarget\(requireTarget, target\)`).MatchString(src) {
		t.Error("claimDriftTx must consult unsupportedTarget — a requireTarget it accepts and never checks is worse than no parameter")
	}
}
