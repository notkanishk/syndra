package reference

import (
	"errors"
	"math"
	"time"
)

// Fresh reads (feature freshness, design §4; guide chapter 4). /subjects is
// served from a mirror unless Syndra asks for a read of the app now with
// ?fresh=true, the contract's form of a request `Cache-Control: no-cache`
// (RFC 9111 §5.2.1.4): one boolean, since max-age=0 is the only value Syndra
// would send. The add-on bounds what a fresh read costs its app: concurrent
// reads collapse into one (request collapsing, as golang.org/x/sync/singleflight
// does it, with a channel instead of a dependency), and a fresh read inside
// the cooldown answers the last value as `cooldown` with retry_after, or 429
// FRESH_READ_COOLDOWN with Retry-After (RFC 6585 §4, RFC 9110 §10.2.3) when
// nothing is kept.

const (
	// freshCooldown is limits.fresh_read_cooldown (PT1S), from when the last
	// read of the app began.
	freshCooldown = time.Second
	// readDwell stands in for an app's latency, so that two requests sent
	// together really overlap one read.
	readDwell = 150 * time.Millisecond
)

// freshState is the mirror and the read in flight, under Addon.mu.
// ponytail: the mirror is kept until a write forgets it; an app that changes
// on its own needs a max age too (the TrueNAS add-on keeps five minutes).
type freshState struct {
	list   []subject
	at     string // taken_at of the read that filled the mirror
	kept   bool
	failed bool          // the last read failed, while it is the last read: the cooldown answers it as it was
	began  time.Time     // when the last read of the app began
	flight chan struct{} // closed when the running read ends; nil when none runs
	gen    int           // moved by every write, so a read begun before one never refills the mirror after it
	reads  int           // health.target_reads: never decreases while the add-on runs
}

var errDown = errors.New("the app cannot be read")

// readApp is one read of the app, counted in target_reads.
func (a *Addon) readApp() ([]subject, string, error) {
	a.mu.Lock()
	a.fresh.reads++
	list, at := a.list(), now()
	a.mu.Unlock()
	time.Sleep(readDwell)
	if a.cfg.Down != nil && a.cfg.Down() {
		return nil, "", errDown
	}
	return list, at, nil
}

// forget drops the mirror, the cooldown (and with it the failure it answers, which is
// only read inside one) and the read in flight; every write
// calls it (record), under a.mu. A read arriving after the write then reads
// the app, never joins or waits out a read begun before it (the TrueNAS
// add-on's fresher.forget does the same).
func (a *Addon) forget() {
	a.fresh.kept = false
	a.fresh.list, a.fresh.at = nil, ""
	a.fresh.began, a.fresh.flight = time.Time{}, nil
	a.fresh.gen++
}

// keep fills the mirror from a read that began at gen, unless a write came
// since.
func (a *Addon) keep(list []subject, at string, gen int) {
	if gen == a.fresh.gen {
		a.fresh.list, a.fresh.at, a.fresh.kept = list, at, true
	}
}

// freshSubjects answers /subjects through the mirror.
func (a *Addon) freshSubjects(fresh bool) reply {
	a.mu.Lock()
	defer a.mu.Unlock()
	f := &a.fresh
	for {
		since := time.Since(f.began)
		switch {
		case !fresh && f.kept:
			return a.mirrored("cached")
		case f.flight != nil:
			// Collapsed into the read already running. It began before this
			// request arrived, so its answer is not current (guide: current
			// means a read begun after the request), and says why.
			fl := f.flight
			a.mu.Unlock()
			<-fl
			a.mu.Lock()
			if f.kept {
				if fresh {
					return a.mirrored("cooldown")
				}
				return a.mirrored("cached")
			}
			continue // the read failed, or a write forgot it: decide again
		case fresh && since < freshCooldown:
			if f.failed {
				// The last read failed: answered as it was, the copy unreachable or
				// the error, not as a read moments ago. Still no second attempt
				// (the TrueNAS add-on's fresher does the same).
				return a.unreached()
			}
			if f.kept {
				return a.mirrored("cooldown")
			}
			rep := problem(429, "FRESH_READ_COOLDOWN", "The app was read under a second ago and nothing is kept to answer from.", nil)
			rep.retryAfter = retryAfter(since)
			return rep
		}
		fl, gen := make(chan struct{}), f.gen
		f.flight, f.began = fl, time.Now()
		a.mu.Unlock()
		list, at, err := a.readApp()
		a.mu.Lock()
		if f.flight == fl { // a write may have forgotten it already
			f.flight = nil
		}
		close(fl)
		if err != nil {
			if gen == f.gen {
				f.failed = true
			}
			return a.unreached()
		}
		if gen == f.gen {
			f.failed = false
		}
		a.keep(list, at, gen)
		return ok(map[string]any{"subjects": list, "current": true, "taken_at": at, "truncated": false})
	}
}

// unreached answers a read that failed: the mirror, unreachable, or the error
// when nothing is kept; under a.mu.
func (a *Addon) unreached() reply {
	if a.fresh.kept {
		return a.mirrored("unreachable")
	}
	return problem(503, "TARGET_UNREACHABLE", "The app could not be read and nothing is kept to answer from.", nil)
}

// mirrored is the mirror's value, not current, with why; under a.mu.
func (a *Addon) mirrored(reason string) reply {
	stale := map[string]any{"reason": reason}
	if reason == "cooldown" {
		stale["retry_after"] = retryAfter(time.Since(a.fresh.began))
	}
	return ok(map[string]any{"subjects": a.fresh.list, "current": false, "stale": stale, "taken_at": a.fresh.at, "truncated": false})
}

// retryAfter is the whole seconds left of the cooldown, rounded up so a
// caller that waits it is never early.
func retryAfter(since time.Duration) int {
	return max(0, int(math.Ceil((freshCooldown - since).Seconds())))
}
