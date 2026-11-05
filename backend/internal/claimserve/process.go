package claimserve

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/redis/go-redis/v9"

	"syndra/internal/claimtel"
	"syndra/internal/services/periodic"
)

// NewRedisClient is the one Redis constructor (F-1): context deadlines
// honoured, and short dial, read, write and pool timeouts, so a hung Redis
// costs a call its deadline, never Zitadel's 3s.
func NewRedisClient(addr string) *redis.Client {
	return redis.NewClient(&redis.Options{
		Addr:                  addr,
		DB:                    0,
		ContextTimeoutEnabled: true,
		DialTimeout:           200 * time.Millisecond,
		ReadTimeout:           200 * time.Millisecond,
		WriteTimeout:          200 * time.Millisecond,
		PoolTimeout:           100 * time.Millisecond,
	})
}

// Process is the claims container (cmd/claims, assessment P4): the door, the
// snapshot, its own recorder, and the relay to the backend. No Postgres, no
// Zitadel, nothing of the admin side: a backend deploy or crash cannot touch
// it.
type Process struct {
	Plane *Plane
	Rec   *claimtel.Recorder
	Out   *Outbox
	rdb   RelayRedis
	door  *Door
}

// NewProcess wires the claims process over rdb. envKey is the bootstrap and
// fallback signing key; devMode lets unsigned calls through when no key is
// configured at all.
func NewProcess(rdb RelayRedis, envKey string, devMode bool) *Process {
	p := &Process{Plane: New(rdb, nil), Rec: claimtel.New(time.Now), Out: NewOutbox(), rdb: rdb}
	p.door = &Door{
		Plane:   func() *Plane { return p.Plane },
		Keys:    func() []SlotKey { return KeysWithEnv(p.Plane.SigningKeys(), envKey) },
		Refresh: func(ctx context.Context) []SlotKey { return KeysWithEnv(p.Plane.RefreshKeys(ctx), envKey) },
		DevMode: func() bool { return devMode },
		Record: func(c claimtel.Call) {
			p.Rec.Record(c)
			p.Out.Call(c)
		},
		OnMiss:     func(u string) { p.Out.Signal(Signal{Kind: "repair", UserID: u}) },
		OnMismatch: func() { p.Out.Signal(Signal{Kind: "nudge"}) },
		Debug:      processDebug{p},
	}
	return p
}

// Router is the claims container's whole surface.
func (p *Process) Router() http.Handler { return Router(p.door.Handler(), p.health) }

// Run keeps the snapshot current and relays to the backend until ctx ends.
// A panic in either loop is recovered, counted and the loop restarted.
func (p *Process) Run(ctx context.Context) {
	_ = p.Plane.Poll(ctx)
	go forever(ctx, "CLAIMS-SNAPSHOT", p.Plane.Run)
	forever(ctx, "CLAIMS-RELAY", func(ctx context.Context) {
		t := time.NewTicker(RelayEvery)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				p.Flush(ctx)
			}
		}
	})
}

// Flush relays once; a failure is logged once per outage and retried.
func (p *Process) Flush(ctx context.Context) {
	fctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := p.Out.Flush(fctx, p.rdb, p.Rec, time.Now()); err != nil && p.Out.Failures() == 1 {
		log.Printf("[DATA PLANE] relay to the backend failed; kept for the next flush: %v", err)
	}
}

func forever(ctx context.Context, name string, run func(context.Context)) {
	for ctx.Err() == nil {
		func() {
			defer periodic.Recover(name)
			run(ctx)
		}()
	}
}

// health is the claims container's /healthz/claims: the data plane's half,
// plus the relay. The backend's /healthz/claims on :8080 is the whole
// picture (detectors, probes, warm); this one says whether this door answers.
func (p *Process) health(w http.ResponseWriter, r *http.Request) {
	now := time.Now()
	pctx, cancel := context.WithTimeout(r.Context(), 100*time.Millisecond)
	perr := p.rdb.Ping(pctx).Err()
	cancel()
	h, hard, soft := Assess(now, p.Rec.Recent(HealthWindowMinutes), p.Rec.LastServed(), perr, p.Plane.Status())
	if p.Out.Failures() > 0 || p.Rec.Status().Failures > 0 {
		soft = append(soft, "RELAY_FAILING")
	}
	if p.Out.Dropped() > 0 {
		soft = append(soft, "RELAY_DROPPED")
	}
	if n := periodic.Panics(); n > 0 {
		soft = append(soft, "BACKGROUND_PANIC")
	}
	code := h.Verdict(hard, soft)
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, code, h)
}

// processDebug is debug mode in the claims process: the watch set comes with
// the snapshot, and a capture goes to the backend, which explains and keeps it.
type processDebug struct{ p *Process }

func (d processDebug) Wants(clientID, userID string) bool {
	cfg := d.p.Plane.Config()
	if cfg == nil || len(cfg.Debug) == 0 {
		return false
	}
	w, ok := cfg.Debug[clientID]
	return ok && w.UserID == userID && time.Now().Unix() < w.Ends
}

func (d processDebug) Capture(c DebugCapture) { d.p.Out.Signal(Signal{Kind: "debug", Capture: &c}) }
