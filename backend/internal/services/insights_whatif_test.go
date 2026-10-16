package services

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"

	"syndra/internal/db"
	"syndra/internal/directory"
	"syndra/internal/models"
)

// whatIfWorld is one org as the cascade readers see it. Nothing touches a
// database; the stubs are the same seams the cascade tests use.
type whatIfWorld struct {
	users   []models.UserProfile
	holders []string                                  // holders of bundle "b1"
	direct  map[string][]roleKey                      // direct grants
	grouped map[string]map[string][]models.BundleRole // user -> bundle -> what their pinned version gives
	rules   []models.MappingRule
	// includes: the roles each bundle included by "b1"'s pinned version gives.
	// includeReads counts svcGetVersionIncludes calls.
	includes     map[string][]models.BundleRole
	includeReads int
	snaps        int
	locked       bool
}

func newWhatIfWorld(names ...string) *whatIfWorld {
	w := &whatIfWorld{direct: map[string][]roleKey{}, grouped: map[string]map[string][]models.BundleRole{}}
	for _, n := range names {
		w.users = append(w.users, models.UserProfile{ID: n, Name: n})
	}
	return w
}

// holds gives the user bundle b1 at a version giving exactly these roles.
func (w *whatIfWorld) holds(user string, roles ...roleKey) {
	w.holders = append(w.holders, user)
	var rs []models.BundleRole
	for _, r := range roles {
		rs = append(rs, models.BundleRole{BundleID: "b1", ProjectID: r.projectID, RoleKey: r.roleKey})
	}
	w.grouped[user] = map[string][]models.BundleRole{"b1": rs}
}

func (w *whatIfWorld) install(t *testing.T) {
	t.Helper()
	resetCascadeDeps(t)
	origDir, origSnap, origObs, origRule := directory.Default, svcInReadSnapshot, svcLatestOrgObservation, svcGetMappingRuleByID
	origLock, origHolders, origIncludes := svcInTxLockingAccess, svcGetBundleHoldersByVersion, svcGetVersionIncludes
	t.Cleanup(func() {
		svcGetBundleHoldersByVersion, svcGetVersionIncludes = origHolders, origIncludes
		directory.Default, svcInReadSnapshot, svcLatestOrgObservation, svcGetMappingRuleByID = origDir, origSnap, origObs, origRule
		svcInTxLockingAccess = origLock
	})
	directory.Default = &snapshotFixtureDirectory{users: w.users}
	svcInReadSnapshot = func(ctx context.Context, fn func(context.Context) error) error {
		w.snaps++
		return fn(ctx)
	}
	svcInTxLockingAccess = func(ctx context.Context, fn func(context.Context) error) error {
		w.locked = true
		return fn(ctx)
	}
	svcLatestOrgObservation = func(context.Context) (db.Observation, error) { return db.Observation{}, db.ErrNoObservation }
	svcGetBundleByID = func(_ context.Context, id string) (models.Bundle, error) {
		if id != "b1" {
			return models.Bundle{}, pgx.ErrNoRows
		}
		return models.Bundle{ID: id}, nil
	}
	svcGetMappingRuleByID = func(_ context.Context, id string) (models.MappingRule, error) {
		for _, r := range w.rules {
			if r.ID == id {
				return r, nil
			}
		}
		return models.MappingRule{}, pgx.ErrNoRows
	}
	svcGetUsersForBundle = func(context.Context, string) ([]string, error) { return w.holders, nil }
	svcGetBundleHoldersByVersion = func(context.Context, string) ([]models.BundleHolder, error) {
		var out []models.BundleHolder
		for _, u := range w.holders {
			out = append(out, models.BundleHolder{BundleID: "b1", UserID: u, VersionID: "v1"})
		}
		return out, nil
	}
	svcGetVersionIncludes = func(context.Context, string) ([]models.BundleInclude, error) {
		w.includeReads++
		var out []models.BundleInclude
		for child := range w.includes {
			out = append(out, models.BundleInclude{ChildBundleID: child, ChildVersionID: "cv-" + child})
		}
		return out, nil
	}
	svcGetRolesForVersion = func(_ context.Context, versionID string) ([]models.BundleRole, error) {
		return w.includes[strings.TrimPrefix(versionID, "cv-")], nil
	}
	svcGetAllKnownUserIDs = func(context.Context) ([]string, error) {
		ids := make([]string, 0, len(w.users))
		for _, u := range w.users {
			ids = append(ids, u.ID)
		}
		return ids, nil
	}
	svcGetDirectGrantsForUser = func(_ context.Context, u string, _ bool) ([]models.DirectGrant, error) {
		var out []models.DirectGrant
		for _, k := range w.direct[u] {
			out = append(out, models.DirectGrant{ProjectID: k.projectID, RoleKey: k.roleKey})
		}
		return out, nil
	}
	svcGetUserBundleRolesGrouped = func(_ context.Context, u string) (map[string][]models.BundleRole, error) {
		return w.grouped[u], nil
	}
	svcGetActiveMappingRules = func(context.Context) ([]models.MappingRule, error) { return w.rules, nil }
}

func (w *whatIfWorld) ask(t *testing.T, c models.InsightsWhatIfChange) models.InsightsWhatIf {
	t.Helper()
	w.install(t)
	got, err := InsightsWhatIf(context.Background(), c)
	if err != nil {
		t.Fatalf("InsightsWhatIf: %v", err)
	}
	if w.snaps != 1 || w.locked {
		t.Fatalf("want one read snapshot and no access lock, got snapshots=%d locked=%v", w.snaps, w.locked)
	}
	return got
}

func wiGrants(keys ...roleKey) []models.RoleGrant {
	g := make([]models.RoleGrant, 0, len(keys))
	for _, k := range keys {
		g = append(g, models.RoleGrant{ProjectID: k.projectID, RoleKey: k.roleKey})
	}
	return g
}

func wiNames(ps []models.InsightWhatIfPerson) []string {
	out := []string{}
	for _, p := range ps {
		out = append(out, p.Name)
	}
	return out
}

var (
	wiLaser = roleKey{"p1", "laser"}
	wiMill  = roleKey{"p1", "mill"}
	wiDoor  = roleKey{"p2", "door"}
)

func wiChg(kind string) models.InsightsWhatIfChange {
	return models.InsightsWhatIfChange{Kind: kind, BundleID: "b1", ProjectID: "p1", RoleKey: "laser"}
}

func TestWhatIf_GivesNot_LossOnlyWhereTheBundleIsTheOnlyRoute(t *testing.T) {
	w := newWhatIfWorld("Zed", "Amy", "Bob")
	w.holds("Zed", wiLaser, wiMill) // only via the bundle: loses laser
	w.holds("Amy", wiLaser)         // also holds laser directly: keeps it
	w.direct["Amy"] = []roleKey{wiLaser}
	w.holds("Bob", wiMill) // bundle never gave laser: unchanged

	got := w.ask(t, wiChg("bundle_gives_not"))
	if want := []string{"Zed"}; !reflect.DeepEqual(wiNames(got.Losses), want) {
		t.Fatalf("losses = %v, want %v", wiNames(got.Losses), want)
	}
	if !reflect.DeepEqual(got.Losses[0].Roles, wiGrants(wiLaser)) {
		t.Fatalf("roles lost = %+v", got.Losses[0].Roles)
	}
	if len(got.Gains) != 0 || got.Unchanged != 2 {
		t.Fatalf("gains=%v unchanged=%d, want none and 2", got.Gains, got.Unchanged)
	}
	if got.Change != wiChg("bundle_gives_not") {
		t.Fatalf("change not echoed: %+v", got.Change)
	}
}

func TestWhatIf_GivesNot_AMappedRoleGoesWithItsTrigger(t *testing.T) {
	w := newWhatIfWorld("Amy")
	w.holds("Amy", wiLaser)
	w.rules = []models.MappingRule{{ID: "r1", SourceProject: "p1", SourceRole: "laser", TargetProject: "p2", TargetRole: "door"}}

	got := w.ask(t, wiChg("bundle_gives_not"))
	if len(got.Losses) != 1 || !reflect.DeepEqual(got.Losses[0].Roles, wiGrants(wiLaser, wiDoor)) {
		t.Fatalf("losses = %+v, want laser and the door it opened", got.Losses)
	}
}

func TestWhatIf_GivesNot_ARoleTheBundleAlsoGetsThroughAnIncludeIsNoLoss(t *testing.T) {
	w := newWhatIfWorld("Amy", "Bob")
	// The bundle gives laser itself AND through included bundle "s".
	w.includes = map[string][]models.BundleRole{"s": {{BundleID: "s", ProjectID: "p1", RoleKey: "laser"}}}
	w.holds("Amy", wiLaser)
	w.holds("Bob", wiLaser, wiMill)

	got := w.ask(t, wiChg("bundle_gives_not"))
	if len(got.Losses) != 0 || got.Unchanged != 2 {
		t.Fatalf("an include still gives it, so removing it takes nothing: %+v", got)
	}
	if w.includeReads != 1 {
		t.Fatalf("includes read %d times for one pinned version, want 1", w.includeReads)
	}
}

func TestWhatIf_GivesNot_AnIncludeGivingSomethingElseDoesNotShieldTheRole(t *testing.T) {
	w := newWhatIfWorld("Amy")
	w.includes = map[string][]models.BundleRole{"s": {{BundleID: "s", ProjectID: "p1", RoleKey: "mill"}}}
	w.holds("Amy", wiLaser, wiMill)

	got := w.ask(t, wiChg("bundle_gives_not"))
	if len(got.Losses) != 1 || !reflect.DeepEqual(got.Losses[0].Roles, wiGrants(wiLaser)) {
		t.Fatalf("losses = %+v, want laser", got.Losses)
	}
}

func TestWhatIf_GivesAlso_GainOnlyForThoseWhoLackIt(t *testing.T) {
	w := newWhatIfWorld("Zed", "Amy", "Bob", "Cy")
	w.holds("Zed", wiMill)
	w.holds("Amy", wiMill)
	w.direct["Amy"] = []roleKey{wiLaser} // already holds it directly: not a gain
	w.holds("Bob", wiMill, wiLaser)      // pinned version already gives it: not a gain
	w.holds("Cy")                        // holds the bundle at an empty version: gains
	w.rules = []models.MappingRule{{ID: "r1", SourceProject: "p1", SourceRole: "laser", TargetProject: "p2", TargetRole: "door"}}

	got := w.ask(t, wiChg("bundle_gives_also"))
	if want := []string{"Cy", "Zed"}; !reflect.DeepEqual(wiNames(got.Gains), want) {
		t.Fatalf("gains = %v, want %v (sorted by name)", wiNames(got.Gains), want)
	}
	// Mapping rules are part of the closure: laser opens the door too.
	if !reflect.DeepEqual(got.Gains[0].Roles, wiGrants(wiLaser, wiDoor)) {
		t.Fatalf("gain roles = %+v", got.Gains[0].Roles)
	}
	if len(got.Losses) != 0 || got.Unchanged != 2 {
		t.Fatalf("losses=%v unchanged=%d, want none and 2", got.Losses, got.Unchanged)
	}
}

func TestWhatIf_BundleGone_EveryHolderLosesWhatNothingElseGives(t *testing.T) {
	w := newWhatIfWorld("Amy", "Bob", "Nobody")
	w.holds("Amy", wiLaser, wiMill)
	w.direct["Amy"] = []roleKey{wiMill}
	w.holds("Bob", wiLaser)
	// Nobody holds no bundle, so is not examined at all.

	got := w.ask(t, models.InsightsWhatIfChange{Kind: "bundle_gone", BundleID: "b1"})
	if want := []string{"Amy", "Bob"}; !reflect.DeepEqual(wiNames(got.Losses), want) {
		t.Fatalf("losses = %v, want %v", wiNames(got.Losses), want)
	}
	if !reflect.DeepEqual(got.Losses[0].Roles, wiGrants(wiLaser)) || !reflect.DeepEqual(got.Losses[1].Roles, wiGrants(wiLaser)) {
		t.Fatalf("Amy keeps mill directly: %+v", got.Losses)
	}
	if got.Unchanged != 0 {
		t.Fatalf("unchanged = %d", got.Unchanged)
	}
}

func TestWhatIf_RuleGone_DropsTheDerivedRole(t *testing.T) {
	w := newWhatIfWorld("Amy", "Bob", "Cy")
	w.rules = []models.MappingRule{
		{ID: "r1", SourceProject: "p1", SourceRole: "laser", TargetProject: "p2", TargetRole: "door"},
		{ID: "r2", SourceProject: "p1", SourceRole: "mill", TargetProject: "p2", TargetRole: "door"},
	}
	w.direct["Amy"] = []roleKey{wiLaser}         // door only through r1: loses it
	w.direct["Bob"] = []roleKey{wiLaser, wiMill} // r2 still gives the door: keeps it
	w.direct["Cy"] = []roleKey{wiMill}           // r1 never fired for Cy

	got := w.ask(t, models.InsightsWhatIfChange{Kind: "rule_gone", RuleID: "r1"})
	if want := []string{"Amy"}; !reflect.DeepEqual(wiNames(got.Losses), want) {
		t.Fatalf("losses = %v, want %v", wiNames(got.Losses), want)
	}
	if !reflect.DeepEqual(got.Losses[0].Roles, wiGrants(wiDoor)) || len(got.Gains) != 0 || got.Unchanged != 2 {
		t.Fatalf("got %+v", got)
	}
}

func TestWhatIf_PeopleMissingFromTheDirectoryAreLeftOut(t *testing.T) {
	w := newWhatIfWorld("Amy")
	w.holds("Amy", wiLaser)
	w.holds("ghost", wiLaser)
	got := w.ask(t, models.InsightsWhatIfChange{Kind: "bundle_gone", BundleID: "b1"})
	if want := []string{"Amy"}; !reflect.DeepEqual(wiNames(got.Losses), want) {
		t.Fatalf("losses = %v, want %v", wiNames(got.Losses), want)
	}
}

func TestWhatIf_UnknownBundleOrRuleIsNotFound(t *testing.T) {
	for name, c := range map[string]models.InsightsWhatIfChange{
		"bundle": {Kind: "bundle_gone", BundleID: "nope"},
		"rule":   {Kind: "rule_gone", RuleID: "nope"},
	} {
		w := newWhatIfWorld("Amy")
		w.install(t)
		_, err := InsightsWhatIf(context.Background(), c)
		if !errors.Is(err, ErrWhatIfNotFound) {
			t.Errorf("%s: err = %v, want ErrWhatIfNotFound", name, err)
		}
	}
}

func TestWhatIf_MalformedChangesAreRefusedBeforeAnyRead(t *testing.T) {
	for name, c := range map[string]models.InsightsWhatIfChange{
		"empty":              {},
		"unknown kind":       {Kind: "bundle_explodes", BundleID: "b1"},
		"gives_also no role": {Kind: "bundle_gives_also", BundleID: "b1", ProjectID: "p1"},
		"gives_not no proj":  {Kind: "bundle_gives_not", BundleID: "b1", RoleKey: "x"},
		"gives_also no id":   {Kind: "bundle_gives_also", ProjectID: "p1", RoleKey: "x"},
		"gone no id":         {Kind: "bundle_gone"},
		"rule no id":         {Kind: "rule_gone"},
		"odd id":             {Kind: "bundle_gone", BundleID: "b1'; --"},
		"stray field":        {Kind: "bundle_gone", BundleID: "b1", RuleID: "r1"},
		"stray role":         {Kind: "rule_gone", RuleID: "r1", RoleKey: "x"},
	} {
		w := newWhatIfWorld("Amy")
		w.install(t)
		_, err := InsightsWhatIf(context.Background(), c)
		if !errors.Is(err, ErrInvalidWhatIf) {
			t.Errorf("%s: err = %v, want ErrInvalidWhatIf", name, err)
		}
		if w.snaps != 0 {
			t.Errorf("%s: a malformed change reached the store", name)
		}
	}
}
