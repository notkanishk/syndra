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

// upstreamSentence is what an operator reads when Zitadel or an add-on fails.
// The upstream's own text (an rpc error, a SQL string, a stack) is for the log.
func upstreamSentence(status int) string {
	switch status {
	case http.StatusBadGateway, http.StatusGatewayTimeout:
		return "A service Syndra depends on gave an answer Syndra could not use. Try again in a moment."
	case http.StatusServiceUnavailable:
		return "A service Syndra depends on is not answering right now. Try again in a moment."
	}
	return "Something went wrong inside Syndra."
}

// rawUpstreamCodes are the codes whose message at 502/503/504 is an upstream's
// own text (a Zitadel rpc error, an add-on body). Every other 5xx message is a
// sentence its handler wrote, and some carry safety ("the change may have
// applied"), so those stay. ponytail: a code list is the ceiling; the upgrade
// is a writeUpstream(w, status, code, err) at each call site, which needs the
// files other waves own.
var rawUpstreamCodes = map[string]bool{
	"ZITADEL_ERROR": true, "SWEEP_ERROR": true, "UPSTREAM_ERROR": true,
	"DRAIN_ERROR": true, "RECONCILE_FAILED": true, "ADOPTION_FAILED": true,
	"RELEASE_FAILED": true, "ADDON_UNREACHABLE": true,
}

// jsonErrorResponse writes the envelope. A 500, and any upstream failure whose
// message is raw upstream text, never shows that text: it goes to the log with
// a reference, and the screen gets a plain sentence and the same reference in
// details.request_id, so the cause stays one grep away.
func jsonErrorResponse(w http.ResponseWriter, status int, errStr, msg string) {
	if status == http.StatusInternalServerError || (status > 500 && rawUpstreamCodes[errStr]) {
		ref := supportReference()
		log.Printf("[HTTP] error status=%d ref=%s code=%s: %s", status, ref, errStr, msg)
		jsonResponse(w, status, ErrorResponse{
			Error:   errStr,
			Message: upstreamSentence(status),
			Details: map[string]string{"request_id": ref},
		})
		return
	}
	jsonResponse(w, status, ErrorResponse{Error: errStr, Message: msg})
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
