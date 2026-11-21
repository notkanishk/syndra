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
	retry := why.RetryAfter
	if retry < 1 {
		retry = int(addons.ProbeInterval().Seconds())
	}
	details := map[string]string{"reason": why.Reason}
	if why.Cause != "" {
		details["cause"] = why.Cause
	}
	w.Header().Set("Retry-After", strconv.Itoa(retry))
	jsonResponse(w, http.StatusServiceUnavailable, ErrorResponse{Error: codeTargetNotRead, Message: message, Details: details, RetryAfter: retry})
}

// writeUnread is writeNotRead for a route's own read, in the operator's words.
func writeUnread(w http.ResponseWriter, r *http.Request, target string, why addons.Why) {
	writeNotRead(w, notReadSentence(r.Context(), target, why), why)
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

// notReadSentence is design §13.1's operator sentence for why, next step
// included: which side is down, and what to do. The add-on's own sentence
// comes first only where it is the one that knows: its app down, or its
// refusal (§13.1). Drafts; the final words are the copy guide's (8e).
func notReadSentence(ctx context.Context, target string, why addons.Why) string {
	system, _ := systemName(ctx, target)
	if why.Detail != "" && (why.Reason == addons.ReasonAppUnreachable || why.Reason == addons.ReasonReadRefused) {
		return why.Detail
	}
	switch why.Reason {
	case addons.ReasonAddonUnreachable:
		s := addonSentence(system, why.Cause)
		if why.RetryAfter > 0 {
			s += " Syndra tries again in " + strconv.Itoa(why.RetryAfter) + " s."
		}
		return s
	case addons.ReasonAppUnreachable:
		// The accepted manifest's product, as a write's outcome names it.
		product := system
		if shape, err := addonsAcceptedShape(ctx, target); err == nil && strings.TrimSpace(shape.Product) != "" {
			product = shape.Product
		}
		return system + "'s add-on is running but can't reach " + product + ". Check that " + product + " is on."
	case addons.ReasonCooldown:
		return system + " was read moments ago. Read it again in " + strconv.Itoa(max(1, why.RetryAfter)) + " s."
	case addons.ReasonAddonOutdated:
		return system + "'s add-on is older than this read. Update the add-on."
	}
	return system + "'s add-on didn't serve this read. Read the add-on's log."
}

// addonSentence is the add-on-unreachable sentence for a cause (§13.1's table).
func addonSentence(system, cause string) string {
	switch cause {
	case addons.CauseWrongHost:
		return "Something answered at " + system + "'s add-on address, but it isn't the add-on registered here. Check the address and the shared secret, then restart the add-on."
	case addons.CauseTimeout:
		return system + "'s add-on started answering and stopped. Syndra keeps trying. If it lasts, read the add-on's log."
	case addons.CauseFault:
		return system + "'s add-on answered with an error it didn't explain. Read the add-on's log."
	case addons.CauseNoKey:
		return "Syndra can't load " + system + "'s shared secret. Check Syndra's secret file."
	}
	return "Syndra can't reach " + system + "'s add-on. Check that its container is running. Syndra keeps trying."
}
