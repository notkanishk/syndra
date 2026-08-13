package main

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
)

// The contract with the client library, pinned against recorded traffic.
//
// This is the test the whole add-on was missing. Every other test drives a fake
// that returns whatever shape the code expects, so the code and the fake can
// agree with each other and both be wrong about the wire — which is what
// happened: `Call` hands back the WHOLE JSON-RPC message and the add-on decoded
// it as if it were the result, so `system.version` failed to decode, every
// mutation was refused by the version gate that failure left empty, and any
// refusal of a call wanting no result read as a success.
//
// The fixtures below are real middleware responses, kept verbatim.

const (
	// TrueNAS SCALE 25.04.2, `system.version`.
	recordedVersionResponse = `{"jsonrpc":"2.0","id":1,"result":"25.04.2.1"}`
	// A refusal. The message is the middleware's own, and it is here to be
	// asserted ABSENT from what this package returns.
	recordedRefusalResponse = `{"jsonrpc":"2.0","id":2,"error":{"code":-32001,` +
		`"message":"[EINVAL] user_update.password: Password must be at least 8 characters",` +
		`"data":{"error":22,"errname":"EINVAL","reason":"user_update.password"}}}`
)

// recordedRPC replays a fixture verbatim, with no envelope help of its own.
type recordedRPC struct {
	response string
	calls    int
}

func (r *recordedRPC) Call(string, int64, any) (json.RawMessage, error) {
	r.calls++
	return json.RawMessage(r.response), nil
}
func (r *recordedRPC) Ping() (string, error) { return "pong", nil }
func (r *recordedRPC) Close() error          { return nil }

func recordedNAS(response string) *NAS {
	n := newNAS(func() (rpc, error) { return &recordedRPC{response: response}, nil }, []string{"25.04"})
	// Probing is what the fixture is FOR in the version test; the others want
	// the one call they make and nothing else.
	n.probed = true
	return n
}

// The result is inside the envelope, not the envelope itself.
func TestAResultIsReadFromInsideTheEnvelope(t *testing.T) {
	n := recordedNAS(recordedVersionResponse)
	n.probed = false

	v, err := n.SystemVersion()
	if err != nil {
		t.Fatalf("a real response must decode: %v", err)
	}
	if v != "25.04.2.1" {
		t.Fatalf("want the result, got %q", v)
	}
	// And the version gate that every mutation passes through is satisfied by
	// it, which is the half that made the failure total rather than cosmetic.
	if ok, why := n.MajorSupported(); !ok {
		t.Fatalf("a supported major must pass the gate: %s", why)
	}
}

// The one that mattered: every mutation this add-on issues passes out == nil,
// so a refusal noticed only when a result was wanted is a refusal never
// noticed. `Call` returns err == nil for a message carrying an error member.
func TestARefusalIsAFailureEvenWhenNoResultIsWanted(t *testing.T) {
	n := recordedNAS(recordedRefusalResponse)

	err := n.call("user.update", []any{1, map[string]any{"password": "x"}}, nil)
	if err == nil {
		t.Fatal("a refused mutation must not be reported as applied")
	}
	if !errors.Is(err, ErrTargetRefused) {
		t.Fatalf("want a refusal, got %v", err)
	}
}

// The middleware echoes the parameter it objected to, and on this call the
// parameters are a member's password. What leaves this package is a
// classification and a numeric code.
func TestTheTargetsOwnErrorTextNeverLeavesTheClient(t *testing.T) {
	n := recordedNAS(recordedRefusalResponse)

	err := n.call("user.update", []any{1, map[string]any{"password": "x"}}, nil)
	if err == nil {
		t.Fatal("the call must fail")
	}
	for _, leaked := range []string{"Password must be at least 8 characters", "user_update.password", "EINVAL"} {
		if strings.Contains(err.Error(), leaked) {
			t.Errorf("the target's own text must not reach the caller: %q", err.Error())
		}
	}
	if !strings.Contains(err.Error(), "-32001") {
		t.Errorf("the code is the handle an operator gets: %q", err.Error())
	}
}

// A refusal on the elevated purge session goes through the same reading. It is
// the one operation that cannot be undone, which makes "the target said no"
// the least optional thing to notice.
func TestTheElevatedCallReadsItsRefusalToo(t *testing.T) {
	c := &recordedRPC{response: recordedRefusalResponse}
	if err := callOnce(c, "user.delete", []any{1}, nil); !errors.Is(err, ErrTargetRefused) {
		t.Fatalf("want a refusal, got %v", err)
	}
}

// A version read that never happened must not leave the gate open. The probe
// runs per connection, so an add-on that started before its NAS recovers on
// its own rather than refusing every mutation until it is restarted.
func TestTheVersionIsReProbedOnANewConnection(t *testing.T) {
	answering := false
	dials := 0
	n := newNAS(func() (rpc, error) {
		dials++
		if !answering {
			return nil, errors.New("connection refused")
		}
		return &recordedRPC{response: recordedVersionResponse}, nil
	}, []string{"25.04"})
	at := time.Unix(1_700_000_000, 0)
	n.now = func() time.Time { return at }

	if ok, _ := n.MajorSupported(); ok {
		t.Fatal("an unread version must not pass the gate")
	}
	answering = true
	at = at.Add(reconnectCooldown + time.Second)

	if err := n.call("user.query", []any{}, nil); err != nil {
		t.Fatalf("the call must succeed once the target answers: %v", err)
	}
	if ok, why := n.MajorSupported(); !ok {
		t.Fatalf("the connection that worked must have re-probed: %s", why)
	}
}

// A failed probe must be retried, and not on every call. It runs on the one
// rate-limited session every operation shares, so a target that answers
// everything except `system.version` would otherwise carry double the traffic
// for as long as it stayed that way.
func TestAFailedProbeIsRetriedOnACooldownRatherThanEveryCall(t *testing.T) {
	c := &recordedRPC{response: `{"jsonrpc":"2.0","id":1,"error":{"code":-32601,"message":"unknown method"}}`}
	n := newNAS(func() (rpc, error) { return c, nil }, []string{"25.04"})
	at := time.Unix(1_700_000_000, 0)
	n.now = func() time.Time { return at }

	for range 5 {
		_ = n.call("user.query", []any{}, nil)
	}
	// Five calls, one probe attempt: the probe and the calls themselves.
	if c.calls != 6 {
		t.Fatalf("want one probe across five calls, got %d wire calls", c.calls)
	}
	if ok, _ := n.MajorSupported(); ok {
		t.Fatal("an unread version must not pass the gate")
	}

	// And past the cooldown it tries again, so a target that starts answering
	// becomes usable without a reconnect.
	at = at.Add(probeCooldown + time.Second)
	before := c.calls
	_ = n.call("user.query", []any{}, nil)
	if c.calls != before+2 {
		t.Fatalf("want the probe retried after the cooldown, got %d new calls", c.calls-before)
	}
}

// The version strings real TrueNAS releases actually answer with.
//
// `TrueNAS-25.10.5` is not a guess: it is what `nas.example.org`
// returned on the first connection to a real NAS, 2026-08-13. Splitting on "."
// from the left made that `TrueNAS-25.10`, which matches no entry in
// TRUENAS_SUPPORTED_MAJORS — so the add-on refused every mutation against a
// release listed as supported, and said "outside the tested range" while naming
// a major nobody had written down.
//
// The recorded fixture in this file carries a bare `25.04.2.1`, which is why no
// test could see it: the parser and the fake agreed with each other, and the
// target was the only thing that disagreed.
func TestMajorOfHandlesTheVersionStringsTrueNASActuallyReturns(t *testing.T) {
	for version, want := range map[string]string{
		"TrueNAS-25.10.5":       "25.10", // observed, nas.example.org
		"TrueNAS-SCALE-24.04.0": "24.04", // the older SCALE prefix
		"25.04.2.1":             "25.04", // the bare form this suite recorded
		"TrueNAS-13.0-U6":       "13.0",  // CORE, for the shape rather than support
	} {
		if got := majorOf(version); got != want {
			t.Errorf("majorOf(%q) = %q, want %q", version, got, want)
		}
	}
}

// A string with no release number in it hands back what was read, so the
// refusal quotes the target's own answer instead of an empty string an operator
// cannot search for.
func TestMajorOfKeepsAnUnparseableVersionVisible(t *testing.T) {
	if got := majorOf("  nightly  "); got != "nightly" {
		t.Errorf("majorOf = %q, want the trimmed input back", got)
	}
}

// The gate itself, against the real string: this is the assertion that would
// have caught it, and the one that keeps it caught.
func TestARealVersionStringPassesTheSupportedGate(t *testing.T) {
	n := newNAS(func() (rpc, error) {
		return &recordedRPC{response: `{"jsonrpc":"2.0","id":1,"result":"TrueNAS-25.10.5"}`}, nil
	}, []string{"25.04", "25.10", "26.04"})
	n.probed = false
	if _, err := n.SystemVersion(); err != nil {
		t.Fatalf("version read: %v", err)
	}
	ok, note := n.MajorSupported()
	if !ok {
		t.Fatalf("a supported release was refused: %s", note)
	}
}
