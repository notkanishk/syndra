package db

import (
	"slices"
	"testing"
	"time"

	"syndra/internal/access"
)

// This exercises the production query and its shared scanner together. A
// source-text guard cannot detect a SELECT returning fewer columns than Scan
// consumes; pgx can only report that mismatch while reading a real result row.
func TestMappingsForRoles_QueryMatchesScanner(t *testing.T) {
	ctx := liveDB(t)
	const target = "mapping-query-regression"
	const projectID = "mapping-query-project"
	const roleKey = "staff"

	if _, err := PG.Exec(ctx, `INSERT INTO targets (target, project_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`, target, projectID); err != nil {
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

// A→B→C: whoever holds A reaches C through B, so an edit of C moves them. A
// one-hop join counted only the holders of B. HeldRecords is the seed and
// access.Closure follows the chain (services.MappingHolders is exactly this,
// minus people who have left); a lapsed grant is not held.
func TestHeldRecords_SeedTheMappingCohortThroughRuleChains(t *testing.T) {
	ctx := liveDB(t)
	suffix := time.Now().Format("150405.000000000")
	proj := "chain-holders-" + suffix
	holderA, holderB, bystander, lapsed := "chain-a-"+suffix, "chain-b-"+suffix, "chain-x-"+suffix, "chain-l-"+suffix
	var ruleIDs []string
	t.Cleanup(func() {
		for _, id := range ruleIDs {
			_, _ = PG.Exec(ctx, `DELETE FROM mapping_rules WHERE id = $1`, id)
		}
		_, _ = PG.Exec(ctx, `DELETE FROM direct_role_grants WHERE zitadel_project_id = $1`, proj)
	})
	for _, r := range [][2]string{{"a", "b"}, {"b", "c"}} {
		id, _, err := CreateMappingRuleAndEnqueue(ctx, "chain-test", proj, r[0], proj, r[1], nil)
		if err != nil {
			t.Fatalf("rule %v: %v", r, err)
		}
		ruleIDs = append(ruleIDs, id)
	}
	for u, role := range map[string]string{holderA: "a", holderB: "b", bystander: "z", lapsed: "a"} {
		if _, err := UpsertDirectGrant(ctx, u, proj, role, "chain-test", "", nil); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := PG.Exec(ctx, `UPDATE direct_role_grants SET expires_at = NOW() - INTERVAL '1 day' WHERE user_id = $1`, lapsed); err != nil {
		t.Fatal(err)
	}
	held, err := HeldRecords(ctx)
	if err != nil {
		t.Fatal(err)
	}
	rules, err := GetActiveMappingRules(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for u, base := range held {
		if access.Closure(base, rules, access.NewRole)[access.Role{ProjectID: proj, RoleKey: "c"}] {
			got = append(got, u)
		}
	}
	slices.Sort(got)
	if !slices.Equal(got, []string{holderA, holderB}) {
		t.Fatalf("holders of c = %v, want the holder of a (two hops) and of b (one), not the lapsed one", got)
	}
}
