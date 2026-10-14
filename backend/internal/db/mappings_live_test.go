package db

import (
	"testing"
)

// This exercises the production query and its shared scanner together. A
// source-text guard cannot detect a SELECT returning fewer columns than Scan
// consumes; pgx can only report that mismatch while reading a real result row.
func TestMappingsForRoles_QueryMatchesScanner(t *testing.T) {
	ctx := liveDB(t)
	const target = "mapping-query-regression"
	const projectID = "mapping-query-project"
	const roleKey = "staff"

	if _, err := PG.Exec(ctx, `INSERT INTO targets (target) VALUES ($1) ON CONFLICT DO NOTHING`, target); err != nil {
		t.Fatalf("seed target: %v", err)
	}
	t.Cleanup(func() {
		if _, err := PG.Exec(ctx, `DELETE FROM target_role_mappings WHERE target = $1`, target); err != nil {
			t.Errorf("clean mappings: %v", err)
		}
		if _, err := PG.Exec(ctx, `DELETE FROM targets WHERE target = $1`, target); err != nil {
			t.Errorf("clean target: %v", err)
		}
	})

	created, err := CreateRoleMapping(ctx, RoleMapping{
		Target: target, ProjectID: projectID, RoleKey: roleKey,
		Field: "group", Value: "lab_staff", CreatedBy: "regression-test",
	})
	if err != nil {
		t.Fatalf("create mapping: %v", err)
	}

	got, err := MappingsForRoles(ctx, target, []RoleRef{{ProjectID: projectID, RoleKey: roleKey}})
	if err != nil {
		t.Fatalf("read mapping through production query and scanner: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("want one mapping, got %+v", got)
	}
	if got[0].ID != created.ID || got[0].Value != "lab_staff" {
		t.Fatalf("wrong mapping returned: %+v", got[0])
	}
	if got[0].CreatedAt.IsZero() || got[0].UpdatedAt.IsZero() {
		t.Fatalf("timestamps were not scanned: %+v", got[0])
	}
}
