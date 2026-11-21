package handlers

import (
	"log"

	"context"
	"errors"
	"net/http"
	"syndra/internal/services/addonop"

	"syndra/internal/addons"
)

// A write to an add-on that did not land says one of two things (design §13.4,
// decision 28), whichever path it came by: adopt, release, a setting, the
// lifecycle, the dormant removal, a password, a physical action and the
// revocation's rotation all answer through writeOutcome.
//
//   - unreached: nothing went out. 503 NOTHING_SENT with Retry-After (RFC 9110
//     §15.6.4, §10.2.3) and details {reason, cause}: the person presses again
//     when the system answers. An interactive write is never queued.
//   - indeterminate: sent, and the answer was lost. 202 {status: "unconfirmed"}
//     (RFC 9110 §15.3.3: accepted, not known to be done). Syndra settles it
//     through GET /calls/{call_id}; sending it again is the wrong reflex.
//
// NOTHING_SENT is Syndra's own code. TARGET_UNREACHABLE is the add-on's wire
// code only, and never answered on Syndra's API (§13.3).

// notLanded is one write that did not land, named for the person reading it.
type notLanded struct {
	// System is the system's name (systemName), never its id.
	System string
	// Product is what the add-on fronts ("TrueNAS"), for app_unreachable's
	// sentence; System when the manifest named none.
	Product string
	Outcome addons.Outcome
	Why     addons.Why
	// Member is a member's own write: one sentence for both outages, since
	// which box is down is not theirs to fix (§13.1).
	Member bool
}

func (n *notLanded) Error() string { return n.System + ": " + string(n.Outcome) }

// notLandedOf names a write's outcome on target. Read for the name only when
// the write did not land.
func notLandedOf(ctx context.Context, target string, outcome addons.Outcome, why addons.Why) *notLanded {
	system, product := systemAndProduct(ctx, target)
	return &notLanded{System: system, Product: product, Outcome: outcome, Why: why}
}

// dispatched is a dispatch's result as a handler answers it: an error only
// when nothing was sent (no OperationID). A call sent whose outcome Syndra
// could not record (addonop.Dispatch leaves the row dispatching) is as unknown
// as a lost answer: indeterminate, never a refusal or a 500 that invites a
// second send.
func dispatched(res addonop.Result, err error) (addonop.Result, error) {
	if err != nil && res.OperationID != "" {
		log.Printf("[ADDON-OP] %v", err)
		res.Outcome = addons.OutcomeIndeterminate
		return res, nil
	}
	return res, err
}

// landed says an outcome is one writeOutcome does not answer: succeeded or a
// refusal, both clear answers the caller words itself.
func landed(o addons.Outcome) bool {
	return o == addons.OutcomeSucceeded || o == addons.OutcomeRejected
}

// outcomeAnswer is writeOutcome's status, body and Retry-After, for a reply
// that carries a write's outcome inside a larger one (the revocation's
// rotation). extra joins an unconfirmed body (the call's id, a resend's state).
func outcomeAnswer(n *notLanded, extra map[string]any) (status int, body any, retryAfter int) {
	if n.Outcome == addons.OutcomeUnreached {
		body, retry := unavailable("NOTHING_SENT", nothingSent(n), n.Why)
		return http.StatusServiceUnavailable, body, retry
	}
	out := map[string]any{}
	for k, v := range extra {
		out[k] = v
	}
	out["status"] = "unconfirmed"
	out["detail"] = "This may have been applied. Syndra is checking; don't send it again."
	return http.StatusAccepted, out, 0
}

// outcomeWord is the one word for a write that did not land, where a reply
// lists several (the dormant removal's accounts): nothing_sent or unconfirmed.
func outcomeWord(o addons.Outcome) string {
	if o == addons.OutcomeUnreached {
		return "nothing_sent"
	}
	return "unconfirmed"
}

// writeOutcome answers a write that did not land (design §13.4).
func writeOutcome(w http.ResponseWriter, n *notLanded, extra map[string]any) {
	status, body, retry := outcomeAnswer(n, extra)
	writeAnswer(w, status, body, retry)
}

// writeNotLanded answers err through writeOutcome when it is a write that did
// not land, and says whether it was one.
func writeNotLanded(w http.ResponseWriter, err error) bool {
	var n *notLanded
	if !errors.As(err, &n) {
		return false
	}
	writeOutcome(w, n, nil)
	return true
}

// nothingSent is NOTHING_SENT's sentence: what happened, who is down
// (whySentence), and the next step.
func nothingSent(n *notLanded) string {
	return "Nothing went out. " + whySentence(n.System, n.Product, n.Why, n.Member) + " Try again when " + n.System + " answers."
}
