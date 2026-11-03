package claimserve

import (
	"net/http"
	"sort"
	"time"

	"syndra/internal/claimtel"
)

// Health is the data plane's half of /healthz/claims: what both doors know
// from memory and one Redis ping. Unauthenticated and minimal: a status,
// reasons that name a class of problem, and ages; no ids, nothing about a
// person. The backend's answer adds the detectors' half.
type Health struct {
	Status         string           `json:"status"` // ok | degraded | failing
	Reasons        []string         `json:"reasons"`
	Calls10m       int64            `json:"calls_10m"`
	Outcomes10m    map[string]int64 `json:"outcomes_10m"`
	LastServedAgeS *int64           `json:"last_served_age_s"`
	Redis          string           `json:"redis"`
	// Snapshot is the data plane's config: its generation, how long since
	// Redis last confirmed it current, and the last-known-good copy's size.
	Snapshot Status `json:"snapshot"`
}

const (
	// HealthWindowMinutes is how far back an outcome counts as "now".
	HealthWindowMinutes = 10
	// SnapshotStaleAfter is ten missed polls: the snapshot still answers, but
	// an edit made since may not have reached it.
	SnapshotStaleAfter = 10 * time.Second
)

// AgeS is seconds since t, nil for the zero time.
func AgeS(now, t time.Time) *int64 {
	if t.IsZero() {
		return nil
	}
	s := int64(now.Sub(t).Seconds())
	return &s
}

// Assess is the data plane's health and its reasons. Hard: a hard outcome in
// the window, Redis down, no snapshot. Soft: a soft outcome, an empty or
// unconfirmed snapshot.
func Assess(now time.Time, recent map[claimtel.Outcome]int64, lastServed time.Time, redisErr error, snap Status) (h Health, hard, soft []string) {
	h = Health{Reasons: []string{}, Outcomes10m: map[string]int64{}, Redis: "ok", Snapshot: snap, LastServedAgeS: AgeS(now, lastServed)}
	for o, n := range recent {
		if n == 0 {
			continue
		}
		h.Calls10m += n
		h.Outcomes10m[string(o)] += n
		switch o.Severity() {
		case claimtel.Hard:
			hard = append(hard, "OUTCOME:"+string(o))
		case claimtel.Soft:
			soft = append(soft, "OUTCOME:"+string(o))
		}
	}
	if redisErr != nil {
		h.Redis = "down"
		hard = append(hard, "REDIS_DOWN")
	}
	switch {
	case !snap.Loaded:
		hard = append(hard, "SNAPSHOT_NOT_LOADED") // every call answers fail closed
	case snap.Projects == 0:
		soft = append(soft, "SNAPSHOT_EMPTY")
	}
	if c := snap.CheckedAgo; snap.Loaded && (c == nil || time.Duration(*c)*time.Second > SnapshotStaleAfter) {
		soft = append(soft, "SNAPSHOT_STALE")
	}
	return h, hard, soft
}

// Verdict sets the status and reasons from hard and soft (each sorted and
// deduplicated, hard first) and returns the HTTP status: 503 when failing.
func (h *Health) Verdict(hard, soft []string) int {
	hard, soft = uniqSorted(hard), uniqSorted(soft)
	h.Reasons = append(append([]string{}, hard...), soft...)
	switch {
	case len(hard) > 0:
		h.Status = "failing"
		return http.StatusServiceUnavailable
	case len(soft) > 0:
		h.Status = "degraded"
	default:
		h.Status = "ok"
	}
	return http.StatusOK
}

func uniqSorted(in []string) []string {
	sort.Strings(in)
	out := in[:0]
	for i, s := range in {
		if i == 0 || s != in[i-1] {
			out = append(out, s)
		}
	}
	return out
}

// MergeRecent adds b's counts into a.
func MergeRecent(a, b map[claimtel.Outcome]int64) map[claimtel.Outcome]int64 {
	out := make(map[claimtel.Outcome]int64, len(a)+len(b))
	for o, n := range a {
		out[o] += n
	}
	for o, n := range b {
		out[o] += n
	}
	return out
}
