package addons

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"syndra/internal/db"
)

// Settling a write (feature calls, design §4 physical rule 3): GET
// /calls/{call_id} answers what the add-on recorded for a write, and is how a
// physical operation whose answer was lost settles without anybody sending it
// again. One reader, CallAnswer, for the member's poll and the operator's list
// alike.

// InDoubtBound is constants.in_doubt_bound: how long a physical call may stay
// in doubt before it is a person's item (Waiting on you, naming the desk).
// Held to the registry by TestInDoubtBoundIsTheContracts.
const InDoubtBound = time.Hour

// CallMemory is constants.call_memory: how long an add-on remembers a call,
// at least. Past it, its unknown is forgetting, not "no effect". Held to the
// registry by TestInDoubtBoundIsTheContracts.
const CallMemory = 30 * 24 * time.Hour

// The three answers of schema/call.json.
const (
	CallRecorded = "recorded"
	CallInDoubt  = "in_doubt"
	CallUnknown  = "unknown"
)

// Settlement is one GET /calls answer as Syndra may act on it.
type Settlement struct {
	// State is recorded (it happened; Reply is the first call's answer),
	// in_doubt (the add-on is still finding out) or unknown (no effect was
	// recorded: the call never arrived or was refused before any write).
	State string
	// Reply is the recorded reply, nil when redacted: the add-on left it out
	// (secret_result) or the registry marks the operation's reply secret, in
	// which case Syndra drops it unread.
	Reply json.RawMessage
}

// ErrNotACallAnswer is a /calls reply that is not one of the three answers
// for the call asked about: no reading, so nothing is concluded from it.
var ErrNotACallAnswer = errors.New("addon: not an answer for this call")

// CallAnswer asks the add-on what it recorded for the call id, a write of the
// operation op. Only an add-on declaring calls is asked (send refuses the
// route otherwise, sending nothing). Any error is "could not tell": the row
// stays as it is.
func CallAnswer(ctx context.Context, target, op, callID string) (Settlement, error) {
	a, err := Get(target)
	if err != nil {
		return Settlement{}, err
	}
	resp, err := a.send(ctx, sendSpec{Method: http.MethodGet, Path: "/calls/" + url.PathEscape(callID)})
	if err != nil {
		return Settlement{}, err
	}
	if resp.Outcome != OutcomeSucceeded {
		return Settlement{}, fmt.Errorf("addon %s: /calls answered %d: %w", target, resp.Status, resp.Err)
	}
	var ans struct {
		CallID *string          `json:"call_id"`
		State  string           `json:"state"`
		Reply  *json.RawMessage `json:"reply"`
	}
	dec := json.NewDecoder(bytes.NewReader(resp.Body))
	if err := dec.Decode(&ans); err != nil {
		return Settlement{}, fmt.Errorf("%w: %v", ErrNotACallAnswer, err)
	}
	// About this call, one of the three, and a reply only with recorded
	// (schema/call.json): anything else says two things about one call.
	switch {
	case ans.CallID == nil || *ans.CallID != callID:
		return Settlement{}, fmt.Errorf("%w: it names another call", ErrNotACallAnswer)
	case ans.State != CallRecorded && ans.State != CallInDoubt && ans.State != CallUnknown:
		return Settlement{}, fmt.Errorf("%w: state %q", ErrNotACallAnswer, ans.State)
	case ans.State != CallRecorded && ans.Reply != nil:
		return Settlement{}, fmt.Errorf("%w: a reply with %s", ErrNotACallAnswer, ans.State)
	}
	out := Settlement{State: ans.State}
	if ans.Reply != nil && string(*ans.Reply) != "null" && !secretResult(op) {
		out.Reply = *ans.Reply
	}
	return out, nil
}

// secretResult is whether op's reply carries a secret: the policy's word
// alone (a manifest cannot make a secret reply keepable), and an id the policy
// does not know is treated as one whose reply is.
func secretResult(op string) bool {
	p, ok := operationPolicy[op]
	return !ok || p.SecretResult
}

// PhysicalHold is the hold every Waiting-on-you read passes (the list, its
// count, Home): a physical call is the add-on's to settle for InDoubtBound,
// and only then a person's. One value, so the list and the count agree.
func PhysicalHold() db.Hold { return db.Hold{Operations: PhysicalOperations(), For: InDoubtBound} }
