package claimtel

import (
	"sync"
	"time"
)

// The sign-in correlator (claims-delivery-integrity S3, assessment §4b): the
// silent-failure detector. Zitadel tells Syndra someone signed in
// (oidc_session.added); the recorder knows which calls Zitadel made. A sign-in
// with no call means Zitadel never reached Syndra for that app, whatever the
// reason (an unbound execution, a changed endpoint, a deny list, DNS); a call
// that was not served means Syndra answered without the person's claims. It is
// black-box with respect to Zitadel's configuration: it does not need to know
// why something broke to say that it did.

const (
	// SignInSettle is how old a sign-in must be before it is checked: the
	// event arrives asynchronously and can trail the calls by a minute.
	SignInSettle = 2 * time.Minute
	// SignInKeep is how long a sign-in waits to be checked. Past it the calls
	// it would match have aged out of the recorder.
	SignInKeep = 15 * time.Minute
	// MatchWithin is how far a call may be from its sign-in, either way.
	MatchWithin = 120 * time.Second
	// NewPersonWithin: a sign-in this soon after the person was created that
	// names no project is the first-sign-in race (SC4), counted, not alarmed.
	NewPersonWithin = 10 * time.Minute
	// EventsMissingAfter: calls with no sign-in event for this long means the
	// detector itself is blind (N7).
	EventsMissingAfter = 24 * time.Hour

	maxPending = 50000 // a term-start burst is a few thousand
)

// SignIn is one sign-in event. Scopes nil means the event did not say.
type SignIn struct {
	At       time.Time // when Zitadel recorded it
	UserID   string
	ClientID string
	Scopes   []string
}

// Verdict is what one checked sign-in came to.
type Verdict string

const (
	Delivered   Verdict = "delivered"
	NotCalled   Verdict = "not_called"
	NotServed   Verdict = "not_served"
	FirstSignIn Verdict = "first_sign_in_before_access"
	// NothingToDeliver: Zitadel listed no grants and Syndra's own facts say
	// the person holds no role in the app's project. The two agree; there was
	// nothing to send. Counted, never alarmed.
	NothingToDeliver Verdict = "nothing_to_deliver"
	// Unchecked: from before this process was watching, waited past
	// SignInKeep, or its calls may have been pushed out of the recorder's
	// ring. Never counted as a failure.
	Unchecked Verdict = "unchecked"
)

// Verdicts lists every verdict once, in a fixed order.
var Verdicts = [...]Verdict{Delivered, NotCalled, NotServed, FirstSignIn, NothingToDeliver, Unchecked}

// Checked is one sign-in's verdict and the evidence for it.
type Checked struct {
	SignIn
	Verdict  Verdict
	Outcome  Outcome  // the worst outcome among the expected functions' calls
	Expected []string // the functions this sign-in should have called
	Missing  []string // expected functions with no call in the window
}

// CallWindow is the recorder's side: what (person, client) called around a time.
type CallWindow interface {
	Window(userID, clientID string, from, to time.Time) (map[string]Outcome, bool)
}

// AppSettings says whether a client's access token is a JWT (only then does
// Zitadel call preaccesstoken). known is false before the reconciler has read
// the app's settings; then any call counts.
type AppSettings func(clientID string) (jwt, known bool)

type signInKey struct {
	user, client string
	at           int64
}

type hourTally struct {
	hour int64
	n    [len(Verdicts)]int64
}

// Correlator holds sign-ins until they are old enough to check. Safe for
// concurrent use. A restart starts it empty: sign-ins from before then are
// unchecked, never failed ("checking since …").
type Correlator struct {
	mu         sync.Mutex
	now        func() time.Time
	since      time.Time
	pending    []SignIn
	seen       map[signInKey]struct{}
	created    map[string]time.Time
	lastSignIn time.Time
	hours      [24]hourTally
	dropped    int64

	// Holds says whether the person holds any role in the client's app's
	// project, by Syndra's own facts (known false when there is no app or no
	// facts read). Nil: never consulted, so no_grants stays a failure. Set
	// once before Check runs.
	Holds func(userID, clientID string) (holds, known bool)
}

// NewCorrelator starts watching now.
func NewCorrelator(now func() time.Time) *Correlator {
	return &Correlator{now: now, since: now(), seen: map[signInKey]struct{}{}, created: map[string]time.Time{}}
}

// Since is when this correlator started watching.
func (c *Correlator) Since() time.Time { return c.since }

// SignIn queues one sign-in to check. A redelivered event is queued once.
func (c *Correlator) SignIn(s SignIn) {
	if s.UserID == "" || s.ClientID == "" {
		return
	}
	k := signInKey{s.UserID, s.ClientID, s.At.UnixMilli()}
	c.mu.Lock()
	defer c.mu.Unlock()
	if s.At.After(c.lastSignIn) {
		c.lastSignIn = s.At
	}
	if _, dup := c.seen[k]; dup {
		return
	}
	if len(c.pending) >= maxPending {
		c.dropped++
		return
	}
	c.seen[k] = struct{}{}
	c.pending = append(c.pending, s)
}

// Created notes that a person was created at `at` (user.human.added).
func (c *Correlator) Created(userID string, at time.Time) {
	if userID == "" {
		return
	}
	c.mu.Lock()
	c.created[userID] = at
	c.mu.Unlock()
}

// Check takes every queued sign-in at least SignInSettle old and gives each a
// verdict. Run once a minute.
func (c *Correlator) Check(calls CallWindow, apps AppSettings) []Checked {
	now := c.now()
	c.mu.Lock()
	var ready []SignIn
	keep := c.pending[:0]
	for _, s := range c.pending {
		if now.Sub(s.At) < SignInSettle {
			keep = append(keep, s)
			continue
		}
		ready = append(ready, s)
		delete(c.seen, signInKey{s.UserID, s.ClientID, s.At.UnixMilli()})
	}
	clear(c.pending[len(keep):])
	c.pending = keep
	created := make(map[string]time.Time, len(ready))
	for _, s := range ready {
		if at, ok := c.created[s.UserID]; ok {
			created[s.UserID] = at
		}
	}
	for u, at := range c.created {
		if now.Sub(at) > SignInKeep+NewPersonWithin {
			delete(c.created, u)
		}
	}
	c.mu.Unlock()

	out := make([]Checked, 0, len(ready))
	for _, s := range ready {
		var ch Checked
		if s.At.Add(-MatchWithin).Before(c.since) || now.Sub(s.At) > SignInKeep {
			ch = Checked{SignIn: s, Verdict: Unchecked}
		} else {
			worst, overflowed := calls.Window(s.UserID, s.ClientID, s.At.Add(-MatchWithin), s.At.Add(MatchWithin))
			jwt, known := apps(s.ClientID)
			at, ok := created[s.UserID]
			newPerson := ok && s.At.Sub(at) >= -time.Minute && s.At.Sub(at) <= NewPersonWithin
			holdsNone := false
			if c.Holds != nil {
				holds, holdsKnown := c.Holds(s.UserID, s.ClientID)
				holdsNone = holdsKnown && !holds
			}
			ch = Classify(s, worst, overflowed, jwt, known, newPerson, holdsNone)
		}
		out = append(out, ch)
	}
	c.tally(out)
	return out
}

// anyFunction stands for "some call, through either function" when the app's
// settings are not known yet.
const anyFunction = "*"

// Classify is one sign-in's verdict from the calls in its window. Pure.
//
// holdsNone is true when Syndra's facts say the person holds no role in the
// app's project: a no_grants call is then the right answer, not a failure.
func Classify(s SignIn, worst map[string]Outcome, overflowed, jwt, known, newPerson, holdsNone bool) Checked {
	ch := Checked{SignIn: s}
	// The functions are exact (assessment Z-e): a JWT access token calls
	// preaccesstoken; an openid scope makes an ID token, which calls
	// preuserinfo.
	if known && jwt {
		ch.Expected = append(ch.Expected, FnPreAccessToken)
	}
	openidKnown := s.Scopes != nil
	for _, sc := range s.Scopes {
		if sc == "openid" {
			ch.Expected = append(ch.Expected, FnPreUserinfo)
			break
		}
	}
	if len(ch.Expected) == 0 {
		if known && openidKnown {
			// A bearer app asked for no ID token: Zitadel calls nothing at
			// sign-in. Whether its claims can ever land is the reconciler's
			// check (R5), not this one.
			ch.Verdict = Unchecked
			return ch
		}
		ch.Expected = []string{anyFunction}
	}

	have := false
	for _, f := range ch.Expected {
		var o Outcome
		var ok bool
		if f == anyFunction {
			for _, w := range worst {
				if !ok || Worse(w, o) {
					o, ok = w, true
				}
			}
		} else {
			o, ok = worst[f]
		}
		if !ok {
			ch.Missing = append(ch.Missing, f)
			continue
		}
		if !have || Worse(o, ch.Outcome) {
			ch.Outcome, have = o, true
		}
	}
	switch {
	case len(ch.Missing) > 0 && overflowed:
		ch.Verdict = Unchecked
	case len(ch.Missing) > 0:
		ch.Verdict = NotCalled
	case Rank(ch.Outcome) <= Rank(ServedLKG):
		// Served, served from the last-known-good copy, served as both for an
		// unknown function, or a shape that by design lands elsewhere: the
		// person's claims went out.
		ch.Verdict = Delivered
	case ch.Outcome == NoGrants && newPerson:
		ch.Verdict = FirstSignIn
	case ch.Outcome == NoGrants && holdsNone:
		ch.Verdict = NothingToDeliver
	default:
		ch.Verdict = NotServed
	}
	return ch
}

func (c *Correlator) tally(out []Checked) {
	c.mu.Lock()
	defer c.mu.Unlock()
	idx := map[Verdict]int{}
	for i, v := range Verdicts {
		idx[v] = i
	}
	for _, ch := range out {
		h := ch.At.Unix() / 3600
		if h < c.now().Unix()/3600-23 {
			h = c.now().Unix() / 3600 // older than the ring: count it now
		}
		t := &c.hours[h%24]
		if t.hour != h {
			*t = hourTally{hour: h}
		}
		t.n[idx[ch.Verdict]]++
	}
}

// Summary is how the last 24 hours of sign-ins came out, by verdict, and how
// many are still waiting to be checked.
func (c *Correlator) Summary() (counts map[Verdict]int64, waiting int, dropped int64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	counts = map[Verdict]int64{}
	for _, v := range Verdicts {
		counts[v] = 0
	}
	now := c.now().Unix() / 3600
	for _, t := range c.hours {
		if t.hour > now-24 && t.hour <= now {
			for i, v := range Verdicts {
				counts[v] += t.n[i]
			}
		}
	}
	return counts, len(c.pending), c.dropped
}

// EventsMissing is the inverse check: calls have arrived in the last 24h but
// no sign-in event has for 24h, so the detector is blind (the event listener
// is broken or unbound). Never true in the first 24h after a start.
func (c *Correlator) EventsMissing(lastCalled time.Time) bool {
	now := c.now()
	c.mu.Lock()
	last := c.lastSignIn
	c.mu.Unlock()
	if last.Before(c.since) {
		last = c.since
	}
	return now.Sub(last) >= EventsMissingAfter && !lastCalled.IsZero() && now.Sub(lastCalled) < EventsMissingAfter
}

// LastSignIn is when the newest sign-in event happened, or zero.
func (c *Correlator) LastSignIn() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.lastSignIn
}
