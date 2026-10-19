package db

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"syndra/internal/models"
)

// A cancellation keeps what it cancelled and what cancelled it, whole: the
// outbox row is pruned, and Change history has to say it afterwards (design A-04 §14.8).
func TestCancelledUnsentDetails(t *testing.T) {
	queued := time.Date(2026, 10, 15, 9, 30, 0, 0, time.FixedZone("x", 5*3600+1800))
	removal := models.QueuedChange{ID: "r1", OpType: "revoke", UserID: "u1", ProjectID: "p1",
		RoleKeys: []string{"community"}, InitiatedBy: "op1", Reason: "left the course", QueuedAt: queued}
	grant := models.QueuedChange{ID: "a1", OpType: "add", UserID: "u1", ProjectID: "p1",
		RoleKeys: []string{"community"}, InitiatedBy: "op1", QueuedAt: queued}

	cases := []struct {
		name       string
		c          models.QueuedChange
		p          EnqueueParams
		kind       string
		action     string
		via        string
		wantReason string
	}{
		{"a removal given back", removal, EnqueueParams{OpType: "add"}, "remove", "grant", "direct", "left the course"},
		{"approving a request", removal, EnqueueParams{OpType: "add", Source: "direct", SourceRef: "req-1"}, "remove", "approve", "direct", "left the course"},
		{"a bundle gives it back", removal, EnqueueParams{OpType: "add", Source: "bundle", SourceRef: "b1"}, "remove", "grant", "bundle", "left the course"},
		{"a grant removed before it went", grant, EnqueueParams{OpType: "revoke", Source: "direct", SourceRef: "g1"}, "give", "remove", "direct", ""},
	}
	for _, tc := range cases {
		raw, err := CancelledUnsentDetails(tc.c, tc.p)
		if err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		var got CancelledUnsent
		if err := json.Unmarshal(raw, &got); err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		c := got.Cancelled
		if c.Kind != tc.kind || c.ProjectID != "p1" || c.RoleKey != "community" || c.UserID != "u1" ||
			c.InitiatedBy != "op1" || c.Reason != tc.wantReason || !c.QueuedAt.Equal(queued) {
			t.Errorf("%s: cancelled %+v", tc.name, c)
		}
		if got.Cause.Action != tc.action || got.Cause.Via != tc.via {
			t.Errorf("%s: cause %+v", tc.name, got.Cause)
		}
	}
}

// The three promises the live tests prove, checkable without a database.
func TestTheAuditPageReadsDetails(t *testing.T) {
	query, _ := buildAuditQuery("", 50, nil)
	if !strings.Contains(query, "COALESCE(cascade_id::text,''), details") {
		t.Errorf("the audit page must read details: %s", query)
	}
}

func TestTheMemberFeedNamesOnlyACancelledRemoval(t *testing.T) {
	for _, want := range []string{
		"'cancelled_removal'",
		"a.action = 'propagation.cancelled_unsent'",
		"a.details->'cancelled'->>'kind' = 'remove'",
		"a.target_zitadel_user_id = $1",
	} {
		if !strings.Contains(myChangesSQL, want) {
			t.Errorf("the feed lost %q", want)
		}
	}
	// No operator name or reason may ride on that branch.
	branch := myChangesSQL[strings.Index(myChangesSQL, "'cancelled_removal'"):]
	if strings.Contains(branch, "initiated_by") || strings.Contains(branch, "'reason'") {
		t.Errorf("the member's cancelled_removal must not read who queued it or why: %s", branch)
	}
	// Every UNION branch has the same 14 columns; the outbox branch once had 12,
	// which fails at runtime, so it pads the two ends it has no value for.
	if !strings.Contains(myChangesSQL, "NULL::timestamptz,\n\t       NULL::timestamptz\n\t  FROM propagation_outbox o") {
		t.Error("the outbox branch must pad ended_before and ends_after")
	}
}
