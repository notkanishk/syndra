package reference

import "testing"

func TestTheStatsZoneFollowsTZ(t *testing.T) {
	for tz, want := range map[string]string{
		"":                 "Asia/Kolkata",  // the reference's default
		":Europe/London":   "Europe/London", // POSIX leading colon
		"America/Santiago": "America/Santiago",
	} {
		name, loc := zoneFromTZ(tz)
		if name != want || loc.String() != want {
			t.Errorf("TZ=%q: got %q (%v), want %q", tz, name, loc, want)
		}
	}
	defer func() {
		if recover() == nil {
			t.Error("an unreadable TZ must be refused, not silently replaced")
		}
	}()
	zoneFromTZ("Not/AZone")
}
