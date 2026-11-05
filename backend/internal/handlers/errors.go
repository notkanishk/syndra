package handlers

import (
	"crypto/rand"
	"encoding/base32"
	"log"
	"net/http"
	"strings"
)

// ErrorResponse is the one error envelope every Syndra HTTP surface speaks:
// a stable code in error, a plain sentence in message, optional details. The
// UI proxy emits the same shape (ui/src/app/api/proxy/[...path]/route.ts).
type ErrorResponse struct {
	Error   string            `json:"error"`
	Message string            `json:"message"`
	Details map[string]string `json:"details,omitempty"`
}

// upstreamSentenceFor is what an operator reads when Zitadel or an add-on fails:
// which thing failed, in the copy guide's words (plain-language-copy §8). The
// upstream's own text (an rpc error, a SQL string, a stack) is for the log. The
// next step and the reference are added where the sentence is shown
// (describeFailure in the UI), so they are not said twice.
func upstreamSentenceFor(status int, code string) string {
	unavailable := status == http.StatusServiceUnavailable
	var who string
	switch code {
	case "ZITADEL_ERROR", "SWEEP_ERROR":
		who = "Zitadel"
	case "ADDON_UNREACHABLE", "TARGET_CAPABILITIES_UNKNOWN", "RECONCILE_FAILED", "LIFECYCLE_NOT_SET", "RELEASE_NOT_CONFIRMED":
		who = "That system"
	}
	switch {
	case who != "" && unavailable:
		return who + " is not answering right now."
	case who != "":
		return who + " gave an answer Syndra could not use."
	case status == http.StatusBadGateway, status == http.StatusGatewayTimeout:
		return "A service Syndra depends on gave an answer Syndra could not use."
	case unavailable:
		return "A service Syndra depends on is not answering right now."
	}
	return "Something went wrong inside Syndra."
}

// upstreamSentence is the sentence for a failure whose code names no one.
func upstreamSentence(status int) string { return upstreamSentenceFor(status, "") }

// jsonErrorResponse writes the envelope. A 500 never shows its message: it goes
// to the log with a reference, and the screen gets a plain sentence and the
// same reference in details.request_id, so the cause stays one grep away. A
// 502/503/504 that carries an upstream's error goes through writeUpstream, not
// here; a 5xx sentence passed here is one the handler wrote.
func jsonErrorResponse(w http.ResponseWriter, status int, errStr, msg string) {
	if status == http.StatusInternalServerError {
		writeSanitised(w, status, errStr, msg)
		return
	}
	jsonResponse(w, status, ErrorResponse{Error: errStr, Message: msg})
}

// writeUpstream answers 502 for a failure of Zitadel or an add-on: the raw
// error (an rpc error, an add-on body) is logged with a reference, and the
// caller gets a plain sentence plus that reference.
func writeUpstream(w http.ResponseWriter, err error, code string) {
	writeUpstreamStatus(w, http.StatusBadGateway, err, code)
}

// writeUpstreamStatus is writeUpstream for the call sites that answer 503.
func writeUpstreamStatus(w http.ResponseWriter, status int, err error, code string) {
	writeSanitised(w, status, code, err.Error())
}

func writeSanitised(w http.ResponseWriter, status int, code, raw string) {
	ref := supportReference()
	log.Printf("[HTTP] error status=%d ref=%s code=%s: %s", status, ref, code, raw)
	jsonResponse(w, status, ErrorResponse{
		Error:   code,
		Message: upstreamSentenceFor(status, code),
		Details: map[string]string{"request_id": ref},
	})
}

// supportReference is a short id an operator can read out and grep for.
func supportReference() string {
	b := make([]byte, 5)
	if _, err := rand.Read(b); err != nil {
		return "unavailable"
	}
	return strings.ToUpper(base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(b))
}

func jsonValidationErrorResponse(w http.ResponseWriter, msg string, details map[string]string) {
	jsonResponse(w, http.StatusBadRequest, ErrorResponse{
		Error:   "VALIDATION_FAILED",
		Message: msg,
		Details: details,
	})
}
