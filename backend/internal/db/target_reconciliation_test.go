package db

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"testing"
)

// 1.14 — the reason is written to a row an operator reads. A closed set checked
// at compile time, not a slice a package could widen and not free text a caller
// could fill with whatever it happens to be holding.
func TestUnreconciledReasonIsAClosedVocabulary(t *testing.T) {
	src := readDBSource(t, "target_reconciliation.go")

	// The set is read out of the declarations rather than retyped here. A hand
	// written list makes the test pass by agreeing with itself: the reason
	// added tomorrow is the one nobody thinks to add in two places, and the
	// gap it leaves is a constant the write refuses at runtime — the sweep's
	// own stubs would never notice.
	declared := regexp.MustCompile(`(?m)^\tUnreconciled\w+\s*=\s*"([^"]+)"`).FindAllStringSubmatch(src, -1)
	if len(declared) < 4 {
		t.Fatalf("expected to find every declared reason; found %d", len(declared))
	}
	for _, d := range declared {
		if !validUnreconciledReason(d[1]) {
			t.Errorf("%q is declared as a reason and must be accepted by the write", d[1])
		}
		// And the refusal has to name the same set it enforces, or it sends a
		// caller looking for a value that is in fact allowed.
		if !strings.Contains(ErrUnreconciledReason.Error(), d[1]) {
			t.Errorf("the refusal must name %q among the reasons it allows: %v", d[1], ErrUnreconciledReason)
		}
	}
	for _, r := range []string{"", "offline", "TARGET_UNREACHABLE", "hunter2"} {
		if validUnreconciledReason(r) {
			t.Errorf("%q is not part of the vocabulary and must be refused", r)
		}
	}

	if !regexp.MustCompile(`(?s)func validUnreconciledReason\(r string\) bool \{\s*switch r \{`).MatchString(src) {
		t.Error("membership must be a switch over the constants — an exported slice is a vocabulary any package can widen before the write runs")
	}
	if regexp.MustCompile(`(?m)^var [A-Z]\w*\s*=\s*\[\]`).MatchString(src) {
		t.Error("no exported package-level slice may stand in for the vocabulary")
	}
}

// The refusals come before anything is opened — PG is nil here, so a guard
// placed after the query would panic rather than return. And the reason refusal
// names the vocabulary, never the value: a caller that reached here holding
// something it should not must not be able to write it into the error either.
func TestReconciliationWritesRefuseBeforeTouchingAnything(t *testing.T) {
	if _, err := MarkTargetUnreconciled(context.Background(), "", UnreconciledUnreachable); !errors.Is(err, ErrNoSuchTarget) {
		t.Fatalf("an unnamed target must be refused, got %v", err)
	}
	if _, err := MarkTargetReconciled(context.Background(), ""); !errors.Is(err, ErrNoSuchTarget) {
		t.Fatalf("an unnamed target must be refused, got %v", err)
	}
	secret := "correct-horse-battery-staple"
	_, err := MarkTargetUnreconciled(context.Background(), "zitadel", secret)
	if !errors.Is(err, ErrUnreconciledReason) {
		t.Fatalf("a reason outside the vocabulary must be refused, got %v", err)
	}
	if strings.Contains(err.Error(), secret) {
		t.Errorf("the refusal must name the vocabulary, not the value it rejected: %v", err)
	}
}

// An outage produces one sweep per tick. Restamping the start on each of them
// would hold the outage permanently one tick old, and the number an operator
// uses to tell a blip from a week would never grow.
func TestAnOutageKeepsItsStartTime(t *testing.T) {
	body := funcBody(t, readDBSource(t, "target_reconciliation.go"), "MarkTargetUnreconciled")
	if !regexp.MustCompile(`unreconciled_since\s*=\s*COALESCE\(target_reconciliation\.unreconciled_since, NOW\(\)\)`).MatchString(body) {
		t.Error("a repeated mark must keep the existing start — in the statement, not by a read-then-write in the caller")
	}
	if !strings.Contains(body, "unreconciled_reason = EXCLUDED.unreconciled_reason") {
		t.Error("the reason may change without the period restarting: unreachable becoming stale is not coming back")
	}
}

// A target read but still flagged unreconciled, or a flag cleared with no read
// behind it, both say something untrue about the same moment — and the second
// reports confidence Syndra does not have.
func TestReturningRecordsTheReadAndEndsThePeriodTogether(t *testing.T) {
	body := funcBody(t, readDBSource(t, "target_reconciliation.go"), "MarkTargetReconciled")
	stmt := regexp.MustCompile(`(?s)ON CONFLICT \(target\) DO UPDATE SET(.*?)RETURNING`).FindStringSubmatch(body)
	if stmt == nil {
		t.Fatal("could not isolate the upsert")
	}
	for _, want := range []string{"last_current_read_at = NOW()", "unreconciled_since   = NULL", "unreconciled_reason  = NULL"} {
		if !strings.Contains(stmt[1], want) {
			t.Errorf("the stamp and the clear must be one statement; %q missing", want)
		}
	}
}

func TestReconciliationSchemaHoldsTheRowTogether(t *testing.T) {
	up, down := addonMigrationSQL(t)
	body := createTableBody(t, up, "target_reconciliation")

	if !strings.Contains(body, "target               TEXT PRIMARY KEY REFERENCES targets(target)") {
		t.Error("one row per registered target, keyed by it")
	}
	// NULL is a third state, not a very old timestamp: never reconciled, long
	// ago, and a minute ago are three different things to an operator.
	if regexp.MustCompile(`last_current_read_at\s+TIMESTAMPTZ\s+NOT NULL`).MatchString(body) {
		t.Error("last_current_read_at must be nullable — a target never read is not one read long ago")
	}
	if !strings.Contains(body, "CONSTRAINT target_reconciliation_reason_check") {
		t.Error("a reason without a period, or a period without a reason, is a row that cannot be rendered")
	}
	// Compared whole rather than by fragment. Containment cannot tell a
	// constraint from a constraint with a tautology in front of it: `CHECK
	// (TRUE OR <the pairing>)` contains every fragment and forbids nothing.
	const wantCheck = "(unreconciled_since IS NULL AND unreconciled_reason IS NULL) OR " +
		"(unreconciled_since IS NOT NULL AND unreconciled_reason IS NOT NULL)"
	got := normaliseSQL(balancedAfter(t, up, "CONSTRAINT target_reconciliation_reason_check CHECK "))
	if got != wantCheck {
		t.Errorf("the pairing check must be exactly the pairing:\n  want %s\n  got  %s", wantCheck, got)
	}

	// Rebuilt by the next sweep, so the rollback may drop it — but only after
	// the guard that refuses the rollback outright has had its say.
	drop := strings.Index(down, "DROP TABLE IF EXISTS target_reconciliation;")
	guard := strings.Index(down, "refusing to drop the target dimension")
	if drop < 0 {
		t.Fatal("the down migration must drop the table it created")
	}
	if guard < 0 || drop < guard {
		t.Error("the drop must come after the refusal guard, so a refused rollback leaves the record intact")
	}
}

// normaliseSQL collapses whitespace and drops line comments so an expression
// can be compared as written rather than as formatted.
func normaliseSQL(s string) string {
	var b strings.Builder
	for _, line := range strings.Split(s, "\n") {
		if i := strings.Index(line, "--"); i >= 0 {
			line = line[:i]
		}
		b.WriteString(" ")
		b.WriteString(line)
	}
	return strings.Join(strings.Fields(b.String()), " ")
}
