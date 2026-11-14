// Package clock is the deployment's time zone (the TZ setting): the one zone
// every calendar day, and every clock window, in Syndra is counted in.
//
// Instants are stored as timestamptz and never move. Only a DAY (or a time of
// day) depends on a zone, and it is always this one:
//
//   - Go: Day(t), or t.In(Loc) before reading a date or a clock off it.
//   - SQL: (ts AT TIME ZONE $n)::date with Name passed in. Never the session's
//     TimeZone (the cluster's is UTC), never 'UTC'. A migration that
//     must name a zone is a one-time re-bucket of this deployment's history
//     (000079) and says so; a new one reads the zone from the caller.
//
// repoguard's TestDaysAreCountedInTheMakerspaceZone holds every caller to it.
package clock

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"time"
	_ "time/tzdata" // the zone must load in a minimal image with no zoneinfo
)

// Name is the zone's IANA name, the form Postgres's AT TIME ZONE takes; Loc is
// the zone itself. Both come from the deployment's TZ, once, at start, and from
// nothing else: no zone is written in code. TZ unset or empty is UTC, as in
// POSIX and Go (time.LoadLocation("") is UTC). A TZ that names no zone refuses
// to start; it is never replaced by a guess.
var Name, Loc = mustLoad(os.Getenv("TZ"))

func mustLoad(tz string) (string, *time.Location) {
	name, loc, err := load(tz)
	if err != nil {
		log.Fatalf("[STARTUP] %v", err)
	}
	return name, loc
}

func load(tz string) (string, *time.Location, error) {
	tz = strings.TrimPrefix(strings.TrimSpace(tz), ":") // POSIX allows TZ=":Area/Location"
	if tz == "" {
		return "UTC", time.UTC, nil
	}
	// "Local" would make the zone whatever the host's is, which is what a
	// stated zone exists to avoid.
	loc, err := time.LoadLocation(tz)
	if err != nil || tz == "Local" {
		return "", nil, fmt.Errorf("TZ=%q is not an IANA time zone name (for example Europe/London, or UTC); refusing to start rather than guess a zone", tz)
	}
	return tz, loc, nil
}

// Now is the current time, read in the makerspace's zone.
func Now() time.Time { return time.Now().In(Loc) }

// Day is t's calendar day in the makerspace's zone, YYYY-MM-DD.
func Day(t time.Time) string { return t.In(Loc).Format(time.DateOnly) }

// Midnight is the instant `day` (YYYY-MM-DD) starts in the makerspace's zone:
// its first instant, 00:00, or, on a day whose 00:00 a daylight-saving change
// skips (America/Santiago, 6 Sep 2026), the instant the clocks resume. Go
// resolves a skipped 00:00 to the previous day's 23:00, so that one case takes
// the end of the zone period it fell in (the add-ons' and the kit's rule too).
func Midnight(day string) (time.Time, error) {
	t, err := time.ParseInLocation(time.DateOnly, day, Loc)
	if err == nil && t.Format(time.DateOnly) != day {
		_, t = t.ZoneBounds()
	}
	return t, err
}

// AddDate is the calendar day years, months and days after `day` (YYYY-MM-DD),
// normalised as time.AddDate does; a day is a date here, never 24 hours. A
// `day` that is not YYYY-MM-DD is returned as it came.
func AddDate(day string, years, months, days int) string {
	d, err := time.Parse(time.DateOnly, day)
	if err != nil {
		return day
	}
	return d.AddDate(years, months, days).Format(time.DateOnly)
}

// Every end date means one thing (owner, 5 Nov 2026): access "until 30 Nov"
// lasts through the whole of 30 Nov in the makerspace's zone, and "for 7 days"
// is seven calendar days, ending as the 7th one does. An end is stored as the
// last second of its day, so it still reads as that day wherever it is shown.
// ui/src/lib/time.ts (endOfDayISO, endAfterDaysISO) is the same rule.

// EndOfDay is the last second of `day` (YYYY-MM-DD) in the makerspace's zone.
func EndOfDay(day string) (time.Time, error) {
	if _, err := time.Parse(time.DateOnly, day); err != nil {
		return time.Time{}, err
	}
	next, err := Midnight(AddDate(day, 0, 0, 1))
	if err != nil {
		return time.Time{}, err
	}
	return next.Add(-time.Second), nil
}

// EndDay is an end date on the wire: what the person means, an ISO 8601 date
// ("2026-11-30"), never an instant the client worked out in a zone of its own.
// Decoding resolves it, here and only here, to EndOfDay in Loc; the column it
// lands in stays timestamptz. Anything but a plain date is refused (strict, one
// shape): "2026-11-30T00:00:00Z" is a client's guess at the day's end.
type EndDay struct{ time.Time }

func (d *EndDay) UnmarshalJSON(b []byte) error {
	var day string
	if err := json.Unmarshal(b, &day); err != nil {
		return fmt.Errorf("an end date is a string YYYY-MM-DD")
	}
	if p, err := time.Parse(time.DateOnly, day); err != nil || p.Format(time.DateOnly) != day {
		return fmt.Errorf("end date %q is not a date YYYY-MM-DD", day)
	}
	t, err := EndOfDay(day)
	d.Time = t
	return err
}

// Instant is the end as the instant it resolved to; nil-safe, for an optional field.
func (d *EndDay) Instant() *time.Time {
	if d == nil {
		return nil
	}
	t := d.Time
	return &t
}

// EndAfterDays is the last second of the n-th day after t's own (both in the
// makerspace's zone): 7 days from any time on 5 Nov ends at 23:59:59 on 12 Nov.
func EndAfterDays(t time.Time, n int) time.Time {
	end, _ := EndOfDay(AddDate(Day(t), 0, 0, n)) // a Day is always a valid day; EndOfDay holds when midnight is skipped (DST)
	return end
}

// EndAfterMonths is the last second of the day n calendar months after t's own.
// A day that does not exist in the target month is clamped to that month's last
// day, as Postgres (+ interval 'N months') and most date libraries do: 31 Aug +
// 6 months is 28 Feb (29 in a leap year), not Go's AddDate overflow into March.
func EndAfterMonths(t time.Time, n int) time.Time {
	y, m, d := t.In(Loc).Date()
	first := time.Date(y, m+time.Month(n), 1, 0, 0, 0, 0, time.UTC) // normalises the year
	last := first.AddDate(0, 1, -1).Day()
	end, _ := EndOfDay(time.Date(first.Year(), first.Month(), min(d, last), 0, 0, 0, 0, time.UTC).Format(time.DateOnly))
	return end
}
