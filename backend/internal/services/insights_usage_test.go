package services

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"syndra/internal/db"
	"syndra/internal/models"
)

func TestParseUsageDays(t *testing.T) {
	for in, want := range map[string]int{"": 90, "1": 1, "90": 90, "365": 365} {
		if got, err := ParseUsageDays(in); err != nil || got != want {
			t.Errorf("%q = %d, %v; want %d", in, got, err, want)
		}
	}
	for _, bad := range []string{"0", "366", "-1", "x", "1.5", " 3", "+3x", "99999999999999999999"} {
		if _, err := ParseUsageDays(bad); !errors.Is(err, ErrInvalidUsageDays) {
			t.Errorf("%q: err = %v", bad, err)
		}
	}
}

type usageStub struct {
	now                   time.Time
	latest                map[string]time.Time
	used                  map[string]map[string]bool
	holders               map[string][]string
	observed              bool
	since                 time.Time
	holdersRead, usedRead bool
}

func (u *usageStub) install(t *testing.T) {
	t.Helper()
	pn, pl, pu, ph := usageNow, usageLatest, usageUsed, usageHolders
	t.Cleanup(func() { usageNow, usageLatest, usageUsed, usageHolders = pn, pl, pu, ph })
	usageNow = func() time.Time { return u.now }
	usageLatest = func(_ context.Context, since time.Time) (map[string]time.Time, error) {
		u.since = since
		return u.latest, nil
	}
	usageUsed = func(_ context.Context, kind string, _ time.Time) (map[string]map[string]bool, error) {
		u.usedRead = true
		if kind != db.UsageSignIn {
			t.Errorf("kind = %q", kind)
		}
		return u.used, nil
	}
	usageHolders = func(context.Context) (map[string][]string, bool, error) {
		u.holdersRead = true
		return u.holders, u.observed, nil
	}
}

func TestInsightsUsage_HeldNotUsed(t *testing.T) {
	now := time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC)
	last := now.Add(-2 * time.Hour)
	u := &usageStub{
		now:    now,
		latest: map[string]time.Time{db.UsageSignIn: last},
		holders: map[string][]string{
			"pA:maker": {"u1", "u2", "u3"},
			"pA:admin": {"u1"},
			"pB:read":  {"u1", "u2"},
			"pC:x:y":   {"u9"}, // role keys may contain ':'
		},
		used: map[string]map[string]bool{
			"u1": {"pA": true},             // used pA, not pB
			"u2": {"pB": true},             // used pB, not pA
			"u3": {"other": true},          // used nothing they hold
			"u8": {"pA": true, "pB": true}, // not a holder; irrelevant
		},
		observed: true,
	}
	u.install(t)
	got, err := InsightsUsage(context.Background(), 30)
	if err != nil {
		t.Fatal(err)
	}
	want := []models.HeldNotUsed{
		{ProjectID: "pA", RoleKey: "maker", People: 2}, // u2, u3
		{ProjectID: "pB", RoleKey: "read", People: 1},  // u1
		{ProjectID: "pC", RoleKey: "x:y", People: 1},
	}
	if !reflect.DeepEqual(got.HeldNotUsed, want) {
		t.Errorf("held_not_used = %+v, want %+v", got.HeldNotUsed, want)
	}
	if !got.Sources.SignIn.Receiving || got.Sources.SignIn.LastAt == nil || !got.Sources.SignIn.LastAt.Equal(last) ||
		got.Sources.Badge.Receiving || got.Sources.Badge.LastAt != nil || got.WindowDays != 30 || !got.HoldersObserved {
		t.Errorf("got %+v", got)
	}
	if want := now.Add(-30 * 24 * time.Hour); !u.since.Equal(want) {
		t.Errorf("window starts %v, want %v", u.since, want)
	}
}

func TestInsightsUsage_NotReceivingNeverReportsUnused(t *testing.T) {
	u := &usageStub{
		now:      time.Now(),
		latest:   map[string]time.Time{}, // no sign-in row in the window
		holders:  map[string][]string{"p:r": {"u1", "u2"}},
		observed: true,
	}
	u.install(t)
	got, err := InsightsUsage(context.Background(), 90)
	if err != nil {
		t.Fatal(err)
	}
	if got.Sources.SignIn.Receiving || got.HeldNotUsed == nil || len(got.HeldNotUsed) != 0 {
		t.Errorf("got %+v; want not receiving and an empty (non-nil) list", got)
	}
	if u.holdersRead || u.usedRead {
		t.Error("silence must short-circuit before any holder or usage read")
	}
	// A badge-only window is not sign-in silence being broken.
	u.latest = map[string]time.Time{db.UsageBadge: time.Now()}
	if got, _ = InsightsUsage(context.Background(), 90); got.Sources.SignIn.Receiving || !got.Sources.Badge.Receiving || len(got.HeldNotUsed) != 0 {
		t.Errorf("badge only: %+v", got)
	}
}

func TestInsightsUsage_NothingObservedSaysNothing(t *testing.T) {
	u := &usageStub{now: time.Now(), latest: map[string]time.Time{db.UsageSignIn: time.Now()}, observed: false}
	u.install(t)
	got, err := InsightsUsage(context.Background(), 90)
	if err != nil || got.HoldersObserved || len(got.HeldNotUsed) != 0 || !got.Sources.SignIn.Receiving {
		t.Errorf("got %+v, %v", got, err)
	}
}

func TestPruneUsageLastSeen_CutoffIs400Days(t *testing.T) {
	now := time.Date(2026, 10, 16, 0, 0, 0, 0, time.UTC)
	pn, pp := usageNow, usagePrune
	t.Cleanup(func() { usageNow, usagePrune = pn, pp })
	usageNow = func() time.Time { return now }
	var cutoff time.Time
	usagePrune = func(_ context.Context, before time.Time) (int64, error) { cutoff = before; return 3, nil }
	if err := PruneUsageLastSeen(context.Background()); err != nil {
		t.Fatal(err)
	}
	if want := now.AddDate(0, 0, -400); !cutoff.Equal(want) {
		t.Errorf("cutoff %v, want %v", cutoff, want)
	}
	usagePrune = func(context.Context, time.Time) (int64, error) { return 0, errors.New("boom") }
	if err := PruneUsageLastSeen(context.Background()); err == nil {
		t.Error("a failed prune must be returned for the runner to log")
	}
}
