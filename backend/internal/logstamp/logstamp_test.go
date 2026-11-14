package logstamp

import (
	"bytes"
	"log"
	"regexp"
	"strings"
	"testing"
	"time"
)

var stampPrefix = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}\.\d{3}Z `)

func stamped(t *testing.T, at time.Time) string {
	t.Helper()
	var b bytes.Buffer
	n, err := stampWriter{&b, func() time.Time { return at }}.Write([]byte("[SYSTEM] hello\n"))
	if err != nil || n != len("[SYSTEM] hello\n") {
		t.Fatalf("Write = %d, %v", n, err)
	}
	return b.String()
}

func TestStampFormat(t *testing.T) {
	ist := time.FixedZone("IST", 5*3600+1800)
	for _, loc := range []*time.Location{time.UTC, ist} {
		line := stamped(t, time.Date(2026, 11, 14, 15, 38, 34, 123e6, loc))
		if !stampPrefix.MatchString(line) || !strings.HasSuffix(line, " [SYSTEM] hello\n") {
			t.Errorf("line %q does not start with the one stamp", line)
		}
	}
}

// Logs are UTC whatever zone the clock reads in: the same instant is the same
// line in every container, and it parses back to the same unix time.
func TestStampIsUTCWhateverTheZone(t *testing.T) {
	ist := time.FixedZone("IST", 5*3600+1800)
	at := time.Date(2026, 11, 14, 10, 8, 34, 123e6, time.UTC)
	a, b := stamped(t, at), stamped(t, at.In(ist))
	if !strings.HasPrefix(a, "2026-11-14T10:08:34.123Z ") || !strings.HasPrefix(b, "2026-11-14T10:08:34.123Z ") {
		t.Fatalf("got %q and %q", a, b)
	}
	p, err := time.Parse(time.RFC3339Nano, strings.Fields(b)[0])
	if err != nil || p.Unix() != at.Unix() {
		t.Fatalf("unix instant differs: %v (%v)", p, err)
	}
}

// The standard logger must add nothing of its own: a dropped log.SetFlags(0)
// would print two stamps on every line.
func TestInstallLeavesOneStamp(t *testing.T) {
	flags, out := log.Flags(), log.Writer()
	t.Cleanup(func() { log.SetFlags(flags); log.SetOutput(out) })
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)
	Install()
	if log.Flags() != 0 {
		t.Fatalf("log flags = %d after install; the logger would stamp a second time", log.Flags())
	}
	if _, ok := log.Writer().(stampWriter); !ok {
		t.Fatalf("log output is %T, not the stamping writer", log.Writer())
	}
}
