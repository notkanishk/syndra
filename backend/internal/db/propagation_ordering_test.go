package db

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// 1.20/1.21/1.26/1.27/2.45/2.46 — dispatch order and version rejection.
//
// `internal/db` has no live-database harness, so these are coherence guards:
// the properties live in the statements, and the statements are what is
// asserted. Each one names the failure it exists to catch, because a guard whose
// mutation is survivable is a guard that agrees with itself.

// The whole hazard in one test. Prioritising revocations is right at the target
// level and catastrophic at the row level: order by the row's own op_type and a
// revoke overtakes an OLDER grant for the same subject, so the grant lands
// afterwards and restores exactly the access being withdrawn — both rows
// applied, neither failed, nobody told.
func TestRevocationPriorityIsBySubjectAndNeverInvertsIntentOrder(t *testing.T) {
	src := readDBSource(t, "propagations.go")
	frag := constValue(t, src, "revocationFirst")

	if !regexp.MustCompile(`r\.user_id\s*=\s*p\.user_id`).MatchString(frag) {
		t.Error("priority must be decided per subject (r.user_id = p.user_id), or a revoke overtakes an older grant for its own subject")
	}
	if !regexp.MustCompile(`r\.op_type\s*=\s*'revoke'`).MatchString(frag) {
		t.Error("the priority signal must be the presence of a revocation, not the row's own type")
	}
	if regexp.MustCompile(`p\.op_type`).MatchString(frag) {
		t.Error("ordering on the CANDIDATE row's op_type is the inversion this exists to prevent")
	}
	// intent_seq is the tiebreak and must be the LAST key: any key after it can
	// reorder two rows of one subject.
	keys := strings.Split(frag, "DESC,")
	if len(keys) != 2 || strings.TrimSpace(keys[1]) != "p.intent_seq" {
		t.Fatalf("intent order must be the final, sole tiebreak; got trailing key %q", frag)
	}
	if !strings.Contains(funcBody(t, src, "ClaimPendingPropagations"), "ORDER BY ` + revocationFirst + `") {
		t.Error("the operator claim must use the shared ordering, or the two claims can disagree about dispatch order")
	}
}

// A version-rejected row is discarded, not attempted. Recording it as `failed`
// shows an operator a phantom failure on the row class where real failures
// matter most, and revocation-first dispatch makes it ordinary rather than rare.
func TestSupersededIsTerminatedWithoutDispatchAndIsNotAFailure(t *testing.T) {
	body := funcBody(t, readDBSource(t, "propagations.go"), "terminateSuperseded")

	if !strings.Contains(body, "status = 'superseded'") {
		t.Error("an overtaken row must terminate as 'superseded'")
	}
	if strings.Contains(body, "'failed'") {
		t.Error("'superseded' must be distinct from 'failed': nothing was attempted and nothing went wrong")
	}
	if !strings.Contains(body, "p.status IN ('pending', 'in_flight')") {
		t.Error("only unresolved rows may be terminated; a settled row is somebody else's record")
	}
	// Asserted as a whole conjunct, not as a substring: `IS NOT NULL OR TRUE`
	// contains the phrase and means the opposite, and it would let a row with no
	// approval — and therefore no version — be judged stale against somebody
	// else's.
	if !regexp.MustCompile(`(?m)^\s*AND p\.plan_subject_id IS NOT NULL$`).MatchString(body) {
		t.Error("rows carrying no approval carry no version, so they can never be judged stale")
	}
	if !strings.Contains(body, "($3::text = '' OR p.id::text = $3::text)") {
		t.Error("the targeted claim narrows the same statement to one row rather than settling the whole target")
	}
	// The discard and its record are one write, for the reason deregistration's
	// are: an approved change is being thrown away, the outbox row explaining it
	// is pruned after the retention window, and the person's timeline is where
	// the explanation has to survive.
	if !regexp.MustCompile(`(?s)WITH discarded AS \(.*RETURNING p\.id, p\.user_id\s*\)\s*INSERT INTO audit_logs`).MatchString(body) {
		t.Error("the audit row must be written by the same statement, or a discard can be recorded by nothing")
	}
	if !strings.Contains(body, `"entitlement."+target+".superseded"`) {
		t.Error("the audit action must name the target and the discard")
	}
}

// The predicate decides whether a LATER decision has already settled. Three
// things make that true, and dropping any one silently changes what it means.
func TestSupersedePredicateComparesSettledLaterVersionsForTheSamePair(t *testing.T) {
	pred := constValue(t, readDBSource(t, "propagations.go"), "supersededByLaterVersion")

	for _, want := range []struct{ frag, why string }{
		{"q.status = 'applied'", "an unsettled later row is not proof the target moved; the drain still dispatches both, in order"},
		{"qs.subject_id = s.subject_id", "without the subject, one person's newer state discards another person's queued change"},
		{"qs.target     = s.target", "without the target, a settled TrueNAS convergence would discard a queued Zitadel one"},
		{"qs.version    > s.version", "strictly greater: an equal version is the same decision, not a later one"},
	} {
		if !strings.Contains(pred, want.frag) {
			t.Errorf("supersede predicate is missing %q — %s", want.frag, want.why)
		}
	}
	if strings.Contains(pred, ">=") {
		t.Error("a row must not supersede itself")
	}
	// The version is read off the row's own approval chain. Taken as an
	// argument it could describe a decision that never existed.
	if !regexp.MustCompile(`ps\.id\s*=\s*p\.plan_subject_id`).MatchString(pred) {
		t.Error("the version must be resolved through the row's own plan subject, not supplied beside it")
	}
}

// Rejected WITHOUT dispatch means the rejection runs before the claim. A guard
// that ran after would have already handed the row to a dispatcher.
func TestEveryClaimRejectsStaleVersionsBeforeItClaims(t *testing.T) {
	src := readDBSource(t, "propagations.go")
	for _, fn := range []string{"ClaimPendingPropagations", "ClaimPropagationByID", "ClaimPendingRevocations"} {
		body := funcBody(t, src, fn)
		reject := strings.Index(body, "terminateSuperseded(ctx,")
		if reject < 0 {
			t.Errorf("%s must reject stale versions; every claim is a way to reach a dispatcher", fn)
			continue
		}
		query := strings.Index(body, "PG.Query")
		if query < 0 {
			t.Fatalf("%s: no query found to order against", fn)
		}
		if reject > query {
			t.Errorf("%s rejects stale versions after claiming; the row has already been dispatched by then", fn)
		}
	}
}

// The background runner's licence, stated in the statement. A caller cannot
// widen it, which is why the restriction is here rather than in the runner.
func TestRevocationClaimReturnsOnlyWithdrawalsAndNeverOvertakesOlderIntent(t *testing.T) {
	body := funcBody(t, readDBSource(t, "propagations.go"), "ClaimPendingRevocations")

	if !strings.Contains(body, "p.op_type = 'revoke'") {
		t.Fatal("the background claim must return only revocations: 'add' confers and 'replace' both confers and withdraws")
	}
	for _, conferring := range []string{"'add'", "'apply'"} {
		if regexp.MustCompile(`p\.op_type\s*(=|IN\s*\([^)]*)` + regexp.QuoteMeta(conferring)).MatchString(body) {
			t.Errorf("the background claim must never select %s: nobody gains access without an operator", conferring)
		}
	}
	guard := regexp.MustCompile(`(?s)NOT EXISTS \(.*?e\.intent_seq < p\.intent_seq\)`).FindString(body)
	if guard == "" {
		t.Fatal("a revocation must not be dispatched ahead of OLDER conferring intent for its subject; draining the grant later restores the access")
	}
	for _, frag := range []string{"e.user_id    = p.user_id", "e.op_type    IN ('add', 'replace')", "e.status     IN ('pending', 'in_flight')"} {
		if !strings.Contains(guard, frag) {
			t.Errorf("the intent-order guard is missing %q", frag)
		}
	}
	if !strings.Contains(guard, "e.project_id IS NOT DISTINCT FROM p.project_id") {
		t.Error("project comparison must be NULL-safe, or an add-on row's NULL project never matches and the guard silently never fires")
	}
}

// A new terminal status that nothing prunes is a row that lives forever, and a
// new terminal status the pending set absorbs is a queue that never empties.
// Both are read off the schema's own vocabulary rather than a list retyped here:
// the status added tomorrow is the one nobody adds in two places.
func TestRetentionAndThePendingSetAgreeWithTheStatusVocabulary(t *testing.T) {
	dir := findMigrationsDir(t)
	up, err := os.ReadFile(filepath.Join(dir, "000026_addon_platform_target_dimension.up.sql"))
	if err != nil {
		t.Fatalf("read migration: %v", err)
	}
	check := regexp.MustCompile(`CHECK \(status IN \(([^)]*)\)\)`).FindStringSubmatch(string(up))
	if check == nil {
		t.Fatal("could not read the outbox status vocabulary from migration 000026")
	}
	var terminal []string
	for _, raw := range strings.Split(check[1], ",") {
		s := strings.Trim(strings.TrimSpace(raw), "'")
		if s == "pending" || s == "in_flight" {
			continue
		}
		terminal = append(terminal, s)
	}
	sort.Strings(terminal)
	if len(terminal) < 4 {
		t.Fatalf("expected at least applied/failed/superseded/abandoned as terminal, got %v", terminal)
	}

	prune := funcBody(t, readDBSource(t, "propagations.go"), "PruneTerminalPropagations")
	pruned := regexp.MustCompile(`status IN \(([^)]*)\)`).FindStringSubmatch(prune)
	if pruned == nil {
		t.Fatal("the prune must name the statuses it deletes")
	}
	var got []string
	for _, raw := range strings.Split(pruned[1], ",") {
		got = append(got, strings.Trim(strings.TrimSpace(raw), "'"))
	}
	sort.Strings(got)
	if strings.Join(got, ",") != strings.Join(terminal, ",") {
		t.Errorf("retention must cover every terminal status: schema says %v, prune deletes %v", terminal, got)
	}

	// And the unresolved set must not absorb any of them: a superseded row
	// counted as pending is a queue depth that never comes down.
	for _, fn := range []string{"GetPendingPropagations", "CountPendingPropagations"} {
		body := funcBody(t, readDBSource(t, "propagations.go"), fn)
		for _, term := range terminal {
			if strings.Contains(body, "'"+term+"'") {
				t.Errorf("%s must not count terminal status %q as unresolved", fn, term)
			}
		}
	}
}

// constValue returns the raw text of a package-level `const name = ...` value,
// backtick-quoted or not.
func constValue(t *testing.T, src, name string) string {
	t.Helper()
	m := regexp.MustCompile(`(?s)const ` + regexp.QuoteMeta(name) + ` = ` + "`" + `(.*?)` + "`").FindStringSubmatch(src)
	if m == nil {
		t.Fatalf("const %s not found", name)
	}
	return m[1]
}
