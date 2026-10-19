package db

import (
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"syndra/internal/models"
)

// Retire keeps its history (owner decision, 19 Oct 2026), against the real schema: the row, its
// versions and its roles stay and stay readable; every list and every "in force" read skips it;
// the people who held it lose their assignments; and its name and rule logic are free to use again.
func TestRetireBundle_KeepsHistoryAndLeavesEveryList(t *testing.T) {
	ctx := liveDB(t)
	stamp := time.Now().Format("150405.000000")
	pid := "pRetire" + stamp
	name := "Retire me " + stamp

	id, err := CreateBundle(ctx, name, "d", "auto", []models.BundleRole{{ProjectID: pid, RoleKey: "door"}})
	if err != nil {
		t.Fatal(err)
	}
	var reused string
	t.Cleanup(func() {
		for _, b := range []string{id, reused} {
			if b != "" {
				_, _ = PG.Exec(ctx, `DELETE FROM bundles WHERE id = $1`, b)
			}
		}
		_, _ = PG.Exec(ctx, `DELETE FROM audit_logs WHERE resource_id = $1`, id)
	})

	// While it is live, its name is taken.
	if _, err := CreateBundle(ctx, name, "", "auto", nil); !IsUniqueViolation(err) {
		t.Fatalf("a live bundle's name is unique: %v", err)
	}

	v, roles, err := LatestVersionRoles(ctx, id)
	if err != nil || len(roles) != 1 {
		t.Fatalf("latest version: %+v %v", roles, err)
	}
	if _, assigned, err := AssignBundleAndEnqueue(ctx, "op", "u-retire-"+stamp, id, v.ID, nil); err != nil || !assigned {
		t.Fatalf("assign: %v %v", assigned, err)
	}
	if err := SetWelcomeBundle(ctx, id); err != nil {
		t.Fatal(err)
	}
	if usage, _ := GetRoleUsageCounts(ctx); usage[pid+":door"].BundleCount != 1 {
		t.Fatalf("a live bundle's role is counted: %+v", usage[pid+":door"])
	}

	if _, err := RetireBundleAndEnqueue(ctx, "op", id, nil); err != nil {
		t.Fatalf("retire: %v", err)
	}

	// Kept: the row says when and by whom, and its versions and roles still read.
	got, err := GetBundleByID(ctx, id)
	if err != nil || got.RetiredAt == nil || got.RetiredBy != "op" || got.Name != name {
		t.Fatalf("a retired bundle still reads by id, with when and by whom: %+v %v", got, err)
	}
	versions, err := ListBundleVersions(ctx, id)
	if err != nil || len(versions) != 1 {
		t.Fatalf("its versions are kept: %+v %v", versions, err)
	}
	if kept, err := GetRolesForVersion(ctx, v.ID); err != nil || len(kept) != 1 {
		t.Fatalf("what its last version gave is kept: %+v %v", kept, err)
	}
	if live, _ := GetAllBundles(ctx); containsBundle(live, id) {
		t.Fatal("the list of bundles in force must not include a retired one")
	}
	retired, err := GetRetiredBundles(ctx)
	if err != nil || !containsBundle(retired, id) {
		t.Fatalf("the retired list names it: %v", err)
	}
	for _, b := range retired {
		if b.ID == id && (b.RetiredAt == nil || b.RetiredBy != "op") {
			t.Fatalf("the retired list carries when and by whom: %+v", b)
		}
	}

	// Not given: nobody holds it, it is not the welcome bundle, and its roles stop counting.
	if holders, _ := GetUsersForBundle(ctx, id); len(holders) != 0 {
		t.Fatalf("people lose what it gave: %v", holders)
	}
	if mine, _ := GetBundlesForUser(ctx, "u-retire-"+stamp); containsBundle(mine, id) {
		t.Fatal("a person's bundles must not include a retired one")
	}
	if w, err := GetWelcomeBundle(ctx); err == nil && w == id {
		t.Fatal("a retired bundle cannot stay the welcome bundle")
	}
	if usage, _ := GetRoleUsageCounts(ctx); usage[pid+":door"].BundleCount != 0 {
		t.Fatalf("a retired bundle's roles are not in use: %+v", usage[pid+":door"])
	}
	if refs, _ := GetAllReferencedRoleKeys(ctx); containsRef(refs, pid) {
		t.Fatal("a retired bundle's roles are not referenced")
	}

	// Nothing about it can change, and it can't be retired twice.
	if err := UpdateBundle(ctx, id, "Renamed", ""); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("rename of a retired bundle: %v", err)
	}
	if err := SetWelcomeBundle(ctx, id); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("a retired bundle can't become the welcome bundle: %v", err)
	}
	if _, err := RetireBundleAndEnqueue(ctx, "op", id, nil); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("retire twice: %v", err)
	}

	// Its name is free again.
	if reused, err = CreateBundle(ctx, name, "", "auto", nil); err != nil {
		t.Fatalf("a retired bundle must not block a new bundle with the same name: %v", err)
	}

	var audited int
	_ = PG.QueryRow(ctx, `SELECT count(*) FROM audit_logs WHERE action = 'bundle.retired' AND resource_id = $1`, id).Scan(&audited)
	if audited != 1 {
		t.Fatalf("bundle.retired is audited once, got %d", audited)
	}
}

// A retired bundle no longer blocks the retire of one it included; its old includes stay history.
func TestRetireBundle_ARetiredIncluderNoLongerBlocks(t *testing.T) {
	ctx := liveDB(t)
	stamp := time.Now().Format("150405.000000")
	mk := func(name string) string {
		id, err := CreateBundle(ctx, name+" "+stamp, "", "auto",
			[]models.BundleRole{{ProjectID: "pRetInc" + stamp, RoleKey: name}})
		if err != nil {
			t.Fatal(err)
		}
		return id
	}
	child, parent := mk("child"), mk("parent")
	t.Cleanup(func() {
		_, _ = PG.Exec(ctx, `DELETE FROM bundle_includes WHERE parent_bundle_id = $1`, parent)
		_, _ = PG.Exec(ctx, `DELETE FROM bundles WHERE id = $1`, parent)
		_, _ = PG.Exec(ctx, `DELETE FROM bundles WHERE id = $1`, child)
	})
	if _, err := SetWorkingInclude(ctx, "op", parent, child, 0); err != nil {
		t.Fatal(err)
	}
	if blockers, _ := RetireBlockers(ctx, child); len(blockers) != 1 {
		t.Fatalf("a live includer blocks: %v", blockers)
	}
	if _, err := RetireBundleAndEnqueue(ctx, "op", parent, nil); err != nil {
		t.Fatal(err)
	}
	if blockers, _ := RetireBlockers(ctx, child); len(blockers) != 0 {
		t.Fatalf("a retired includer must not block: %v", blockers)
	}
	if includers, _ := ListIncluders(ctx, child); len(includers) != 0 {
		t.Fatalf("a retired includer is not listed: %+v", includers)
	}
	// A retired bundle can't be taken into another.
	other := mk("other")
	t.Cleanup(func() { _, _ = PG.Exec(ctx, `DELETE FROM bundles WHERE id = $1`, other) })
	if _, err := SetWorkingInclude(ctx, "op", other, parent, 0); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("including a retired bundle: %v", err)
	}
}

func TestRetireRule_KeepsTheRuleAndLeavesEveryRead(t *testing.T) {
	ctx := liveDB(t)
	stamp := time.Now().Format("150405.000000")
	sp, sr, tp, tr := "pRule"+stamp, "from", "pRule"+stamp, "to"
	create := func() string {
		id, _, err := CreateMappingRuleAndEnqueue(ctx, "op", sp, sr, tp, tr, "auto", nil)
		if err != nil {
			t.Fatal(err)
		}
		return id
	}
	id := create()
	var again string
	t.Cleanup(func() {
		for _, r := range []string{id, again} {
			if r != "" {
				_, _ = PG.Exec(ctx, `DELETE FROM mapping_rules WHERE id = $1`, r)
			}
		}
		_, _ = PG.Exec(ctx, `DELETE FROM audit_logs WHERE resource_id = $1`, id)
	})
	if _, _, err := CreateMappingRuleAndEnqueue(ctx, "op", sp, sr, tp, tr, "auto", nil); !IsUniqueViolation(err) {
		t.Fatalf("a rule in force is unique: %v", err)
	}
	if usage, _ := GetRoleUsageCounts(ctx); usage[tp+":"+tr].RuleCount != 1 {
		t.Fatalf("a rule in force counts: %+v", usage[tp+":"+tr])
	}

	if _, err := RetireMappingRuleAndEnqueue(ctx, "op", id, nil); err != nil {
		t.Fatalf("retire: %v", err)
	}

	got, err := GetMappingRuleByID(ctx, id)
	if err != nil || got.RetiredAt == nil || got.RetiredBy != "op" || got.TargetRole != tr {
		t.Fatalf("a retired rule still reads by id, with when and by whom: %+v %v", got, err)
	}
	if active, _ := GetActiveMappingRules(ctx); containsRule(active, id) {
		t.Fatal("a retired rule must not be evaluated: every cascade and closure reads this list")
	}
	if retired, err := GetRetiredMappingRules(ctx); err != nil || !containsRule(retired, id) {
		t.Fatalf("the retired list names it: %v", err)
	}
	if usage, _ := GetRoleUsageCounts(ctx); usage[tp+":"+tr].RuleCount != 0 {
		t.Fatalf("a retired rule is not in use: %+v", usage[tp+":"+tr])
	}
	if _, err := RetireMappingRuleAndEnqueue(ctx, "op", id, nil); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("retire twice: %v", err)
	}
	if _, err := UpdateMappingRuleAndEnqueue(ctx, "op", id, sp, sr, tp, "other", nil); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("update of a retired rule: %v", err)
	}

	// The same rule can be made again.
	again = create()

	var audited int
	_ = PG.QueryRow(ctx, `SELECT count(*) FROM audit_logs WHERE action = 'mapping_rule.retired' AND resource_id = $1`, id).Scan(&audited)
	if audited != 1 {
		t.Fatalf("mapping_rule.retired is audited once, got %d", audited)
	}
}

func containsBundle(bs []models.Bundle, id string) bool {
	for _, b := range bs {
		if b.ID == id {
			return true
		}
	}
	return false
}

func containsRule(rs []models.MappingRule, id string) bool {
	for _, r := range rs {
		if r.ID == id {
			return true
		}
	}
	return false
}

func containsRef(refs [][2]string, project string) bool {
	for _, r := range refs {
		if r[0] == project {
			return true
		}
	}
	return false
}
