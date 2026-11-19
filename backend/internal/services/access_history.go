package services

import (
	"context"
	"errors"
	"sort"
	"strings"
	"time"

	"syndra/internal/clock"
	"syndra/internal/db"
	"syndra/internal/directory"
	"syndra/internal/models"
)

// AccessHistoryCap is the most spans one read takes from the records. A
// person with more in a year has the oldest endings cut off, and says so
// (AccessHistory.Truncated).
const AccessHistoryCap = 500

// ErrInvalidHistorySince is a since that is not a makerspace day within the
// last year, today included.
var ErrInvalidHistorySince = errors.New("since must be a day (YYYY-MM-DD) from a year ago to today")

// AccessHistory is userID's roles since the makerspace day `since` (A-24 §9,
// access over time): each role's runs from Syndra's records, and its standing
// from the same snapshot the person page reads, since holding is observed and
// records only explain it.
//
// A run's start is never guessed. Where no record dates it (a rule's role, a
// role Zitadel shows that Syndra never gave, a bundle holder moved between
// versions, a direct grant given by hand and since ended) From is nil, and the
// card draws it from the window's edge as not on record. A role held today
// that no record gives still has its run: what Zitadel shows is on the card.
func AccessHistory(ctx context.Context, userID, since string) (models.AccessHistory, error) {
	start, err := historyStart(since, clock.Now())
	if err != nil {
		return models.AccessHistory{}, err
	}
	spans, err := svcAccessHistorySpans(ctx, userID, start, AccessHistoryCap+1)
	if err != nil {
		return models.AccessHistory{}, err
	}
	truncated := len(spans) > AccessHistoryCap
	if truncated {
		spans = spans[:AccessHistoryCap]
	}
	direct, err := svcGetDirectGrantsForUser(ctx, userID, false)
	if err != nil {
		return models.AccessHistory{}, err
	}
	snap := personSnapshot(ctx)
	live, _, err := snap.For(userID)
	if err != nil {
		return models.AccessHistory{}, err
	}
	observed, err := snap.Observed(userID)
	if err != nil {
		return models.AccessHistory{}, err
	}
	standings, err := snap.Standing(userID)
	if err != nil {
		return models.AccessHistory{}, err
	}
	for _, g := range direct {
		spans = append(spans, db.AccessSpan{ProjectID: g.ProjectID, RoleKey: g.RoleKey, From: &g.CreatedAt})
	}
	runs := foldRuns(spans, live, observed)

	keys := make([]roleKey, 0, len(runs))
	for k := range runs {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].projectID != keys[j].projectID {
			return keys[i].projectID < keys[j].projectID
		}
		return keys[i].roleKey < keys[j].roleKey
	})
	type name struct {
		name     string
		resolved bool
	}
	names := map[string]name{}
	out := models.AccessHistory{Since: since, Truncated: truncated, Roles: make([]models.RoleHistory, 0, len(keys))}
	for _, k := range keys {
		row := models.RoleHistory{ProjectID: k.projectID, RoleKey: k.roleKey,
			Standing: standings.Of(k.projectID, k.roleKey), Runs: runs[k]}
		if r := live[k]; r != nil {
			row.ProjectName, row.ProjectNameResolved, row.Ends = r.ProjectName, r.ProjectNameResolved, r.Ends
		} else {
			n, ok := names[k.projectID]
			if !ok {
				n.name, n.resolved, _ = directory.Default.ProjectName(ctx, k.projectID)
				names[k.projectID] = n
			}
			row.ProjectName, row.ProjectNameResolved = n.name, n.resolved
		}
		out.Roles = append(out.Roles, row)
	}
	return out, nil
}

// historyStart is the instant the makerspace day `since` begins, refused
// unless it is a day from a year before today to today.
func historyStart(since string, now time.Time) (time.Time, error) {
	start, err := clock.Midnight(since)
	today := clock.Day(now)
	if err != nil || since > today || since < clock.AddDate(today, -1, 0, 0) {
		return time.Time{}, ErrInvalidHistorySince
	}
	return start, nil
}

// foldRuns joins each role's spans into runs: spans that overlap or touch are
// one run, since the role was held throughout. Every role the records give
// today has an open run: a rule's role, which no record dates, adds one from
// no known start, and so does a role Zitadel shows with nothing given open.
//
// A span with no start says only when it ended, so it is placed by its end and
// never joins a run across a gap: it joins only a run still open, and one that
// ends inside the run before it adds nothing to it. Joined to an earlier run
// that had ended, it would draw the months between as held, which no record says.
func foldRuns(spans []db.AccessSpan, live map[roleKey]*models.EffectiveRole, observed map[string]bool) map[roleKey][]models.RoleRun {
	by := map[roleKey][]db.AccessSpan{}
	for _, s := range spans {
		k := roleKey{projectID: s.ProjectID, roleKey: s.RoleKey}
		by[k] = append(by[k], s)
	}
	open := func(k roleKey) bool {
		for _, s := range by[k] {
			if s.To == nil {
				return true
			}
		}
		return false
	}
	undated := func(k roleKey) { by[k] = append(by[k], db.AccessSpan{ProjectID: k.projectID, RoleKey: k.roleKey}) }
	for k, r := range live {
		rule := false
		for _, why := range r.Reasons {
			rule = rule || why.Kind == "mapping"
		}
		if rule || !open(k) {
			undated(k)
		}
	}
	for key, seen := range observed {
		p, r, _ := strings.Cut(key, ":")
		if k := (roleKey{projectID: p, roleKey: r}); seen && !open(k) {
			undated(k)
		}
	}

	out := make(map[roleKey][]models.RoleRun, len(by))
	for k, ss := range by {
		// A span is placed by its start, or with none by its end; one with
		// neither (open, undated) first, since it may have begun before anything.
		at := func(s db.AccessSpan) *time.Time {
			if s.From != nil {
				return s.From
			}
			return s.To
		}
		sort.SliceStable(ss, func(i, j int) bool {
			a, b := at(ss[i]), at(ss[j])
			return a == nil && b != nil || a != nil && b != nil && a.Before(*b)
		})
		var runs []models.RoleRun
		for _, s := range ss {
			if n := len(runs); n > 0 {
				last := &runs[n-1]
				switch {
				case last.To == nil:
					continue // still open: it covers everything after it
				case s.From != nil && !s.From.After(*last.To):
					if s.To == nil || s.To.After(*last.To) {
						last.To = s.To
					}
					continue
				case s.From == nil && s.To != nil && !s.To.After(*last.To):
					continue // ended inside the run before it: nothing to add
				}
			}
			runs = append(runs, models.RoleRun{From: s.From, To: s.To})
		}
		out[k] = runs
	}
	return out
}
