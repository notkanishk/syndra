// Package extcall is the one way Syndra calls a system it does not own: a
// timeout per attempt, one classification of what may be tried again, one
// backoff formula, a read-back for a write whose outcome is unknown, and a
// request id on every log line.
//
// The safety rule it exists to hold: nothing is sent twice unless sending it
// twice is harmless. A read may be repeated. A write is repeated only when the
// other side said it did not process it (429). Any other failure of a write —
// no answer, a timeout, a 5xx — is AMBIGUOUS: the write may have landed. Then
// the caller's read-back decides, and without one the failure goes back to the
// caller unretried (the outbox owns that retry, with a live read first).
package extcall

import (
	"context"
	crand "crypto/rand"
	"encoding/hex"
	"errors"
	"io"
	"log"
	"math/rand/v2"
	"net"
	"net/http"
	"strconv"
	"time"
)

// Policy is how one kind of call is retried.
type Policy struct {
	Attempts int           // total tries, at least 1
	Base     time.Duration // the first wait
	Max      time.Duration // no wait is longer than this (required)
	Timeout  time.Duration // per attempt, including reading the body; 0 = none
}

// Backoff is the wait after `spent` failed tries (0 for the first wait):
// min(Max, Base·2^spent), with equal jitter — uniform in [d/2, d]. Half the
// wait is guaranteed, so spacing never collapses; half is random, so callers
// that failed together do not retry together.
func Backoff(p Policy, spent int) time.Duration {
	d := p.Base
	for i := 0; i < spent && d < p.Max; i++ {
		d *= 2
	}
	if p.Max > 0 && d > p.Max {
		d = p.Max
	}
	if d <= 1 {
		return d
	}
	half := d / 2
	return half + time.Duration(rand.Int64N(int64(d-half)+1))
}

// RetryableStatus is the transport classification of an answer: 408, 429 and
// 5xx are worth another try; every other status is the other side's final
// word. 409 is final here — what it MEANS (often "already done") is the
// caller's to decide, not the transport's.
func RetryableStatus(code int) bool {
	return code == http.StatusRequestTimeout || code == http.StatusTooManyRequests || code >= 500
}

// NoAnswer reports whether err is a call that got no answer for a reason
// another try could fix: a timeout or a broken connection. A cancelled context
// is not one — that is the caller giving up.
func NoAnswer(err error) bool {
	if err == nil || errors.Is(err, context.Canceled) {
		return false
	}
	var ne net.Error
	return errors.As(err, &ne) || errors.Is(err, context.DeadlineExceeded) ||
		errors.Is(err, io.ErrUnexpectedEOF) || errors.Is(err, io.EOF)
}

// Call describes one logical call.
type Call struct {
	Name string // for logs, e.g. "POST /management/v1/users/u/grants"
	// Idempotent: sending it twice is harmless (a read). Everything else is a
	// write and is never repeated after an ambiguous failure.
	Idempotent bool
	// ReadBack answers, for a write whose outcome is unknown, whether it
	// landed. nil means the ambiguity goes back to the caller.
	ReadBack func(ctx context.Context) (landed bool, err error)
	// Subsystem is the caller's log tag ("ZITADEL"), so a retry is logged
	// where an operator already greps for that system. Default "SYSTEM".
	Subsystem string
}

func (c Call) tag() string {
	if c.Subsystem == "" {
		return "SYSTEM"
	}
	return c.Subsystem
}

// Op is one attempt. It returns the answer, whatever its status, or the
// transport error.
type Op func(ctx context.Context) (*http.Response, error)

type ridKey struct{}

// RequestID is the id Do gave this call, for the op to send as a header.
func RequestID(ctx context.Context) string {
	s, _ := ctx.Value(ridKey{}).(string)
	return s
}

// sleep is a seam so tests can watch the spacing without waiting it out.
var sleep = func(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

// landed is the answer Do gives when a read-back proved an ambiguous write
// landed. A synthetic 200 with no body: the caller asked for an effect, not
// for the other side's reply, and the effect is there.
func landed() *http.Response {
	return &http.Response{StatusCode: http.StatusOK, Body: http.NoBody, Header: http.Header{"X-Syndra-Read-Back": {"landed"}}}
}

// Do runs op under p. It returns the first answer that is final (any 2xx or
// non-retryable status, for the caller to interpret), or the last failure
// once retrying is no longer allowed: the last answer if there was one,
// otherwise the transport error.
func Do(ctx context.Context, p Policy, c Call, op Op) (*http.Response, error) {
	rid := newID()
	ctx = context.WithValue(ctx, ridKey{}, rid)
	attempts := max(p.Attempts, 1)

	for n := 0; ; n++ {
		resp, err := attempt(ctx, p, op)
		status := 0
		if err == nil {
			status = resp.StatusCode
		}
		if err == nil && !RetryableStatus(status) {
			return resp, nil
		}
		if ctx.Err() != nil { // the caller gave up; its error, not ours
			closeBody(resp)
			return nil, ctx.Err()
		}
		if err != nil && !NoAnswer(err) {
			return nil, err // a failure no retry fixes (bad request, auth)
		}

		// 429 is the only failure that proves the write was not processed.
		ambiguous := !c.Idempotent && status != http.StatusTooManyRequests
		if ambiguous {
			if c.ReadBack == nil {
				log.Printf("[%s] %s req=%s: outcome unknown (%s); not repeating a write", c.tag(), c.Name, rid, outcome(status, err))
				return resp, err
			}
			ok, rerr := c.ReadBack(ctx)
			if rerr == nil && ok {
				closeBody(resp)
				log.Printf("[%s] %s req=%s: outcome unknown (%s); read-back shows it landed", c.tag(), c.Name, rid, outcome(status, err))
				return landed(), nil
			}
			log.Printf("[%s] %s req=%s: outcome unknown (%s); read-back did not show it landed (err=%v), not repeating", c.tag(), c.Name, rid, outcome(status, err), rerr)
			return resp, err
		}

		if n+1 >= attempts {
			log.Printf("[%s] %s req=%s: giving up after %d tries (%s)", c.tag(), c.Name, rid, n+1, outcome(status, err))
			return resp, err
		}
		closeBody(resp)
		wait := Backoff(p, n)
		log.Printf("[%s] %s req=%s: try %d failed (%s); retrying in %s", c.tag(), c.Name, rid, n+1, outcome(status, err), wait)
		if err := sleep(ctx, wait); err != nil {
			return nil, err
		}
	}
}

// attempt runs op under the per-attempt timeout. The timeout's cancel rides
// on the body, so a caller reading the answer is not cut off when Do returns.
func attempt(ctx context.Context, p Policy, op Op) (*http.Response, error) {
	if p.Timeout <= 0 {
		return op(ctx)
	}
	actx, cancel := context.WithTimeout(ctx, p.Timeout)
	resp, err := op(actx)
	if err != nil || resp == nil {
		cancel()
		return resp, err
	}
	resp.Body = cancelOnClose{resp.Body, cancel}
	return resp, nil
}

type cancelOnClose struct {
	io.ReadCloser
	cancel context.CancelFunc
}

func (b cancelOnClose) Close() error { defer b.cancel(); return b.ReadCloser.Close() }

func closeBody(r *http.Response) {
	if r != nil && r.Body != nil {
		r.Body.Close()
	}
}

func outcome(status int, err error) string {
	if err != nil {
		return err.Error()
	}
	return strconv.Itoa(status) + " " + http.StatusText(status)
}

func newID() string {
	var b [8]byte
	_, _ = crand.Read(b[:]) // crypto/rand never fails on supported platforms
	return hex.EncodeToString(b[:])
}
