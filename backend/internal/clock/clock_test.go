package clock

import (
	"testing"
	"time"
)

func TestLoadReadsTZAndFallsBackToTheMakerspace(t *testing.T) {
	for _, tc := range []struct{ tz, want string }{
		{"", Default},
		{"Local", Default},
		{"Not/AZone", Default},
		{"Asia/Kolkata", "Asia/Kolkata"},
		{":Asia/Kolkata", "Asia/Kolkata"},
		{"Europe/London", "Europe/London"},
	} {
		if name, loc := load(tc.tz); name != tc.want || loc.String() != tc.want {
			t.Errorf("load(%q) = %s, %s; want %s", tc.tz, name, loc, tc.want)
		}
	}
}

// 23:00 UTC is 04:30 the next morning in India: that sign-in belongs to the
// next day, not the UTC one.
func TestDayIsTheMakerspaceDayNotTheUTCOne(t *testing.T) {
	_, ist := load("Asia/Kolkata")
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
	_, ist := load("Asia/Kolkata")
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
