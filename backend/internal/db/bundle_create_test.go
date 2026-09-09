package db

import "testing"

// v1's note is the only trace of what a bundle was created with, and it is read
// by a human six months later in the version history. "Created empty." used to
// be the only thing it could say, because an empty v1 was the only thing
// creation could produce.
func TestInitialVersionNote_SaysWhatV1Contains(t *testing.T) {
	cases := map[int]string{
		0: "Created empty.",
		1: "Created with 1 role.",
		2: "Created with 2 roles.",
		7: "Created with 7 roles.",
	}
	for roles, want := range cases {
		if got := InitialVersionNote(roles); got != want {
			t.Errorf("InitialVersionNote(%d) = %q, want %q", roles, got, want)
		}
	}
}
