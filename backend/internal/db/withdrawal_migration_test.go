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
	if !strings.Contains(claim, "p.withdraws_only") {
		t.Error("the background claim must read the declaration, or an add-on revocation is queued and nothing ever claims it")
	}
	// And the surface reads it, so one that has not drained is visible. The two
	// together are the property: a revocation is either draining or on a screen.
	src := readDBSource(t, "unconfirmed_revocations.go")
	if !strings.Contains(src, "p.withdraws_only") {
		t.Error("the unconfirmed-revocation surface must read the declaration, or a stuck lock appears nowhere")
	}
}
