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

// §29's surface — the finding's schema and the code that reads it.
//
// `internal/db` has no live-DB harness, so what is asserted is that the
// migration and the Go layer agree about the two properties that make a
// standing finding usable: it is one per account, and a resolution is a
// decision with an owner.
func TestABindingConflictIsOnePerAccountAndAResolutionHasAnOwner(t *testing.T) {
	dir := findMigrationsDir(t)
	raw, err := os.ReadFile(filepath.Join(dir, "000039_binding_conflicts.up.sql"))
	if err != nil {
		t.Fatalf("read migration: %v", err)
	}
	up := string(raw)

	// One standing finding per account. A re-drive re-detects the same
	// disagreement, and stacking rows turns one problem into a growing list of
	// the same problem — which reads as it getting worse.
	if !strings.Contains(up, "idx_binding_conflict_open") ||
		!strings.Contains(up, "WHERE resolved_at IS NULL") {
		t.Error("the open-finding index must be partial on unresolved rows, or a re-drive stacks duplicates")
	}
	// And a resolved row without an actor is a finding that closed itself.
	if !strings.Contains(up, "binding_conflict_resolution_is_attributed") {
		t.Error("a resolution must carry who decided and what they decided, together or not at all")
	}
	// A subject cannot conflict with themselves: that is a detector bug, and
	// rendering it puts a person's name on a screen for no reason.
	if !strings.Contains(up, "binding_conflict_is_between_two_subjects") {
		t.Error("the two claimants must be constrained to differ")
	}

	// The upsert has to match the partial index, or the idempotency it claims
	// is a unique violation at runtime instead.
	src := readDBSource(t, "binding_conflicts.go")
	if !strings.Contains(src, "ON CONFLICT (target, username) WHERE resolved_at IS NULL DO NOTHING") {
		t.Error("the insert must name the partial index it relies on, or a re-drive raises instead of no-opping")
	}
	// The losing binding is deleted before the winning one is written: the
	// unique index on (target, username) refuses the insert while the loser
	// still holds it.
	resolve := funcBody(t, src, "ResolveBindingConflict")
	del := strings.Index(resolve, "DELETE FROM target_account_bindings")
	ins := strings.Index(resolve, "INSERT INTO target_account_bindings")
	if del < 0 || ins < 0 || del > ins {
		t.Error("the losing binding must be forgotten before the winning one is written")
	}
}
