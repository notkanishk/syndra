package db

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// 7.2, 8.2 — migration-coherence guards. `internal/db` has no live-database
// harness, so what is asserted is that the schema says what the code assumes
// and that the rollback refuses where it must.

func readMigrationPair(t *testing.T, name string) (up, down string) {
	t.Helper()
	dir := findMigrationsDir(t)
	u, err := os.ReadFile(filepath.Join(dir, name+".up.sql"))
	if err != nil {
		t.Fatalf("read up migration: %v", err)
	}
	d, err := os.ReadFile(filepath.Join(dir, name+".down.sql"))
	if err != nil {
		t.Fatalf("read down migration: %v", err)
	}
	return string(u), string(d)
}

// The uniqueness that makes resolution deterministic. Two rows binding one
// role's `group` to different values is not a richer mapping — it is a resolver
// returning whichever the database ordered first, and a subject whose access
// depends on that ordering.
func TestMappingUniquenessIncludesTheField(t *testing.T) {
	up, _ := readMigrationPair(t, "000029_target_role_mappings")

	body := createTableBody(t, up, "target_role_mappings")
	unique := regexp.MustCompile(`UNIQUE \(([^)]*)\)`).FindStringSubmatch(body)
	if unique == nil {
		t.Fatal("target_role_mappings must declare its uniqueness")
	}
	for _, col := range []string{"target", "project_id", "role_key", "field"} {
		if !strings.Contains(unique[1], col) {
			t.Errorf("the binding key must include %q, or two targets (or two fields) collide: %s", col, unique[1])
		}
	}
	// `value` must NOT be in it: including it would make two values for one
	// field legal, which is the ambiguity the key exists to forbid.
	if strings.Contains(unique[1], "value") {
		t.Error("the key must not include value, or one role's field can bind to two values at once")
	}
}

// The reverse lookup the lifecycle trigger makes on every grant change, and the
// forward one the resolver makes. Both indexed, because both run on the hot
// path of an access change.
func TestMappingLookupsAreIndexedInBothDirections(t *testing.T) {
	up, _ := readMigrationPair(t, "000029_target_role_mappings")
	for _, want := range []string{
		"ON target_role_mappings (target, project_id, role_key)",
		"ON target_role_mappings (project_id, role_key)",
	} {
		if !strings.Contains(up, want) {
			t.Errorf("missing index %q", want)
		}
	}
}

// Version history is a historical record and a rollback compares against it. A
// record that can be edited is a comparison against nothing.
func TestPublishedMappingVersionsAreImmutable(t *testing.T) {
	up, _ := readMigrationPair(t, "000029_target_role_mappings")
	// Both directions, and the second migration is where DELETE arrived: 000029
	// guarded UPDATE only, so a published version could be deleted outright —
	// and deleting the row erases more than editing one field of it, entries
	// cascading away with it.
	guard, _ := readMigrationPair(t, "000031_mapping_version_delete_guard")
	if !regexp.MustCompile(`(?s)CREATE TRIGGER target_mapping_versions_immutable\s+BEFORE UPDATE OR DELETE ON target_mapping_versions`).MatchString(guard) {
		t.Fatal("a published mapping version must be immutable against UPDATE and DELETE")
	}
	if !regexp.MustCompile(`(?s)CREATE TRIGGER target_mapping_version_entries_immutable\s+BEFORE UPDATE OR DELETE ON target_mapping_version_entries`).MatchString(guard) {
		t.Fatal("the version's entries are its content and must be immutable too")
	}
	if !strings.Contains(up, "UNIQUE (target, version)") {
		t.Error("versions are per target: a global sequence makes \"TrueNAS v2\" meaningless")
	}
}

// A mapping deleted is a role that silently stops reaching anything, and the
// resolver would converge every holder to an empty set. The rollback refuses
// rather than doing that, and says where to do it properly.
func TestTheMappingRollbackRefusesWhileAnyBindingExists(t *testing.T) {
	_, down := readMigrationPair(t, "000029_target_role_mappings")
	if !strings.Contains(down, "RAISE EXCEPTION") {
		t.Fatal("the rollback must refuse rather than resolve every holder to nothing")
	}
	guard := strings.Index(down, "RAISE EXCEPTION")
	drop := strings.Index(down, "DROP TABLE IF EXISTS target_role_mappings")
	if drop < 0 || drop < guard {
		t.Error("the table may only be dropped after the guard that refuses the rollback")
	}
}

// 8.2 — the bound that stops a temporary measure becoming permanent by
// inattention. Asserted as a WHOLE expression rather than by fragment: a
// containment check cannot tell a constraint from one with a tautology in front
// of it.
func TestADenialMustBeBoundedInTime(t *testing.T) {
	up, _ := readMigrationPair(t, "000030_allowances")

	const want = `CHECK (
        direction <> 'deny' OR expires_at IS NOT NULL OR review_date IS NOT NULL
    )`
	if !strings.Contains(up, want) {
		t.Fatalf("the denial bound must be exactly:\n%s", want)
	}
	// The whole-expression match above already pins the implication shape: it
	// applies to denials only, and an additive allowance — somebody being given
	// something, which does not rot the same way — passes it unconditionally.
}

// Lifting is one act with two facts, and half of it is a row nothing can
// render.
func TestLiftingAnAllowanceIsRecordedWhole(t *testing.T) {
	up, _ := readMigrationPair(t, "000030_allowances")
	const want = `CHECK (
        (lifted_at IS NULL AND lifted_by IS NULL) OR
        (lifted_at IS NOT NULL AND lifted_by IS NOT NULL)
    )`
	if !strings.Contains(up, want) {
		t.Fatalf("the lifted pair must be exactly:\n%s", want)
	}
}

// Dropping allowances restores access somebody decided to withhold, which is
// the one direction a rollback must never move access in.
func TestTheAllowanceRollbackRefusesWhileAnyIsInForce(t *testing.T) {
	_, down := readMigrationPair(t, "000030_allowances")
	if !strings.Contains(down, "lifted_at IS NULL") {
		t.Fatal("the guard must count allowances IN FORCE, not every row ever written")
	}
	guard := strings.Index(down, "RAISE EXCEPTION")
	drop := strings.Index(down, "DROP TABLE IF EXISTS allowances")
	if guard < 0 || drop < 0 || drop < guard {
		t.Fatal("the table may only be dropped after the guard that refuses the rollback")
	}
}

// The Go vocabulary and the schema's CHECK are one vocabulary. A direction the
// code writes and the schema forbids is a runtime constraint violation on an
// operator's screen.
func TestTheAllowanceDirectionVocabularyIsOne(t *testing.T) {
	up, _ := readMigrationPair(t, "000030_allowances")
	check := regexp.MustCompile(`direction IN \(([^)]*)\)`).FindStringSubmatch(up)
	if check == nil {
		t.Fatal("the schema must declare the direction vocabulary")
	}
	for _, v := range []string{AllowanceAllow, AllowanceDeny} {
		if !strings.Contains(check[1], "'"+v+"'") {
			t.Errorf("the schema does not accept %q, which the Go layer writes", v)
		}
	}
	declared := len(strings.Split(check[1], ","))
	if declared != 2 {
		t.Errorf("the schema declares %d directions and the Go layer knows 2; a value with no constant is one nothing can read back", declared)
	}
}

// Every column the store writes is a column some migration declares.
func TestAllowanceAndMappingWritesMatchTheSchema(t *testing.T) {
	for _, tc := range []struct{ file, table, source string }{
		{"000029_target_role_mappings", "target_role_mappings", "mappings.go"},
		{"000030_allowances", "allowances", "allowances.go"},
	} {
		up, _ := readMigrationPair(t, tc.file)
		body := createTableBody(t, up, tc.table)
		src := readDBSource(t, tc.source)

		re := regexp.MustCompile(`INSERT INTO ` + tc.table + ` \(([^)]*)\)`)
		for _, m := range re.FindAllStringSubmatch(src, -1) {
			for _, col := range strings.Split(m[1], ",") {
				col = strings.TrimSpace(col)
				if !regexp.MustCompile(`(?m)^\s*` + regexp.QuoteMeta(col) + `\s`).MatchString(body) {
					t.Errorf("%s writes column %q, which the migration does not declare", tc.table, col)
				}
			}
		}
	}
}

// The conflict target is named, so only the binding uniqueness is absorbed. A
// bare `ON CONFLICT DO NOTHING` would swallow every future constraint too, and
// a caller would get "already exists" for a row that failed something else.
func TestTheDuplicateBindingAbsorptionNamesItsArbiter(t *testing.T) {
	body := funcBody(t, readDBSource(t, "mappings.go"), "CreateRoleMapping")
	if !strings.Contains(body, "ON CONFLICT (target, project_id, role_key, field) DO NOTHING") {
		t.Fatal("the absorbed conflict must name the binding key, or a later constraint is absorbed with it")
	}
}

// The blast radius a mapping edit would move. Direct grants alone would
// understate it on exactly the mappings that matter most: a role held through a
// bundle is held just as much as one granted by hand.
func TestTheMappingCohortCountsEverySourceOfTheRole(t *testing.T) {
	body := funcBody(t, readDBSource(t, "mappings.go"), "MappingHolders")
	for _, frag := range []string{"FROM direct_role_grants", "user_bundle_assignments", "bundle_version_roles",
		// The rule arm, which the comment above the query promised and the
		// query did not have: a role a rule derives is held just as much.
		"mapping_rules",
		// And a lapsed grant is not held, so counting it overstates the cohort
		// in the other direction.
		"expires_at IS NULL OR expires_at > NOW()"} {
		if !strings.Contains(body, frag) {
			t.Errorf("the cohort read is missing %q — a holder it misses is a person an edit moves silently", frag)
		}
	}
	// The UNION on its own line, not merely somewhere in the text: `-- UNION`
	// contains the word and joins nothing, which would leave the bundle half
	// present in the source and absent from the result.
	if !regexp.MustCompile(`(?m)^\s*UNION\s*$`).MatchString(body) {
		t.Error("the two halves must actually be unioned")
	}
}

// Roles are matched in PAIRS. Two independent IN lists would cross-join, so a
// role of one name in a project the subject holds nothing in would confer
// whatever that name means somewhere else.
func TestMappingsForRolesMatchesProjectAndRoleTogether(t *testing.T) {
	body := funcBody(t, readDBSource(t, "mappings.go"), "MappingsForRoles")
	if !strings.Contains(body, "unnest($2::text[], $3::text[])") {
		t.Fatal("project and role must be unnested as pairs, not matched independently")
	}
	if !strings.Contains(body, "held.project_id = m.project_id AND held.role_key = m.role_key") {
		t.Error("both halves of the pair must be compared")
	}
}

// `MAX(version)+1` is a read and a write with a gap in the middle, under a
// comment claiming two concurrent publishes "cannot both claim the same one".
// Migration 000026's snapshot trigger takes an advisory lock for exactly this;
// the pattern was not reused until it was pointed at.
func TestTheMappingVersionAllocationIsSerialised(t *testing.T) {
	src := readDBSource(t, "mappings.go")
	for _, fn := range []string{"PublishMappingVersion", "RollbackMappingVersion"} {
		body := funcBody(t, src, fn)
		if !strings.Contains(body, "pg_advisory_xact_lock") {
			t.Errorf("%s must serialise per target, or the version it allocates is not the one it stores", fn)
		}
	}
	// And the lock must be taken BEFORE the number is read, or it serialises
	// nothing that matters.
	body := funcBody(t, src, "PublishMappingVersion")
	if strings.Index(body, "pg_advisory_xact_lock") > strings.Index(body, "COALESCE(MAX(version), 0)") {
		t.Error("the lock must precede the allocation")
	}
}

// A finalizer nothing calls, taking a raw query string, with no in_flight
// guard, sitting under the comment "Every finalizer below is guarded". The
// next person to need one would have found it and used it.
func TestNoUnguardedPropagationFinalizerSurvives(t *testing.T) {
	if strings.Contains(readDBSource(t, "propagations.go"), "func execPropagation(") {
		t.Fatal("execPropagation is dead, unguarded, and takes a query string — it must not exist")
	}
}
