package clock

import (
	"strings"
	"testing"
	"time"
)

func TestLoadReadsTZAndNothingElse(t *testing.T) {
	for _, tc := range []struct{ tz, want string }{
		{"", "UTC"}, // POSIX and Go: unset is UTC, not a zone chosen for the deployment
		{"  ", "UTC"},
		{"UTC", "UTC"},
		{"Asia/Kolkata", "Asia/Kolkata"},
		{":Asia/Kolkata", "Asia/Kolkata"},
		{"Europe/London", "Europe/London"},
	} {
		name, loc, err := load(tc.tz)
		if err != nil || name != tc.want || loc.String() != tc.want {
			t.Errorf("load(%q) = %s, %v, %v; want %s", tc.tz, name, loc, err, tc.want)
		}
	}
}

// A TZ that names no zone is an error with the value in it, never a silent
// replacement: a day counted in a zone nobody chose is a wrong count.
func TestLoadRefusesAZoneItCannotRead(t *testing.T) {
	for _, tz := range []string{"Not/AZone", "Local", "Asia/Kolkatta"} {
		if _, _, err := load(tz); err == nil || !strings.Contains(err.Error(), tz) {
			t.Errorf("load(%q) err = %v; want a refusal naming it", tz, err)
		}
	}
}

// 23:00 UTC is 04:30 the next morning in India: that sign-in belongs to the
// next day, not the UTC one.
func TestDayIsTheMakerspaceDayNotTheUTCOne(t *testing.T) {
	ist := zone(t, "Asia/Kolkata")
	old := Loc
	Loc = ist
	t.Cleanup(func() { Loc = old })

	if got := Day(time.Date(2026, 11, 4, 23, 0, 0, 0, time.UTC)); got != "2026-11-05" {
		t.Fatalf("Day(23:00 UTC 4 Nov) = %s, want 2026-11-05", got)
	}
	if got := Day(time.Date(2026, 11, 4, 18, 29, 59, 0, time.UTC)); got != "2026-11-04" {
		t.Fatalf("Day(18:29:59 UTC 4 Nov) = %s, want 2026-11-04 (23:59:59 IST)", got)
	}
	m, err := Midnight("2026-11-05")
	if err != nil || !m.Equal(time.Date(2026, 11, 4, 18, 30, 0, 0, time.UTC)) {
		t.Fatalf("Midnight(2026-11-05) = %v, %v; want 18:30 UTC 4 Nov", m, err)
	}
}

// "Until 30 Nov" holds through the whole of 30 Nov in India, not until UTC
// midnight (05:30 IST); "for 7 days" ends as the 7th calendar day does.
func TestEndsAreTheWholeOfTheirDay(t *testing.T) {
	ist := zone(t, "Asia/Kolkata")
	old := Loc
	Loc = ist
	t.Cleanup(func() { Loc = old })

	end, err := EndOfDay("2026-11-30")
	if err != nil || !end.Equal(time.Date(2026, 11, 30, 18, 29, 59, 0, time.UTC)) {
		t.Fatalf("EndOfDay(30 Nov) = %v, %v; want 23:59:59 IST", end, err)
	}
	if Day(end) != "2026-11-30" {
		t.Fatalf("an end must still read as its own day, got %s", Day(end))
	}
	if _, err := EndOfDay("30 Nov"); err == nil {
		t.Fatal("a day that is not YYYY-MM-DD must be refused")
	}
	// 02:00 IST on 6 Nov is still 5 Nov in UTC: the count starts from the 6th.
	if got := EndAfterDays(time.Date(2026, 11, 5, 20, 30, 0, 0, time.UTC), 7); !got.Equal(time.Date(2026, 11, 13, 18, 29, 59, 0, time.UTC)) {
		t.Fatalf("EndAfterDays(02:00 IST 6 Nov, 7) = %v, want 23:59:59 IST 13 Nov", got)
	}
	if got := EndAfterDays(time.Date(2026, 12, 25, 6, 0, 0, 0, time.UTC), 10); Day(got) != "2027-01-04" {
		t.Fatalf("EndAfterDays across the year = %s, want 2027-01-04", Day(got))
	}
}

// A day's midnight is its first instant: where a daylight-saving change skips
// 00:00 (America/Santiago, 6 Sep 2026) it is the instant the clocks resume, not
// the previous day's 23:00, and that day still ends as its own day does. A
// statistics bucket starts there (addon-contract §9.2), as the add-ons' do.
func TestASkippedMidnightIsWhenTheClocksResume(t *testing.T) {
	santiago := zone(t, "America/Santiago")
	old := Loc
	Loc = santiago
	t.Cleanup(func() { Loc = old })

	m, err := Midnight("2026-09-06")
	if err != nil || !m.Equal(time.Date(2026, 9, 6, 4, 0, 0, 0, time.UTC)) || Day(m) != "2026-09-06" {
		t.Fatalf("Midnight(6 Sep, Santiago) = %v, %v; want 01:00 -03:00 (04:00 UTC), on its own day", m, err)
	}
	end, err := EndOfDay("2026-09-05")
	if err != nil || Day(end) != "2026-09-05" || !end.Add(time.Second).Equal(m) {
		t.Fatalf("EndOfDay(5 Sep, Santiago) = %v, %v; want the second before 6 Sep begins", end, err)
	}
	if got := AddDate("2026-12-31", 0, 0, 1); got != "2027-01-01" {
		t.Fatalf("AddDate(31 Dec, +1 day) = %s", got)
	}
	if got := AddDate("2026-11-12", 0, -11, 0); got != "2025-12-12" {
		t.Fatalf("AddDate(12 Nov, -11 months) = %s", got)
	}
}

func zone(t *testing.T, name string) *time.Location {
	t.Helper()
	_, loc, err := load(name)
	if err != nil {
		t.Fatal(err)
	}
	return loc
}
