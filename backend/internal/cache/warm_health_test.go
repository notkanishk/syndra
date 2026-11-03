package cache

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"syndra/internal/models"
	"syndra/internal/zitadel"
)

// A person Zitadel 404s is gone, not failing: counted apart, left out of the
// ratio, and not asked about again on the next pass (assessment D1, F-6). A
// real failure beside it still counts as one.
func TestWarm_ZitadelNotFoundIsGoneNotRetried(t *testing.T) {
	resetCacheDeps(t)
	fakeRedis(t)
	origIDs, origFind, origAnswers := dbGetAllKnownUserIDs, cacheFindUser, directoryAnswers
	t.Cleanup(func() {
		dbGetAllKnownUserIDs, cacheFindUser, directoryAnswers = origIDs, origFind, origAnswers
		goneUsers.Delete("uGone")
	})
	dbGetAllKnownUserIDs = func(context.Context) ([]string, error) { return []string{"uOk", "uGone", "uFlaky"}, nil }
	directoryAnswers = func(context.Context) bool { return true }
	holding(models.DirectGrant{ProjectID: "pA", RoleKey: "viewer"})
	asked := map[string]int{}
	cacheFindUser = func(_ context.Context, id string) (models.UserProfile, bool, error) {
		asked[id]++
		switch id {
		case "uGone":
			return models.UserProfile{}, false, fmt.Errorf("directory: get user %s: %w", id, &zitadel.StatusError{Code: 404})
		case "uFlaky":
			return models.UserProfile{}, false, errors.New("timeout")
		}
		return models.UserProfile{ID: id}, true, nil
	}

	for pass := 0; pass < 2; pass++ {
		if err := Warm(context.Background()); err != nil {
			t.Fatalf("warm: %v", err)
		}
		rep, ok := LastWarm()
		if !ok || rep.Skipped || rep.Refreshed != 1 || rep.Of != 2 || rep.Failed != 1 || rep.Gone != 1 {
			t.Fatalf("pass %d: report = %+v, want 1 of 2 refreshed, 1 failed, 1 gone", pass, rep)
		}
	}
	if asked["uGone"] != 1 {
		t.Errorf("a gone person was asked about %d times across two passes, want once", asked["uGone"])
	}
	if asked["uFlaky"] != 2 {
		t.Errorf("a failing person must be retried each pass, asked %d times", asked["uFlaky"])
	}
}

// A skipped warm is recorded as one, with how many people it left unrefreshed,
// so /healthz/claims can say so instead of the facts quietly ageing out.
func TestWarm_SkippedIsReported(t *testing.T) {
	resetCacheDeps(t)
	fakeRedis(t)
	origIDs, origAnswers := dbGetAllKnownUserIDs, directoryAnswers
	t.Cleanup(func() { dbGetAllKnownUserIDs, directoryAnswers = origIDs, origAnswers })
	dbGetAllKnownUserIDs = func(context.Context) ([]string, error) { return []string{"u1", "u2", "u3"}, nil }
	directoryAnswers = func(context.Context) bool { return false }

	if err := Warm(context.Background()); err != nil {
		t.Fatalf("warm: %v", err)
	}
	rep, ok := LastWarm()
	if !ok || !rep.Skipped || rep.Of != 3 || rep.Refreshed != 0 {
		t.Fatalf("report = %+v, want a skipped pass over 3 people", rep)
	}
}

// The TTL is a backstop well past the warm interval: two missed 12h warms
// must not expire anyone.
func TestCacheTTL_OutlivesSeveralMissedWarms(t *testing.T) {
	if cacheTTL < 7*24*60*60*1e9 {
		t.Fatalf("cacheTTL = %s; it must outlive many missed 12h warms (F-6)", cacheTTL)
	}
}
