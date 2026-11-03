package handlers

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"runtime/debug"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"syndra/internal/claimserve"
	"syndra/internal/claimtel"
	"syndra/internal/db"
	"syndra/internal/services/periodic"
)

// claimDelivery counts every Actions v2 call (claims-delivery-integrity S1).
var claimDelivery = claimtel.New(time.Now)

// recordClaimCall is where a finished call goes. Tests swap it to see the
// outcome a request came to.
var recordClaimCall = claimDelivery.Record

// withClaimDelivery wraps the WHOLE data-plane route, signature check
// included, so every call is counted exactly once whatever stops it: a
// refused signature, an undecodable body, a degraded answer, or a panic. A
// panic is answered 500 here and counted as one; Zitadel issues the token
// without Syndra's claims either way, and the count is what makes it loud.
func withClaimDelivery(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		call := &claimtel.Call{Outcome: claimtel.BadRequest, At: time.Now()}
		defer func() {
			if rec := recover(); rec != nil {
				call.Outcome = claimtel.Panic
				log.Printf("[PANIC] data plane %s %s: %v\n%s", r.Method, r.URL.Path, rec, debug.Stack())
				jsonErrorResponse(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Something went wrong handling that request.")
			}
			call.Elapsed = time.Since(call.At)
			recordClaimCall(*call)
		}()
		next(w, r.WithContext(claimtel.With(r.Context(), call)))
	}
}

// FlushClaimDelivery folds the counters into Postgres. Run every minute; a
// failed flush keeps them for the next one.
func FlushClaimDelivery(ctx context.Context) error {
	return claimDelivery.Flush(ctx, dbWriteClaimDelivery)
}

// ---------------------------------------------------------------------------
// Repair on miss (F-7). A token whose facts are missing is the first sign
// that a person was never compiled (a new member, a grant made only in
// Zitadel's console). The miss answers degraded at once, and the person is
// recompiled off the hot path so the next token finds their facts.
//
// Deduplicated (one pending entry per person), bounded (a full queue drops,
// counted) and rate-limited (one worker, at most 20 compiles a second), so a
// term-start burst of misses cannot become a compile storm on the database
// and directory that every sign-in also needs.
// ---------------------------------------------------------------------------

const (
	repairQueueSize = 4096
	repairEvery     = 50 * time.Millisecond // 20 a second
	repairTimeout   = 10 * time.Second
)

type repairQueue struct {
	ch      chan string
	pending sync.Map
	queued  atomic.Int64
	dropped atomic.Int64
	done    atomic.Int64
	failed  atomic.Int64
}

var claimRepair = &repairQueue{ch: make(chan string, repairQueueSize)}

// enqueue asks for userID's facts to be recompiled. Never blocks.
func (q *repairQueue) enqueue(userID string) {
	if _, dup := q.pending.LoadOrStore(userID, struct{}{}); dup {
		return
	}
	select {
	case q.ch <- userID:
		q.queued.Add(1)
	default:
		q.pending.Delete(userID)
		q.dropped.Add(1)
	}
}

// RunClaimRepair compiles queued people until ctx ends.
func RunClaimRepair(ctx context.Context) {
	claimRepair.run(ctx, repairEvery)
}

func (q *repairQueue) run(ctx context.Context, every time.Duration) {
	tick := time.NewTicker(every)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case userID := <-q.ch:
			q.repair(ctx, userID)
			select {
			case <-ctx.Done():
				return
			case <-tick.C:
			}
		}
	}
}

func (q *repairQueue) repair(ctx context.Context, userID string) {
	defer q.pending.Delete(userID)
	defer periodic.Recover("CLAIM-REPAIR")
	c, cancel := context.WithTimeout(ctx, repairTimeout)
	defer cancel()
	if err := cacheRefreshUser(c, userID); err != nil {
		q.failed.Add(1)
		log.Printf("[CACHE] repair: recompiling %s after a cache miss failed: %v", userID, err)
		return
	}
	q.done.Add(1)
}

type repairStats struct {
	Queued  int64 `json:"queued"`
	Done    int64 `json:"done"`
	Failed  int64 `json:"failed"`
	Dropped int64 `json:"dropped"`
	Waiting int   `json:"waiting"`
}

func (q *repairQueue) stats() repairStats {
	return repairStats{Queued: q.queued.Load(), Done: q.done.Load(), Failed: q.failed.Load(), Dropped: q.dropped.Load(), Waiting: len(q.ch)}
}

// ---------------------------------------------------------------------------
// GET /healthz/claims: is claim delivery healthy right now? Unauthenticated
// and minimal, for an external monitor: a status, reasons that name a class
// of problem, and ages. No ids, no names, nothing about a person.
// ---------------------------------------------------------------------------

// healthWindowMinutes is how far back an outcome counts as "now".
const healthWindowMinutes = 10

// warmStaleAfter is past one missed 12h warm.
const warmStaleAfter = 13 * time.Hour

type claimsHealth struct {
	Status          string           `json:"status"` // ok | degraded | failing
	Reasons         []string         `json:"reasons"`
	Calls10m        int64            `json:"calls_10m"`
	Outcomes10m     map[string]int64 `json:"outcomes_10m"`
	LastServedAgeS  *int64           `json:"last_served_age_s"`
	Redis           string           `json:"redis"`
	WarmAgeS        *int64           `json:"warm_age_s"`
	WarmRefreshed   *int             `json:"warm_refreshed,omitempty"`
	WarmOf          *int             `json:"warm_of,omitempty"`
	WarmFailed      *int             `json:"warm_failed,omitempty"`
	TelemetryFailed int              `json:"telemetry_flush_failures"`
	BackgroundPanic int64            `json:"background_panics"`
	// Snapshot is the data plane's config: its generation, how long since
	// Redis last confirmed it current, and the last-known-good copy's size.
	Snapshot claimserve.Status `json:"snapshot"`
}

// snapshotStaleAfter is ten missed polls: the snapshot still answers, but an
// edit made since may not have reached it.
const snapshotStaleAfter = 10 * time.Second

func ageS(now, t time.Time) *int64 {
	if t.IsZero() {
		return nil
	}
	s := int64(now.Sub(t).Seconds())
	return &s
}

func handleClaimsHealth(w http.ResponseWriter, r *http.Request) {
	now := time.Now()
	h := claimsHealth{Reasons: []string{}, Outcomes10m: map[string]int64{}, Redis: "ok"}
	hard, soft := []string{}, []string{}

	for o, n := range claimDelivery.Recent(healthWindowMinutes) {
		h.Calls10m += n
		h.Outcomes10m[string(o)] = n
		switch o.Severity() {
		case claimtel.Hard:
			hard = append(hard, "OUTCOME:"+string(o))
		case claimtel.Soft:
			soft = append(soft, "OUTCOME:"+string(o))
		}
	}
	h.LastServedAgeS = ageS(now, claimDelivery.LastServed())

	pingCtx, cancel := context.WithTimeout(r.Context(), 100*time.Millisecond)
	if err := redisPing(pingCtx); err != nil {
		h.Redis = "down"
		hard = append(hard, "REDIS_DOWN")
	}
	cancel()

	if wr, ok := cacheLastWarm(); ok {
		h.WarmAgeS = ageS(now, wr.At)
		h.WarmRefreshed, h.WarmOf, h.WarmFailed = &wr.Refreshed, &wr.Of, &wr.Failed
		switch {
		case wr.Skipped:
			soft = append(soft, "WARM_SKIPPED")
		case wr.Failed > 0:
			soft = append(soft, "WARM_FAILURES")
		}
		if now.Sub(wr.At) > warmStaleAfter {
			soft = append(soft, "WARM_STALE")
		}
	}
	if st := claimDelivery.Status(); st.Failures > 0 {
		h.TelemetryFailed = st.Failures
		soft = append(soft, "TELEMETRY_FLUSH_FAILED")
	}
	if h.BackgroundPanic = backgroundPanics(); h.BackgroundPanic > 0 {
		soft = append(soft, "BACKGROUND_PANIC")
	}
	h.Snapshot = claimPlane.Status()
	switch {
	case !h.Snapshot.Loaded:
		hard = append(hard, "SNAPSHOT_NOT_LOADED") // every call answers fail closed
	case h.Snapshot.Projects == 0:
		soft = append(soft, "SNAPSHOT_EMPTY")
	}
	if c := h.Snapshot.CheckedAgo; h.Snapshot.Loaded && (c == nil || time.Duration(*c)*time.Second > snapshotStaleAfter) {
		soft = append(soft, "SNAPSHOT_STALE")
	}

	sort.Strings(hard)
	sort.Strings(soft)
	h.Reasons = append(hard, soft...)
	status := http.StatusOK
	switch {
	case len(hard) > 0:
		h.Status, status = "failing", http.StatusServiceUnavailable
	case len(soft) > 0:
		h.Status = "degraded"
	default:
		h.Status = "ok"
	}
	w.Header().Set("Cache-Control", "no-store")
	jsonResponse(w, status, h)
}

// ---------------------------------------------------------------------------
// GET /api/v1/claims/delivery?window=24h (operator): every call by outcome
// class and app, each app's last delivery, the payload shapes Zitadel has
// sent, and the telemetry's own health.
// ---------------------------------------------------------------------------

const maxDeliveryWindow = 90 * 24 * time.Hour

// parseDeliveryWindow reads "30m", "24h" or "7d"; empty is 24h.
func parseDeliveryWindow(s string) (time.Duration, error) {
	if s == "" {
		return 24 * time.Hour, nil
	}
	var d time.Duration
	if days, ok := strings.CutSuffix(s, "d"); ok {
		n, err := strconv.Atoi(days)
		if err != nil {
			return 0, err
		}
		d = time.Duration(n) * 24 * time.Hour
	} else {
		var err error
		if d, err = time.ParseDuration(s); err != nil {
			return 0, err
		}
	}
	if d <= 0 || d > maxDeliveryWindow {
		return 0, fmt.Errorf("window must be between 1m and 90d")
	}
	return d, nil
}

type deliveryRow struct {
	db.ClaimDeliveryRow
	P50MS *float64 `json:"p50_le_ms"`
	P99MS *float64 `json:"p99_le_ms"`
}

type deliveryFinding struct {
	Kind      string    `json:"kind"`
	Severity  string    `json:"severity"`
	Subject   string    `json:"subject"`
	FirstSeen time.Time `json:"first_seen"`
}

func handleClaimDelivery(w http.ResponseWriter, r *http.Request) {
	window, err := parseDeliveryWindow(r.URL.Query().Get("window"))
	if err != nil {
		jsonValidationErrorResponse(w, "Invalid window", map[string]string{"window": err.Error()})
		return
	}
	since := time.Now().Add(-window)
	ctx := r.Context()

	rows, err := dbClaimDeliveryWindow(ctx, since)
	if err != nil {
		jsonErrorResponse(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Could not read claim delivery.")
		return
	}
	last, err := dbClaimDeliveryLast(ctx)
	if err != nil {
		jsonErrorResponse(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Could not read claim delivery.")
		return
	}
	fps, err := dbClaimPayloadFingerprints(ctx)
	if err != nil {
		jsonErrorResponse(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Could not read claim delivery.")
		return
	}

	totals := map[string]int64{}
	for _, o := range claimtel.Outcomes {
		totals[string(o)] = 0
	}
	out := make([]deliveryRow, 0, len(rows))
	for _, row := range rows {
		totals[string(row.Outcome)] += row.N
		out = append(out, deliveryRow{
			ClaimDeliveryRow: row,
			P50MS:            claimtel.UpperBoundMS(row.Hist, 0.50),
			P99MS:            claimtel.UpperBoundMS(row.Hist, 0.99),
		})
	}
	findings := []deliveryFinding{}
	for _, f := range fps {
		if f.Changed && !f.FirstSeen.Before(since) {
			findings = append(findings, deliveryFinding{
				Kind: "PAYLOAD_SHAPE_CHANGED", Severity: "info", Subject: f.Function + " " + f.FP, FirstSeen: f.FirstSeen,
			})
		}
	}

	jsonResponse(w, http.StatusOK, map[string]any{
		"window":       window.String(),
		"since":        since.UTC(),
		"totals":       totals,
		"rows":         out,
		"last":         last,
		"fingerprints": fps,
		"findings":     findings,
		"telemetry": map[string]any{
			"flush":        claimDelivery.Status(),
			"active_pairs": claimDelivery.ActivePairs(),
			"repair":       claimRepair.stats(),
		},
	})
}
