package reference

import (
	"encoding/json"
	"net/http"
	"slices"
	"time"
)

// Feature calls: one physical operation, item.checkout, over a small lending
// catalogue, and GET /calls/{call_id}, which says what was recorded for any
// write (AUTHORING.md, "Physical operations and GET /calls"). The rules, each a
// MUST the add-on's own tests hold (calls_test.go):
//
//  1. A preview (preview: true, no call_id) answers the would-be record and
//     writes nothing: no log record, no call entry.
//  2. At most once. A confirm writes an intent before the loan and closes it
//     after; while a subject has an open intent, every checkout for that
//     subject, under any call_id, is 409 CALL_UNSETTLED. Nothing re-executes
//     a call whose intent has no result.
//  3. Settled through /calls: an open intent is settled by reading the target
//     (the loan is there: recorded; it is not: unknown), never while the
//     target cannot be read (in_doubt).
//  4. Naturally idempotent: an item the subject already holds is no_change,
//     with the record, and no log record; one someone else holds is a refusal.
//  5. Serialised per subject, so two checkouts cannot both pass the limit.
//  6. Remembered: through once, for the life of the process (ponytail: a real
//     add-on keeps call records on disk for constants.call_memory).

// item is one thing on the shelf; its category is a printer, so what a member
// may borrow follows the printers they may use.
type item struct{ id, label, category string }

var shelf = []item{
	{"item-0001", "Nozzle set, 0.4 mm", "mk4-a"},
	{"item-0002", "Textured build plate", "mk4-a"},
	{"item-0003", "Spool holder", "mk4-b"},
	{"item-0004", "Filament dryer", "xl-1"},
}

// loanLimit is how many items a member may hold at once.
//
// ponytail: a constant; a limit field in the manifest when a kit check needs one.
const loanLimit = 1

const loanPeriod = 7 * 24 * time.Hour

type loan struct {
	subject    string
	since, due time.Time
}

// intent is a confirmed checkout written before the loan, closed once its
// result is known. Open, it is the mutation-log intent with no result.
type intent struct {
	callID, subject, item, digest string
}

func shelved(id string) (item, bool) {
	i := slices.IndexFunc(shelf, func(it item) bool { return it.id == id })
	if i < 0 {
		return item{}, false
	}
	return shelf[i], true
}

// checkout is item.checkout, a preview or a confirm. The confirm runs through
// once like every write; the preview runs the same gates and writes nothing.
func (a *Addon) checkout(req operationRequest) reply {
	itemID, _ := req.Params["item_id"].(string)
	preview := req.Preview != nil && *req.Preview
	switch {
	case blank(req.Subject):
		return problem(400, "BAD_REQUEST", "subject is required.", nil)
	case preview && req.CallID != "":
		// A preview is never recorded, so an id on one would name nothing.
		return problem(400, "BAD_REQUEST", "A preview carries no call_id.", nil)
	case !preview && blank(req.CallID):
		return problem(400, "BAD_REQUEST", "call_id is required.", nil)
	case blank(itemID) || len(req.Params) != 1:
		return problem(400, "OPERATION_FAILED", "item.checkout takes one parameter, item_id.", nil)
	}
	if preview {
		if refused := a.reachableAndActive(); refused != nil {
			return *refused
		}
		a.mu.Lock()
		defer a.mu.Unlock()
		rec, held, refused := a.decideLoan(req.Subject, itemID)
		if refused != nil {
			return *refused
		}
		return loanReply(req, rec, held, "This is what checking it out would record. Nothing was done.")
	}
	identity := map[string]any{"name": req.Operation, "subject": req.Subject, "params": req.Params}
	return a.once(req.CallID, "operation", identity, func() reply {
		if refused := a.reachableAndActive(); refused != nil {
			return *refused
		}
		defer a.serialise(req.Subject)()
		// One section from the checks to the loan: no other subject's
		// checkout can take the item between them.
		a.mu.Lock()
		rec, held, refused := a.decideLoan(req.Subject, itemID)
		if refused == nil && !held {
			a.gap() // defects.go: nothing, unless checkout-unserialised
			a.intents[req.Subject] = &intent{callID: req.CallID, subject: req.Subject, item: itemID, digest: identityDigest(identity)}
			if a.lose("before") {
				a.mu.Unlock()
				return problem(500, "OPERATION_FAILED", "The checkout's answer was lost.", nil)
			}
			since := time.Now().UTC()
			a.loans[itemID] = &loan{subject: req.Subject, since: since, due: since.Add(loanPeriod)}
			a.record("item.checkout " + itemID + " " + req.Subject)
			rec = a.loanRecord(itemID)
		}
		a.mu.Unlock()
		switch {
		case refused != nil:
			return *refused
		case held:
			// Already this subject's: nothing written, and no log record.
			return loanReply(req, rec, true, "You already have this item. Nothing changed.")
		}
		if a.lose("after") {
			return problem(500, "OPERATION_FAILED", "The checkout's answer was lost.", nil)
		}
		a.mu.Lock()
		delete(a.intents, req.Subject) // the result is the reply once records
		a.mu.Unlock()
		return loanReply(req, rec, false, "Checked out.")
	})
}

// reachableAndActive refuses a checkout the add-on cannot make now: never
// queued, because the member is at the shelf now or not.
func (a *Addon) reachableAndActive() *reply {
	if refused := a.refuseUnlessActive(); refused != nil {
		return refused
	}
	if a.cfg.Down != nil && a.cfg.Down() {
		r := problem(503, "TARGET_UNREACHABLE", "The lending ledger cannot be reached; nothing was done.", nil)
		return &r
	}
	return nil
}

// decideLoan is every check before the app is written, in the order a member
// can act on: an unsettled earlier call, an account, the item, the category,
// someone else's loan, their own loan, the limit. held is the subject holding
// it already; rec is the record made, or the one that would be. Under a.mu.
func (a *Addon) decideLoan(subject, itemID string) (rec record, held bool, refused *reply) {
	refuse := func(r reply) (record, bool, *reply) { return record{}, false, &r }
	if a.openIntent(subject) != nil {
		return refuse(problem(409, "CALL_UNSETTLED", "An earlier checkout for this member is still being settled, so this one was not run.", nil))
	}
	acct := a.accounts[a.bindings[subject]]
	if acct == nil {
		return refuse(problem(422, "OPERATION_FAILED", "This subject has no account here.", nil))
	}
	it, known := shelved(itemID)
	switch {
	case !known:
		return refuse(problem(404, "UNKNOWN_RESOURCE", "No item has that id.", map[string]any{"param": "item_id"}))
	case acct.state["enabled"] != true || !slices.Contains(acct.state["printers"].([]string), it.category):
		return refuse(problem(403, "NOT_ENTITLED", "This member may not borrow from that item's category.", map[string]any{"param": "item_id"}))
	}
	if l := a.loans[itemID]; l != nil && l.subject != subject {
		return refuse(problem(409, "RESOURCE_TAKEN", "Another member has this item.", map[string]any{"param": "item_id"}))
	} else if l != nil {
		return a.loanRecord(itemID), true, nil
	}
	if n := a.holding(subject); n >= loanLimit {
		return refuse(problem(409, "LIMIT_REACHED", "This member already holds as many items as they may.",
			map[string]any{"limit": loanLimit, "unit": "items", "count": n}))
	}
	since := time.Now().UTC()
	return record{ID: it.id, Label: it.label, Category: it.category, Since: stamp(since), Due: stamp(since.Add(loanPeriod))}, false, nil
}

func (a *Addon) holding(subject string) int {
	n := 0
	for _, l := range a.loans {
		if l.subject == subject {
			n++
		}
	}
	return n
}

// loanRecord is common/records.json's record for a loan. Under a.mu.
func (a *Addon) loanRecord(itemID string) record {
	it, _ := shelved(itemID)
	l := a.loans[itemID]
	return record{ID: it.id, Label: it.label, Category: it.category, Since: stamp(l.since), Due: stamp(l.due)}
}

// loans is what the subject holds, oldest id first. Under a.mu.
func (a *Addon) loansOf(subject string) []record {
	var out []record
	for _, it := range shelf {
		if l := a.loans[it.id]; l != nil && l.subject == subject {
			out = append(out, a.loanRecord(it.id))
		}
	}
	return out
}

func loanReply(req operationRequest, rec record, held bool, detail string) reply {
	outcome := "succeeded"
	if held {
		outcome = "no_change"
	}
	return ok(map[string]any{"operation": req.Operation, "subject": req.Subject, "outcome": outcome, "detail": detail,
		"result": map[string]any{"records": []record{rec}, "taken_at": now(), "current": true}})
}

func stamp(t time.Time) string { return t.UTC().Format(time.RFC3339) }

// lose is Config.LoseReply: test-only, the answer lost before or after the
// loan was written, its intent left open.
func (a *Addon) lose(stage string) bool { return a.cfg.LoseReply != nil && a.cfg.LoseReply(stage) }

// callState is GET /calls/{call_id} (feature calls): what this add-on
// recorded for a write of any kind. recorded, with the first reply byte for
// byte; in_doubt while it runs or its intent is open; unknown otherwise (never
// seen, or refused: only a 2xx is recorded).
func (a *Addon) callState(r *http.Request, _ []byte) reply {
	if !a.serves("calls") {
		return problem(404, "UNKNOWN_ROUTE", "This add-on has no such route.", nil)
	}
	id := r.PathValue("call_id")
	a.settle(id)
	a.mu.Lock()
	defer a.mu.Unlock()
	doc := map[string]any{"call_id": id, "state": "unknown"}
	if c, seen := a.calls[id]; seen && c.busy {
		doc["state"] = "in_doubt"
	} else if seen {
		doc["state"], doc["reply"] = "recorded", json.RawMessage(c.reply.body)
	} else if a.intentByCall(id) != nil {
		doc["state"] = "in_doubt"
	}
	// ponytail: no registered id has secret_result, so nothing is redacted
	// here; one that has it is recorded without its secret, and so answered.
	return ok(doc)
}

// settle concludes an open intent by reading the target: the loan is there,
// so the call happened and its reply is recorded under its call_id; it is
// not, so nothing happened and the intent is dropped. Never while the target
// cannot be read, and never while the call is still running.
func (a *Addon) settle(callID string) {
	if a.cfg.Down != nil && a.cfg.Down() {
		return
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	in := a.intentByCall(callID)
	if in == nil || (a.calls[callID] != nil && a.calls[callID].busy) {
		return
	}
	delete(a.intents, in.subject)
	if l := a.loans[in.item]; l != nil && l.subject == in.subject {
		rec := a.loanRecord(in.item)
		a.calls[callID] = &call{kind: "operation", digest: in.digest, reply: loanReply(
			operationRequest{Operation: "item.checkout", Subject: in.subject}, rec, false, "Checked out.")}
	}
}

func (a *Addon) intentByCall(callID string) *intent {
	for _, in := range a.intents {
		if in.callID == callID {
			return in
		}
	}
	return nil
}
