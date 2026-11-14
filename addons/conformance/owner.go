package conformance

import (
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"net/http"
	"slices"
	"strings"
)

// Whose account is it? This file is the kit's only answer. Every reply about
// the kit's subject in a -mutate run (a plan for it, an apply or operation for
// it, the end-of-run plan included) passes through observe, and observe asks
// owner; nothing else in the kit compares account ids.

type ownership int

const (
	unnamed ownership = iota // the reply names no account
	ours                     // the account the kit's own create made
	foreign                  // any other: the kit never writes to it again, and reports it
)

// owner is the one rule.
//
//  1. Until the kit's creating apply has answered 200 with a string
//     account_id that was not there before the run, the kit owns nothing, so
//     any account named for its subject is foreign: the subject was proven
//     unbound, it has none, and a correct add-on cannot name one. That answer
//     (claim) is the one moment the kit comes to own an account, and only
//     while nothing foreign has been seen.
//     The claimed account must also be labelled as the kit's: its username,
//     from the create's reply or else from /subjects (a read, before any
//     further write), begins "conformance-". A member's account that the
//     add-on answers the create with is never the kit's, however new.
//  2. After it, the kit owns exactly that account; any other id is foreign.
//
// owner reads account_id from the reply object itself: only an absent key is
// unnamed. A null, an empty string or any non-string can never be told apart
// from someone else's account, so it is foreign, and never claimable.
func (k *kit) owner(reply map[string]any, claim bool) ownership {
	id, present := reply["account_id"]
	s, isString := id.(string)
	switch {
	case !present:
		return unnamed
	case !isString || s == "":
		return foreign
	case k.created.id != "" && s == k.created.id:
		return ours
	case k.created.id == "" && claim && len(k.foreign) == 0 && !slices.Contains(k.before, s):
		if name := k.username(reply, s); strings.HasPrefix(name, subjectPrefix) {
			k.created.id, k.created.username = s, name
			return ours
		}
	}
	return foreign
}

// username is the name of the account a create answered: from the reply or,
// when the reply gives none, from /subjects.
func (k *kit) username(reply map[string]any, id string) string {
	name, _ := reply["username"].(string)
	if name == "" {
		for _, s := range objects(k.ok(&check{}, k.send(request{method: http.MethodGet, target: "/subjects"}), "subjects.json")["subjects"]) {
			if s["account_id"] == id {
				name, _ = s["username"].(string)
			}
		}
	}
	return name
}

// frozen: the kit writes no more, to any subject. A foreign account, or a plan
// before a write that did not name the kit's own (toOwned), stops it.
func (k *kit) frozen() bool { return len(k.foreign) > 0 || k.stopReason != "" }

// subjectWrite is one write the kit sent to its subject, as the last line
// needs it: when, what (never a secret value), and what came back.
type subjectWrite struct {
	step, what, answer string
	answered2xx        bool
	namedForeign       bool
	namedOurs          bool // the reply named the kit's own account
	afterCreate        bool // sent once the kit owned an account
	creates            bool // the apply meant to create the kit's account
}

// about says whether a request is about the kit's subject, and whether it is
// a write to it: an apply or operation for it, or a plan for it alone.
func (k *kit) about(r request) (about, write bool) {
	body, _ := r.body.(map[string]any)
	switch path, _, _ := strings.Cut(r.target, "?"); {
	case path == "/apply" || strings.HasPrefix(path, "/operations/"):
		// An effect: read operation is no write (contract.writes, the one rule).
		return body["subject"] == k.cfg.Subject, k.c.writes(r.method, r.target)
	case path == "/plan":
		if subjects, _ := body["subjects"].([]any); len(subjects) == 1 {
			s, _ := subjects[0].(map[string]any)
			return s["subject"] == k.cfg.Subject, false
		}
	}
	return false, false
}

// observe holds a reply about the kit's subject to owner() and records the
// write. It runs for every request sent about the subject, answered or not,
// from the moment the subject was proven unbound (k.before is read).
func (k *kit) observe(r request, ex *exchange) {
	about, write := k.about(r)
	if k.before == nil || !about {
		return
	}
	doc := ex.doc()
	if path, _, _ := strings.Cut(r.target, "?"); path == "/plan" { // the subject's account is its one outcome's
		outs := objects(doc["outcomes"])
		doc = nil
		if len(outs) == 1 {
			doc = outs[0]
		}
	}
	answered, afterCreate := ex.err == nil && ex.status/100 == 2, k.created.id != ""
	o := k.owner(doc, r.creates && ex.err == nil && ex.status == http.StatusOK)
	// Once the kit has an account, a plan or an apply answered 2xx that names
	// none says the subject is bound to nothing (an operation's reply carries
	// no account_id by contract, so it says nothing either way).
	if path, _, _ := strings.Cut(r.target, "?"); afterCreate && answered && o == unnamed && !strings.HasPrefix(path, "/operations/") {
		c := k.at("bound-to-created")
		c.ran = true
		c.fails = append(c.fails, fmt.Sprintf("during %s the subject was bound to %s and the reply names no account", k.step, k.created.id))
	}
	if r.creates && o != unnamed {
		k.at("create-is-new").expect(o == ours, "the apply that creates the kit's account answered account_id %s, "+
			"which was there before the run (or is not a usable id): a create bound an account it did not make", display(doc))
	}
	if write {
		w := subjectWrite{step: k.step, what: summary(r), answered2xx: answered, namedForeign: o == foreign, namedOurs: o == ours, afterCreate: afterCreate, creates: r.creates}
		switch {
		case ex.err != nil:
			w.answer = "unanswered"
		case o != unnamed:
			w.answer = fmt.Sprintf("answered %d naming account_id %s", ex.status, display(doc))
		default:
			w.answer = strings.TrimSpace(fmt.Sprintf("answered %d %s", ex.status, ex.code()))
		}
		k.writes = append(k.writes, w)
	}
	switch o {
	case ours:
		k.at("bound-to-created").mark()
	case foreign:
		k.sawForeign(display(doc))
	}
}

// sawForeign freezes every write to the subject (send refuses them, with no
// exception) and fails bound-to-created. The failure is recorded even after
// an earlier freeze: it is what the freeze is about.
func (k *kit) sawForeign(id string) {
	if slices.Contains(k.foreign, id) {
		return
	}
	owned := "it owned no account yet"
	if k.created.id != "" {
		owned = "it owns account_id " + k.created.id
	}
	c := k.at("bound-to-created")
	c.ran = true
	c.fails = append(c.fails, fmt.Sprintf("during %s the kit's subject was answered with account_id %s, which the kit does not own "+
		"(%s): the kit stopped writing", k.step, id, owned))
	k.foreign = append(k.foreign, id)
}

// summary says what a write asked for, never a parameter's value: an
// operation's parameters may be a member's password.
func summary(r request) string {
	body, _ := r.body.(map[string]any)
	path, _, _ := strings.Cut(r.target, "?")
	if path == "/apply" {
		desired, _ := json.Marshal(body["desired"])
		return "apply " + string(desired)
	}
	params, _ := body["params"].(map[string]any)
	return fmt.Sprintf("%s with parameters %v (values not shown)", strings.TrimPrefix(path, "/operations/"), slices.Sorted(maps.Keys(params)))
}

// foreignLine is the last line's account of the accounts the kit does not
// own: everything of the kit's that may have reached them. Conservatively,
// that is every write to the subject whose reply named a foreign account, was
// not a 2xx, or never came, and the create and every write after it whose
// reply did not name the kit's own account (an operation's never does); every write
// after the first foreign id was refused by the kit itself, so none is
// listed. "Did not touch" only when the list is empty.
func (k *kit) foreignLine() string {
	var may []string
	for _, w := range k.writes {
		if w.namedForeign || !w.answered2xx || ((w.afterCreate || w.creates) && !w.namedOurs) {
			may = append(may, fmt.Sprintf("%s: %s (%s)", w.step, w.what, w.answer))
		}
	}
	what := "which the kit does not own and did not touch"
	if len(may) > 0 {
		what = "which the kit does not own (it may have been made by the writes listed); what of the kit's may have reached it: " +
			strings.Join(may, "; ") + "; nothing after that"
	}
	return fmt.Sprintf("subject %q was answered with account_id %s, %s: check it", k.cfg.Subject, strings.Join(k.foreign, ", "), what)
}

// display is an account_id as the report shows it: the id itself, or its JSON
// (null, "", a number) when it is not a usable id.
func display(reply map[string]any) string {
	if s, ok := reply["account_id"].(string); ok && s != "" {
		return s
	}
	raw, _ := json.Marshal(reply["account_id"])
	return string(raw)
}

// names says whether a reply object carries account_id at all: the contract's
// "an account exists" (standing travels with it).
func names(reply map[string]any) bool {
	_, present := reply["account_id"]
	return present
}

// exists reads the plan for the kit's subject before the first write, by
// presence, never by value: an account_id of any type, a standing, a conflict,
// or any effect but "apply" for a desired state that creates, all mean
// something is already there, and the kit refuses to write.
func exists(pre map[string]any) bool {
	_, standing := pre["standing"]
	_, conflict := pre["conflict"]
	return names(pre) || standing || conflict || pre["effect"] != "apply"
}

// idsOf lists the account ids /subjects reports. An id that is not a
// non-empty string is left out: /subjects is schema-checked, so the before
// read is refused before it could matter, and such an id is foreign anyway.
func idsOf(doc map[string]any) []string {
	ids := []string{}
	for _, s := range objects(doc["subjects"]) {
		if id, ok := s["account_id"].(string); ok && id != "" {
			ids = append(ids, id)
		}
	}
	slices.Sort(ids)
	return ids
}

// owns asks owner() about a bare id, as read from /subjects.
func (k *kit) owns(id string) bool { return k.owner(map[string]any{"account_id": id}, false) == ours }

// toOwned is the one way a write reaches the kit's subject once the kit owns
// an account. It reads the plan for desired immediately before, and sends only
// when owner() says that plan names exactly the owned account. A plan naming
// no account, another one, or none at all sends nothing and stops the kit
// writing, as a foreign account does; send refuses any write to the subject
// after the create that did not come through here. write may return nil when
// there is nothing to send.
func (k *kit) toOwned(c *check, desired map[string]any, write func(plan map[string]any) *request) (map[string]any, *exchange) {
	plan := k.planOne(c, desired)
	if k.owner(plan, false) != ours {
		if !k.frozen() {
			what := "no account"
			switch {
			case plan == nil:
				what = "nothing (it could not be read)"
			case names(plan):
				what = "account_id " + display(plan)
			}
			k.stopReason = fmt.Sprintf("during %s the plan read before a write for the subject named %s, not the kit's account %s", k.step, what, k.created.id)
		}
		k.halting = true // nothing after this is judged
		return plan, nil
	}
	r := write(plan)
	if r == nil {
		return plan, nil
	}
	r.owned = true
	return plan, k.send(*r)
}

// viaOwned is toOwned for a write with no desired state of its own (an
// operation, a probe): the plan read before it is {account: true}.
func (k *kit) viaOwned(c *check) func(request) *exchange {
	return func(r request) *exchange {
		if _, ex := k.toOwned(c, map[string]any{k.m.account: true}, func(map[string]any) *request { return &r }); ex != nil {
			return ex
		}
		return &exchange{method: r.method, target: r.target, err: errors.New("not sent: the plan before it did not name the kit's account")}
	}
}
