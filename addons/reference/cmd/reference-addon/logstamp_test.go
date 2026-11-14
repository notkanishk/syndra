package main

import (
	"bytes"
	"regexp"
	"strings"
	"testing"
	"time"
)

var stampPrefix = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}\.\d{3}(Z|[+-]\d{2}:\d{2}) `)

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

func TestStampIsTheSameInstantInAnyZone(t *testing.T) {
	ist := time.FixedZone("IST", 5*3600+1800)
	at := time.Date(2026, 11, 14, 10, 8, 34, 123e6, time.UTC)
	a, b := stamped(t, at), stamped(t, at.In(ist))
	if !strings.HasPrefix(b, "2026-11-14T15:38:34.123+05:30 ") || !strings.HasPrefix(a, "2026-11-14T10:08:34.123Z ") {
		t.Fatalf("got %q and %q", a, b)
	}
	pa, err1 := time.Parse(time.RFC3339Nano, strings.Fields(a)[0])
	pb, err2 := time.Parse(time.RFC3339Nano, strings.Fields(b)[0])
	if err1 != nil || err2 != nil || pa.Unix() != pb.Unix() || pa.Unix() != at.Unix() {
		t.Fatalf("unix instants differ: %v %v (%v, %v)", pa, pb, err1, err2)
	}
}
