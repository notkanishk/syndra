package addons

import (
	"bytes"
	"slices"
	"strings"
	"testing"
)

// Syndra reads a reply's names as the add-on reads a request's: exactly
// (RFC 8259 §4) and once (RFC 7493 §2.3), through DecodeReply. A reply may
// carry more (AUTHORING.md chapter 14, rule 1), so a name Syndra does not
// know is skipped, and a case variant of one it does know is exactly such a
// name: skipped, never read as that name, as encoding/json alone would.

func TestAReplysCaseVariantIsSkippedNeverRead(t *testing.T) {
	// `Effect` is not `effect`: the reply names no effect, so it said
	// nothing Syndra can record (indeterminate), never "applied".
	out := applyAgainst(t, 200, []byte(`{"subject":"sub-1","Effect":"applied","username":"charlotte"}`))
	if out.Outcome != OutcomeIndeterminate || out.Effect != "" {
		t.Errorf("Effect was read as effect: %+v", out)
	}

	// `Kind` is not `kind`: no kind, and the manifest lists Kind as a key
	// this backend does not know.
	m, err := decodeManifest(bytes.Replace(contractFixture(t, "manifest_truenas.json"), []byte(`"kind"`), []byte(`"Kind"`), 1))
	if err != nil {
		t.Fatal(err)
	}
	if m.Kind != "" || !slices.ContainsFunc(m.Unknown, func(u string) bool { return strings.HasPrefix(u, `"Kind":`) }) {
		t.Errorf("Kind was read as kind: kind %q, unknown %v", m.Kind, m.Unknown)
	}

	// A refusal's `Code` is not its `code`.
	if code, _ := refusalOf([]byte(`{"Code":"PLAN_STALE","error":"BAD_REQUEST"}`)); code != "BAD_REQUEST" {
		t.Errorf("Code was read as code: %q", code)
	}
}

func TestAReplyNamingAMemberTwiceIsNotRead(t *testing.T) {
	// Which of two effects did it mean? Neither is recorded: indeterminate,
	// as any 2xx whose body does not decode.
	out := applyAgainst(t, 200, []byte(`{"subject":"sub-1","effect":"blocked","effect":"applied"}`))
	if out.Outcome != OutcomeIndeterminate || out.Err == nil {
		t.Errorf("a duplicate effect was read: %+v", out)
	}
	// A duplicate inside a member Syndra skips is still a duplicate.
	out = applyAgainst(t, 200, []byte(`{"subject":"sub-1","effect":"applied","x-truenas-note":{"a":1,"a":2}}`))
	if out.Outcome != OutcomeIndeterminate {
		t.Errorf("a duplicate inside a skipped member was read: %+v", out)
	}

	fixture := contractFixture(t, "manifest_truenas.json")
	if _, err := decodeManifest(bytes.Replace(fixture, []byte(`"kind"`), []byte(`"kind":"identity","kind"`), 1)); err == nil {
		t.Error("a manifest naming its kind twice was accepted")
	}

	// A read's `result` twice is no reading.
	r := ReadReply{Operation: "health.get", Outcome: OutcomeSucceeded,
		Body: []byte(`{"outcome":"succeeded","result":{"current":true},"result":{"current":false}}`)}
	if _, err := r.answer(); err == nil {
		t.Error("a read answered under result twice was read")
	}
}

func TestAReplyCarryingMoreStillReads(t *testing.T) {
	out := applyAgainst(t, 200, []byte(`{"subject":"sub-1","effect":"applied","username":"charlotte",
		"x-truenas-note":{"Effect":"blocked"},"standing":{"usable":true,"waiting_on":"none","x-extra":1}}`))
	if out.Outcome != OutcomeSucceeded || out.Effect != "applied" || out.Standing == nil || !out.Standing.Usable {
		t.Errorf("a reply carrying more was not read: %+v", out)
	}
}
