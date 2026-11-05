package services

import (
	"context"
	"slices"
	"sort"
	"testing"
	"time"

	"syndra/internal/access"
	"syndra/internal/db"
	"syndra/internal/directory"
	"syndra/internal/models"
)

// parity is one small deployment every "who holds what" reader is run over:
// a dated direct grant feeding a rule chain, a bundle whose pinned version
// gives a role through an include, a person who has left, and observed grants
// that only partly match the records.
type parity struct {
	users   []models.UserProfile
	direct  map[string][]models.DirectGrant
	bundles map[string][]models.Bundle     // user -> assigned bundles
	pinned  map[string][]models.BundleRole // user -> roles their pinned versions give
	seen    map[string][]db.ObservedGrant  // user -> what Zitadel holds
	rules   []models.MappingRule
}

func parityFixture() parity {
	june := time.Date(2027, 6, 30, 0, 0, 0, 0, time.UTC)
	return parity{
		users: []models.UserProfile{
			{ID: "alice", Name: "Alice", Status: "active"},
			{ID: "bob", Name: "Bob", Status: "active"},
			{ID: "carol", Name: "Carol", Status: "active"},
			{ID: "dave", Name: "Dave", Status: "locked"}, // has left
			{ID: "erin", Name: "Erin", Status: "active"}, // holds only in Zitadel
		},
		direct: map[string][]models.DirectGrant{
			"alice": {{UserID: "alice", ProjectID: "p", RoleKey: "a", ExpiresAt: &june}},
			"carol": {{UserID: "carol", ProjectID: "p", RoleKey: "z"}},
			"dave":  {{UserID: "dave", ProjectID: "p", RoleKey: "a"}},
		},
		bundles: map[string][]models.Bundle{"bob": {{ID: "b1", Name: "Makers", PinnedVersion: 2}}},
		pinned: map[string][]models.BundleRole{"bob": {
			{BundleID: "b1", ProjectID: "p", RoleKey: "x"},
			{BundleID: "b1", ProjectID: "p", RoleKey: "a", ViaBundleID: "b2"}, // through an include
		}},
		seen: map[string][]db.ObservedGrant{
			"alice": {{UserID: "alice", ProjectID: "p", RoleKeys: []string{"a", "b"}}}, // q:c never arrived
			"bob":   {{UserID: "bob", ProjectID: "p", RoleKeys: []string{"x", "a", "b"}}, {UserID: "bob", ProjectID: "q", RoleKeys: []string{"c"}}},
			"dave":  {{UserID: "dave", ProjectID: "p", RoleKeys: []string{"a", "b"}}},
			"erin":  {{UserID: "erin", ProjectID: "p", RoleKeys: []string{"a", "b"}}},
		},
		rules: []models.MappingRule{
			{ID: "r1", SourceProject: "p", SourceRole: "a", TargetProject: "p", TargetRole: "b"},
			{ID: "r2", SourceProject: "p", SourceRole: "b", TargetProject: "q", TargetRole: "c"},
		},
	}
}

func (f parity) people() []string {
	out := make([]string, len(f.users))
	for i, u := range f.users {
		out[i] = u.ID
	}
	return out
}

// records is the base each person holds in Syndra's records, as HeldRecords
// reads it in one query.
func (f parity) records() map[string]map[access.Role]bool {
	out := map[string]map[access.Role]bool{}
	add := func(u, p, r string) {
		if out[u] == nil {
			out[u] = map[access.Role]bool{}
		}
		out[u][access.Role{ProjectID: p, RoleKey: r}] = true
	}
	for u, gs := range f.direct {
		for _, g := range gs {
			add(u, g.ProjectID, g.RoleKey)
		}
	}
	for u, rs := range f.pinned {
		for _, r := range rs {
			add(u, r.ProjectID, r.RoleKey)
		}
	}
	return out
}

func (f parity) install(t *testing.T) {
	t.Helper()
	resetGovernanceDeps(t)
	origDir, origHeld, origByID := directory.Default, svcHeldRecords, svcGetBundleByID
	origObs, origSeen := svcLatestOrgObservation, svcObservedGrantsFor
	t.Cleanup(func() {
		directory.Default, svcHeldRecords, svcGetBundleByID = origDir, origHeld, origByID
		svcLatestOrgObservation, svcObservedGrantsFor = origObs, origSeen
	})
	directory.Default = &snapshotFixtureDirectory{users: f.users}
	svcGetDirectGrantsForUser = func(_ context.Context, u string, _ bool) ([]models.DirectGrant, error) { return f.direct[u], nil }
	svcGetBundlesForUser = func(_ context.Context, u string) ([]models.Bundle, error) { return f.bundles[u], nil }
	svcGetUserBundleRolesGrouped = func(_ context.Context, u string) (map[string][]models.BundleRole, error) {
		out := map[string][]models.BundleRole{}
		for _, r := range f.pinned[u] {
			out[r.BundleID] = append(out[r.BundleID], r)
		}
		return out, nil
	}
	svcGetBundleByID = func(_ context.Context, id string) (models.Bundle, error) { return models.Bundle{ID: id, Name: id}, nil }
	svcGetActiveMappingRules = func(context.Context) ([]models.MappingRule, error) { return f.rules, nil }
	svcHeldRecords = func(context.Context) (map[string]map[access.Role]bool, error) { return f.records(), nil }
	svcLatestOrgObservation = func(context.Context) (db.Observation, error) {
		return db.Observation{ObservedAt: time.Now(), Complete: true}, nil
	}
	svcObservedGrantsFor = func(_ context.Context, u string) ([]db.ObservedGrant, error) { return f.seen[u], nil }
}

// viewHolders is who the person view (collectUserRoles) says holds a role.
func viewHolders(t *testing.T, f parity, role access.Role) []string {
	t.Helper()
	var out []string
	for _, u := range f.people() {
		roleMap, _, err := collectUserRoles(context.Background(), u)
		if err != nil {
			t.Fatal(err)
		}
		if roleMap[roleKey{role.ProjectID, role.RoleKey}] != nil {
			out = append(out, u)
		}
	}
	sort.Strings(out)
	return out
}

// The mapping cohort agrees with the person view for every role the fixture
// touches, except that people who have left are not in it. Includes and the
// whole chain are followed; nobody holding only in Zitadel is counted.
func TestParity_MappingHoldersAgreeWithThePersonView(t *testing.T) {
	f := parityFixture()
	f.install(t)
	for _, role := range []access.Role{access.NewRole("p", "a"), access.NewRole("p", "b"), access.NewRole("q", "c"), access.NewRole("p", "x"), access.NewRole("p", "z")} {
		got, err := MappingHolders(context.Background(), role.ProjectID, role.RoleKey)
		if err != nil {
			t.Fatal(err)
		}
		want := slices.DeleteFunc(viewHolders(t, f, role), func(u string) bool { return u == "dave" })
		if !slices.Equal(got, want) {
			t.Errorf("%v: cohort %v, person view (minus people who left) %v", role, got, want)
		}
	}
	// Spelled out once, so the parity above is not vacuous: bob reaches q:c
	// through an include and two rules; dave has left; erin is only observed.
	got, _ := MappingHolders(context.Background(), "q", "c")
	if !slices.Equal(got, []string{"alice", "bob"}) {
		t.Fatalf("holders of q:c = %v, want alice and bob", got)
	}
}

// The token compiler, the cascades and the person view share access.Closure;
// this pins the person view to it directly for every fixture person.
func TestParity_PersonViewIsTheCoreClosure(t *testing.T) {
	f := parityFixture()
	f.install(t)
	records := f.records()
	for _, u := range f.people() {
		roleMap, _, err := collectUserRoles(context.Background(), u)
		if err != nil {
			t.Fatal(err)
		}
		want := access.Closure(records[u], f.rules, access.NewRole)
		if len(roleMap) != len(want) {
			t.Errorf("%s: view %d roles, core %d", u, len(roleMap), len(want))
		}
		for k := range want {
			if roleMap[roleKey{k.ProjectID, k.RoleKey}] == nil {
				t.Errorf("%s: core holds %v, the view does not", u, k)
			}
		}
	}
}

// A rule role ends when its source ends, on the person view as in the token:
// alice's direct grant of a ends in June, so b and c (through two rules) end
// then too. bob's come from a bundle and never end.
func TestParity_ARuleRoleEndsWithItsSourceOnThePersonView(t *testing.T) {
	f := parityFixture()
	f.install(t)
	june := time.Date(2027, 6, 30, 0, 0, 0, 0, time.UTC)
	alice, _, err := collectUserRoles(context.Background(), "alice")
	if err != nil {
		t.Fatal(err)
	}
	for _, k := range []roleKey{{"p", "a"}, {"p", "b"}, {"q", "c"}} {
		if r := alice[k]; r == nil || r.Ends == nil || !r.Ends.Equal(june) {
			t.Errorf("alice %v ends %v, want %v", k, r, june)
		}
	}
	bob, _, err := collectUserRoles(context.Background(), "bob")
	if err != nil {
		t.Fatal(err)
	}
	for k, r := range bob {
		if r.Ends != nil {
			t.Errorf("bob %v ends %v, want never (a bundle gives it)", k, r.Ends)
		}
	}
}

// Bundle and rule counts are observed. Bob's bundle is held: Zitadel shows
// every role his pinned version gives, the included one too. The rule counts
// separate who should have the target (records) from who has it (Zitadel).
func TestHolding_BundleAndRuleCountsAreObserved(t *testing.T) {
	f := parityFixture()
	f.install(t)

	b, err := BundleHolding(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if b.Given["b1"] != 1 || b.Observed["b1"] != 1 {
		t.Fatalf("bundle: given %v observed %v", b.Given, b.Observed)
	}

	r, err := RuleHolding(context.Background(), f.rules)
	if err != nil {
		t.Fatal(err)
	}
	// r1 (a→b) applies to alice, bob, dave (records); all three hold p:b.
	// r2 (b→c) applies to the same three; only bob holds q:c in Zitadel.
	// erin holds a and b only in Zitadel: no rule applies to her.
	if r.Given["r1"] != 3 || r.Observed["r1"] != 3 || r.Given["r2"] != 3 || r.Observed["r2"] != 1 {
		t.Fatalf("rules: given %v observed %v", r.Given, r.Observed)
	}

	// Bob's include stops arriving: the bundle is no longer held, though it is
	// still assigned.
	f.seen["bob"] = []db.ObservedGrant{{UserID: "bob", ProjectID: "p", RoleKeys: []string{"x"}}}
	if b, _ = BundleHolding(context.Background()); b.Given["b1"] != 1 || b.Observed["b1"] != 0 {
		t.Fatalf("partly delivered bundle: given %v observed %v", b.Given, b.Observed)
	}
}

// Never read: the observed numbers are absent (nil), not zero.
func TestHolding_NeverObservedIsNotZero(t *testing.T) {
	f := parityFixture()
	f.install(t)
	svcLatestOrgObservation = func(context.Context) (db.Observation, error) { return db.Observation{}, db.ErrNoObservation }
	b, err := BundleHolding(context.Background())
	if err != nil || b.Observed != nil || b.Given["b1"] != 1 {
		t.Fatalf("bundle: %+v %v", b, err)
	}
	r, err := RuleHolding(context.Background(), f.rules)
	if err != nil || r.Observed != nil || r.Given["r1"] != 3 {
		t.Fatalf("rule: %+v %v", r, err)
	}
}
