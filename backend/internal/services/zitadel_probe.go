package services

import (
	"context"
	"sync"
	"time"

	"syndra/internal/zitadel"
)

// Whether Zitadel is actually answering, for the governance indicators.
//
// This used to be `zitadel.MgmtClient != nil`, which is a question about
// configuration wearing the words of a question about the network. The
// indicator drives a banner reading "Zitadel is not answering" and gates the
// Send button on Pending changes — so during a real outage, with the client
// configured, it stayed green and Send stayed enabled, and the one screen whose
// job is to say whether the queue can move said it could.
//
// The propagation drain already had the honest version (`propagation/deps.go`):
// a limit-1 grant list, cheap enough to double as a probe. The difference is
// that a drain runs when an operator asks, and indicators are polled by every
// open dashboard. So the same call is memoised for `probeTTL`: long enough that
// a room full of tabs costs one request, short enough that an operator watching
// the banner after a restart is not reading a stale answer for long.
//
// A failure is NOT cached for the full TTL. Recovery is the moment somebody is
// waiting on, and making them wait out a window measured for the healthy case
// is how a fixed system goes on looking broken.
const (
	probeTTL       = 30 * time.Second
	probeFailedTTL = 5 * time.Second
)

var probe struct {
	sync.Mutex
	at        time.Time
	reachable bool
	valid     bool
}

// zitadelAnswering asks Zitadel a real question, at most once per TTL.
func zitadelAnswering(ctx context.Context) bool {
	// Local-policy-only mode: no client to ask, and no amount of waiting will
	// produce one. Answer immediately and never spend a probe on it.
	if zitadel.MgmtClient == nil {
		return false
	}

	probe.Lock()
	defer probe.Unlock()

	ttl := probeTTL
	if probe.valid && !probe.reachable {
		ttl = probeFailedTTL
	}
	if probe.valid && time.Since(probe.at) < ttl {
		return probe.reachable
	}

	_, err := zitadel.MgmtClient.ListAllGrants(ctx, zitadel.SearchParams{Limit: 1})
	probe.at, probe.reachable, probe.valid = time.Now(), err == nil, true
	return probe.reachable
}

// resetZitadelProbe drops the memo. Tests only — a cached answer from one case
// must never decide another.
func resetZitadelProbe() {
	probe.Lock()
	defer probe.Unlock()
	probe.valid = false
}
