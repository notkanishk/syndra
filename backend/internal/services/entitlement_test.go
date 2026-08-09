package services

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"syndra/internal/models"

	"syndra/internal/db"
)

// 7.7/7.8, 8.3/8.4 — resolution is two layers and nothing else.

type resolverFixture struct {
	roles      []db.RoleRef
	mappings   []db.RoleMapping
	allowances []db.Allowance
}

func (f *resolverFixture) install(t *testing.T) {
	t.Helper()
	er, mf, af := svcEffectiveRoleRefs, dbMappingsForRoles, dbAllowancesInForce
	svcEffectiveRoleRefs = func(context.Context, string) ([]db.RoleRef, error) { return f.roles, nil }
	dbMappingsForRoles = func(context.Context, string, []db.RoleRef) ([]db.RoleMapping, error) {
		return f.mappings, nil
	}
	dbAllowancesInForce = func(context.Context, string, string) ([]db.Allowance, error) {
		return f.allowances, nil
	}
	t.Cleanup(func() { svcEffectiveRoleRefs, dbMappingsForRoles, dbAllowancesInForce = er, mf, af })
}

func mapping(role, field, value string) db.RoleMapping {
	return db.RoleMapping{Target: "truenas", ProjectID: "pLab", RoleKey: role, Field: field, Value: value}
}

func deny(field, value string) db.Allowance {
	return db.Allowance{
		ID: "a1", Target: "truenas", Field: field, Value: value,
		Direction: db.AllowanceDeny, ActorID: "op_1", Reason: "safety review",
	}
}

func resolve(t *testing.T, f *resolverFixture) EntitlementSet {
	t.Helper()
	f.install(t)
	set, err := ResolveEntitlements(context.Background(), "u1", "truenas")
	if err != nil {
		t.Fatal(err)
	}
	return set
}

// The role half comes from the mappings and from nothing else. A role nobody
// mapped is a role that reaches this target in no way at all.
func TestRoleDerivationComesOnlyFromMappings(t *testing.T) {
	set := resolve(t, &resolverFixture{
		roles: []db.RoleRef{{ProjectID: "pLab", RoleKey: "maker"}, {ProjectID: "pLab", RoleKey: "unmapped"}},
		mappings: []db.RoleMapping{
			mapping("maker", "group", "lab_makers"),
			mapping("maker", "quota_bytes", "50000000000"),
		},
	})
	if want := map[string][]string{"group": {"lab_makers"}, "quota_bytes": {"50000000000"}}; !reflect.DeepEqual(set.Fields, want) {
		t.Fatalf("two mappings on one role must contribute both fields, got %v", set.Fields)
	}
	if !set.Lifecycle.Enabled || !set.Lifecycle.SMBEnabled {
		t.Error("holding a mapped role must resolve the account to enabled")
	}
}

// Stable under repetition, and stable under the order the database happened to
// return. A set that reorders is a set that looks like a change to a
// level-triggered apply, which would converge the target on every pass.
func TestResolutionIsStable(t *testing.T) {
	f := &resolverFixture{
		roles: []db.RoleRef{{ProjectID: "pLab", RoleKey: "maker"}, {ProjectID: "pLab", RoleKey: "welder"}},
		mappings: []db.RoleMapping{
			mapping("welder", "group", "lab_welders"),
			mapping("maker", "group", "lab_makers"),
			mapping("maker", "group", "lab_makers"), // the same binding twice
		},
	}
	first := resolve(t, f)
	second := resolve(t, f)
	if !reflect.DeepEqual(first, second) {
		t.Fatal("re-resolution must produce the same set")
	}
	if want := []string{"lab_makers", "lab_welders"}; !reflect.DeepEqual(first.Fields["group"], want) {
		t.Errorf("values must be sorted and deduplicated, got %v", first.Fields["group"])
	}
}

// Deny beats derivation. A denial that lost to the role layer would be a
// suspension an operator recorded and the system ignored.
func TestASubtractiveAllowanceRemovesAccessAndSaysWho(t *testing.T) {
	set := resolve(t, &resolverFixture{
		roles: []db.RoleRef{{ProjectID: "pLab", RoleKey: "maker"}},
		mappings: []db.RoleMapping{
			mapping("maker", "group", "lab_makers"),
			mapping("maker", "group", "lab_printing"),
		},
		allowances: []db.Allowance{deny("group", "lab_printing")},
	})

	if want := []string{"lab_makers"}; !reflect.DeepEqual(set.Fields["group"], want) {
		t.Fatalf("the denied value must be removed and the rest kept, got %v", set.Fields["group"])
	}
	// A subject holding a role whose access they do not have is a trap unless
	// it is visible, so the reason travels with the absence.
	if len(set.Suppressed) != 1 {
		t.Fatalf("the carve-out must be reported, got %+v", set.Suppressed)
	}
	s := set.Suppressed[0]
	if s.Value != "lab_printing" || s.ActorID != "op_1" || s.Reason != "safety review" {
		t.Errorf("the carve-out must carry its actor and reason: %+v", s)
	}
	// Still enabled: they hold a mapped role, and one value being suspended is
	// not the same as reaching nothing.
	if !set.Lifecycle.Enabled {
		t.Error("suspending one value must not disable the account")
	}
}

// A field every value of which is suppressed must still appear, empty. An
// absent field and an empty one mean different things to a level-triggered
// apply: "do not manage this" and "make it empty".
func TestAFullySuppressedFieldResolvesToEmptyRatherThanAbsent(t *testing.T) {
	set := resolve(t, &resolverFixture{
		roles:      []db.RoleRef{{ProjectID: "pLab", RoleKey: "maker"}},
		mappings:   []db.RoleMapping{mapping("maker", "group", "lab_makers")},
		allowances: []db.Allowance{deny("group", "lab_makers")},
	})
	got, present := set.Fields["group"]
	if !present {
		t.Fatal("the field must still be managed")
	}
	if len(got) != 0 {
		t.Fatalf("and must resolve to nothing, got %v", got)
	}
}

// Losing the last mapped role disables — derived, and it clears itself when a
// role returns. Nothing special-cases restoration because nothing special-cased
// suspension.
func TestLifecycleFollowsWhetherAnyMappedRoleIsHeld(t *testing.T) {
	none := resolve(t, &resolverFixture{roles: []db.RoleRef{{ProjectID: "pLab", RoleKey: "unmapped"}}})
	if none.Lifecycle.Enabled || none.Lifecycle.SMBEnabled {
		t.Fatal("no mapped role must resolve to disabled")
	}
	back := resolve(t, &resolverFixture{
		roles:    []db.RoleRef{{ProjectID: "pLab", RoleKey: "maker"}},
		mappings: []db.RoleMapping{mapping("maker", "group", "lab_makers")},
	})
	if !back.Lifecycle.Enabled {
		t.Fatal("regaining a mapped role must resolve it back through the same path")
	}
}

// An operator suspension is a different lock from a derived one, on a target
// with no field to tell them apart. It survives re-resolution while the subject
// still holds the role, which a derived lock never would.
func TestAnOperatorSuspensionSurvivesHoldingTheRole(t *testing.T) {
	set := resolve(t, &resolverFixture{
		roles:      []db.RoleRef{{ProjectID: "pLab", RoleKey: "maker"}},
		mappings:   []db.RoleMapping{mapping("maker", "group", "lab_makers")},
		allowances: []db.Allowance{deny(FieldEnabled, "true")},
	})
	if set.Lifecycle.Enabled {
		t.Fatal("a role grant must not undo a deliberate suspension")
	}
	// An account that cannot be used cannot be used over SMB either; reporting
	// otherwise would describe a state the target cannot hold.
	if set.Lifecycle.SMBEnabled {
		t.Error("disabling the account must take SMB with it")
	}
	var found bool
	for _, s := range set.Suppressed {
		if s.Field == FieldEnabled {
			found = true
		}
	}
	if !found {
		t.Error("the suspension must be visible, or the two locks are indistinguishable")
	}
}

// SMB alone is its own suspension: an account that works, over a protocol that
// does not.
func TestSMBCanBeSuspendedWithoutDisablingTheAccount(t *testing.T) {
	set := resolve(t, &resolverFixture{
		roles:      []db.RoleRef{{ProjectID: "pLab", RoleKey: "maker"}},
		mappings:   []db.RoleMapping{mapping("maker", "group", "lab_makers")},
		allowances: []db.Allowance{deny(FieldSMBEnabled, "true")},
	})
	if !set.Lifecycle.Enabled || set.Lifecycle.SMBEnabled {
		t.Fatalf("want enabled without SMB, got %+v", set.Lifecycle)
	}
}

// A mapping that got in another way must not fight the derived lifecycle on
// every resolution. Refused at the write, and ignored here as well.
func TestALifecycleMappingIsIgnoredByTheResolver(t *testing.T) {
	set := resolve(t, &resolverFixture{
		roles: []db.RoleRef{{ProjectID: "pLab", RoleKey: "maker"}},
		mappings: []db.RoleMapping{
			mapping("maker", FieldEnabled, "false"),
			mapping("maker", "group", "lab_makers"),
		},
	})
	if _, present := set.Fields[FieldEnabled]; present {
		t.Fatal("a lifecycle field must never be resolved from a mapping")
	}
	if !set.Lifecycle.Enabled {
		t.Fatal("and must not be able to disable an account by being present")
	}

	// And it must not COUNT as reaching the target either. A role mapped only
	// to a lifecycle field maps to nothing the add-on can act on, so treating
	// it as coverage would enable an account with no entitlement behind it —
	// and, worse, keep it enabled after every real mapping is removed.
	lifecycleOnly := resolve(t, &resolverFixture{
		roles:    []db.RoleRef{{ProjectID: "pLab", RoleKey: "maker"}},
		mappings: []db.RoleMapping{mapping("maker", FieldEnabled, "true")},
	})
	if lifecycleOnly.Lifecycle.Enabled {
		t.Fatal("a lifecycle-only mapping is not a mapped entitlement")
	}
}

// The additive arm is not built. A row that got in around the write must not
// confer access from a code path nobody reviewed.
func TestAnAdditiveAllowanceConfersNothing(t *testing.T) {
	set := resolve(t, &resolverFixture{
		roles:    []db.RoleRef{{ProjectID: "pLab", RoleKey: "maker"}},
		mappings: []db.RoleMapping{mapping("maker", "group", "lab_makers")},
		allowances: []db.Allowance{
			// Names a value the role already confers, so an arm that resolved
			// it as a denial would REMOVE access rather than merely fail to add
			// any — the direction that matters.
			{ID: "a2", Target: "truenas", Field: "group", Value: "lab_makers",
				Direction: db.AllowanceAllow, ActorID: "op_1", Reason: "temporary"},
			{ID: "a3", Target: "truenas", Field: "group", Value: "lab_admins",
				Direction: db.AllowanceAllow, ActorID: "op_1", Reason: "temporary"},
		},
	})
	if want := []string{"lab_makers"}; !reflect.DeepEqual(set.Fields["group"], want) {
		t.Fatalf("an additive allowance must neither confer nor remove, got %v", set.Fields["group"])
	}
	if len(set.Suppressed) != 0 {
		t.Fatalf("and must suppress nothing: %+v", set.Suppressed)
	}
}

// 7.4 — structural validation, before the add-on is asked anything.
func TestMappingFieldValidation(t *testing.T) {
	declared := []string{"group", "quota_bytes"}

	if err := ValidateMappingField(declared, "group"); err != nil {
		t.Fatalf("a declared field must be accepted: %v", err)
	}
	for _, field := range []string{FieldEnabled, FieldSMBEnabled} {
		err := ValidateMappingField(append(declared, field), field)
		if !errors.Is(err, db.ErrMappingInvalid) {
			t.Errorf("%s must be refused as a mapping target even when the add-on declares it: %v", field, err)
		}
	}
	err := ValidateMappingField(declared, "path_grant")
	if !errors.Is(err, db.ErrMappingInvalid) {
		t.Fatalf("an undeclared field must be refused: %v", err)
	}
	// The declared set is named: an operator whose field is not in the schema
	// needs to know what is.
	if got := err.Error(); !strings.Contains(got, "group") || !strings.Contains(got, "quota_bytes") {
		t.Errorf("the refusal must name the declared fields, got %q", got)
	}
	if err := ValidateMappingField(declared, "  "); !errors.Is(err, db.ErrMappingInvalid) {
		t.Errorf("a blank field must be refused: %v", err)
	}
}

// 8.11/8.12 — the third band.
//
// Every entitlement attributes to a source role or a derivation rule, and every
// SUPPRESSED entitlement attributes to the allowance suppressing it, with its
// actor and time. A subject can hold a role whose access they do not have, and
// that is a trap unless it is visible.
func TestTheAllowanceBandCarriesTheWholeHistoryWithItsAttribution(t *testing.T) {
	past := time.Now().Add(-24 * time.Hour)
	future := time.Now().Add(24 * time.Hour)

	orig := svcAllowancesForSubject
	t.Cleanup(func() { svcAllowancesForSubject = orig })
	svcAllowancesForSubject = func(context.Context, string) ([]db.Allowance, error) {
		return []db.Allowance{
			{ID: "a1", Target: "truenas", Field: "group", Value: "lab_printing",
				Direction: db.AllowanceDeny, ActorID: "op_1", Reason: "safety review",
				CreatedAt: past, ExpiresAt: &future},
			{ID: "a2", Target: "truenas", Field: "enabled", Value: "true",
				Direction: db.AllowanceDeny, ActorID: "op_2", Reason: "open incident",
				CreatedAt: past, ReviewDate: &past},
			{ID: "a3", Target: "truenas", Field: "group", Value: "lab_laser",
				Direction: db.AllowanceDeny, ActorID: "op_1", Reason: "expired dues",
				CreatedAt: past, ExpiresAt: &past},
			{ID: "a4", Target: "truenas", Field: "group", Value: "lab_wood",
				Direction: db.AllowanceDeny, ActorID: "op_1", Reason: "lifted early",
				CreatedAt: past, ReviewDate: &future, LiftedAt: &past, LiftedBy: "op_3"},
		}, nil
	}

	band, err := allowanceBand(context.Background(), "u1")
	if err != nil {
		t.Fatal(err)
	}
	if len(band) != 4 {
		t.Fatalf("the band must carry the whole history, not only what is in force: %d rows", len(band))
	}
	byID := map[string]models.AllowanceBand{}
	for _, b := range band {
		byID[b.ID] = b
	}

	// Every row attributes to who decided and why.
	for id, want := range map[string]string{"a1": "op_1", "a2": "op_2", "a3": "op_1", "a4": "op_1"} {
		if byID[id].ActorID != want || byID[id].Reason == "" {
			t.Errorf("%s must attribute to its actor and reason: %+v", id, byID[id])
		}
		if byID[id].CreatedAt == "" {
			t.Errorf("%s must carry when it was decided", id)
		}
	}

	if !byID["a1"].InForce || byID["a1"].Ended != "" {
		t.Errorf("an unexpired allowance is in force and has not ended: %+v", byID["a1"])
	}
	// A passed review date surfaces the decision and never makes it.
	if !byID["a2"].InForce || !byID["a2"].ReviewDue {
		t.Errorf("a passed review date must surface WITHOUT lifting: %+v", byID["a2"])
	}
	// Lapsed and lifted are different states, and the band says which.
	if byID["a3"].InForce || byID["a3"].EndedBy != "the expiry date" {
		t.Errorf("a lapsed allowance must say the date ended it: %+v", byID["a3"])
	}
	if byID["a4"].InForce || byID["a4"].EndedBy != "op_3" {
		t.Errorf("a lifted allowance must name who ended it: %+v", byID["a4"])
	}
}

// A read failure must not look like "no carve-outs". An empty band and an
// unread band are identical to a surface, and one of them means this person is
// suspended from something the view cannot show.
func TestAnUnreadableBandSaysSoRatherThanLookingEmpty(t *testing.T) {
	orig := svcAllowancesForSubject
	t.Cleanup(func() { svcAllowancesForSubject = orig })
	svcAllowancesForSubject = func(context.Context, string) ([]db.Allowance, error) {
		return nil, errors.New("db down")
	}

	band, err := allowanceBand(context.Background(), "u1")
	if err == nil {
		t.Fatal("the failure must be reported")
	}
	if band == nil {
		t.Error("and the band must still be a list rather than nil, so a surface renders empty rather than crashing")
	}
}
