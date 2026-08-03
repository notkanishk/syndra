package db

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// The four files that own atomic cascade mutations. Every one of them used to write its own audit
// row on the line above its enqueueCascadeRows call.
var cascadeMutationFiles = []string{
	"cascade.go", "bundles.go", "grants.go", "bundle_versions.go",
}

var auditInsertPattern = regexp.MustCompile(`(?i)INSERT\s+INTO\s+audit_logs`)

// C6 (ISC-44) — the structural half of the fix.
//
// The audit row and the outbox rows a cascade produces describe the same event, and only
// enqueueCascadeRows knows the id that ties them together. Before this, eleven *AndEnqueue
// functions each wrote their own audit row immediately before calling it, and the id was minted
// and discarded — so "the audit row names its cascade" could only ever be a convention that
// eleven separate functions had to keep, and a twelfth would not have known about.
//
// This guard makes forgetting impossible rather than unlikely: if any cascade mutation grows its
// own audit insert again, the audit row it writes will have no cascade id and this test says so
// before a reviewer has to notice.
func TestCascadeMutationsDoNotWriteTheirOwnAuditRows(t *testing.T) {
	for _, file := range cascadeMutationFiles {
		src, err := os.ReadFile(file)
		if err != nil {
			t.Fatalf("read %s: %v", file, err)
		}
		body := string(src)

		// enqueueCascadeRows is the one legitimate writer, and it lives in cascade.go.
		if file == "cascade.go" {
			body = strings.Replace(body, funcBody(t, body, "enqueueCascadeRows"), "", 1)
		}
		if auditInsertPattern.MatchString(body) {
			t.Errorf("%s writes an audit row outside enqueueCascadeRows — it will carry no "+
				"cascade id, and the console's trace link for that event will be a dash", file)
		}
	}
}

// The other half: enqueueCascadeRows must actually stamp the column, and must leave it NULL when
// the event cascaded to nobody. A cascade id pointing at zero outbox rows would render in the
// audit log as a link to a Change history entry that does not exist.
func TestEnqueueCascadeRowsStampsTheAuditRow(t *testing.T) {
	src, err := os.ReadFile("cascade.go")
	if err != nil {
		t.Fatalf("read cascade.go: %v", err)
	}
	fb := funcBody(t, string(src), "enqueueCascadeRows")

	if !auditInsertPattern.MatchString(fb) {
		t.Fatal("enqueueCascadeRows must write the cascade's audit rows")
	}
	if !strings.Contains(fb, "cascade_id") {
		t.Error("the audit insert must carry cascade_id, or the trace column is an inference again")
	}
	if !strings.Contains(fb, "cascadeGroupVisible(params)") {
		t.Error("the stamp must be gated on cascadeGroupVisible — an id is a handle into Change " +
			"history, and a write that screen does not group has nothing to point at")
	}
}

// The stamp and the Change history filter have to agree about what a cascade is. They did not,
// once, and the failure was silent in exactly the way that matters: a direct grant's removal
// stamped an id, the audit column rendered it as a link, and the link opened a page whose query
// excludes source='direct'. A real pending revoke displayed as an empty history.
func TestCascadeGroupVisibilityMatchesTheHistoryFilter(t *testing.T) {
	cases := []struct {
		name   string
		params []EnqueueParams
		want   bool
	}{
		{"no writes at all", nil, false},
		{"a bundle cascade", []EnqueueParams{{Source: "bundle"}}, true},
		{"a rule cascade", []EnqueueParams{{Source: "rule"}}, true},
		{"a lifecycle cascade", []EnqueueParams{{Source: "lifecycle_cascade"}}, true},
		// The P1. DeleteDirectGrantAndEnqueue goes through enqueueCascadeRows because its ledger
		// delete, audit row and outbox rows must commit together — not because it is a cascade.
		{"a direct grant's removal", []EnqueueParams{{Source: "direct"}}, false},
		// Empty defaults to direct, and must be read the same way as the explicit value.
		{"an unnamed source defaults to direct", []EnqueueParams{{}}, false},
		{"backfill is not a cascade either", []EnqueueParams{{Source: "external_backfill"}}, false},
	}
	for _, tc := range cases {
		if got := cascadeGroupVisible(tc.params); got != tc.want {
			t.Errorf("%s: cascadeGroupVisible = %v, want %v", tc.name, got, tc.want)
		}
	}
}

// Source-coherence: the query must read the shared list rather than spell it out. A second copy
// is how the two drifted apart the first time.
func TestGetCascadeGroupsReadsTheSharedSourceList(t *testing.T) {
	src, err := os.ReadFile("cascade.go")
	if err != nil {
		t.Fatalf("read cascade.go: %v", err)
	}
	fb := funcBody(t, string(src), "GetCascadeGroups")

	if strings.Contains(fb, "'bundle'") || strings.Contains(fb, "'rule'") {
		t.Error("GetCascadeGroups must not inline the source list — pass cascadeGroupSources, so " +
			"adding a source cannot update the query without updating the audit stamp")
	}
	if !strings.Contains(fb, "cascadeGroupSources") {
		t.Error("GetCascadeGroups must filter on cascadeGroupSources")
	}
	// Both the outer query and the subquery, or the glance list and the filter disagree.
	if n := strings.Count(fb, "source = ANY($1::text[])"); n != 2 {
		t.Errorf("expected the source filter in both the outer query and the subquery, found %d", n)
	}
}

// The column the two tests above depend on.
func TestAuditLogsHasNullableCascadeID(t *testing.T) {
	dir := findMigrationsDir(t)
	up, err := os.ReadFile(filepath.Join(dir, "000023_audit_cascade_id.up.sql"))
	if err != nil {
		t.Fatalf("read migration: %v", err)
	}
	sql := string(up)

	if !strings.Contains(sql, "ADD COLUMN IF NOT EXISTS cascade_id UUID") {
		t.Error("audit_logs must gain a cascade_id column")
	}
	// NOT NULL would need a backfill, and there is no honest one: matching pre-000023 audit rows
	// to outbox batches by timestamp proximity is mostly right, and mostly right is the wrong
	// standard for a record of who may operate a laser cutter.
	if strings.Contains(sql, "cascade_id UUID NOT NULL") {
		t.Error("cascade_id must be nullable — rows written before it existed have no true value")
	}
	// No foreign key: outbox rows are drained and pruned, and an audit row has to outlive the
	// queue that carried out its consequence.
	if strings.Contains(sql, "REFERENCES pending_zitadel_propagations") {
		t.Error("cascade_id must not reference the outbox — pruning it would break the audit tail")
	}
}
