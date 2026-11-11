package db

import (
	"context"
	"errors"
	"strings"
	"testing"
)

// Guard: 000091 adds targets.name, additive, with the CHECK that is the one
// definition of a name, and the down migration drops exactly that.
func TestMigration91_AddsTargetName(t *testing.T) {
	up, down := readMigrationPair(t, "000091_target_name")
	flat := strings.Join(strings.Fields(up), " ")
	if !strings.Contains(flat, "ALTER TABLE targets ADD COLUMN IF NOT EXISTS name TEXT") ||
		!strings.Contains(flat, "CHECK (name IS NULL OR (btrim(name) <> '' AND char_length(name) <= 60))") {
		t.Error("000091 up must add a nullable targets.name with its CHECK")
	}
	if strings.Contains(up, "DROP") || strings.Contains(up, "UPDATE") || strings.Contains(up, "NOT NULL") {
		t.Error("000091 up must be additive")
	}
	if strings.TrimSpace(down) != "ALTER TABLE targets DROP COLUMN IF EXISTS name;" {
		t.Error("000091 down must drop targets.name and nothing else")
	}
}

// A name is written with its audit row, clears back to NULL, and the schema
// refuses a long one without writing either.
func TestATargetIsNamedAndTheChangeIsAudited(t *testing.T) {
	ctx := liveDB(t)
	if _, err := PG.Exec(ctx, `INSERT INTO targets (target) VALUES ('laser') ON CONFLICT DO NOTHING`); err != nil {
		t.Fatal(err)
	}
	clean := func() {
		_, _ = PG.Exec(context.Background(), `DELETE FROM audit_logs WHERE action = $1 AND resource_id = 'laser'`, TargetNameAudit)
		_, _ = PG.Exec(context.Background(), `UPDATE targets SET name = NULL WHERE target = 'laser'`)
	}
	clean()
	t.Cleanup(clean)

	if err := SetTargetName(ctx, "laser", "Big laser", "u-kane"); err != nil {
		t.Fatal(err)
	}
	if name, err := TargetName(ctx, "laser"); err != nil || name != "Big laser" {
		t.Fatalf("name %q, %v", name, err)
	}
	if err := SetTargetName(ctx, "laser", "", "u-kane"); err != nil {
		t.Fatal(err)
	}
	if name, err := TargetName(ctx, "laser"); err != nil || name != "" {
		t.Fatalf("a cleared name must read as none, got %q, %v", name, err)
	}
	if err := SetTargetName(ctx, "laser", strings.Repeat("x", 61), "u-kane"); !errors.Is(err, ErrTargetNameInvalid) {
		t.Fatalf("a 61-character name must be refused by the schema, got %v", err)
	}
	if err := SetTargetName(ctx, "nowhere", "Nowhere", "u-kane"); err == nil {
		t.Fatal("an unknown target has no name to set")
	}

	rows, err := PG.Query(ctx, `SELECT actor_zitadel_user_id, details->>'from', details->>'to' FROM audit_logs
		WHERE action = $1 AND resource_id = 'laser' ORDER BY created_at`, TargetNameAudit)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var got []string
	for rows.Next() {
		var actor string
		var from, to *string
		if err := rows.Scan(&actor, &from, &to); err != nil {
			t.Fatal(err)
		}
		got = append(got, actor+":"+deref(from)+">"+deref(to))
	}
	if want := "u-kane:>Big laser,u-kane:Big laser>"; strings.Join(got, ",") != want {
		t.Errorf("audit %v, want %s (one row per write that landed, none for the refused one)", got, want)
	}
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
