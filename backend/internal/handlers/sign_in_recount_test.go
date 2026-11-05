package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"syndra/internal/clock"
	"syndra/internal/db"
	"syndra/internal/zitadel"
)

type recountHarness struct {
	*lookupHarness
	first, through *string
	replaced       *struct {
		first, through string
		counts         map[db.SignInDayKey]int
	}
}

func stubRecount(t *testing.T, first, through string) *recountHarness {
	t.Helper()
	h := &recountHarness{lookupHarness: stubLookup(t), first: &first, through: &through}
	pending, replace := dbSignInRecountPending, dbReplaceSignInDays
	t.Cleanup(func() { dbSignInRecountPending, dbReplaceSignInDays = pending, replace })
	dbSignInRecountPending = func(context.Context) (*string, *string, error) { return h.first, h.through, nil }
	dbReplaceSignInDays = func(_ context.Context, f, th string, c map[db.SignInDayKey]int) error {
		h.replaced = &struct {
			first, through string
			counts         map[db.SignInDayKey]int
		}{f, th, c}
		return nil
	}
	return h
}

func signIn(user, client, at string) zitadel.LoggedEvent {
	return zitadel.LoggedEvent{Type: "oidc_session.added", AggregateID: "s-" + user + at, AggregateType: "oidc_session",
		At:      mustAt(at),
		Payload: json.RawMessage(`{"userID":"` + user + `","clientID":"` + client + `"}`)}
}

// inIST pins the makerspace's zone for one test, whatever TZ the machine has.
func inIST(t *testing.T) {
	t.Helper()
	ist, err := time.LoadLocation("Asia/Kolkata")
	if err != nil {
		t.Fatal(err)
	}
	old := clock.Loc
	clock.Loc = ist
	t.Cleanup(func() { clock.Loc = old })
}

func mustAt(s string) time.Time {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		panic(err)
	}
	return t
}

// A sign-in at 23:00 UTC on 3 Nov is 04:30 on 4 Nov in India: the recount puts
// it on the 4th, where the UTC-day total had put it on the 3rd.
func TestRecountSignInDays_CountsOnTheMakerspaceDayFromZitadelsLog(t *testing.T) {
	inIST(t)
	h := stubRecount(t, "2026-11-03", "2026-11-04")
	h.events = []zitadel.LoggedEvent{
		signIn("u1", "111@portal", "2026-11-03T05:00:00Z"), // 10:30 IST on the 3rd
		signIn("u1", "111@portal", "2026-11-03T23:00:00Z"), // 04:30 IST on the 4th
		signIn("u1", "111@portal", "2026-11-03T23:00:00Z"), // the same sign-in twice: counted once
		signIn("u2", "222@wiki", "2026-11-04T12:00:00Z"),
		signIn("u3", "nobody@x", "2026-11-04T12:00:00Z"),   // app unknown
		signIn("u4", "111@portal", "2026-11-02T18:00:00Z"), // 23:30 IST on the 2nd: outside
	}

	if err := RecountSignInDays(context.Background()); err != nil {
		t.Fatal(err)
	}
	if h.replaced == nil || h.replaced.first != "2026-11-03" || h.replaced.through != "2026-11-04" {
		t.Fatalf("replaced %+v", h.replaced)
	}
	want := map[db.SignInDayKey]int{
		{AppID: "app-1", Day: "2026-11-03"}: 1,
		{AppID: "app-1", Day: "2026-11-04"}: 1,
		{AppID: "app-2", Day: "2026-11-04"}: 1,
		{AppID: "", Day: "2026-11-04"}:      1,
	}
	if len(h.replaced.counts) != len(want) {
		t.Fatalf("counts = %v, want %v", h.replaced.counts, want)
	}
	for k, n := range want {
		if h.replaced.counts[k] != n {
			t.Fatalf("counts = %v, want %v", h.replaced.counts, want)
		}
	}
	// One read, from the makerspace's midnight starting the 3rd to the one ending the 4th.
	q := h.searched[0]
	if !q.Since.Equal(mustAt("2026-11-02T18:30:00Z")) || !q.Until.Equal(mustAt("2026-11-04T18:30:00Z")) {
		t.Fatalf("searched %v .. %v", q.Since, q.Until)
	}
}

// A day still being written is not replaced; nothing waiting means nothing read.
func TestRecountSignInDays_WaitsAndStops(t *testing.T) {
	today := clock.Day(time.Now())
	h := stubRecount(t, "2026-11-03", today)
	if err := RecountSignInDays(context.Background()); err != nil || h.replaced != nil || len(h.searched) != 0 {
		t.Fatalf("today is still being written: err %v, replaced %+v, searched %d", err, h.replaced, len(h.searched))
	}
	h.through = nil
	if err := RecountSignInDays(context.Background()); err != nil || h.replaced != nil || len(h.searched) != 0 {
		t.Fatalf("nothing waits: err %v, replaced %+v, searched %d", err, h.replaced, len(h.searched))
	}
}

// Zitadel or the directory unreachable: the old totals and the request stay.
func TestRecountSignInDays_LeavesEverythingOnFailure(t *testing.T) {
	h := stubRecount(t, "2026-11-03", "2026-11-04")
	h.zErr = errors.New("down")
	if err := RecountSignInDays(context.Background()); err == nil || h.replaced != nil {
		t.Fatalf("zitadel down: err %v, replaced %+v", err, h.replaced)
	}
	h.zErr, h.appsErr = nil, errors.New("down")
	if err := RecountSignInDays(context.Background()); err == nil || h.replaced != nil {
		t.Fatalf("apps unreadable: err %v, replaced %+v", err, h.replaced)
	}
}

// A full answer is never read as the whole one: the window is halved until
// each half fits, and each half keeps only its own events.
func TestReadSignInEvents_HalvesAFullAnswer(t *testing.T) {
	stubLookup(t)
	since, until := mustAt("2026-11-03T00:00:00Z"), mustAt("2026-11-04T00:00:00Z")
	e1, e2 := signIn("u1", "111@portal", "2026-11-03T06:00:00Z"), signIn("u2", "111@portal", "2026-11-03T18:00:00Z")
	calls := 0
	zitadelSearchEvents = func(_ context.Context, q zitadel.EventSearch) ([]zitadel.LoggedEvent, bool, error) {
		calls++
		if q.Until.Sub(q.Since) > 12*time.Hour {
			return []zitadel.LoggedEvent{e1}, true, nil // full: there is more
		}
		return []zitadel.LoggedEvent{e1, e2}, false, nil // both halves answer both; each keeps its own
	}
	got, err := readSignInEvents(context.Background(), since, until)
	if err != nil || calls != 3 || len(got) != 2 || !got[0].At.Equal(e1.At) || !got[1].At.Equal(e2.At) {
		t.Fatalf("got %d events in %d calls (%v)", len(got), calls, err)
	}
	zitadelSearchEvents = func(context.Context, zitadel.EventSearch) ([]zitadel.LoggedEvent, bool, error) {
		return nil, true, nil
	}
	if _, err := readSignInEvents(context.Background(), since, until); err == nil {
		t.Fatal("more than the limit in one minute must be an error, not a guess")
	}
}
