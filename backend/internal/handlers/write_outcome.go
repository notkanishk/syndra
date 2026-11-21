package handlers

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"

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
	system, _ := systemName(ctx, target)
	product := system
	if shape, err := addonsAcceptedShape(ctx, target); err == nil && strings.TrimSpace(shape.Product) != "" {
		product = shape.Product
	}
	return &notLanded{System: system, Product: product, Outcome: outcome, Why: why}
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
		retryAfter = n.Why.RetryAfter
		if retryAfter <= 0 {
			// The monitor's next probe: when Syndra next learns whether it answers.
			retryAfter = int(addons.ProbeInterval().Seconds())
		}
		details := map[string]string{}
		for k, v := range map[string]string{"reason": n.Why.Reason, "cause": n.Why.Cause} {
			if v != "" {
				details[k] = v
			}
		}
		return http.StatusServiceUnavailable, ErrorResponse{
			Error: "NOTHING_SENT", Message: nothingSent(n), Details: details, RetryAfter: retryAfter,
		}, retryAfter
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
	status, body, retryAfter := outcomeAnswer(n, extra)
	if retryAfter > 0 {
		w.Header().Set("Retry-After", strconv.Itoa(retryAfter))
	}
	jsonResponse(w, status, body)
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

// nothingSent is NOTHING_SENT's sentence: what happened, who is down, and the
// next step.
func nothingSent(n *notLanded) string {
	return "Nothing went out. " + whySentence(n) + " Try again when " + n.System + " answers."
}

// whySentence is design §13.1's sentence for who is not answering: the
// operator's says which side and what to check, the member's is one sentence
// for both. The add-on's own words come first where it sent some. Drafts; the
// final words go through the copy guide with 8e.
func whySentence(n *notLanded) string {
	s, why := n.System, n.Why
	if n.Member {
		return s + " isn't answering right now."
	}
	var out string
	switch why.Reason {
	case addons.ReasonAddonUnreachable:
		switch why.Cause {
		case addons.CauseWrongHost:
			out = "Something answered at " + s + "'s add-on address, but it isn't the add-on registered here. Check the address and the shared secret, then restart the add-on."
		case addons.CauseTimeout:
			out = s + "'s add-on started answering and stopped. If it lasts, read the add-on's log."
		case addons.CauseFault:
			out = s + "'s add-on answered with an error it didn't explain. Read the add-on's log."
		case addons.CauseNoKey:
			out = "Syndra can't load " + s + "'s shared secret. Check Syndra's secret file."
		default:
			out = "Syndra can't reach " + s + "'s add-on. Check that its container is running."
		}
		if why.RetryAfter > 0 {
			out += " Syndra tries again in " + strconv.Itoa(why.RetryAfter) + " s."
		}
	case addons.ReasonAppUnreachable:
		out = sentenceOr(why.Detail, s+"'s add-on is running but can't reach "+n.Product+". Check that "+n.Product+" is on.")
	case addons.ReasonAddonOutdated:
		out = s + "'s add-on doesn't offer this. Update the add-on."
	case addons.ReasonCooldown:
		out = s + " was read moments ago."
	default:
		// read_refused, or no reason known: the add-on's words, else plainly.
		out = sentenceOr(why.Detail, s+" couldn't take it just now.")
	}
	return out
}

// sentenceOr is the add-on's own sentence, ended with a full stop, else ours.
func sentenceOr(theirs, ours string) string {
	theirs = strings.TrimSpace(theirs)
	if theirs == "" {
		return ours
	}
	if !strings.HasSuffix(theirs, ".") {
		theirs += "."
	}
	return theirs
}
