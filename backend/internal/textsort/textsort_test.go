package textsort

import (
	"reflect"
	"testing"
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
