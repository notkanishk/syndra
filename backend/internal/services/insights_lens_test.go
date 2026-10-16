package services

import (
	"context"
	"errors"
	"testing"
	"time"

	"syndra/internal/db"
	"syndra/internal/models"
)

func lensName(p, r string) (string, string) { return "P-" + p, "R-" + r }

func TestParseLensDays_DefaultsAndClamps(t *testing.T) {
	for in, want := range map[string]int{"": 30, "1": 1, "365": 365, "0": 1, "-5": 1, "366": 365, "99999": 365} {
		if got, err := ParseLensDays(in); err != nil || got != want {
			t.Errorf("%q = %d, %v; want %d", in, got, err, want)
		}
	}
	for _, bad := range []string{"x", "1.5", " 3", "99999999999999999999"} {
		if _, err := ParseLensDays(bad); !errors.Is(err, ErrInvalidLens) {
			t.Errorf("%q: err = %v", bad, err)
		}
	}
}

func TestParseLensGroup(t *testing.T) {
	for in, want := range map[string]LensGroup{
		"left":          {Kind: LensLeft},
		"new_term":      {Kind: LensNewTerm},
		"team:Fab Lab":  {Kind: LensTeam, Team: "Fab Lab"},
		"team:  Wood  ": {Kind: LensTeam, Team: "Wood"},
		"team:a:b":      {Kind: LensTeam, Team: "a:b"},
	} {
		if got, err := ParseLensGroup(in); err != nil || got != want {
			t.Errorf("%q = %+v, %v; want %+v", in, got, err, want)
		}
	}
	long := make([]byte, 101)
	for i := range long {
		long[i] = 'a'
	}
	for _, bad := range []string{"", "Left", "everyone", "team", "team:", "team:   ", "team:a\nb", "team:" + string(long)} {
		if _, err := ParseLensGroup(bad); !errors.Is(err, ErrInvalidLens) {
			t.Errorf("%q: err = %v", bad, err)
		}
	}
	if (LensGroup{Kind: LensTeam, Team: "Wood"}).String() != "team:Wood" {
		t.Error("team group does not round-trip")
	}
}

func TestTallyEnding_WindowEdges(t *testing.T) {
	now := time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC)
	until := now.AddDate(0, 0, 30)
	held := map[string]bool{"p:r": true}
	people := []lensPerson{
		{ID: "exact", Observed: held}, {ID: "after", Observed: held}, {ID: "ended", Observed: held},
		{ID: "now", Observed: held}, {ID: "soon", Observed: held}, {ID: "soon2", Observed: held},
	}
	grants := []db.ExpiringGrant{
		{UserID: "exact", ProjectID: "p", RoleKey: "r", ExpiresAt: until},                  // exactly N days: in
		{UserID: "after", ProjectID: "p", RoleKey: "r", ExpiresAt: until.Add(time.Second)}, // just past: out
		{UserID: "ended", ProjectID: "p", RoleKey: "r", ExpiresAt: now.Add(-time.Second)},  // already ended: out
		{UserID: "now", ProjectID: "p", RoleKey: "r", ExpiresAt: now},                      // ends this instant: ended
		{UserID: "soon", ProjectID: "p", RoleKey: "r", ExpiresAt: now.Add(48 * time.Hour)}, // in, and the earliest
		{UserID: "soon2", ProjectID: "p", RoleKey: "r", ExpiresAt: now.Add(72 * time.Hour)},
	}
	got := tallyEnding(grants, people, now, until, lensName)
	if len(got) != 1 || got[0].People != 3 {
		t.Fatalf("got %+v, want one role with 3 people", got)
	}
	if want := now.Add(48 * time.Hour); !got[0].EarliestEnd.Equal(want) {
		t.Errorf("earliest = %v, want %v", got[0].EarliestEnd, want)
	}
	if got[0].ProjectName != "P-p" || got[0].RoleLabel != "R-r" {
		t.Errorf("names not used: %+v", got[0])
	}
}

func TestTallyEnding_OnlyAccessThatReallyEnds(t *testing.T) {
	now := time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC)
	end := now.Add(time.Hour)
	people := []lensPerson{
		{ID: "holds", Observed: map[string]bool{"p:r": true}},
		{ID: "not-in-zitadel", Observed: map[string]bool{}},                                                 // recorded, not held
		{ID: "also-bundle", Observed: map[string]bool{"p:r": true}, Indirect: map[string]bool{"p:r": true}}, // still held another way
	}
	grants := []db.ExpiringGrant{
		{UserID: "holds", ProjectID: "p", RoleKey: "r", ExpiresAt: end},
		{UserID: "not-in-zitadel", ProjectID: "p", RoleKey: "r", ExpiresAt: end},
		{UserID: "also-bundle", ProjectID: "p", RoleKey: "r", ExpiresAt: end},
		{UserID: "stranger", ProjectID: "p", RoleKey: "r", ExpiresAt: end},
	}
	got := tallyEnding(grants, people, now, now.AddDate(0, 0, 30), lensName)
	if len(got) != 1 || got[0].People != 1 {
		t.Fatalf("got %+v, want 1 person", got)
	}
	if none := tallyEnding(nil, people, now, now.AddDate(0, 0, 30), lensName); none == nil || len(none) != 0 {
		t.Errorf("no grants should be [], got %#v", none)
	}
}

func TestTallyEnding_SortsByPeopleThenName(t *testing.T) {
	now := time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC)
	end := now.Add(time.Hour)
	obs := func(keys ...string) map[string]bool {
		m := map[string]bool{}
		for _, k := range keys {
			m[k] = true
		}
		return m
	}
	people := []lensPerson{{ID: "1", Observed: obs("b:r", "a:r", "c:r")}, {ID: "2", Observed: obs("c:r")}}
	var grants []db.ExpiringGrant
	for _, g := range []db.ExpiringGrant{
		{UserID: "1", ProjectID: "b", RoleKey: "r"}, {UserID: "1", ProjectID: "a", RoleKey: "r"},
		{UserID: "1", ProjectID: "c", RoleKey: "r"}, {UserID: "2", ProjectID: "c", RoleKey: "r"},
	} {
		g.ExpiresAt = end
		grants = append(grants, g)
	}
	got := tallyEnding(grants, people, now, now.AddDate(0, 0, 30), lensName)
	order := ""
	for _, r := range got {
		order += r.ProjectID
	}
	if order != "cab" {
		t.Errorf("order = %q, want cab", order)
	}
}

func TestPickGroup(t *testing.T) {
	people := []lensPerson{
		{ID: "act", Status: "active", Team: "Wood"},
		{ID: "ina", Status: "inactive"}, {ID: "loc", Status: "LOCKED"}, {ID: "del", Status: "deleted"},
		{ID: "dep", Status: "departed", Team: " wood "}, {ID: "ini", Status: "initial"}, {ID: "uns", Status: "unspecified"},
	}
	ids := func(ps []lensPerson) string {
		s := ""
		for _, p := range ps {
			s += p.ID + ","
		}
		return s
	}
	if got := ids(pickGroup(LensGroup{Kind: LensLeft}, people, nil, time.Time{})); got != "ina,loc,del,dep," {
		t.Errorf("left = %s", got)
	}
	if got := ids(pickGroup(LensGroup{Kind: LensTeam, Team: "WOOD"}, people, nil, time.Time{})); got != "act,dep," {
		t.Errorf("team = %s", got)
	}
	if got := pickGroup(LensGroup{Kind: LensTeam, Team: "Metal"}, people, nil, time.Time{}); len(got) != 0 {
		t.Errorf("unknown team = %v", got)
	}
}

func TestPickGroup_NewTermBoundary(t *testing.T) {
	start := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	people := []lensPerson{{ID: "on"}, {ID: "before"}, {ID: "after"}, {ID: "never"}}
	seen := map[string]time.Time{
		"on": start, "before": start.Add(-time.Nanosecond), "after": start.AddDate(0, 0, 40),
	}
	got := pickGroup(LensGroup{Kind: LensNewTerm}, people, seen, start)
	if len(got) != 2 || got[0].ID != "on" || got[1].ID != "after" {
		t.Errorf("new_term = %+v; want on, after (not before, not never-seen)", got)
	}
}

func lensStub(t *testing.T, now time.Time, world lensWorld) {
	t.Helper()
	n, g, f, p, r := lensNow, lensGrants, lensFirstSeen, lensProjects, lensRead
	t.Cleanup(func() { lensNow, lensGrants, lensFirstSeen, lensProjects, lensRead = n, g, f, p, r })
	lensNow = func() time.Time { return now }
	lensGrants = func(context.Context, time.Time) ([]db.ExpiringGrant, error) {
		return []db.ExpiringGrant{{UserID: "u1", ProjectID: "p", RoleKey: "r", ExpiresAt: now.Add(time.Hour)}}, nil
	}
	lensFirstSeen = func(context.Context) (map[string]time.Time, error) {
		return map[string]time.Time{"u1": time.Date(2026, 8, 2, 0, 0, 0, 0, time.UTC)}, nil
	}
	lensProjects = func(context.Context) ([]models.ProjectCatalog, error) {
		return []models.ProjectCatalog{{ID: "p", Name: "Laser", Roles: []models.ProjectRole{{Key: "r", Label: "Operator"}}}}, nil
	}
	lensRead = func(context.Context, bool) (lensWorld, error) { return world, nil }
}

func TestInsightsLensEnding_NothingObservedIsNotAGuess(t *testing.T) {
	now := time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC)
	lensStub(t, now, lensWorld{Observed: false, People: []lensPerson{{ID: "u1"}}})
	got, err := InsightsLensEnding(context.Background(), 1000)
	if err != nil || got.HoldersObserved || got.Roles == nil || len(got.Roles) != 0 || got.WindowDays != 365 {
		t.Errorf("got %+v, %v", got, err)
	}
}

func TestInsightsLensGroup_ObservedHoldersAndNames(t *testing.T) {
	now := time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC) // Monsoon term starts 1 Aug
	world := lensWorld{Observed: true, People: []lensPerson{
		{ID: "u1", Status: "active", Observed: map[string]bool{"p:r": true, "q:x": true}},
		{ID: "u2", Status: "inactive", Observed: map[string]bool{"p:r": true}},
		{ID: "u3", Status: "locked"},
	}}
	lensStub(t, now, world)

	left, err := InsightsLensGroup(context.Background(), LensGroup{Kind: LensLeft})
	if err != nil || left.Size != 2 || left.Group != "left" || len(left.Roles) != 1 {
		t.Fatalf("left = %+v, %v", left, err)
	}
	if r := left.Roles[0]; r.People != 1 || r.ProjectName != "Laser" || r.RoleLabel != "Operator" {
		t.Errorf("role = %+v", r)
	}

	term, err := InsightsLensGroup(context.Background(), LensGroup{Kind: LensNewTerm})
	if err != nil || term.Size != 1 || len(term.Roles) != 2 {
		t.Fatalf("new_term = %+v, %v", term, err)
	}
	// q:x is not in the directory: it is named by its key, not dropped.
	if term.Roles[0].ProjectName != "Laser" || term.Roles[1].ProjectName != "q" || term.Roles[1].RoleLabel != "x" {
		t.Errorf("roles = %+v", term.Roles)
	}
}
