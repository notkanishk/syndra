package addonop

import (
	"context"
	"errors"
	"testing"

	"syndra/internal/db"
)

// A-13: the page's one automatic resend is the same change sent again, not a
// second one, so the limiter must not be what answers it: a member at their
// limit whose reply was lost still gets the resend.
func TestTheAutomaticResendDoesNotSpendTheRateLimit(t *testing.T) {
	t.Setenv("ADDON_MEMBER_OP_LIMIT", "3")
	h := newHarness(t)
	h.recentOps = 3

	req := passwordSet()
	req.RetryOf = "rec-first"
	if _, err := Dispatch(context.Background(), req); err != nil {
		t.Fatalf("the resend must go through at the limit: %v", err)
	}
	if len(h.rateQueries) != 0 {
		t.Errorf("the resend must not be counted against the limit: %v", h.rateQueries)
	}
	if len(h.begun) != 1 || h.begun[0].RetryOf != "rec-first" {
		t.Fatalf("the record must link the resend to its first send: %+v", h.begun)
	}
	if len(h.calls) != 1 || h.calls[0].Params["password"] != theSecret {
		t.Fatal("the resend carries the same value, once")
	}
}

// A refused resend (a second one, or one naming another call) sends nothing.
func TestARefusedResendSendsNothing(t *testing.T) {
	h := newHarness(t)
	h.beginErr = db.ErrNotResendable

	req := passwordSet()
	req.RetryOf = "rec-first"
	_, err := Dispatch(context.Background(), req)
	if !errors.Is(err, db.ErrNotResendable) || len(h.calls) != 0 {
		t.Fatalf("a refused resend must be reported and must not reach the add-on: %v, %d calls", err, len(h.calls))
	}
}

// `about` is written only for the four operations that carry no secret. A
// caller that hands a password operation one has it dropped, whatever it is.
func TestAPasswordOperationsRecordCarriesNoAbout(t *testing.T) {
	h := newHarness(t)
	req := passwordSet()
	req.About = theSecret
	if _, err := Dispatch(context.Background(), req); err != nil {
		t.Fatal(err)
	}
	if len(h.begun) != 1 || h.begun[0].About != "" {
		t.Fatalf("a password operation's record must never carry about: %+v", h.begun)
	}
	for _, op := range []string{"password.set", "password.rotate"} {
		if aboutOperations[op] {
			t.Errorf("%s must not be able to carry about", op)
		}
	}
}
