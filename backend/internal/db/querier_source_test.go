package db

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// Every statement in this package runs through `querier(ctx)` (§17).
//
// A source guard, because the failure it prevents is invisible to every test in
// the repository. A statement written against `PG` directly is correct-looking,
// compiles, and passes — and inside an access-mutation transaction it reaches
// past that transaction, with two consequences that both commit silently:
//
//   - A READ answers from the world before the caller's own uncommitted write.
//     That is how the lifecycle trigger came to queue "disable this account"
//     for somebody who had just gained their first mapped role: the resolver
//     read the pool, and on the pool the role was not there yet.
//   - A WRITE commits on its own and survives the caller's rollback. A mapping
//     edit landed that way while the convergences meant to follow it did not.
//
// Neither is reachable by a test that fakes this layer, and both are one
// forgotten `PG.` away. So the rule is enforced on the source rather than
// remembered per function.
func TestEveryStatementRunsOnTheAmbientTransaction(t *testing.T) {
	// The exceptions, each with the reason it is one. They are transaction
	// BOUNDARIES rather than statements: a boundary that joined an ambient
	// transaction would inherit its rollback, and for these three that is the
	// bug rather than the fix.
	allowed := map[string][]string{
		// The seam InTx itself opens through, and the thing this guard is about.
		"tx.go": {"PG.Begin", "return PG"},
		// A session-scoped advisory lock needs its own connection held for the
		// duration; taking one from the ambient transaction would release it at
		// that transaction's commit rather than at the drain's end.
		"propagations.go": {"PG.Acquire"},
		// Deliberately independent of the caller: the record must survive a
		// dispatch that dies mid-call, which is exactly when the caller's
		// transaction rolls back. A record written and then rolled back is a
		// dispatch with no trace, which is the one thing it exists to prevent.
		"addon_operations.go": {"PG.Begin"},
		// Top-level operations with no caller transaction to join, each of which
		// commits a decision the caller is then told about.
		"log_anchor.go": {"PG.Begin"},
		"plans.go":      {"PG.Begin"},
	}

	direct := regexp.MustCompile(`\bPG\.\w+`)
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("read package dir: %v", err)
	}
	// Counted, because a source-reading guard's failure mode is passing on
	// nothing. This one walks a directory and greps: if the walk found no files,
	// or the package stopped using the accessor entirely, every assertion below
	// is vacuously satisfied and the guard reports success having examined
	// nothing. That is the same shape as the deploy check that passed with a
	// dead route sitting on the target.
	examined, uses := 0, 0
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		raw, err := os.ReadFile(filepath.Clean(name))
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		examined++
		uses += strings.Count(string(raw), "querier(ctx)")
		for _, line := range strings.Split(string(raw), "\n") {
			for _, use := range direct.FindAllString(line, -1) {
				if permitted(allowed[name], use) {
					continue
				}
				t.Errorf("%s reaches the pool directly (%s); use querier(ctx) so the statement joins the ambient transaction:\n\t%s",
					name, use, strings.TrimSpace(line))
			}
		}
	}
	if examined < 20 {
		t.Fatalf("only %d source files examined; this guard is reading the wrong directory and would pass on anything", examined)
	}
	if uses < 100 {
		t.Fatalf("only %d querier(ctx) call sites found, against 126 when this was written — "+
			"either the accessor was renamed or the walk missed the package, and either way "+
			"this guard is no longer watching what it claims to", uses)
	}
}

func permitted(allowed []string, use string) bool {
	for _, a := range allowed {
		if strings.Contains(a, use) || strings.Contains(use, a) {
			return true
		}
	}
	return false
}

// A parameter's Go type and the type the SQL gives it have to agree (§23).
//
// Found on the deployment: `($1 || ' days')::interval` makes $1 TEXT, pgx has
// no plan for encoding a Go int as text, and every prune since this was written
// failed with "unable to encode 30 into text format" — logged as non-fatal, so
// the outbox simply never pruned. Nothing could have caught it here: this
// package has no live database, and the call site logs and continues.
//
// So the guard is on the shape. Interval arithmetic either takes a string the
// caller built or goes through make_interval; what it must not do is
// concatenate a parameter the caller passes as a number.
func TestIntervalParametersAreNotConcatenatedIntoText(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("read package dir: %v", err)
	}
	concat := regexp.MustCompile(`\(\$\d+\s*\|\|\s*'[^']*'\)::interval`)
	// Same reason as above: the positive form has to be present, or a rename of
	// the package's interval arithmetic leaves this greping for a pattern
	// nothing could match and reporting success.
	intervals := 0
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		raw, err := os.ReadFile(filepath.Clean(name))
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		intervals += strings.Count(string(raw), "make_interval")
		if match := concat.FindString(string(raw)); match != "" {
			t.Errorf("%s builds an interval by concatenating a bind parameter (%s); "+
				"that makes it TEXT and a numeric argument cannot be encoded for it — use make_interval",
				name, match)
		}
	}
	if intervals < 2 {
		t.Fatalf("found %d make_interval call sites, expected the two this fixed; "+
			"the package's interval arithmetic moved and this guard is watching nothing", intervals)
	}
}
