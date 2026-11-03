package main

import (
	"net/http"
	"testing"
)

// A-13 §4: the member's page resends the same password once, at once, after
// the first send's reply was lost. The resend is a new call id, so it is a new
// call here, not a replay, and nothing about the first one (answered or not)
// may refuse it: setting the same value twice converges on that value.
func TestTheSamePasswordSentAgainUnderANewCallIsAccepted(t *testing.T) {
	s, m := opServer(t)

	first := postOperation(t, s, "password.set",
		`{"call_id":"c-first","subject":"sub-1","actor":"sub-1","params":{"password":"`+theSecret+`"}}`)
	second := postOperation(t, s, "password.set",
		`{"call_id":"c-resend","subject":"sub-1","actor":"sub-1","params":{"password":"`+theSecret+`"}}`)
	if first.Code != http.StatusOK || second.Code != http.StatusOK {
		t.Fatalf("both sends must be accepted: %d, %d (%s)", first.Code, second.Code, second.Body.String())
	}
	if len(m.updates) != 2 || m.updates[1]["password"] != theSecret {
		t.Fatalf("the resend must reach the target with the same value: %v", m.updates)
	}
}
