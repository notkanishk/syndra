package services

import (
	"context"
	"errors"
	"testing"
	"time"

	"syndra/internal/clock"
	"syndra/internal/db"
	"syndra/internal/models"
)

// Access over time (A-24 §9): the runs are Syndra's records, holding is the
// standing, and a start no record gives is nil, never a date.

func historyDay(t *testing.T, ymd string) *time.Time {
	t.Helper()
	d, err := clock.Midnight(ymd)
	if err != nil {
		t.Fatal(err)
	}
	return &d
}

// thisYear is a since every test can use: 1 Jan of the makerspace's year.
func thisYear() string { return clock.Day(clock.Now())[:4] + "-01-01" }

// The year's dates the tests use, always before today: early in the year.
func early(t *testing.T, mmdd string) *time.Time {
	return historyDay(t, clock.Day(clock.Now())[:4]+"-"+mmdd)
}

func spansFrom(t *testing.T, spans ...db.AccessSpan) *time.Time {
	t.Helper()
	var asked time.Time
	orig := svcAccessHistorySpans
	t.Cleanup(func() { svcAccessHistorySpans = orig })
	svcAccessHistorySpans = func(_ context.Context, _ string, since time.Time, _ int) ([]db.AccessSpan, error) {
		asked = since
		return spans, nil
	}
	return &asked
}

func readHistory(t *testing.T) models.AccessHistory {
	t.Helper()
	h, err := AccessHistory(context.Background(), "dev_admin", thisYear())
	if err != nil {
		t.Fatal(err)
	}
	return h
}

func roleIn(t *testing.T, h models.AccessHistory, project, role string) models.RoleHistory {
	t.Helper()
	for _, r := range h.Roles {
		if r.ProjectID == project && r.RoleKey == role {
			return r
		}
	}
	t.Fatalf("%s/%s not in the history: %+v", project, role, h.Roles)
	return models.RoleHistory{}
}

func sameTime(a, b *time.Time) bool {
	return a == nil && b == nil || a != nil && b != nil && a.Equal(*b)
}

func assertRuns(t *testing.T, got []models.RoleRun, want ...models.RoleRun) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("runs %+v, want %+v", got, want)
	}
	for i := range want {
		if !sameTime(got[i].From, want[i].From) || !sameTime(got[i].To, want[i].To) {
			t.Fatalf("run %d is %v to %v, want %v to %v", i, got[i].From, got[i].To, want[i].From, want[i].To)
		}
	}
}

func completeRead(t *testing.T, held ...db.ObservedGrant) {
	t.Helper()
	observedStore(t, db.Observation{ObservedAt: time.Now(), Complete: true}, nil, held)
}

// A role given and taken away this year is one grey run between the two
// records, and Zitadel no longer showing it is its standing. Given again, it
// is a second run on the same line, open today, with its end ahead.
func TestAccessHistory_AGrantThenARevoke(t *testing.T) {
	ends := early(t, "12-31")
	noRecordsFor(t, []models.DirectGrant{{ProjectID: "p1", RoleKey: "lathe", CreatedAt: *early(t, "09-01"), ExpiresAt: ends}})
	completeRead(t, db.ObservedGrant{ProjectID: "p1", RoleKeys: []string{"lathe"}})
	asked := spansFrom(t,
		db.AccessSpan{ProjectID: "p1", RoleKey: "laser", From: early(t, "03-03"), To: early(t, "08-14")},
		db.AccessSpan{ProjectID: "p1", RoleKey: "lathe", From: early(t, "02-02"), To: early(t, "05-31")})

	h := readHistory(t)
	if want := historyDay(t, thisYear()); !asked.Equal(*want) {
		t.Fatalf("records read from %v, want the makerspace's midnight %v", asked, want)
	}
	laser := roleIn(t, h, "p1", "laser")
	assertRuns(t, laser.Runs, models.RoleRun{From: early(t, "03-03"), To: early(t, "08-14")})
	assertStanding(t, laser.Standing, models.StandingNotHeld)

	lathe := roleIn(t, h, "p1", "lathe")
	assertRuns(t, lathe.Runs,
		models.RoleRun{From: early(t, "02-02"), To: early(t, "05-31")},
		models.RoleRun{From: early(t, "09-01")})
	assertStanding(t, lathe.Standing, models.StandingHeld)
	if !sameTime(lathe.Ends, ends) {
		t.Fatalf("ends %v, want the grant's %v", lathe.Ends, ends)
	}
}

// Zitadel shows a role and no record gives it: it is on the card, held, with
// no start, never the window's first day passed off as one.
func TestAccessHistory_HeldTodayWithNoRecord(t *testing.T) {
	noRecordsFor(t, nil)
	completeRead(t, db.ObservedGrant{ProjectID: "p1", RoleKeys: []string{"kiln"}})
	spansFrom(t)

	kiln := roleIn(t, readHistory(t), "p1", "kiln")
	assertRuns(t, kiln.Runs, models.RoleRun{})
	assertStanding(t, kiln.Standing, models.StandingHeld)
}

// A bundle's role runs from the assignment, held today, and nothing undated is
// added to it: the assignment is its record.
func TestAccessHistory_ABundleGivenRole(t *testing.T) {
	noRecordsFor(t, nil)
	svcGetBundlesForUser = func(context.Context, string) ([]models.Bundle, error) {
		return []models.Bundle{{ID: "b1", Name: "Welcome", PinnedVersion: 1}}, nil
	}
	svcGetUserBundleRolesGrouped = func(context.Context, string) (map[string][]models.BundleRole, error) {
		return map[string][]models.BundleRole{"b1": {{BundleID: "b1", ProjectID: "p3d", RoleKey: "member"}}}, nil
	}
	completeRead(t, db.ObservedGrant{ProjectID: "p3d", RoleKeys: []string{"member"}})
	spansFrom(t, db.AccessSpan{ProjectID: "p3d", RoleKey: "member", From: early(t, "01-12")})

	member := roleIn(t, readHistory(t), "p3d", "member")
	assertRuns(t, member.Runs, models.RoleRun{From: early(t, "01-12")})
	assertStanding(t, member.Standing, models.StandingHeld)
	if member.Ends != nil {
		t.Fatalf("a bundle's role never ends, got %v", member.Ends)
	}
}

// A grant that expired ends its run on the day the sweep recorded; a grant
// given by hand has no start on record, and the run says so.
func TestAccessHistory_AnExpiry(t *testing.T) {
	noRecordsFor(t, nil)
	completeRead(t)
	spansFrom(t,
		db.AccessSpan{ProjectID: "p1", RoleKey: "trained", From: early(t, "02-02"), To: early(t, "06-01")},
		db.AccessSpan{ProjectID: "p1", RoleKey: "welder", To: early(t, "04-04")})

	h := readHistory(t)
	assertRuns(t, roleIn(t, h, "p1", "trained").Runs, models.RoleRun{From: early(t, "02-02"), To: early(t, "06-01")})
	welder := roleIn(t, h, "p1", "welder")
	assertRuns(t, welder.Runs, models.RoleRun{To: early(t, "04-04")})
	assertStanding(t, welder.Standing, models.StandingNotHeld)
}

// A rule's role has no record of when it began, even beside a dated grant:
// the rule may have given it first.
func TestAccessHistory_ARulesRoleHasNoKnownStart(t *testing.T) {
	noRecordsFor(t, []models.DirectGrant{
		{ProjectID: "p1", RoleKey: "trained", CreatedAt: *early(t, "03-03")},
		{ProjectID: "p1", RoleKey: "welder", CreatedAt: *early(t, "04-04")},
	})
	svcGetActiveMappingRules = func(context.Context) ([]models.MappingRule, error) {
		return []models.MappingRule{
			{ID: "r1", SourceProject: "p1", SourceRole: "trained", TargetProject: "p2", TargetRole: "member"},
			{ID: "r2", SourceProject: "p1", SourceRole: "trained", TargetProject: "p1", TargetRole: "welder"},
		}, nil
	}
	completeRead(t)
	spansFrom(t)

	h := readHistory(t)
	assertRuns(t, roleIn(t, h, "p1", "trained").Runs, models.RoleRun{From: early(t, "03-03")})
	assertRuns(t, roleIn(t, h, "p2", "member").Runs, models.RoleRun{})
	// Given by hand on 4 Apr, and by a rule since a day no record says.
	assertRuns(t, roleIn(t, h, "p1", "welder").Runs, models.RoleRun{})
}

// Spans that overlap or touch are one run; a gap between them is two.
func TestFoldRuns_JoinsWhatTouches(t *testing.T) {
	k := roleKey{projectID: "p", roleKey: "r"}
	at := func(m time.Month) *time.Time { d := time.Date(2026, m, 1, 0, 0, 0, 0, time.UTC); return &d }
	runs := foldRuns([]db.AccessSpan{
		{ProjectID: "p", RoleKey: "r", From: at(5), To: at(7)},
		{ProjectID: "p", RoleKey: "r", From: at(2), To: at(3)},
		{ProjectID: "p", RoleKey: "r", From: at(3), To: at(4)},
		{ProjectID: "p", RoleKey: "r", From: at(6), To: at(9)},
	}, nil, nil)[k]
	assertRuns(t, runs, models.RoleRun{From: at(2), To: at(4)}, models.RoleRun{From: at(5), To: at(9)})

	// A span with no start never bridges a gap (PR 217 review): a grant Feb to
	// Apr, and one given by hand that expired in Jul, are two runs, May and Jun
	// never drawn as held. One that ends inside a run adds nothing to it, and
	// one before an open run joins it.
	runs = foldRuns([]db.AccessSpan{
		{ProjectID: "p", RoleKey: "r", To: at(7)},
		{ProjectID: "p", RoleKey: "r", From: at(2), To: at(4)},
		{ProjectID: "p", RoleKey: "r", To: at(3)},
	}, nil, nil)[k]
	assertRuns(t, runs, models.RoleRun{From: at(2), To: at(4)}, models.RoleRun{To: at(7)})
	runs = foldRuns([]db.AccessSpan{
		{ProjectID: "p", RoleKey: "r", From: at(8)},
		{ProjectID: "p", RoleKey: "r"},
		{ProjectID: "p", RoleKey: "r", To: at(5)},
	}, nil, nil)[k]
	assertRuns(t, runs, models.RoleRun{})
}

// More spans than the cap: the oldest are left out and the answer says so.
func TestAccessHistory_TheCapIsSaid(t *testing.T) {
	noRecordsFor(t, nil)
	completeRead(t)
	many := make([]db.AccessSpan, AccessHistoryCap+1)
	for i := range many {
		many[i] = db.AccessSpan{ProjectID: "p", RoleKey: "r", To: early(t, "01-02")}
	}
	var limit int
	orig := svcAccessHistorySpans
	t.Cleanup(func() { svcAccessHistorySpans = orig })
	svcAccessHistorySpans = func(_ context.Context, _ string, _ time.Time, n int) ([]db.AccessSpan, error) {
		limit = n
		return many[:min(n, len(many))], nil
	}
	if h := readHistory(t); !h.Truncated || limit != AccessHistoryCap+1 {
		t.Fatalf("truncated %v asked for %d, want true and %d", h.Truncated, limit, AccessHistoryCap+1)
	}
	svcAccessHistorySpans = func(context.Context, string, time.Time, int) ([]db.AccessSpan, error) {
		return many[:AccessHistoryCap], nil
	}
	if readHistory(t).Truncated {
		t.Fatal("exactly the cap is everything, not cut short")
	}
}

// since is a makerspace day from a year ago to today, nothing else.
func TestAccessHistory_SinceIsBounded(t *testing.T) {
	today := clock.Day(clock.Now())
	for _, ok := range []string{today, clock.AddDate(today, -1, 0, 0), thisYear()} {
		if _, err := historyStart(ok, clock.Now()); err != nil {
			t.Errorf("since %q refused: %v", ok, err)
		}
	}
	for _, bad := range []string{"", "2026-1-1", "2026-02-30", "yesterday", clock.AddDate(today, 0, 0, 1), clock.AddDate(today, -1, 0, -1)} {
		if _, err := historyStart(bad, clock.Now()); !errors.Is(err, ErrInvalidHistorySince) {
			t.Errorf("since %q: %v, want ErrInvalidHistorySince", bad, err)
		}
	}
}
