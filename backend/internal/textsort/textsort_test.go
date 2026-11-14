package textsort

import (
	"reflect"
	"sort"
	"testing"

	"golang.org/x/text/language"
)

// The cases mirror ui/src/lib/__tests__/sort.test.ts: the backend and the
// screen must agree on what alphabetical means.
func TestStringsReadsNamesTheWayTheScreenDoes(t *testing.T) {
	for _, tc := range []struct {
		name     string
		in, want []string
	}{
		{"case aside", []string{"mkauth", "Kanishk Singh", "claude code", "Admin Makerspace", "netbird"},
			[]string{"Admin Makerspace", "claude code", "Kanishk Singh", "mkauth", "netbird"}},
		{"accents aside", []string{"Zoë", "Émile", "Eugene", "Edge"}, []string{"Edge", "Émile", "Eugene", "Zoë"}},
		{"numbers are numbers", []string{"Lab 10", "Lab 2", "lab 1"}, []string{"lab 1", "Lab 2", "Lab 10"}},
	} {
		got := append([]string(nil), tc.in...)
		Strings(got)
		if !reflect.DeepEqual(got, tc.want) {
			t.Errorf("%s: got %v, want %v", tc.name, got, tc.want)
		}
	}
}

func TestByNameBreaksTiesByTheTieKey(t *testing.T) {
	type row struct{ name, id string }
	rows := []row{{"Lab", "b"}, {"lab", "a"}, {"Aa", "z"}}
	ByName(rows, func(r row) string { return r.name }, func(r row) string { return r.id })
	want := []row{{"Aa", "z"}, {"lab", "a"}, {"Lab", "b"}}
	if !reflect.DeepEqual(rows, want) {
		t.Fatalf("got %v, want %v", rows, want)
	}
}

// An order is a display: in Swedish å sorts after z, in the root order it
// sits with the As. The collator takes the deployment's locale.
func TestTheOrderFollowsTheLocale(t *testing.T) {
	names := []string{"Åsa", "Zoe", "Anna"}
	order := func(tag language.Tag) []string {
		c := newCollator(tag)
		out := append([]string(nil), names...)
		sort.SliceStable(out, func(i, j int) bool { return c.CompareString(out[i], out[j]) < 0 })
		return out
	}
	if got := order(language.Und); got[0] != "Anna" || got[1] != "Åsa" {
		t.Errorf("root order = %v; want Anna, Åsa, Zoe", got)
	}
	if got := order(language.MustParse("sv")); got[2] != "Åsa" {
		t.Errorf("sv order = %v; want Anna, Zoe, Åsa", got)
	}
}
