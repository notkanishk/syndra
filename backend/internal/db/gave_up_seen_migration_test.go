package db

import (
	"os"
	"strings"
	"testing"
)

// Schema/code coherence: CountRetries and MarkGaveUpSeen read and write
// gave_up_seen, so migration 000082 must create it keyed by actor, and its
// down must drop it.
func TestGaveUpSeenMigration_CoversTheQueries(t *testing.T) {
	up, err := os.ReadFile("../../db/migrations/000082_gave_up_seen_per_person.up.sql")
	if err != nil {
		t.Fatalf("read up: %v", err)
	}
	down, err := os.ReadFile("../../db/migrations/000082_gave_up_seen_per_person.down.sql")
	if err != nil {
		t.Fatalf("read down: %v", err)
	}
	for _, want := range []string{"CREATE TABLE IF NOT EXISTS gave_up_seen", "PRIMARY KEY", "seen_at TIMESTAMPTZ NOT NULL"} {
		if !strings.Contains(string(up), want) {
			t.Errorf("000082 up must contain %q", want)
		}
	}
	if !strings.Contains(string(down), "DROP TABLE IF EXISTS gave_up_seen") {
		t.Error("000082 down must drop the table up created")
	}
	src, _ := os.ReadFile("propagations.go")
	if !strings.Contains(string(src), "gave_up_seen WHERE actor = $1") ||
		!strings.Contains(funcBody(t, string(src), "CountRetries"), "gaveUpRow") ||
		!strings.Contains(funcBody(t, string(src), "ListGaveUp"), "gaveUpRow") {
		t.Error("CountRetries and ListGaveUp must both read the asking person's own marker (gaveUpRow)")
	}
	if !strings.Contains(funcBody(t, string(src), "MarkGaveUpSeen"), "ON CONFLICT (actor)") {
		t.Error("MarkGaveUpSeen must write the caller's own row, not a shared one")
	}
}
