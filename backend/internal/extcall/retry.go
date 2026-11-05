package extcall

import "time"

// One retry model for every change Syndra sends that can come back unanswered
// (owner, 5 Nov 2026): a Zitadel outbox row and an add-on operation alike. Each
// system's rule is a Policy (how many tries, how far apart); everything else —
// the states, the next try, the words on screen — is shared, so the two cannot
// drift into saying different things about the same situation.
//
// The rules themselves.
var (
	// OutboxRetry is a Zitadel (or add-on entitlement) outbox row: the first
	// send plus OUTBOX_MAX_RETRIES more, 1m doubling to 30m. The row holds
	// desired state, so the background drain can retry it on its own.
	OutboxRetry = Policy{Attempts: 6, Base: time.Minute, Max: 30 * time.Minute}

	// AddonResend is a secret-bearing add-on operation (a member's storage
	// password): the first send and ONE resend, by the member's own page.
	// Once, because the backend never keeps the secret (A-09 §3, A-13 §4):
	// only the open page can send it again, and a resend that could repeat
	// would be a loop in a browser tab against the target's single shared,
	// rate-limited session. The page waits Backoff(AddonResend, 0) first.
	AddonResend = Policy{Attempts: 2, Base: 10 * time.Second, Max: 10 * time.Second}
)

// The two states an unanswered change can be in. One never sent, sent, or
// failed has none: its own status says that (re-audit, 6 Nov 2026: "sent" and
// "failed" were never produced, so the words for them were dead).
const (
	RetryRetrying = "retrying" // the target didn't take it; Syndra tries again at NextTry
	RetryWaiting  = "waiting"  // nothing will try again until a person acts
)

// RetryState is where one change stands, in the shape every surface reads.
type RetryState struct {
	State string `json:"state"`
	// Try is the try that comes next while retrying, otherwise the last made.
	Try     int        `json:"try"`
	Of      int        `json:"of"`
	NextTry *time.Time `json:"next_try,omitempty"`
}

// Retrying is the state of a change `tried` times without an answer, whose
// next try is due at `next`. Past its last try it is waiting: nothing sends it
// again until a person acts (a row the outbox gives up on is Failed instead,
// said by its own status).
func (p Policy) Retrying(tried int, next *time.Time) RetryState {
	if tried >= p.Attempts {
		return RetryState{State: RetryWaiting, Try: tried, Of: p.Attempts}
	}
	return RetryState{State: RetryRetrying, Try: tried + 1, Of: p.Attempts, NextTry: next}
}

// NextTryAfter is when a change that failed its `tried`-th try is next sent.
func (p Policy) NextTryAfter(now time.Time, tried int) time.Time {
	return now.Add(Backoff(p, tried-1))
}

// AuditMark is how every automatic try is marked in the audit log, whichever
// system sent it: who retried (automatic), and which try of how many. One
// shape, so the history reads the same for a Zitadel grant and a storage
// password. Callers add their own facts beside it.
func (p Policy) AuditMark(try int) map[string]any {
	return map[string]any{"retried_by": "automatic", "try": try, "of": p.Attempts}
}
