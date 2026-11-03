// Package claimdebug is claims debug mode (A-14 §6): for one app, for 15
// minutes, it keeps each real sign-in of exactly ONE chosen account — what
// Zitadel sent, what Syndra answered, and the five checks — in memory only.
//
//   - The 15 minutes is enforced here, server-side: Wants turns false at the
//     end whatever the page does, and a timer records the end.
//   - Filtering happens before anything is held: Wants is asked with the
//     sign-in's client id and user id, so other people's sign-ins are never
//     buffered.
//   - Capture is a non-blocking send after the answer is written; a full queue
//     drops the capture, never delays a token.
//   - Entries live an hour, at most 200 per app, and vanish on restart. The
//     body is kept (emails included, this is for debugging); request headers,
//     where Zitadel's signature travels, are never captured at all.
//
// ponytail: one process. With more than one API replica each keeps its own
// sessions and captures, so a sign-in lands on whichever served it; pin debug
// to one replica (or move this to Redis) if the API is ever scaled out.
package claimdebug

import (
	"context"
	"encoding/json"
	"sync"
	"sync/atomic"
	"time"

	"syndra/internal/services/periodic"
)

// Window, Keep and Cap are the owner's numbers (3 Nov).
const (
	Window = 15 * time.Minute
	Keep   = time.Hour
	Cap    = 200
)

// Session is one app's debug run. EndedBy is "" while it runs, "system" when
// it ended by itself, else the operator who turned it off.
type Session struct {
	ApplicationID string     `json:"application_id"`
	ClientID      string     `json:"-"`
	UserID        string     `json:"user_id"`
	StartedBy     string     `json:"started_by"`
	StartedAt     time.Time  `json:"started_at"`
	EndsAt        time.Time  `json:"ends_at"`
	EndedAt       *time.Time `json:"ended_at,omitempty"`
	EndedBy       string     `json:"ended_by,omitempty"`
}

// On is whether the session still captures at t.
func (s Session) On(t time.Time) bool { return s.EndedAt == nil && t.Before(s.EndsAt) }

// Capture is what the data plane hands over for one sign-in.
type Capture struct {
	At        time.Time
	Function  string
	ClientID  string
	UserID    string
	ProjectID string
	Grants    any
	Request   []byte
	Response  []byte
	Duration  time.Duration
}

// Entry is one captured sign-in as the list shows it.
type Entry struct {
	Seq        int64           `json:"seq"`
	At         time.Time       `json:"at"`
	Function   string          `json:"function"`
	ClientID   string          `json:"client_id"`
	UserID     string          `json:"user_id"`
	Grants     any             `json:"grants"`
	Request    json.RawMessage `json:"request"`
	Response   json.RawMessage `json:"response"`
	DurationMS float64         `json:"duration_ms"`
	// Checks is the five-check trail as evaluated when the sign-in arrived.
	Checks any `json:"checks"`
}

// Recorder holds every app's debug state.
type Recorder struct {
	now     func() time.Time
	explain func(ctx context.Context, c Capture) any
	onEnd   func(Session)

	active   atomic.Int32 // sessions on; zero keeps the data plane's check free
	mu       sync.Mutex
	sessions map[string]*Session // by app id
	byClient map[string]string   // client id -> app id, while on
	entries  map[string][]Entry  // by app id, oldest first
	seq      int64
	queue    chan Capture
}

// New starts a recorder. explain computes the checks for a capture (off the
// latency path); onEnd records a session that ended by itself.
func New(now func() time.Time, explain func(context.Context, Capture) any, onEnd func(Session)) *Recorder {
	r := &Recorder{
		now: now, explain: explain, onEnd: onEnd,
		sessions: map[string]*Session{}, byClient: map[string]string{}, entries: map[string][]Entry{},
		queue: make(chan Capture, 64),
	}
	go r.work()
	return r
}

// Start turns debug on for an app, or, while it is on, moves the capture to
// another account. A switch never moves the end. It returns the session and
// whether this was a switch.
func (r *Recorder) Start(appID, clientID, userID, actor string) (Session, bool) {
	r.Sweep() // a session that ran out ends (and is recorded) before a new one
	r.mu.Lock()
	defer r.mu.Unlock()
	t := r.now()
	if s, ok := r.sessions[appID]; ok && s.On(t) {
		s.UserID = userID
		return *s, true
	}
	s := &Session{ApplicationID: appID, ClientID: clientID, UserID: userID, StartedBy: actor, StartedAt: t, EndsAt: t.Add(Window)}
	r.sessions[appID] = s
	r.byClient[clientID] = appID
	r.active.Add(1)
	time.AfterFunc(Window, r.Sweep)
	return *s, false
}

// Stop turns debug off by hand. False when it wasn't on.
func (r *Recorder) Stop(appID, actor string) (Session, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	s, ok := r.sessions[appID]
	if !ok || !s.On(r.now()) {
		return Session{}, false
	}
	r.endLocked(s, actor, r.now())
	return *s, true
}

// Sweep ends every session past its end and records each as ended by
// itself. The timer calls it; tests call it after moving the clock.
func (r *Recorder) Sweep() {
	r.mu.Lock()
	var ended []Session
	t := r.now()
	for _, s := range r.sessions {
		if s.EndedAt == nil && !t.Before(s.EndsAt) {
			r.endLocked(s, "system", s.EndsAt)
			ended = append(ended, *s)
		}
	}
	r.mu.Unlock()
	for _, s := range ended {
		if r.onEnd != nil {
			r.onEnd(s)
		}
	}
}

func (r *Recorder) endLocked(s *Session, by string, at time.Time) {
	s.EndedAt, s.EndedBy = &at, by
	delete(r.byClient, s.ClientID)
	r.active.Add(-1)
}

// Session is an app's current or last session.
func (r *Recorder) Session(appID string) (Session, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	s, ok := r.sessions[appID]
	if !ok {
		return Session{}, false
	}
	return *s, true
}

// Wants is the data plane's question: is this sign-in the one being watched?
// Free while no session is on anywhere.
func (r *Recorder) Wants(clientID, userID string) bool {
	if r == nil || r.active.Load() == 0 {
		return false
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	s, ok := r.sessions[r.byClient[clientID]]
	return ok && s.UserID == userID && s.On(r.now())
}

// Watching is every session on now, by client id: whose sign-ins a data
// plane in another process should hand over (claimserve.DebugWatch).
func (r *Recorder) Watching() map[string]Session {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := map[string]Session{}
	t := r.now()
	for client, app := range r.byClient {
		if s, ok := r.sessions[app]; ok && s.On(t) {
			out[client] = *s
		}
	}
	return out
}

// Record queues a capture without blocking; a full queue drops it.
func (r *Recorder) Record(c Capture) {
	select {
	case r.queue <- c:
	default:
	}
}

// work files captures until the queue closes. A panic in one (explain reads
// Zitadel and Postgres) loses that capture, not the worker, and never the
// process: this goroutine shares it with the claims data plane.
func (r *Recorder) work() {
	for c := range r.queue {
		r.safeAdd(c)
	}
}

func (r *Recorder) safeAdd(c Capture) {
	defer periodic.Recover("CLAIM-DEBUG")
	r.add(c)
}

// add files one capture, re-checking the filter: the account may have been
// switched between the answer and now.
func (r *Recorder) add(c Capture) {
	var checks any
	if r.explain != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		checks = r.explain(ctx, c)
		cancel()
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	appID := r.byClient[c.ClientID]
	s, ok := r.sessions[appID]
	if !ok || s.UserID != c.UserID {
		return
	}
	r.seq++
	list := append(r.pruneLocked(appID), Entry{
		Seq: r.seq, At: c.At, Function: c.Function, ClientID: c.ClientID, UserID: c.UserID, Grants: c.Grants,
		Request: rawOrNull(c.Request), Response: rawOrNull(c.Response),
		DurationMS: float64(c.Duration.Microseconds()) / 1000, Checks: checks,
	})
	if len(list) > Cap {
		list = list[len(list)-Cap:]
	}
	r.entries[appID] = list
}

func (r *Recorder) pruneLocked(appID string) []Entry {
	list := r.entries[appID]
	cut := r.now().Add(-Keep)
	i := 0
	for i < len(list) && list[i].At.Before(cut) {
		i++
	}
	return list[i:]
}

// SignIns lists an app's captured sign-ins after seq, newest first.
func (r *Recorder) SignIns(appID string, after int64) []Entry {
	r.mu.Lock()
	defer r.mu.Unlock()
	list := r.pruneLocked(appID)
	r.entries[appID] = list
	out := []Entry{}
	for i := len(list) - 1; i >= 0; i-- {
		if list[i].Seq > after {
			out = append(out, list[i])
		}
	}
	return out
}

func rawOrNull(b []byte) json.RawMessage {
	if !json.Valid(b) {
		return json.RawMessage("null")
	}
	return json.RawMessage(b)
}
