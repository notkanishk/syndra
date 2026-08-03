package db

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// C4 — the reopen rule is "when the grant changes", and the whole of it lives in two places: the
// stored `acknowledged_expires_at` and the join that compares it. There is no invalidation job and
// nothing that fires on an update, deliberately — the `db` package has no live-DB harness, so a
// rule enforced by a trigger somewhere would be a rule nothing here could check.
//
// These guard the two halves against the shortcut versions of each.

func TestAcknowledgementStoresWhatWasAcknowledged(t *testing.T) {
	dir := findMigrationsDir(t)
	up, err := os.ReadFile(filepath.Join(dir, "000024_grant_expiry_acknowledgement.up.sql"))
	if err != nil {
		t.Fatalf("read migration: %v", err)
	}
	sql := string(up)

	// Without this column the reopen rule cannot exist: "was this acknowledged" would be
	// answerable but "acknowledged as of WHAT" would not, and an extension would silently keep a
	// decision somebody made about a date that has gone.
	if !strings.Contains(sql, "acknowledged_expires_at TIMESTAMP WITH TIME ZONE NOT NULL") {
		t.Error("the acknowledged expiry must be stored, NOT NULL — it is what the decision was about")
	}
	// One row per grant. Stacking acknowledgements here would make "is this acknowledged now" a
	// question about ordering; audit_logs is where the history goes.
	if !strings.Contains(sql, "grant_id UUID PRIMARY KEY") {
		t.Error("one acknowledgement per grant, or currency becomes a question about ordering")
	}
	// Unlike audit_logs.cascade_id (000023) and onboarding_triggers.bundle_id (000021), this row
	// is an annotation on a live grant rather than history, and the grant's deletion by the expiry
	// sweep is the normal end of its life.
	if !strings.Contains(sql, "REFERENCES direct_role_grants(id) ON DELETE CASCADE") {
		t.Error("an acknowledgement must not outlive the grant it annotates")
	}
}

// The read is the enforcement. If the join ever stops comparing dates, every acknowledgement
// becomes permanent, and it becomes permanent silently — the rows simply stay hidden.
func TestExpiringReadEnforcesTheReopenRule(t *testing.T) {
	src, err := os.ReadFile("grants.go")
	if err != nil {
		t.Fatalf("read grants.go: %v", err)
	}
	fb := funcBody(t, string(src), "GetExpiringDirectGrantsWithAcknowledgements")

	joinsOnTheDate := regexp.MustCompile(`a\.acknowledged_expires_at\s*=\s*g\.expires_at`)
	if !joinsOnTheDate.MatchString(fb) {
		t.Error("the acknowledgement join MUST compare acknowledged_expires_at to the grant's " +
			"current expires_at — that comparison IS the reopen rule, and without it an " +
			"extended grant keeps a decision made about a date it no longer has")
	}
	if !strings.Contains(fb, "LEFT JOIN") {
		t.Error("the join must be a LEFT JOIN — an unacknowledged grant is the common case and " +
			"must still appear in the queue")
	}
}

// The write half: acknowledging a date the grant no longer carries is refused, not stored. Stored,
// it would be accepted, never apply, and leave the operator believing they had recorded something.
func TestAcknowledgeChecksTheDateItWasGiven(t *testing.T) {
	src, err := os.ReadFile("grants.go")
	if err != nil {
		t.Fatalf("read grants.go: %v", err)
	}
	fb := funcBody(t, string(src), "AcknowledgeGrantExpiry")

	if !strings.Contains(fb, "FOR UPDATE") {
		t.Error("the grant's expiry must be read FOR UPDATE — otherwise an extension committing " +
			"alongside this makes the check it performs meaningless")
	}
	if !strings.Contains(fb, "ErrGrantExpiryMoved") {
		t.Error("a moved expiry must be refused, so the operator is told what changed rather " +
			"than given a write that silently never applies")
	}
	if !strings.Contains(fb, "current.Equal(expiresAt)") {
		t.Error("the stored date must be compared to the one the operator was shown")
	}
}
