package cache

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"syndra/internal/claimserve"
	"syndra/internal/db"
	"syndra/internal/models"
)

// A person the directory no longer lists is gone, not failing: counted apart,
// left out of the ratio, their facts kept, and nothing asked of Zitadel about
// them (assessment D1, F-6, SC3). A real failure beside it still counts.
func TestWarm_GoneFromTheDirectoryIsGoneNotAsked(t *testing.T) {
	resetCacheDeps(t)
	noPace(t)
	s := fakeRedis(t)
	origIDs, origFind := dbGetAllKnownUserIDs, cacheFindUser
	t.Cleanup(func() { dbGetAllKnownUserIDs, cacheFindUser = origIDs, origFind })
	dbGetAllKnownUserIDs = func(context.Context) ([]string, error) { return []string{"uOk", "uGone", "uFlaky"}, nil }
	warmDirectory(t, "uOk", "uFlaky")
	s.hashes[claimserve.FactsKey("uGone")] = map[string]string{"pA": `{"roles":["viewer"]}`, claimserve.MetaField: "x"}
	asked := 0
	cacheFindUser = func(context.Context, string) (models.UserProfile, bool, error) {
		asked++
		return models.UserProfile{}, false, nil
	}
	dbGetDirectGrantsForUser = func(_ context.Context, id string, _ bool) ([]models.DirectGrant, error) {
		if id == "uFlaky" {
			return nil, fmt.Errorf("timeout")
		}
		return []models.DirectGrant{{ProjectID: "pA", RoleKey: "viewer"}}, nil
	}
	dbGetActiveMappingRules = func(context.Context) ([]models.MappingRule, error) { return nil, nil }

	for pass := 0; pass < 2; pass++ {
		if err := Warm(context.Background()); err != nil {
			t.Fatalf("warm: %v", err)
		}
		rep, ok := LastWarm()
		if !ok || rep.Skipped || rep.Refreshed != 1 || rep.Of != 2 || rep.Failed != 1 || rep.Gone != 1 {
			t.Fatalf("pass %d: report = %+v, want 1 of 2 refreshed, 1 failed, 1 gone", pass, rep)
		}
	}
	if asked != 0 {
		t.Errorf("a warm asked the directory about %d people one by one; it must read only its snapshot", asked)
	}
	if !s.has("uGone", "pA") {
		t.Error("a gone person's facts were dropped by a warm")
	}
}

// A skipped warm is recorded as one, with how many people it left unrefreshed,
// so /healthz/claims can say so instead of the facts quietly ageing out.
func TestWarm_SkippedIsReported(t *testing.T) {
	resetCacheDeps(t)
	fakeRedis(t)
	origIDs, origSnap := dbGetAllKnownUserIDs, directorySnapshot
	t.Cleanup(func() { dbGetAllKnownUserIDs, directorySnapshot = origIDs, origSnap })
	dbGetAllKnownUserIDs = func(context.Context) ([]string, error) { return []string{"u1", "u2", "u3"}, nil }
	directorySnapshot = func(context.Context) ([]models.UserProfile, bool) { return nil, false }

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

// A warm is spread, not one burst: it waits warmPace between people, and a
// cancelled context stops it mid-pass.
func TestWarm_IsPacedAndStops(t *testing.T) {
	resetCacheDeps(t)
	fakeRedis(t)
	orig, origIDs := warmPace, dbGetAllKnownUserIDs
	t.Cleanup(func() { warmPace, dbGetAllKnownUserIDs = orig, origIDs })
	warmPace = 20 * time.Millisecond
	dbGetAllKnownUserIDs = func(context.Context) ([]string, error) { return []string{"u1", "u2", "u3", "u4", "u5", "u6"}, nil }
	warmDirectory(t, "u1", "u2", "u3", "u4", "u5", "u6")
	holding(models.DirectGrant{ProjectID: "pA", RoleKey: "viewer"})

	start := time.Now()
	if err := Warm(context.Background()); err != nil {
		t.Fatal(err)
	}
	if took := time.Since(start); took < 5*warmPace {
		t.Errorf("six people warmed in %s: not paced at %s", took, warmPace)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	if err := Warm(ctx); err == nil {
		t.Error("a cancelled warm ran to the end")
	}
}

// The S1b proof (assessment §5): a warm of 10,000 people against counting
// fakes does O(own projects) Redis commands per person, a constant number of
// global reads per pass, one directory snapshot, and no per-person directory
// or IdP call. Before S1b the same pass SCANned the whole keyspace per person
// (about 1M SCAN calls at 5,000 people) and re-read every global table per
// person.
func TestWarm_TenThousandPeopleScale(t *testing.T) {
	if testing.Short() {
		t.Skip("scale proof")
	}
	resetCacheDeps(t)
	noPace(t)
	s := fakeRedis(t)
	const people = 10000
	ids := make([]string, people)
	for i := range ids {
		ids[i] = fmt.Sprintf("u%05d", i)
	}
	origIDs, origFind := dbGetAllKnownUserIDs, cacheFindUser
	t.Cleanup(func() { dbGetAllKnownUserIDs, cacheFindUser = origIDs, origFind })

	var global, perPerson, find atomic.Int64
	dbGetAllKnownUserIDs = func(context.Context) ([]string, error) { global.Add(1); return ids, nil }
	dbGetActiveMappingRules = func(context.Context) ([]models.MappingRule, error) {
		global.Add(1)
		return []models.MappingRule{{SourceProject: "p0", SourceRole: "member", TargetProject: "p1", TargetRole: "viewer"}}, nil
	}
	dbGetAllBundles = func(context.Context) ([]models.Bundle, error) {
		global.Add(1)
		return []models.Bundle{{ID: "b1", Name: "Makers"}}, nil
	}
	dbListServiceAccounts = func(context.Context) ([]db.ServiceAccountRow, error) { global.Add(1); return nil, nil }
	// Three projects each, as the capacity model assumes (§4.0a).
	dbGetDirectGrantsForUser = func(_ context.Context, id string, _ bool) ([]models.DirectGrant, error) {
		perPerson.Add(1)
		return []models.DirectGrant{{ProjectID: "p0", RoleKey: "member"}, {ProjectID: "p2", RoleKey: "staff"}}, nil
	}
	dbGetUserBundleRoles = func(context.Context, string) (map[string][]models.BundleRole, error) {
		perPerson.Add(1)
		return map[string][]models.BundleRole{"b1": {{BundleID: "b1", ProjectID: "p2", RoleKey: "trained"}}}, nil
	}
	snapshots := warmDirectory(t, ids...)
	cacheFindUser = func(context.Context, string) (models.UserProfile, bool, error) {
		find.Add(1)
		return models.UserProfile{}, false, nil
	}

	// Everyone starts on the old layout, as on the first warm after S1b.
	for _, id := range ids {
		s.kv[claimserve.LegacyFactsKey(id, "p0")] = `{"roles":["member"]}`
	}
	for pass := 1; pass <= 2; pass++ {
		global.Store(0)
		perPerson.Store(0)
		s.ops = map[string]int{}
		start := time.Now()
		if err := Warm(context.Background()); err != nil {
			t.Fatal(err)
		}
		took := time.Since(start)
		rep, _ := LastWarm()
		if rep.Refreshed != people {
			t.Fatalf("pass %d: %+v", pass, rep)
		}
		redisOps := 0
		for _, n := range s.ops {
			redisOps += n
		}
		t.Logf("measured: pass %d over %d people in %s: %d Redis commands (%.2f a person: %v), %d global reads, %d per-person DB reads, %d directory snapshots, %d per-person directory reads",
			pass, people, took.Round(time.Millisecond), redisOps, float64(redisOps)/people, s.ops, global.Load(), perPerson.Load(), *snapshots, find.Load())

		// Per person: one HKEYS and one MULTI (three projects ride in it). Per
		// pass: one sweep for old keys, until a sweep finds none, and the L1
		// canary's one MULTI (S6).
		if wantRedis := 2*people + 2; redisOps != wantRedis || s.ops["SCAN"] != 1 {
			t.Errorf("pass %d: %d Redis commands (%v), want %d", pass, redisOps, s.ops, wantRedis)
		}
		if len(s.kv) != 0 {
			t.Errorf("pass %d: %d old keys survived the warm", pass, len(s.kv))
		}
		if global.Load() != 4 {
			t.Errorf("pass %d: %d global reads, want 4 (known ids, rules, bundles, marks) whatever the head count", pass, global.Load())
		}
		if perPerson.Load() != 2*people {
			t.Errorf("pass %d: %d per-person DB reads, want 2 a person (grants, bundles)", pass, perPerson.Load())
		}
	}
	if *snapshots != 2 || find.Load() != 0 {
		t.Errorf("directory: %d snapshots over 2 passes (want 2), %d per-person reads (want 0)", *snapshots, find.Load())
	}
	if got := len(s.roles(t, "u00042", "p1")); got != 1 {
		t.Errorf("a rule-derived project was not compiled: %v", s.hashes[claimserve.FactsKey("u00042")])
	}
}
