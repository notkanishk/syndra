package repoguard

import (
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// Every day in Syndra is the makerspace's day (owner, 5 Nov 2026: "get
// everything in one IST time zone"). An instant never moves; a day depends on
// a zone, and until 5 Nov 2026 some were the UTC day, so sign-ins after 18:30
// UTC (midnight in India) were counted on the wrong day. internal/clock is the
// one place a zone is chosen. This guard refuses the ways a day was, or could
// again be, taken in another one:
//
//   - Go: time.Local or t.Zone() (the process's zone, UTC wherever TZ is
//     unset), a day formatted off .UTC(), a date formatted with no zone named
//     on the line, and an expiry of now + N×24h (an end is the end of a
//     calendar day: clock.EndAfterDays).
//   - SQL, in Go and in migrations from 000079 on: a day taken through the
//     session's zone (::date or date_trunc on a timestamp with no AT TIME ZONE,
//     CURRENT_DATE, a date cast to timestamptz) or through 'UTC' by name.
//
// A date-only value travelling as text ($1::date) and an hourly UTC bucket
// (claim telemetry: 5-minute and hour windows, never days) are fine.
var (
	goBanned = []struct {
		why string
		re  *regexp.Regexp
	}{
		{"the process's zone, not the makerspace's", regexp.MustCompile(`time\.Local\b|\.Zone\(\)`)},
		{"the UTC day of an instant", regexp.MustCompile(`\.UTC\(\)\.Format\((time\.DateOnly|"2006-01-02"|"2 Jan 2006")`)},
		{"an expiry of now + N×24h, not the end of a calendar day", regexp.MustCompile(`(?i)expir\w*\s*:?=.*\.Add\(.*24\s*\*\s*time\.Hour`)},
	}
	dateFormat = regexp.MustCompile(`\.Format\((time\.DateOnly|"2006-01-02"|"2 Jan 2006"|"02 Jan 2006")\)`)
	sqlBanned  = []struct {
		why string
		re  *regexp.Regexp
	}{
		{"a day through 'UTC' by name", regexp.MustCompile(`(?i)AT TIME ZONE\s+'UTC'`)},
		{"today in the session's zone", regexp.MustCompile(`(?i)\bCURRENT_DATE\b|\bLOCALTIMESTAMP\b|now\(\)::date`)},
		{"a calendar bucket in the session's zone", regexp.MustCompile(`(?i)date_trunc\('(day|week|month|year)',\s*[^,)]+\)`)},
		{"a date read as an instant in the session's zone", regexp.MustCompile(`(?i)(\bday|\))::timestamptz`)},
	}
	// ::date that is not a parameter's ($1::date), a literal or a column cast
	// already inside AT TIME ZONE: a timestamp's day in the session's zone.
	sessionDate = regexp.MustCompile(`(?i)[\w)]::date\b`)
	paramDate   = regexp.MustCompile(`\$\d+::date\b`)
)

func TestDaysAreCountedInTheMakerspaceZone(t *testing.T) {
	root := filepath.Join(repoRoot(t), "backend")
	var found []string
	report := func(path string, n int, why string) {
		rel, _ := filepath.Rel(root, path)
		found = append(found, rel+":"+strconv.Itoa(n)+": "+why)
	}
	checkSQL := func(path string, n int, line string) {
		for _, b := range sqlBanned {
			if b.re.MatchString(line) {
				report(path, n, b.why)
			}
		}
		if sessionDate.MatchString(paramDate.ReplaceAllString(line, "")) && !strings.Contains(strings.ToUpper(line), "AT TIME ZONE") {
			report(path, n, "a timestamp's day in the session's zone")
		}
	}

	for _, dir := range []string{"internal", "cmd"} {
		err := filepath.Walk(filepath.Join(root, dir), func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if info.IsDir() && path == filepath.Join(root, "internal", "clock") {
				return filepath.SkipDir
			}
			if info.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			src, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			for i, line := range strings.Split(string(src), "\n") {
				if strings.HasPrefix(strings.TrimSpace(line), "//") {
					continue
				}
				for _, b := range goBanned {
					if b.re.MatchString(line) {
						report(path, i+1, b.why)
					}
				}
				if dateFormat.MatchString(line) && !strings.Contains(line, "clock.") {
					report(path, i+1, "a date formatted with no zone named: use clock.Day or .In(clock.Loc)")
				}
				checkSQL(path, i+1, line)
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}

	migrations, err := filepath.Glob(filepath.Join(root, "db", "migrations", "*.up.sql"))
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range migrations {
		if filepath.Base(path) < "000079" { // history: written before the one zone
			continue
		}
		src, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		for i, line := range strings.Split(string(src), "\n") {
			if !strings.HasPrefix(strings.TrimSpace(line), "--") {
				checkSQL(path, i+1, line)
			}
		}
	}

	for _, f := range found {
		t.Error(f)
	}
}
