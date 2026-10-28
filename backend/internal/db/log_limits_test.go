package db

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The two operations logs list newest first with a LIMIT and the webhook log is
// pruned by age; both are an index scan only if created_at is indexed. Reads
// every *.up.sql rather than pinning one migration (see backend/README.md).
func TestLogTables_HaveCreatedAtIndex(t *testing.T) {
	dir := findMigrationsDir(t)
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]bool{"webhook_events": false, "onboarding_triggers": false}
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".up.sql") {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			t.Fatal(err)
		}
		for _, stmt := range strings.Split(string(raw), ";") {
			s := strings.Join(strings.Fields(strings.ToLower(stmt)), " ")
			for table := range want {
				if strings.Contains(s, "create index") && strings.Contains(s, " on "+table+" (created_at") {
					want[table] = true
				}
			}
		}
	}
	for table, ok := range want {
		if !ok {
			t.Errorf("no migration indexes %s(created_at)", table)
		}
	}
}

// The log reads stop at logListLimit, newest first, and the prune takes only
// rows past the retention window.
func TestWebhookEvents_ListIsBoundedAndPruneKeepsRecent(t *testing.T) {
	ctx := liveDB(t)
	const prefix = "log-limit-test-"
	cleanup := func() {
		_, _ = PG.Exec(ctx, `DELETE FROM webhook_events WHERE idempotency_key LIKE $1`, prefix+"%")
	}
	cleanup()
	t.Cleanup(cleanup)

	_, err := PG.Exec(ctx, `
		INSERT INTO webhook_events (event_type, user_id, source_project, idempotency_key, created_at)
		SELECT 'x', 'u', 'p', $1::text || g::text, NOW() - g * INTERVAL '1 second'
		  FROM generate_series(1, $2::int) g`, prefix, logListLimit+5)
	if err != nil {
		t.Fatal(err)
	}
	events, err := GetWebhookEvents(ctx, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != logListLimit {
		t.Fatalf("got %d events, want the newest %d", len(events), logListLimit)
	}
	for i := 1; i < len(events); i++ {
		if events[i].CreatedAt.After(events[i-1].CreatedAt) {
			t.Fatal("events are not newest first")
		}
	}

	_, err = PG.Exec(ctx, `
		INSERT INTO webhook_events (event_type, user_id, source_project, idempotency_key, created_at)
		VALUES ('x', 'u', 'p', $1::text || 'old', NOW() - INTERVAL '100 days')`, prefix)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := PruneWebhookEvents(ctx, WebhookEventRetentionDays); err != nil {
		t.Fatal(err)
	}
	var old, recent int
	if err := PG.QueryRow(ctx, `SELECT COUNT(*) FILTER (WHERE idempotency_key = $1::text || 'old'), COUNT(*) FILTER (WHERE idempotency_key <> $1::text || 'old')
		FROM webhook_events WHERE idempotency_key LIKE $1::text || '%'`, prefix).Scan(&old, &recent); err != nil {
		t.Fatal(err)
	}
	if old != 0 || recent != logListLimit+5 {
		t.Fatalf("prune left old=%d recent=%d, want 0 and %d", old, recent, logListLimit+5)
	}
}
