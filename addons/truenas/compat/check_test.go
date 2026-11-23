package compat

import (
	"strings"
	"testing"
	"testing/fstest"
)

func TestLoadRefusesWhatIsNotACapture(t *testing.T) {
	file := func(body string) fstest.MapFS {
		return fstest.MapFS{"baseline/TrueNAS-1.methods.json": {Data: []byte(body)}}
	}
	for _, c := range []struct {
		name      string
		fsys      fstest.MapFS
		synthetic bool
		want      string
	}{
		{"nothing shipped", fstest.MapFS{}, false, "no capture is shipped"},
		{"not JSON", file("{"), false, "TrueNAS-1.methods.json is not a capture: unexpected end"},
		{"no version", file(`{"methods": {"user.query": {}}}`), false, "is not a capture: no version or no methods"},
		{"no methods", file(`{"version": "1"}`), false, "is not a capture: no version or no methods"},
		{"synthetic outside tests", file(`{"_synthetic": true, "version": "1", "methods": {"user.query": {}}}`), false,
			"is synthetic: only tests may load it"},
	} {
		_, err := load(c.fsys, c.synthetic)
		if err == nil || !strings.Contains(err.Error(), c.want) || strings.Contains(err.Error(), "<nil>") {
			t.Errorf("%s: %v, want an error naming %q", c.name, err, c.want)
		}
	}
	b, err := load(file(`{"_synthetic": true, "version": "1", "methods": {"user.query": {}}}`), true)
	if err != nil || len(b) != 1 {
		t.Errorf("synthetic in a test: %v, %d captures, want one", err, len(b))
	}
}
