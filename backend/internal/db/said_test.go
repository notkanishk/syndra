package db

import "testing"

// An audit line's said is the operator's reason, or a cancellation's reason
// for the change it cancelled; never one of Syndra's labels (task 1.8a).
func TestDetailsSaid(t *testing.T) {
	for details, want := range map[string]string{
		`{"reason":" term start "}`:                         "term start",
		`{"reason":"Direct access removal"}`:                "",
		`{"cancelled":{"reason":"left the course"}}`:        "left the course",
		`{"cancelled":{"reason":"Mapping rule cascade"}}`:   "",
		`{"cancelled":{"kind":"give"},"cause":{"via":"x"}}`: "",
		``: "",
	} {
		if got := detailsSaid([]byte(details)); got != want {
			t.Errorf("detailsSaid(%s) = %q, want %q", details, got, want)
		}
	}
}
