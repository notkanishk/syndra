package conformance

import (
	"bytes"
	"net/http"
	"net/url"
	"slices"
)

// Feature calls: GET /calls/{call_id} and the physical operations it settles.
// What a black-box kit can prove is here; what it cannot (a reply lost
// mid-call, two calls racing a limit) is the add-on's own test, and those
// checks say so.

// callChecks are the checks of feature calls; each skips without it.
var callChecks = []string{"calls.unknown", "calls.recorded", "operations.physical", "calls.unsettled", "calls.serialised"}

const notDeclared = "feature calls not declared"

// callsRead is the reads-only half: an id never used answers unknown, valid
// against call.json. Without the feature every calls check skips here.
func (k *kit) callsRead(c *check) {
	if !k.m.has("calls") {
		for _, id := range callChecks {
			k.at(id).skip(notDeclared)
		}
		k.physicalUndeclared(k.at("operations.physical"))
		return
	}
	k.neverUsed(c)
}

// neverUsed asks /calls about an id nobody sent: unknown, and no reply.
func (k *kit) neverUsed(c *check) {
	id := k.cfg.Subject + "-never-used-" + canary()[len("conformance-canary-"):]
	doc := k.ok(c, k.send(request{method: http.MethodGet, target: "/calls/" + url.PathEscape(id)}), "call.json")
	if doc != nil {
		c.expect(str(doc, "state") == "unknown" && doc["reply"] == nil, "/calls on an id never used answered state %q", str(doc, "state"))
		c.expect(str(doc, "call_id") == id, "/calls answered for call_id %q, asked %q", str(doc, "call_id"), id)
	}
}

// physicalUndeclared fails a physical operation declared without feature
// calls: it could be neither previewed nor settled.
func (k *kit) physicalUndeclared(c *check) {
	for _, id := range k.physicalOps(false) {
		c.fail("%s is physical and declared without feature calls", id)
	}
}

// physicalOps is every declared effect: physical operation, in id order;
// available only, when asked.
func (k *kit) physicalOps(available bool) []string {
	var out []string
	for id, ok := range k.m.ops {
		if k.c.operations[id].Effect == "physical" && (ok || !available) {
			out = append(out, id)
		}
	}
	slices.Sort(out)
	return out
}

// callsWrite is the -mutate half, after the operations step: the
// password.set's call_id answers recorded with its first reply, and one
// physical operation is previewed, confirmed, repeated and replayed.
func (k *kit) callsWrite() {
	if !k.m.has("calls") {
		return
	}
	why := "a black-box kit cannot lose a reply or know the add-on's limit; the add-on's own test holds it"
	k.at("calls.unsettled").skip("%s", why)
	k.at("calls.serialised").skip("%s", why)
	k.recorded(k.at("calls.recorded"))
	k.physical(k.at("operations.physical"))
}

func (k *kit) recorded(c *check) {
	if k.passwordCall == "" {
		c.skip("no password.set was answered 200, so no write's call_id is known")
		return
	}
	k.callAnswers(c, k.passwordCall, k.passwordReply)
}

// callAnswers asserts /calls says recorded for id, with first's reply.
func (k *kit) callAnswers(c *check, id string, first []byte) {
	doc := k.ok(c, k.send(request{method: http.MethodGet, target: "/calls/" + url.PathEscape(id)}), "call.json")
	if doc == nil {
		return
	}
	if c.expect(str(doc, "state") == "recorded", "/calls/%s answered state %q, want recorded", id, str(doc, "state")) {
		reply, _ := doc["reply"].(map[string]any)
		c.expect(same(reply, (&exchange{body: first}).doc()), "/calls/%s answered a reply that is not the first one: %v", id, doc["reply"])
	}
}

// physical previews, confirms, repeats and replays one physical operation on
// the item the operator named, which it leaves checked out to the kit's
// subject (Report.Left says so).
func (k *kit) physical(c *check) {
	for _, id := range k.physicalOps(false) {
		if !k.m.confirm[id] { // a failure, never a pass: without -item nothing else is judged
			c.fail("%s is physical and declared without confirm: true", id)
		}
	}
	ops := k.physicalOps(true)
	switch {
	case len(ops) == 0:
		c.skip("no physical operation is declared available")
		return
	case k.cfg.Item == "":
		c.skip("name a test item with -item; it will be checked out to the kit's subject")
		return
	}
	op := ops[0] // ponytail: the first; one -item names one operation's thing
	var param string
	for _, p := range k.c.operations[op].Params {
		if p.Required {
			param = p.Name
			break
		}
	}
	params := map[string]any{param: k.cfg.Item}
	route := "/operations/" + op
	send := k.viaOwned(c)
	result := func(ex *exchange, want, what string) map[string]any {
		doc := k.ok(c, ex, "operation_response.json")
		if doc == nil {
			return nil
		}
		c.expect(str(doc, "outcome") == want, "%s answered outcome %q, want %s", what, str(doc, "outcome"), want)
		if schema := k.c.operations[op].Result; schema != "" {
			r, ok := doc["result"].(map[string]any)
			if c.expect(ok, "%s answered no result", what) {
				if err := k.c.validate(schema, r); err != nil {
					c.fail("%s: the result %v", what, err)
				}
			}
		}
		return doc
	}

	preview := k.operationBody(op, "", params)
	delete(preview, "call_id")
	preview["preview"] = true
	before, okBefore := k.logRecords(c)
	result(send(request{method: http.MethodPost, target: route, body: preview, schema: "operation_request.json"}), "succeeded", "the preview")
	k.unwritten(c, before, okBefore, "the preview")
	k.neverUsed(c) // a preview left no call behind under any id

	callID := k.callID()
	before, okBefore = k.logRecords(c)
	confirm := send(request{method: http.MethodPost, target: route, body: k.operationBody(op, callID, params), schema: "operation_request.json"})
	if result(confirm, "succeeded", "the confirm") == nil {
		return
	}
	k.lent = k.cfg.Item
	k.advanced(c, before, okBefore, "the confirm")
	k.callAnswers(c, callID, confirm.body)

	before, okBefore = k.logRecords(c)
	result(send(request{method: http.MethodPost, target: route, body: k.operationBody(op, k.callID(), params), schema: "operation_request.json"}),
		"no_change", "the same item under a new call_id")
	k.unwritten(c, before, okBefore, "the same item again")

	replay := send(request{method: http.MethodPost, target: route, body: k.operationBody(op, callID, params), schema: "operation_request.json"})
	c.expect(replay.status == confirm.status && bytes.Equal(replay.body, confirm.body),
		"the confirm's call_id again answered %d %s, not the recorded reply byte for byte", replay.status, replay.code())
}
