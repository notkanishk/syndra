package services

import (
	"context"
	"errors"
	"fmt"
	"time"

	"syndra/internal/db"
	"syndra/internal/models"
)

// Insights "As of a date" (I5). The audit log cannot rebuild the whole access
// graph for a past date, so Syndra keeps one snapshot per local day from the day
// it was switched on. A date before the first snapshot says so; nothing is guessed.

// MaxTimelineDays is the longest range the timeline answers, both ends counted.
const MaxTimelineDays = 400

var (
	// ErrInvalidAsOfDate is a date that is not YYYY-MM-DD, or is in the future.
	ErrInvalidAsOfDate = errors.New("date must be a real date, YYYY-MM-DD, no later than today")
	// ErrInvalidTimelineRange is a malformed, reversed or over-long from/to.
	ErrInvalidTimelineRange = fmt.Errorf("from and to must be real dates, YYYY-MM-DD, from no later than to, at most %d days apart", MaxTimelineDays)
)

// NoSnapshotError says there is nothing recorded on or before the date asked
// about. FirstDay is the earliest day that is recorded, nil when none is.
type NoSnapshotError struct{ FirstDay *string }

func (e *NoSnapshotError) Error() string { return "nothing was recorded on or before that date" }

// parseDay accepts only a real date written exactly YYYY-MM-DD.
func parseDay(s string) (time.Time, error) {
	t, err := time.Parse(dateLayout, s)
	if err != nil || t.Format(dateLayout) != s {
		return time.Time{}, fmt.Errorf("%q is not a YYYY-MM-DD date", s)
	}
	return t, nil
}

// ParseAsOfDate validates ?date= against the server's local today.
func ParseAsOfDate(s string) (string, error) {
	if _, err := parseDay(s); err != nil || s > trendNow().Format(dateLayout) { // ISO dates sort as text
		return "", ErrInvalidAsOfDate
	}
	return s, nil
}

// ParseTimelineRange validates ?from=&to= (inclusive days).
func ParseTimelineRange(from, to string) (string, string, error) {
	f, err := parseDay(from)
	if err != nil {
		return "", "", ErrInvalidTimelineRange
	}
	t, err := parseDay(to)
	if err != nil || t.Before(f) || t.Sub(f)/(24*time.Hour) >= MaxTimelineDays {
		return "", "", ErrInvalidTimelineRange
	}
	return from, to, nil
}

// The reads, as seams. The clock and the audit count are trends'.
var (
	snapshotHas        = db.InsightsSnapshotExists
	snapshotBuild      = buildInsightsSnapshot
	snapshotInsert     = db.InsertInsightsSnapshot
	snapshotOnOrBefore = db.InsightsSnapshotOnOrBefore
	snapshotBounds     = db.InsightsSnapshotBounds
)

// TakeDailyInsightsSnapshot is the daily runner's job: if today (the server's
// local date) has no row, build one and store it. Cheap when it has, so it can
// run every few hours; an error is returned for the runner to log and retry on
// its next tick.
func TakeDailyInsightsSnapshot(ctx context.Context) error {
	day := trendNow().Format(dateLayout)
	has, err := snapshotHas(ctx, day)
	if err != nil || has {
		return err
	}
	snap, err := snapshotBuild(ctx, day)
	if err != nil {
		return err
	}
	_, err = snapshotInsert(ctx, snap) // a concurrent writer winning is fine: false, nil
	return err
}

// buildInsightsSnapshot reads the topology and the holder counts from ONE access
// snapshot, so the two cannot disagree about who was in the graph.
//
// role_people is what Zitadel shows holding each role (the only reading a screen
// may call people). Before anything has ever been observed that reading is nil,
// and the recorded one (people Syndra gave the role to) is stored instead.
func buildInsightsSnapshot(ctx context.Context, day string) (db.InsightsSnapshot, error) {
	snap, err := newAccessSnapshot(ctx)
	if err != nil {
		return db.InsightsSnapshot{}, err
	}
	topology, err := topologyFromSnapshot(snap)
	if err != nil {
		return db.InsightsSnapshot{}, err
	}
	people, err := snap.ObservedHolderCounts()
	if err == nil && people == nil {
		people, err = snap.HolderCounts()
	}
	if err != nil {
		return db.InsightsSnapshot{}, err
	}
	if people == nil {
		people = map[string]int{}
	}
	return db.InsightsSnapshot{Day: day, Topology: topology, RolePeople: people}, nil
}

// sumChanges counts audit rows that give or take away access in [fromDay, to).
func sumChanges(ctx context.Context, from, to time.Time, offset int) (map[string]int, error) {
	rows, err := trendAuditDays(ctx, from, to, accessChangeActions(), offset)
	if err != nil {
		return nil, err
	}
	perDay := map[string]int{}
	for _, r := range rows {
		perDay[r.Day.Format(dateLayout)] += r.N
	}
	return perDay, nil
}

// InsightsAsOf is the latest snapshot on or before `day` (already validated).
// ChangesSince counts from the local midnight that starts the snapshot's day.
func InsightsAsOf(ctx context.Context, day string) (models.InsightsAsOf, error) {
	snap, err := snapshotOnOrBefore(ctx, day)
	if err != nil {
		return models.InsightsAsOf{}, err
	}
	first, _, err := snapshotBounds(ctx)
	if err != nil {
		return models.InsightsAsOf{}, err
	}
	if snap == nil {
		return models.InsightsAsOf{}, &NoSnapshotError{FirstDay: first}
	}
	now := trendNow()
	from, err := time.ParseInLocation(dateLayout, snap.Day, now.Location())
	if err != nil {
		return models.InsightsAsOf{}, err
	}
	_, offset := now.Zone()
	perDay, err := sumChanges(ctx, from, now.Add(24*time.Hour), offset)
	if err != nil {
		return models.InsightsAsOf{}, err
	}
	total := 0
	for _, n := range perDay {
		total += n
	}
	return models.InsightsAsOf{
		Day: snap.Day, TakenAt: snap.TakenAt, Topology: snap.Topology, RolePeople: snap.RolePeople,
		FirstDay: *first, ChangesSince: total,
	}, nil
}

// InsightsTimeline is one row per day from..to (inclusive, already validated),
// zero where nothing changed, from one grouped read.
func InsightsTimeline(ctx context.Context, from, to string) (models.InsightsTimeline, error) {
	now := trendNow()
	start, err := time.ParseInLocation(dateLayout, from, now.Location())
	if err != nil {
		return models.InsightsTimeline{}, err
	}
	end, err := time.ParseInLocation(dateLayout, to, now.Location())
	if err != nil {
		return models.InsightsTimeline{}, err
	}
	_, offset := now.Zone()
	perDay, err := sumChanges(ctx, start, end.AddDate(0, 0, 1), offset)
	if err != nil {
		return models.InsightsTimeline{}, err
	}
	first, last, err := snapshotBounds(ctx)
	if err != nil {
		return models.InsightsTimeline{}, err
	}
	out := models.InsightsTimeline{Days: []models.TimelineDay{}, SnapshotDays: [2]*string{first, last}}
	for d := start; !d.After(end); d = d.AddDate(0, 0, 1) {
		k := d.Format(dateLayout)
		out.Days = append(out.Days, models.TimelineDay{Day: k, Changes: perDay[k]})
	}
	return out, nil
}
