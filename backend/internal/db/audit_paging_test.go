package db

import (
	"strings"
	"testing"
	"time"
)

// The `db` package has no live-DB harness, so these guard the part that fails
// silently rather than loudly: a mis-numbered placeholder does not error, it
// returns the wrong page.

func TestBuildAuditQuery_NewestPage(t *testing.T) {
	query, args := buildAuditQuery("", 50, nil)

	if strings.Contains(query, "WHERE") {
		t.Errorf("unfiltered newest page should have no WHERE clause: %s", query)
	}
	if !strings.HasSuffix(strings.TrimSpace(query), "LIMIT $1;") {
		t.Errorf("limit should be the only placeholder: %s", query)
	}
	if len(args) != 1 || args[0] != 50 {
		t.Errorf("expected args [50], got %v", args)
	}
}

// C6 — the trace column reads this column, and it is selected as text so a NULL (every row
// written before migration 000023, and every event that cascaded to nobody) arrives as "" rather
// than making the scan fail.
func TestBuildAuditQuery_SelectsTheCascadeID(t *testing.T) {
	query, _ := buildAuditQuery("", 50, nil)

	if !strings.Contains(query, "COALESCE(cascade_id::text,'')") {
		t.Errorf("the audit page must read cascade_id, NULL-safe: %s", query)
	}
}

func TestBuildAuditQuery_OrdersByTheWholeCursorKey(t *testing.T) {
	query, _ := buildAuditQuery("", 50, nil)

	// Without the id tiebreak the order within a same-instant batch is
	// undefined, which makes every cursor built from it meaningless.
	if !strings.Contains(query, "ORDER BY created_at DESC, id DESC") {
		t.Errorf("ordering must match the cursor key exactly: %s", query)
	}
}

func TestBuildAuditQuery_CursorPagesOnTheTuple(t *testing.T) {
	at := time.Date(2026, 8, 2, 9, 38, 0, 0, time.UTC)
	query, args := buildAuditQuery("", 25, &AuditCursor{CreatedAt: at, ID: "a-9"})

	// A cascade writes several audit rows in ONE transaction, so they share a
	// created_at to the nanosecond. Comparing the timestamp alone would return
	// the rest of that batch forever, or skip it.
	if !strings.Contains(query, "(created_at, id) < ($1, $2)") {
		t.Errorf("cursor must compare the tuple, not the timestamp: %s", query)
	}
	if len(args) != 3 || args[0] != at || args[1] != "a-9" || args[2] != 25 {
		t.Errorf("expected [%v a-9 25], got %v", at, args)
	}
	if !strings.HasSuffix(strings.TrimSpace(query), "LIMIT $3;") {
		t.Errorf("limit must follow the cursor placeholders: %s", query)
	}
}

func TestBuildAuditQuery_ScopeAndCursorTogether(t *testing.T) {
	at := time.Date(2026, 8, 2, 9, 38, 0, 0, time.UTC)
	query, args := buildAuditQuery("u-1", 10, &AuditCursor{CreatedAt: at, ID: "a-9"})

	if !strings.Contains(query, "(actor_zitadel_user_id = $1 OR target_zitadel_user_id = $1)") {
		t.Errorf("scope should reuse one placeholder for both sides: %s", query)
	}
	if !strings.Contains(query, "(created_at, id) < ($2, $3)") {
		t.Errorf("cursor placeholders must shift past the scope: %s", query)
	}
	if !strings.HasSuffix(strings.TrimSpace(query), "LIMIT $4;") {
		t.Errorf("limit must be last: %s", query)
	}
	if len(args) != 4 || args[0] != "u-1" || args[3] != 10 {
		t.Errorf("unexpected args %v", args)
	}
	if strings.Count(query, "WHERE") != 1 || !strings.Contains(query, " AND ") {
		t.Errorf("both clauses should join into one WHERE: %s", query)
	}
}
