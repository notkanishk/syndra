package db

import (
	"context"
	"errors"
	"sync"

	"syndra/internal/models"
)

// Never invisibly (owner, 15 Oct): an action that would cancel a change still
// waiting for you to send — a removal given back, a grant removed before it went —
// is refused until the operator has seen exactly which changes it cancels.
// The operator didn't queue them, or doesn't remember why; undoing them is a
// decision, not a side effect.
//
// A gate rides an operator's request. withdrawUndelivered consults it at the
// one place cancellations happen, so no screen can cancel silently, whether
// or not it was built to say so. A request with no gate (expiry, onboarding,
// other system work) cancels and is audited as before; so does a plan the
// operator reviewed, whose rows name the cancellations.

// ErrWouldCancel is the refusal; CancelGate.Refused has what it would cancel.
var ErrWouldCancel = errors.New("this would cancel changes that are waiting for you to send")

// CancelGate is one request's acknowledgement, and what it refused.
type CancelGate struct {
	mu       sync.Mutex
	acked    map[string]bool
	reviewed bool
	refused  []models.QueuedChange
	// asks says whether the request's caller is someone to ask, read at the
	// chokepoint, after authentication has put them in the context.
	asks func(context.Context) bool
}

type cancelGateKey struct{}

// GuardCancellations gates an operator's request. acked is the list of queued
// change ids the operator was shown and agreed to cancel.
func GuardCancellations(ctx context.Context, acked []string, asks func(context.Context) bool) (context.Context, *CancelGate) {
	g := &CancelGate{acked: make(map[string]bool, len(acked)), asks: asks}
	for _, id := range acked {
		g.acked[id] = true
	}
	return context.WithValue(ctx, cancelGateKey{}, g), g
}

// CancellationsReviewed marks a plan apply: its rows named every cancellation
// before the operator applied it.
func CancellationsReviewed(ctx context.Context) context.Context {
	if g, ok := ctx.Value(cancelGateKey{}).(*CancelGate); ok && g != nil {
		g.mu.Lock()
		g.reviewed = true
		g.mu.Unlock()
	}
	return ctx
}

// Refused is what the request would have cancelled, if it was refused.
func (g *CancelGate) Refused() []models.QueuedChange {
	g.mu.Lock()
	defer g.mu.Unlock()
	return append([]models.QueuedChange(nil), g.refused...)
}

// AdmitCancellations reports whether these cancellations may go ahead, recording a refusal.
func AdmitCancellations(ctx context.Context, changes []models.QueuedChange) bool {
	g, ok := ctx.Value(cancelGateKey{}).(*CancelGate)
	if !ok || g == nil {
		return true
	}
	if g.asks != nil && !g.asks(ctx) {
		return true
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.reviewed {
		return true
	}
	for _, c := range changes {
		if !g.acked[c.ID] {
			g.refused = append(g.refused, changes...)
			return false
		}
	}
	return true
}
