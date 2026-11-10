package services

import (
	"testing"
	"time"

	"syndra/internal/db"
	"syndra/internal/models"
)

func read(complete bool) models.ObservationBasis {
	now := time.Now()
	return models.ObservationBasis{ReadAt: &now, Current: true, Truncated: !complete}
}

func row(op, status string, seq int64, keys ...string) db.RoleDelivery {
	d := db.RoleDelivery{ProjectID: "p", OpType: op, Status: status, IntentSeq: seq}
	if op == "replace" {
		d.RoleKeys = keys
	} else {
		d.RoleKey = keys[0]
	}
	return d
}

func confirmed(d db.RoleDelivery) db.RoleDelivery { d.Confirmed = true; return d }

// The design's table (one-path design.md, W3b), one case per line of it.
func TestStanding_TheDesignTable(t *testing.T) {
	seen := map[string]bool{"p:r": true}
	none := map[string]bool{}
	for _, tc := range []struct {
		name  string
		rows  []db.RoleDelivery
		seen  map[string]bool
		basis models.ObservationBasis
		want  models.RoleStanding // "" means no standing (nil)
	}{
		{"a waiting give", []db.RoleDelivery{row("add", "pending", 1, "r")}, none, read(true), models.StandingWaiting},
		{"a give being sent", []db.RoleDelivery{row("add", "in_flight", 1, "r")}, none, read(true), models.StandingWaiting},
		{"a waiting removal is still held", []db.RoleDelivery{row("revoke", "pending", 1, "r")}, seen, read(true), models.StandingRemoving},
		{"a sent removal Zitadel still shows", []db.RoleDelivery{row("revoke", "applied", 1, "r")}, seen, read(true), models.StandingRemoving},
		{"a sent removal Zitadel no longer shows", []db.RoleDelivery{row("revoke", "applied", 1, "r")}, none, read(true), models.StandingNotHeld},
		{"a sent give not seen back", []db.RoleDelivery{row("add", "applied", 1, "r")}, none, read(true), models.StandingSent},
		{"a sent give not seen back, Zitadel never read", []db.RoleDelivery{row("add", "applied", 1, "r")}, none, models.ObservationBasis{}, models.StandingSent},
		{"a sent give Zitadel shows", []db.RoleDelivery{row("add", "applied", 1, "r")}, seen, read(true), models.StandingHeld},
		{"a confirmed give", []db.RoleDelivery{confirmed(row("add", "applied", 1, "r"))}, seen, read(true), models.StandingHeld},
		{"a confirmed give gone by hand", []db.RoleDelivery{confirmed(row("add", "applied", 1, "r"))}, none, read(true), models.StandingNotHeld},
		{"a give that failed", []db.RoleDelivery{row("add", "failed", 1, "r")}, none, read(true), models.StandingNotHeld},
		{"no row, Zitadel shows it (not given by Syndra)", nil, seen, read(true), models.StandingHeld},
		{"no row, complete read without it", nil, none, read(true), models.StandingNotHeld},
		{"no row, incomplete read without it", nil, none, read(false), ""},
		{"no row, Zitadel never read", nil, none, models.ObservationBasis{}, ""},
		{"a sent removal, incomplete read without it", []db.RoleDelivery{row("revoke", "applied", 1, "r")}, none, read(false), ""},
		// Holding is observed: a row on its way never overrides what Zitadel shows.
		{"a give on its way to a role Zitadel already shows (a second source)", []db.RoleDelivery{row("add", "pending", 1, "r")}, seen, read(true), models.StandingHeld},
		{"a removal on its way of a role Zitadel doesn't show", []db.RoleDelivery{row("revoke", "pending", 1, "r")}, none, read(true), models.StandingNotHeld},
		{"a removal on its way, incomplete read without it", []db.RoleDelivery{row("revoke", "pending", 1, "r")}, none, read(false), ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := newStandings(db.NewDeliveries(tc.rows...), tc.seen, tc.basis).Of("p", "r")
			assertStanding(t, got, tc.want)
		})
	}
}

// The latest row decides: an add a later revoke took back is a removal, and
// the other way round.
func TestStanding_TheLatestRowDecides(t *testing.T) {
	seen := map[string]bool{"p:r": true}
	// Only the latest add/revoke per role reaches here (RoleDeliveries), but a
	// replace on the project competes with it by intent_seq.
	got := newStandings(db.NewDeliveries(
		confirmed(row("add", "applied", 1, "r")),
		row("replace", "pending", 2, "other"),
	), seen, read(true)).Of("p", "r")
	assertStanding(t, got, models.StandingRemoving)

	got = newStandings(db.NewDeliveries(
		row("add", "pending", 3, "r"),
		row("replace", "pending", 2, "other"),
	), map[string]bool{}, read(true)).Of("p", "r")
	assertStanding(t, got, models.StandingWaiting)
}

// A person's own complete read, newer than the org sweep, settles their
// absences (it rewrote all of their rows); an older one never overrides a
// newer sweep, and an incomplete one settles nothing.
func TestPersonBasis_TheNewerReadIsTheFooting(t *testing.T) {
	now := time.Now()
	capped := models.ObservationBasis{ReadAt: &now, Current: true, Truncated: true}
	newer := db.Observation{ObservedAt: now.Add(time.Minute), Complete: true}
	older := db.Observation{ObservedAt: now.Add(-time.Minute), Complete: true}
	newerCapped := db.Observation{ObservedAt: now.Add(time.Minute), Complete: false, Error: "the read reached its safety limit"}
	for _, tc := range []struct {
		name string
		org  models.ObservationBasis
		own  *db.Observation
		want bool
	}{
		{"own complete read after a capped sweep", capped, &newer, true},
		{"own complete read before a capped sweep", capped, &older, false},
		{"own capped read after a capped sweep", capped, &newerCapped, false},
		{"own complete read, no sweep ever", models.ObservationBasis{}, &newer, true},
		{"never read on their own", capped, nil, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := personBasis(tc.org, tc.own).ConcludesAbsence(); got != tc.want {
				t.Fatalf("concludes absence %v, want %v", got, tc.want)
			}
		})
	}
}

// A replace gives what it names and takes the rest.
func TestStanding_AReplaceGivesWhatItNames(t *testing.T) {
	st := newStandings(db.NewDeliveries(row("replace", "pending", 1, "r", "s")), map[string]bool{"p:t": true}, read(true))
	assertStanding(t, st.Of("p", "r"), models.StandingWaiting)
	assertStanding(t, st.Of("p", "t"), models.StandingRemoving)
	// Another project is untouched by it.
	assertStanding(t, st.Of("q", "t"), models.StandingNotHeld)
}

func assertStanding(t *testing.T, got *models.RoleStanding, want models.RoleStanding) {
	t.Helper()
	switch {
	case want == "" && got != nil:
		t.Fatalf("standing %q, want none: absence can't be concluded", *got)
	case want != "" && got == nil:
		t.Fatalf("no standing, want %q", want)
	case want != "" && *got != want:
		t.Fatalf("standing %q, want %q", *got, want)
	}
}
