package main

import (
	"errors"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
)

// Lifecycle state (design §18).
//
// The read-only flag is three-valued rather than two, because "stop writing"
// and "stop starting writes but let the ones in flight settle" are different
// instructions and API-key rotation needs the second. Both will happen: the key
// carries an expiry, and TrueNAS majors break.
//
// All three are configuration and none requires a redeploy — a maintenance mode
// you have to restart into is a maintenance mode nobody uses.
const (
	// LifecycleActive serves everything.
	LifecycleActive = "active"
	// LifecycleDraining refuses NEW mutations, lets issued ones settle, and
	// keeps serving reads. This is what makes a rotation or an upgrade safe.
	LifecycleDraining = "draining"
	// LifecycleReadOnly refuses every mutation while serving reads.
	LifecycleReadOnly = "read_only"
)

// errLifecycleRefusal is returned to a mutation the current state forbids.
//
// It is answered on the wire as 503 with Retry-After, which is what an HTTP
// service already says for "not now, come back" — and what the backend reads as
// queued rather than failed. Treating a deliberate maintenance window as a
// failure would convert every pending change into a failed row during exactly
// the window an operator is least able to notice.
var errLifecycleRefusal = errors.New("the add-on is not accepting mutations")

// lifecycle holds the state and counts what is still in flight.
//
// The in-flight count is what makes `draining` mean anything: without it,
// "drained" is a guess, and the operator rotating a key has no way to know when
// it is safe. It is not merely reported — `Drained` is what an operator waits
// on before pulling the credential out from under a call.
type lifecycle struct {
	mu     sync.RWMutex
	state  string
	reason string

	inFlight atomic.Int64
}

func newLifecycle(state string) *lifecycle {
	l := &lifecycle{}
	if err := l.Set(state, "startup"); err != nil {
		// An unrecognised configured state must not silently become `active`:
		// an operator who typed `readonly` and got a serving add-on has the
		// opposite of what they asked for. Refusing into the safest state is
		// the only direction that cannot surprise them.
		l.state, l.reason = LifecycleReadOnly, "configured lifecycle state was not recognised"
	}
	return l
}

// Set changes the state. The vocabulary is closed and checked by a switch: this
// value is read by an operator and written from configuration, and a fourth
// value nothing understands would be a serving add-on that reports something
// nobody can act on.
func (l *lifecycle) Set(state, reason string) error {
	switch state {
	case LifecycleActive, LifecycleDraining, LifecycleReadOnly:
	default:
		return errors.New("lifecycle state must be one of " +
			strings.Join([]string{LifecycleActive, LifecycleDraining, LifecycleReadOnly}, ", "))
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	l.state, l.reason = state, reason
	return nil
}

func (l *lifecycle) State() (state, reason string) {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.state, l.reason
}

// Begin marks a mutation in flight, or refuses it.
//
// The only gate. There was a bare `AcceptsMutations` beside it, used by nothing
// but its own tests, and it exposed exactly the check-then-act shape Begin
// exists to prevent: a caller could ask whether it may write, be told yes, and
// count itself in after a drain had already concluded it was finished. A
// predicate that must never be used is one somebody eventually uses.
//
// Reads are never gated. A draining or read-only add-on that stopped answering
// `/subjects` would make the backend's drift sweep record an unreconciled
// target — reporting a maintenance window as an outage, and manufacturing the
// absence of evidence the sweep exists to avoid fabricating.
//
// The check and the increment are one call because they cannot be two: between
// a caller asking whether it may write and recording that it is writing, a
// drain could conclude it had finished.
func (l *lifecycle) Begin() (done func(), err error) {
	// Held for write, not read: the increment has to be inside the same
	// critical section as the check, or a drain can observe zero in flight
	// between a caller being allowed and that caller counting itself.
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.state != LifecycleActive {
		return nil, errLifecycleRefusal
	}
	l.inFlight.Add(1)
	var once sync.Once
	return func() { once.Do(func() { l.inFlight.Add(-1) }) }, nil
}

// InFlight is how many mutations have begun and not finished.
func (l *lifecycle) InFlight() int64 { return l.inFlight.Load() }

// Drained reports whether a draining add-on has finished settling. Meaningless
// while active — an active add-on is always about to accept another one — so it
// answers false rather than inviting the question.
func (l *lifecycle) Drained() bool {
	state, _ := l.State()
	return state != LifecycleActive && l.inFlight.Load() == 0
}

// retryAfterSeconds is how long a refused caller is told to wait.
//
// It is the field that distinguishes a deliberate refusal from an add-on that
// fell over: without it the backend records the change as indeterminate — it
// may have got halfway — rather than as queued. Short, because a maintenance
// window is usually minutes and the backend is polling rather than sleeping.
const retryAfterSeconds = "30"

// writeLifecycleRefusal answers a mutation the current state forbids.
//
// 503 with Retry-After is what an HTTP service already says for "not now, come
// back", so the contract borrows it rather than inventing a code. The state is
// named in the body because "unavailable" and "we put it in read-only on
// purpose" send an operator to different places.
func writeLifecycleRefusal(w http.ResponseWriter, state string) {
	w.Header().Set("Retry-After", retryAfterSeconds)
	writeJSON(w, http.StatusServiceUnavailable, map[string]string{
		"error":     "LIFECYCLE_REFUSAL",
		"lifecycle": state,
	})
}

// LifecycleRequest is an operator changing the state at runtime.
//
// A body rather than a query parameter, so the signed-mode MAC covers it: the
// state is an authorisation decision — `read_only` is how an operator stops
// this add-on writing during an incident — and a value an on-path peer could
// edit is not one.
type LifecycleRequest struct {
	ContractVersion int    `json:"contract_version"`
	State           string `json:"state"`
	// Reason is what an operator will read on the health surface while
	// wondering why nothing is applying. Required, because "read_only" with no
	// explanation is indistinguishable from a bug.
	Reason string `json:"reason"`
}

// handleLifecycle sets the state without a redeploy.
//
// §18 says all three states are configuration and none requires a restart — a
// maintenance mode you have to restart into is a maintenance mode nobody uses —
// and until this existed the only setter was an environment variable read at
// startup, which is exactly the restart it says is not needed.
func (s *server) handleLifecycle(w http.ResponseWriter, r *http.Request, body []byte) {
	var req LifecycleRequest
	if err := decodeStrict(body, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "BAD_REQUEST"})
		return
	}
	if !writeContractRefusal(w, req.ContractVersion) {
		return
	}
	if strings.TrimSpace(req.Reason) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error":  "REASON_REQUIRED",
			"detail": "a state nobody explained is indistinguishable from a fault",
		})
		return
	}
	if err := s.life.Set(req.State, req.Reason); err != nil {
		// The vocabulary, never the value. This is the one route whose input is
		// echoed onto a surface an operator reads, and the closed set is the
		// whole of what may appear there.
		writeJSON(w, http.StatusUnprocessableEntity, map[string]any{
			"error":  "UNKNOWN_LIFECYCLE_STATE",
			"detail": "state must be one of " + strings.Join([]string{LifecycleActive, LifecycleDraining, LifecycleReadOnly}, ", "),
		})
		return
	}
	state, reason := s.life.State()
	writeJSON(w, http.StatusOK, map[string]any{
		"state":  state,
		"reason": reason,
		// What an operator waits on before pulling a credential out from under a
		// call. Reported here so the state change and the answer to "is it safe
		// yet" arrive together.
		"in_flight": s.life.InFlight(),
		"drained":   s.life.Drained(),
	})
}
