package claimserve

import (
	"context"
	"fmt"
	"testing"
	"time"

	"syndra/internal/claimserve/claimservetest"
	"syndra/internal/claimtel"
)

// The probe needs its own served call even when Syndra's own Zitadel login,
// under the same account, landed seconds before with no grants (seen on prod
// 3 Nov 2026): the merged window would show only the worse outcome.
func TestRemoteServed_SeesItsOwnCallBesideAWorseNeighbour(t *testing.T) {
	r := claimservetest.New()
	ctx := context.Background()
	t0 := time.UnixMilli(1_793_730_000_000)
	key := callsKey("sa", "syndra-service")
	for _, rec := range []string{
		fmt.Sprintf("%d%s%s%s%s", t0.UnixMilli(), sep, claimtel.FnPreAccessToken, sep, claimtel.NoGrants),
		fmt.Sprintf("%d%s%s%s%s", t0.Add(5*time.Second).UnixMilli(), sep, claimtel.FnPreAccessToken, sep, claimtel.Served),
	} {
		r.RPush(ctx, key, rec)
	}
	if !RemoteServed(ctx, r, "sa", "syndra-service", claimtel.FnPreAccessToken, t0.Add(4*time.Second), t0.Add(6*time.Second)) {
		t.Fatal("the served call was hidden by its neighbour")
	}
	if RemoteServed(ctx, r, "sa", "syndra-service", claimtel.FnPreAccessToken, t0.Add(-time.Second), t0.Add(time.Second)) {
		t.Fatal("a no_grants call counted as served")
	}
	if RemoteServed(ctx, r, "sa", "syndra-service", claimtel.FnPreUserinfo, t0, t0.Add(10*time.Second)) {
		t.Fatal("another function's call counted")
	}
}
