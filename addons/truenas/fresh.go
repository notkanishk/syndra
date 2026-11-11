package main

import (
	"errors"
	"fmt"
	"math"
	"net/http"
	"strconv"
	"sync"
	"time"
)

// Fresh reads (feature freshness, design §4, guide "Fresh reads").
//
// Every read in scope (/subjects, /reach, /homes, and the effect: read
// operations whose result says how fresh it is) goes through one fresher, so
// the rules live in one place:
//
//   - without `fresh`, a copy younger than freshMaxAge answers, labelled
//     `cached` (RFC 9111 §4.2: served while fresh, never called current);
//   - with `fresh` (RFC 9111 §5.2.1.4 `no-cache`: read the app now), the NAS
//     is read, unless a read of the same thing began inside the cooldown:
//     then the copy answers `cooldown` with `retry_after`, or, with nothing
//     at all to serve, 429 FRESH_READ_COOLDOWN with Retry-After (RFC 6585 §4,
//     RFC 9110 §10.2.3);
//   - concurrent reads of one thing make one read of the NAS (request
//     collapsing, golang.org/x/sync/singleflight's shape, a few lines here
//     rather than a dependency); a joiner arrived after the read began, so its
//     answer is never current (B1);
//   - a NAS that cannot be reached answers the copy, `unreachable` (RFC 5861
//     stale-if-error), never an error while there is a copy; a fresh read
//     inside the cooldown of a read that failed answers the same, never
//     "read moments ago", and never 429 when there is a copy or a mirror;
//   - a refusal (an operation's 4xx) drops the copy: what it said is no longer
//     what the NAS holds;
//   - every write forgets every copy, cooldown and read under way
//     (server.mutate), so the first read after a write is a read of the NAS
//     begun after it: a member waiting on their own password sees it land on
//     the next poll.
//
// ponytail: the cooldown is per key (one route, or one route and subject);
// add a target-wide token bucket if many members' "Read again" ever loads the
// NAS.

const (
	// freshMaxAge is how long a copy answers a read that did not ask for a
	// fresh one. Folder permissions and accounts change by hand, rarely, and a
	// dozen ACL reads per page view would put the NAS's latency in front of
	// every member.
	freshMaxAge = 5 * time.Minute
	// freshCooldown is limits.fresh_read_cooldown (constants.cooldown_default,
	// PT30S).
	freshCooldown = 30 * time.Second
)

// Observed is when a read reply was taken from the NAS and how far to trust
// it: the three keys every reply in scope carries (design §4).
type Observed struct {
	TakenAt string `json:"taken_at"`
	Current bool   `json:"current"`
	// Stale says why, present exactly when Current is false.
	Stale *Stale `json:"stale,omitempty"`
}

// least is the less fresh of o and p, a reply made of both: current only when
// both are, else the one not current, the older when neither is. Each keeps
// the invariant (stale exactly when not current), so the answer does. A p
// never read (no taken_at) says nothing.
// ponytail: taken_at compared as text, which orders while both are rfc3339's
// UTC seconds.
func (o Observed) least(p Observed) Observed {
	if p.TakenAt == "" || p.Current || (!o.Current && o.TakenAt <= p.TakenAt) {
		return o
	}
	return p
}

// with is the freshness of a reply o made with a part p read for it
// (storage.status and the folder permissions behind its places). Read just
// now, the reply is no fresher than its part (least). Answered from a copy,
// it says what the copy is, its own reason, never the part's: the part's
// label was true when the copy was made, and a copy replaying it would say
// "cooldown, read moments ago" to every poll for five minutes, or during an
// outage. It is only dated no later than its oldest part.
func (o Observed) with(p Observed) Observed {
	if o.Current {
		return o.least(p)
	}
	if p.TakenAt != "" && p.TakenAt < o.TakenAt {
		o.TakenAt = p.TakenAt
	}
	return o
}

// errCooldown is a fresh read inside the cooldown with nothing at all to
// serve: no copy and no failure. A finished read leaves one or the other, so
// it answers only a read begun and gone without either, never an outage.
type errCooldown struct{ retryAfter int }

func (e errCooldown) Error() string {
	return fmt.Sprintf("a fresh read was asked for inside the cooldown; try again in %d s", e.retryAfter)
}

// opFailure is a read operation that failed, with the status it is answered
// with.
type opFailure struct {
	status int
	err    error
}

func (e opFailure) Error() string { return e.err.Error() }
func (e opFailure) Unwrap() error { return e.err }

type keptRead struct {
	val any
	at  time.Time
}

type flight struct {
	done chan struct{}
	val  any
	at   time.Time
	err  error
}

// fresher keeps the last read of each thing. Its zero value is ready.
type fresher struct {
	mu sync.Mutex
	// now is the clock; nil is time.Now. A seam for the cooldown tests.
	now func() time.Time
	// cooldown is the cooldown; zero is freshCooldown. A seam for the kit's
	// run, which waits it out between its checks.
	cooldown time.Duration
	// gen moves on every forget, so a read that began before a write never
	// becomes the copy after it.
	gen     uint64
	kept    map[string]keptRead
	began   map[string]time.Time
	flights map[string]*flight
	// failed is the last read's error, while it is the last read: what a
	// fresh read inside its cooldown answers with.
	failed map[string]error
}

func (f *fresher) cooldownOr() time.Duration {
	if f.cooldown > 0 {
		return f.cooldown
	}
	return freshCooldown
}

// cooldownISO is the cooldown as the manifest declares it (ISO 8601).
func (f *fresher) cooldownISO() string {
	return "PT" + strconv.Itoa(int(math.Ceil(f.cooldownOr().Seconds()))) + "S"
}

func (f *fresher) clock() time.Time {
	if f.now != nil {
		return f.now()
	}
	return time.Now()
}

// read answers key from the copy, from a read of the NAS now (live), or from
// a read already under way. live returns the value and the instant of the read.
func (f *fresher) read(key string, fresh bool, live func() (any, time.Time, error)) (any, Observed, error) {
	f.mu.Lock()
	if f.kept == nil {
		f.kept, f.began, f.flights, f.failed = map[string]keptRead{}, map[string]time.Time{}, map[string]*flight{}, map[string]error{}
	}
	now := f.clock()
	k, kept := f.kept[key]
	wait := f.waitLocked(key, now)
	switch fl := f.flights[key]; {
	case !fresh && kept && now.Sub(k.at) < freshMaxAge:
		f.mu.Unlock()
		return k.val, staleAt(k.at, "cached", wait), nil
	case fl != nil:
		f.mu.Unlock()
		<-fl.done
		if fl.err == nil {
			reason := "cached"
			if fresh {
				reason = "cooldown"
			}
			return fl.val, staleAt(fl.at, reason, f.wait(key)), nil
		}
		return f.fallback(key, fl.err)
	case fresh && wait > 0:
		failed := f.failed[key]
		f.mu.Unlock()
		if failed != nil {
			// The last read failed: answered as it was, the copy unreachable or
			// the error, not as a read moments ago. Still no second attempt.
			return f.fallback(key, failed)
		}
		if kept {
			return k.val, staleAt(k.at, "cooldown", wait), nil
		}
		return nil, Observed{}, errCooldown{retryAfter: wait}
	}
	fl := &flight{done: make(chan struct{})}
	f.flights[key], f.began[key] = fl, now
	gen := f.gen
	f.mu.Unlock()

	fl.val, fl.at, fl.err = live()

	f.mu.Lock()
	if f.flights[key] == fl { // a write may have forgotten it already
		delete(f.flights, key)
	}
	if gen == f.gen {
		if fl.err == nil {
			f.kept[key] = keptRead{val: fl.val, at: fl.at}
			delete(f.failed, key)
		} else {
			f.failed[key] = fl.err
			if refusal(fl.err) {
				delete(f.kept, key)
			}
		}
	}
	close(fl.done)
	f.mu.Unlock()
	if fl.err != nil {
		return f.fallback(key, fl.err)
	}
	return fl.val, Observed{TakenAt: rfc3339(fl.at), Current: true}, nil
}

// fallback answers a failed read: the copy, labelled unreachable (a slow or
// failing app yields stale, never an error), unless the failure is a decision
// about the request (an operation's 4xx: no bound account) rather than the NAS
// failing to be read, or there is no copy to serve.
func (f *fresher) fallback(key string, err error) (any, Observed, error) {
	if refusal(err) {
		return nil, Observed{}, err
	}
	f.mu.Lock()
	k, kept := f.kept[key]
	f.mu.Unlock()
	if !kept {
		return nil, Observed{}, err
	}
	return k.val, Observed{TakenAt: rfc3339(k.at), Stale: unreachableSince(k.at)}, nil
}

// refusal is a failure that is a decision about the request (an operation's
// 4xx other than 429: no bound account), not the NAS failing to be read.
func refusal(err error) bool {
	of := opFailure{}
	return errors.As(err, &of) && of.status < 500 && of.status != http.StatusTooManyRequests
}

// forget drops every copy, cooldown, failure and read under way: after a
// write, the next read of anything is a read of the NAS that began after it,
// never a join of one that began before (addons/reference/fresh.go's forget).
func (f *fresher) forget() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.gen++
	clear(f.kept)
	clear(f.began)
	clear(f.flights)
	clear(f.failed)
}

func (f *fresher) wait(key string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.waitLocked(key, f.clock())
}

// waitLocked is the whole seconds until a fresh read of key may read the NAS,
// rounded up, so a caller told to wait never arrives early: 0 outside the
// cooldown.
func (f *fresher) waitLocked(key string, now time.Time) int {
	began, ok := f.began[key]
	if !ok {
		return 0
	}
	left := began.Add(f.cooldownOr()).Sub(now)
	if left <= 0 {
		return 0
	}
	return int(math.Ceil(left.Seconds()))
}

func staleAt(at time.Time, reason string, retryAfter int) Observed {
	o := Observed{TakenAt: rfc3339(at), Stale: &Stale{Reason: reason, RetryAfter: retryAfter}}
	switch reason {
	case "cached":
		o.Stale.Note = "The add-on's copy of a read of the NAS at " + rfc3339(at) + "."
	case "cooldown":
		// Never 0: a cooldown answer always says when to ask again.
		o.Stale.RetryAfter = max(1, retryAfter)
		retryAfter = o.Stale.RetryAfter
		o.Stale.Note = "The NAS was read moments ago, at " + rfc3339(at) + "; it can be read again in " +
			strconv.Itoa(retryAfter) + " s."
	}
	return o
}

func rfc3339(at time.Time) string { return at.UTC().Format(time.RFC3339) }

// writeCooldown answers a fresh read inside the cooldown that has nothing to
// serve: 429 FRESH_READ_COOLDOWN, Retry-After the seconds left. False when err
// is not one.
func writeCooldown(w http.ResponseWriter, err error) bool {
	var e errCooldown
	if !errors.As(err, &e) {
		return false
	}
	w.Header().Set("Retry-After", strconv.Itoa(e.retryAfter))
	writeError(w, http.StatusTooManyRequests, CodeFreshReadCooldown, e.Error(), nil)
	return true
}
