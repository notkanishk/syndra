package services

import (
	"context"
	"errors"
	"testing"

	"syndra/internal/zitadel"
)

// A client that is configured, and answers or does not on demand.
//
// It embeds the package's existing stub so this file only has to say the one
// thing it is about: what happens when Zitadel is *there* and *silent*. That
// combination is the whole bug — the old check was `MgmtClient != nil`, which
// is true throughout an outage.
type probeClient struct {
	failingRoleClient
	err   error
	calls int
}

func (p *probeClient) ListAllGrants(
	_ context.Context, _ zitadel.SearchParams,
) (*zitadel.SearchResult[zitadel.UserGrant], error) {
	p.calls++
	if p.err != nil {
		return nil, p.err
	}
	return &zitadel.SearchResult[zitadel.UserGrant]{}, nil
}

func withClient(t *testing.T, c zitadel.ZitadelClient) {
	t.Helper()
	previous := zitadel.MgmtClient
	zitadel.MgmtClient = c
	resetZitadelProbe()
	t.Cleanup(func() {
		zitadel.MgmtClient = previous
		resetZitadelProbe()
	})
}

func TestProbeSaysUnreachableWhenNoClientIsConfigured(t *testing.T) {
	withClient(t, nil)
	if zitadelAnswering(context.Background()) {
		t.Fatal("local-policy-only mode has nothing to ask; it must report unreachable")
	}
}

// The case the old implementation could not express.
func TestProbeSaysUnreachableWhenConfiguredButSilent(t *testing.T) {
	client := &probeClient{err: errors.New("dial tcp: i/o timeout")}
	withClient(t, client)

	if zitadelAnswering(context.Background()) {
		t.Fatal("a configured client that does not answer is not reachable — this is the outage the banner exists for")
	}
	if client.calls == 0 {
		t.Fatal("nothing was asked; the probe has to make a real call or it is the old check again")
	}
}

func TestProbeSaysReachableWhenZitadelAnswers(t *testing.T) {
	withClient(t, &probeClient{})
	if !zitadelAnswering(context.Background()) {
		t.Fatal("a client that answers is reachable")
	}
}

func TestProbeAsksOnceForABurstOfDashboards(t *testing.T) {
	client := &probeClient{}
	withClient(t, client)

	for range 20 {
		zitadelAnswering(context.Background())
	}
	if client.calls != 1 {
		t.Fatalf("indicators are polled by every open dashboard; want 1 call within the TTL, got %d", client.calls)
	}
}

// Recovery is what somebody is waiting for. Caching a failure for as long as a
// success would leave a fixed system looking broken for the full window.
func TestFailureIsCachedForLessTimeThanSuccess(t *testing.T) {
	if probeFailedTTL >= probeTTL {
		t.Fatalf("a failed probe must expire sooner than a good one: failed=%s, ok=%s", probeFailedTTL, probeTTL)
	}
}

// The seam itself, not just the function behind it.
//
// Every test above calls `zitadelAnswering` directly, so all of them still
// pass if `svcZitadelReachable` is quietly pointed back at `MgmtClient != nil`
// — which is how the weak check survived in the first place. This asserts
// through the variable the indicators actually read.
func TestTheIndicatorSeamUsesTheRealProbe(t *testing.T) {
	withClient(t, &probeClient{err: errors.New("connection refused")})

	if svcZitadelReachable(context.Background()) {
		t.Fatal("svcZitadelReachable reported a silent Zitadel as reachable — it is wired to a configuration check, not a probe")
	}
}
