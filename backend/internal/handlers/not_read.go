package handlers

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"syndra/internal/addons"
)

// One reply for every read (design §13.3). A read with data is 200 and carries
// `read` (addons.ReadOf); a read with no copy is 503 TARGET_NOT_READ with
// Retry-After (RFC 9110 §15.6.4, §10.2.3) in the one envelope, and no data
// member at all. The statuses are HTTP's own: a stale answer served while the
// origin cannot be reached is a 200 (RFC 9111 §4.2.4, RFC 5861).

// codeTargetNotRead is the one code for a read with nothing to give.
const codeTargetNotRead = "TARGET_NOT_READ"

// writeNotRead answers a read that has no copy, with message as the sentence.
func writeNotRead(w http.ResponseWriter, message string, why addons.Why) {
	body, retry := unavailable(codeTargetNotRead, message, why)
	writeAnswer(w, http.StatusServiceUnavailable, body, retry)
}

// unavailable is the one 503 envelope for a system that did not answer: a
// read with no copy (TARGET_NOT_READ) or a write where nothing went out
// (NOTHING_SENT). details carry {reason, cause}; Retry-After is the add-on's,
// else the monitor's next probe, when Syndra next learns whether it answers.
// An outdated add-on is not fixed by waiting: no time to try again at.
func unavailable(code, message string, why addons.Why) (ErrorResponse, int) {
	retry := why.RetryAfter
	if retry < 1 && why.Reason != addons.ReasonAddonOutdated {
		retry = int(addons.ProbeInterval().Seconds())
	}
	details := map[string]string{"reason": why.Reason}
	if why.Cause != "" {
		details["cause"] = why.Cause
	}
	return ErrorResponse{Error: code, Message: message, Details: details, RetryAfter: max(retry, 0)}, max(retry, 0)
}

// writeAnswer writes status and body, with Retry-After when retry is set: the
// one writer for TARGET_NOT_READ, NOTHING_SENT and unconfirmed.
func writeAnswer(w http.ResponseWriter, status int, body any, retry int) {
	if retry > 0 {
		w.Header().Set("Retry-After", strconv.Itoa(retry))
	}
	jsonResponse(w, status, body)
}

// writeUnread is writeNotRead for a route's own read, in the operator's words.
func writeUnread(w http.ResponseWriter, r *http.Request, target string, why addons.Why) {
	system, product := systemAndProduct(r.Context(), target)
	writeNotRead(w, whySentence(system, product, why, false), why)
}

// writeIfNotRead answers err when it is a read with no copy (addons.NotRead),
// and reports whether it did.
func writeIfNotRead(w http.ResponseWriter, r *http.Request, target string, err error) bool {
	var nr addons.NotRead
	if !errors.As(err, &nr) {
		return false
	}
	writeUnread(w, r, target, nr.Why)
	return true
}

// systemAndProduct is target's name and the product its accepted manifest
// says it fronts ("TrueNAS"), the system's name when it names none.
func systemAndProduct(ctx context.Context, target string) (system, product string) {
	system, _ = systemName(ctx, target)
	product = system
	if shape, err := addonsAcceptedShape(ctx, target); err == nil && strings.TrimSpace(shape.Product) != "" {
		product = shape.Product
	}
	return system, product
}

// whySentence is design §13.1's sentence for why a system did not answer,
// next step included: the operator's says which side is down and what to
// check; a member's is one sentence for both, since which box is down is not
// theirs to fix. The add-on's own words come first only where it is the one
// that knows: its app down, or its refusal. Drafts; the final words are the
// copy guide's (8e).
func whySentence(system, product string, why addons.Why, member bool) string {
	if member && why.Unreachable() {
		return system + " isn't answering right now."
	}
	if member && (why.Reason == addons.ReasonReadRefused || why.Reason == addons.ReasonAddonOutdated) {
		// Not an outage, and not the member's to fix or wait out.
		return system + " can't do this right now. Ask makerspace staff."
	}
	if why.Reason == addons.ReasonAppUnreachable || why.Reason == addons.ReasonReadRefused {
		if d := strings.TrimSpace(why.Detail); d != "" {
			if !strings.HasSuffix(d, ".") {
				d += "."
			}
			return d
		}
	}
	switch why.Reason {
	case addons.ReasonAddonUnreachable:
		var s string
		switch why.Cause {
		case addons.CauseWrongHost:
			s = "Something answered at " + system + "'s add-on address, but it isn't the add-on registered here. Check the address and the shared secret, then restart the add-on."
		case addons.CauseTimeout:
			s = system + "'s add-on started answering and stopped. Syndra keeps trying. If it lasts, read the add-on's log."
		case addons.CauseFault:
			s = system + "'s add-on answered with an error it didn't explain. Read the add-on's log."
		case addons.CauseNoKey:
			s = "Syndra can't load " + system + "'s shared secret. Check Syndra's secret file."
		default:
			s = "Syndra can't reach " + system + "'s add-on. Check that its container is running. Syndra keeps trying."
		}
		if why.RetryAfter > 0 {
			s += " Syndra tries again in " + strconv.Itoa(why.RetryAfter) + " s."
		}
		return s
	case addons.ReasonAppUnreachable:
		if why.Unusable {
			return product + " didn't give its add-on an answer it could use. Check " + product + ", then read again."
		}
		return system + "'s add-on is running but can't reach " + product + ". Check that " + product + " is on."
	case addons.ReasonCooldown:
		return system + " was read moments ago. Read it again in " + strconv.Itoa(max(1, why.RetryAfter)) + " s."
	case addons.ReasonBusy:
		// Not an outage: the add-on's queue or the app was busy, and nothing was sent.
		name := product
		if member {
			name = system
		}
		return name + " is busy right now. Try again in " + strconv.Itoa(max(1, why.RetryAfter)) + " s."
	case addons.ReasonAddonOutdated:
		return system + "'s add-on doesn't offer this. Update the add-on."
	}
	return system + "'s add-on didn't take this. Read the add-on's log."
}
