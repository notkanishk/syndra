package db

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Migration coherence for the withdrawal declaration (§17).
//
// Three readers have to agree about one column, and none of them can be checked
// against a running database from here: the migration that creates it, the
// claim the background runner uses, and the surface that lists unconfirmed
// revocations. A drift between any two of them is the bug this whole change is
// closing — a lock nothing claims, or a lock nothing shows.
func TestTheWithdrawalDeclarationIsCoherentEndToEnd(t *testing.T) {
	dir := findMigrationsDir(t)
	raw, err := os.ReadFile(filepath.Join(dir, "000035_withdrawal_only_convergences.up.sql"))
	if err != nil {
		t.Fatalf("read migration: %v", err)
	}
	up := string(raw)

	// FALSE by default is the safe direction, and it is the whole of why adding
	// this column cannot widen what the runner dispatches: a row that forgets to
	// declare itself waits for an operator, exactly as before.
	if !strings.Contains(up, "withdraws_only BOOLEAN NOT NULL DEFAULT FALSE") {
		t.Error("withdraws_only must default FALSE: an undeclared row must wait for an operator, never be claimed")
	}
	// Only on an apply. A revoke is already a withdrawal by its op_type and an
	// add is already not one; a flag that could appear on either would be a
	// second, disagreeing answer to the same question.
	if !strings.Contains(up, "CHECK (NOT withdraws_only OR op_type = 'apply')") {
		t.Error("the flag must be confined to entitlement applies, or two columns answer the same question differently")
	}

	// The claim reads it, so a target revocation drains without an operator.
	claim := funcBody(t, readDBSource(t, "propagations.go"), "ClaimPendingRevocations")
	if !strings.Contains(claim, "runnerSends") || !strings.Contains(readDBSource(t, "propagations.go"), "OR p.withdraws_only)`") {
		t.Error("the background claim must read the declaration, or an add-on revocation is queued and nothing ever claims it")
	}
	// And the surface reads it, so one that has not drained is visible. The two
	// together are the property: a revocation is either draining or on a screen.
	src := readDBSource(t, "unconfirmed_revocations.go")
	if !strings.Contains(src, "p.withdraws_only") {
		t.Error("the unconfirmed-revocation surface must read the declaration, or a stuck lock appears nowhere")
	}
}

// §30 — a CHECK over nullable columns has to say what it means about NULL
// before it says anything else.
//
// 000039 wrote `btrim(resolved_by) <> ”` in the satisfied arm. With
// `resolved_by` NULL that term is NULL, the arm is NULL, the whole expression
// is `false OR NULL` = NULL — and a NULL CHECK PASSES. The constraint written
// to stop a finding closing itself permitted exactly that, and the partial
// unique index followed it: a self-closed row drops out of the open index and a
// second standing finding lands on the same account.
//
// 000038's own comment names this trap about STRICT. Writing it down did not
// stop it being written two migrations later, so this reads the SQL.
func TestAResolutionArmSaysWhatItMeansAboutNull(t *testing.T) {
	dir := findMigrationsDir(t)
	raw, err := os.ReadFile(filepath.Join(dir, "000040_a_resolution_cannot_close_itself.up.sql"))
	if err != nil {
		t.Fatalf("read migration: %v", err)
	}
	up := string(raw)

	// Comments stripped first, and whitespace collapsed. This migration's own
	// comment QUOTES the broken constraint to explain it, and the first version
	// of this guard read that quotation instead of the statement — the same
	// failure as a guard greping prose, in the guard written about it.
	var statements []string
	for _, line := range strings.Split(up, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "--") {
			continue
		}
		statements = append(statements, line)
	}
	sql := strings.Join(strings.Fields(strings.Join(statements, " ")), " ")
	if !strings.Contains(sql, "ADD CONSTRAINT binding_conflict_resolution_is_attributed") {
		t.Fatalf("the migration does not re-add the constraint; this guard is reading nothing: %q", sql)
	}

	// The whole statement, not "up to the first `)`" — which lands inside
	// `btrim(resolved_by)` and truncates the arm being examined.
	arm := between(t, sql, "ADD CONSTRAINT binding_conflict_resolution_is_attributed", ";")
	for _, col := range []string{"resolved_by", "resolution"} {
		if !strings.Contains(arm, col+" IS NOT NULL") {
			t.Errorf("the satisfied arm does not require %s IS NOT NULL — btrim(NULL) is NULL, "+
				"the arm evaluates to NULL, and a NULL CHECK passes", col)
		}
		if !strings.Contains(arm, "btrim("+col+")") {
			t.Errorf("and it must still refuse an empty %s, or a resolution can be attributed to a blank", col)
		}
	}
}
