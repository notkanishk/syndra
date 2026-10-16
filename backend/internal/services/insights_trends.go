package services

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"syndra/internal/db"
	"syndra/internal/directory"
	"syndra/internal/models"
)

// Insights trends (I4): how access has been moving over a period. Read-only.

// TrendPeriod is the window asked about.
type TrendPeriod string

const (
	Trend30Days TrendPeriod = "30d"
	TrendTerm   TrendPeriod = "term"
	TrendYear   TrendPeriod = "year"
)

// ErrInvalidTrendPeriod is any ?period= other than 30d, term or year.
var ErrInvalidTrendPeriod = errors.New("period must be 30d, term or year")

// ParseTrendPeriod defaults to the current term when none is given.
func ParseTrendPeriod(s string) (TrendPeriod, error) {
	switch p := TrendPeriod(s); p {
	case "":
		return TrendTerm, nil
	case Trend30Days, TrendTerm, TrendYear:
		return p, nil
	}
	return "", fmt.Errorf("%w: %q", ErrInvalidTrendPeriod, s)
}

// academicTerms is the year cut into terms, first day to last, inclusive and
// gapless (a date is in exactly one). Month and day are the server's local date.
//
// ponytail: academic terms assumed; owner to confirm.
var academicTerms = []struct {
	name                 string
	startMonth, startDay int
	endMonth, endDay     int
}{
	{"Spring", 1, 1, 5, 31},
	{"Summer", 6, 1, 7, 31},
	{"Monsoon", 8, 1, 12, 31},
}

// trendWindow is a period resolved to instants: From is local midnight of the
// first day, To local midnight of the last day (both inclusive days).
type trendWindow struct {
	From, To time.Time
	Label    string
}

const dateLayout = "2006-01-02"

// windowFor resolves a period at `now`, in now's own location.
func windowFor(p TrendPeriod, now time.Time) trendWindow {
	loc := now.Location()
	day := func(y int, m time.Month, d int) time.Time { return time.Date(y, m, d, 0, 0, 0, 0, loc) }
	y, m, d := now.Date()
	today := day(y, m, d)
	switch p {
	case Trend30Days:
		return trendWindow{From: today.AddDate(0, 0, -29), To: today, Label: "Last 30 days"}
	case TrendYear:
		return trendWindow{From: today.AddDate(0, 0, -364), To: today, Label: "Last 12 months"}
	}
	md := int(m)*100 + d
	for _, t := range academicTerms {
		if md >= t.startMonth*100+t.startDay && md <= t.endMonth*100+t.endDay {
			return trendWindow{
				From:  day(y, time.Month(t.startMonth), t.startDay),
				To:    day(y, time.Month(t.endMonth), t.endDay),
				Label: fmt.Sprintf("%s %d", t.name, y),
			}
		}
	}
	return trendWindow{From: today, To: today, Label: "Today"} // unreachable: terms cover the year
}

// Audit actions, grouped by what a change MEANS for a person. Every name is one
// the code writes (db/cascade.go, db/grants.go, db/propagation_enqueue.go,
// db/bundle_versions.go, handlers/access.go).
//
//	through_bundle  bundle.assigned
//	by_request      access_request.approved, access_request.self_approved
//	by_hand         direct_grant.upserted, direct_grant.replaced
//	taken_away      direct_grant.removed, direct_grant.revoked,
//	                direct_grant.revoked_by_expiry, bundle.unassigned
//
// Left out on purpose: bundle.holder_moved (repinning a holder to another version
// of a bundle they already hold — the row cannot say whether the new version gave
// anything). direct_grant.* rows an approved request itself made are dropped in
// SQL (db.AuditDayCounts) so a request is by_request only, not also by_hand.
var (
	actionsThroughBundle = []string{"bundle.assigned"}
	actionsByRequest     = []string{"access_request.approved", "access_request.self_approved"}
	actionsByHand        = []string{"direct_grant.upserted", "direct_grant.replaced"}
	actionsTakenAway     = []string{"direct_grant.removed", "direct_grant.revoked", "direct_grant.revoked_by_expiry", "bundle.unassigned"}
)

// leftStatuses are the directory states that mean a person is no longer a
// member: the ones the entitlement path already treats as "not a member"
// (applyDirectoryStanding), plus "departed", the word the drift queue uses.
var leftStatuses = map[string]bool{"inactive": true, "locked": true, "deleted": true, "departed": true}

const (
	trendTopN      = 5
	noReasonGiven  = "No reason given"
	trendDayLayout = dateLayout
)

// The reads, as seams: the whole content of this report is what it does with
// them, and a test should not need a database to move any one.
var (
	trendNow         = time.Now
	trendFindings    = InsightsFindings
	trendAuditDays   = db.AuditDayCounts
	trendRequests    = db.RequestsInWindow
	trendOutside     = db.OutsideBundleRoles
	trendSendSeconds = db.SendSeconds
	trendDrift       = db.DriftInWindow
	trendEndingSoon  = db.GrantsEndingSoon
	trendDriftOpen   = db.CountPendingDrift
	trendRoleHolders = func(ctx context.Context, userID string) (int, error) {
		roles, _, err := collectUserRolesHook(ctx, userID)
		return len(roles), err
	}
)

// InsightsTrends reads each aggregate once. The directory (Zitadel over HTTP)
// and the findings are read before the database snapshot opens, so a slow answer
// doesn't hold a transaction.
func InsightsTrends(ctx context.Context, period TrendPeriod) (models.InsightsTrends, error) {
	now := trendNow()
	w := windowFor(period, now)
	from, to := w.From, w.To.AddDate(0, 0, 1) // [from, to): To is an inclusive day

	findings, err := trendFindings(ctx)
	if err != nil {
		return models.InsightsTrends{}, err
	}
	users, err := directory.Default.Users(ctx)
	if err != nil {
		return models.InsightsTrends{}, err
	}
	projects, err := directory.Default.Projects(ctx)
	if err != nil {
		return models.InsightsTrends{}, err
	}
	// ponytail: asks per person who has left — collectUserRolesHook, the same
	// read the findings' snapshot makes — not per member. A handful at this size.
	left := 0
	for _, u := range users {
		if !leftStatuses[strings.ToLower(u.Status)] {
			continue
		}
		n, err := trendRoleHolders(ctx, u.ID)
		if err != nil {
			return models.InsightsTrends{}, err
		}
		if n > 0 {
			left++
		}
	}

	out := models.InsightsTrends{
		Window: models.TrendWindow{From: w.From.Format(dateLayout), To: w.To.Format(dateLayout), Label: w.Label},
		Upkeep: models.TrendUpkeep{
			PeopleWhoLeftHolding: left,
			RolesNobodyHas:       len(findings.Unused.Roles),
			BundlesNothingNew:    len(findings.Unused.Bundles),
			Overlaps:             len(findings.Overlaps),
		},
	}

	var (
		days     []db.TrendDay
		reqs     db.RequestTrendRows
		outside  []models.TrendOutsideRole
		sends    []float64
		drift    db.DriftTrendRows
		ending   []db.EndingSoonRow
		driftOpn int
	)
	all := append(append(append(append([]string{}, actionsThroughBundle...), actionsByRequest...), actionsByHand...), actionsTakenAway...)
	_, offset := now.Zone()
	err = svcInReadSnapshot(ctx, func(ctx context.Context) error {
		var err error
		if days, err = trendAuditDays(ctx, from, to, all, offset); err != nil {
			return err
		}
		if reqs, err = trendRequests(ctx, from, to); err != nil {
			return err
		}
		if outside, err = trendOutside(ctx, from, to, trendTopN); err != nil {
			return err
		}
		if sends, err = trendSendSeconds(ctx, from, to); err != nil {
			return err
		}
		if drift, err = trendDrift(ctx, from, to); err != nil {
			return err
		}
		if ending, err = trendEndingSoon(ctx, now); err != nil {
			return err
		}
		driftOpn, err = trendDriftOpen(ctx)
		return err
	})
	if err != nil {
		return models.InsightsTrends{}, err
	}

	out.Weekly, out.Totals = bucketWeeks(days, w.From, minDate(w.To, now))
	out.Requests = models.TrendRequests{
		CameIn: reqs.CameIn, Approved: reqs.Approved, Declined: reqs.Declined, Withdrawn: reqs.Withdrawn,
		DeclinedReasons:     groupReasons(reqs.DeclineNotes, trendTopN),
		MedianHoursToDecide: medianOver(reqs.DecideSeconds, 3600),
		OutsideBundles:      outside,
	}
	out.Sending = models.TrendSending{
		MedianMinutesApprovalToSent: medianOver(sends, 60),
		Sample:                      len(sends),
	}
	out.Drift = models.TrendDrift{
		Found: drift.Found, Cleared: drift.Cleared,
		MedianHoursToClear: medianOver(drift.ClearSeconds, 3600),
	}
	out.Upkeep.DriftOpen = driftOpn

	names := make(map[string]string, len(projects))
	for _, p := range projects {
		names[p.ID] = p.Name
	}
	out.EndingSoon = make([]models.TrendEndingSoon, 0, len(ending))
	for _, e := range ending {
		name := names[e.ProjectID]
		if name == "" {
			name = e.ProjectID // a project the directory no longer lists is still a project
		}
		out.EndingSoon = append(out.EndingSoon, models.TrendEndingSoon{
			ProjectID: e.ProjectID, ProjectName: name, D30: e.D30, D60: e.D60, D90: e.D90,
		})
	}
	return out, nil
}

// minDate is the earlier of the window's last day and today: the weekly series
// stops at today rather than running on through a term that has not happened.
func minDate(last, now time.Time) time.Time {
	y, m, d := now.Date()
	today := time.Date(y, m, d, 0, 0, 0, 0, now.Location())
	if last.After(today) {
		return today
	}
	return last
}

// bucketWeeks sums per-day counts into Monday-started weeks from `from` through
// `through` (inclusive days), one row per week even when nothing happened, so a
// chart has no gaps. Dates are civil: only year/month/day of each time count.
func bucketWeeks(days []db.TrendDay, from, through time.Time) ([]models.TrendWeek, models.TrendTotals) {
	civil := func(t time.Time) time.Time {
		y, m, d := t.Date()
		return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
	}
	monday := func(t time.Time) time.Time {
		t = civil(t)
		return t.AddDate(0, 0, -((int(t.Weekday()) + 6) % 7))
	}
	kind := map[string]int{}
	for _, a := range actionsThroughBundle {
		kind[a] = 0
	}
	for _, a := range actionsByRequest {
		kind[a] = 1
	}
	for _, a := range actionsByHand {
		kind[a] = 2
	}
	for _, a := range actionsTakenAway {
		kind[a] = 3
	}

	weeks := []models.TrendWeek{}
	index := map[string]int{}
	for w := monday(from); !w.After(civil(through)); w = w.AddDate(0, 0, 7) {
		index[w.Format(dateLayout)] = len(weeks)
		weeks = append(weeks, models.TrendWeek{WeekStart: w.Format(dateLayout)})
	}
	var totals models.TrendTotals
	for _, d := range days {
		i, ok := index[monday(d.Day).Format(dateLayout)]
		k, known := kind[d.Action]
		if !ok || !known {
			continue
		}
		switch k {
		case 0:
			weeks[i].ThroughBundle += d.N
			totals.Given += d.N
		case 1:
			weeks[i].ByRequest += d.N
			totals.Given += d.N
		case 2:
			weeks[i].ByHand += d.N
			totals.Given += d.N
		default:
			weeks[i].TakenAway += d.N
			totals.TakenAway += d.N
		}
	}
	return weeks, totals
}

// groupReasons counts decline notes by trimmed text, case-insensitively, empty
// as "No reason given", and keeps the top n (most first, then A to Z). A group
// shows the wording of its first note.
func groupReasons(notes []string, n int) []models.TrendReason {
	type group struct {
		shown string
		n     int
	}
	groups := map[string]*group{}
	var order []string
	for _, note := range notes {
		note = strings.TrimSpace(note)
		if note == "" {
			note = noReasonGiven
		}
		k := strings.ToLower(note)
		g := groups[k]
		if g == nil {
			g = &group{shown: note}
			groups[k] = g
			order = append(order, k)
		}
		g.n++
	}
	out := make([]models.TrendReason, 0, len(order))
	for _, k := range order {
		out = append(out, models.TrendReason{Reason: groups[k].shown, N: groups[k].n})
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].N != out[j].N {
			return out[i].N > out[j].N
		}
		return strings.ToLower(out[i].Reason) < strings.ToLower(out[j].Reason)
	})
	if len(out) > n {
		out = out[:n]
	}
	return out
}

// median is the middle of xs (the mean of the two middle ones when even);
// ok is false for none. xs is not modified.
func median(xs []float64) (float64, bool) {
	if len(xs) == 0 {
		return 0, false
	}
	s := append([]float64(nil), xs...)
	sort.Float64s(s)
	if len(s)%2 == 1 {
		return s[len(s)/2], true
	}
	return (s[len(s)/2-1] + s[len(s)/2]) / 2, true
}

// medianOver is the median of seconds, expressed in units of `per` seconds and
// rounded to one decimal; nil when there is nothing to take a median of.
func medianOver(seconds []float64, per float64) *float64 {
	m, ok := median(seconds)
	if !ok {
		return nil
	}
	v := math.Round(m/per*10) / 10
	return &v
}
