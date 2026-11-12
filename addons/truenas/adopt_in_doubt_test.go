package main

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// 5.3d: the one adoption without a person (design §5, AUTHORING "Look up
// before you create"). An unbound account holding the derived name is adopted
// only when this subject's own logged user.create asked for that very uid and
// its result is unknown; anything else stays BINDING_CONFLICT.

const charlotteApply = `{"call_id":"%s","subject":"sub-1","email":"charlotte@example.org","desired":{"group":["lab_makers"],"enabled":true}}`

func applyCall(id string) string { return strings.Replace(charlotteApply, "%s", id, 1) }

func planCharlotte(t *testing.T, s *server) ApplyOutcome {
	t.Helper()
	rr := httptest.NewRecorder()
	planInto(t, s, rr, `{"subjects":[{"subject":"sub-1","email":"charlotte@example.org","desired":{"group":["lab_makers"],"enabled":true}}]}`)
	var plan PlanResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &plan); err != nil || len(plan.Outcomes) != 1 {
		t.Fatalf("want one planned outcome, got %d %s", rr.Code, rr.Body.String())
	}
	return plan.Outcomes[0]
}

// A crash after the create's intent and before its result, then a start: the
// account is on the NAS, Recover settles the create unknown, and the next
// create adopts it, answers as a create, and logs the bind. A second create is
// the ordinary bound path; after a release the settled doubt adopts nothing.
func TestACreateLeftInDoubtByACrashIsAdoptedByTheNextCreate(t *testing.T) {
	s, m := applyServer(t, `[]`)
	durable := s.log.sync
	// The disk goes as soon as the account is made: the create's result and
	// the binding's intent never land, which is what a crash leaves.
	m.after = map[string]func(){"user.create": func() { s.log.sync = func() error { return errors.New("disk gone") } }}
	if rr := postApply(t, s, applyCall("c1")); rr.Code != http.StatusInternalServerError {
		t.Fatalf("want the first call to fail after the create, got %d %s", rr.Code, rr.Body.String())
	}
	if len(m.creates) != 1 {
		t.Fatalf("want the account made once, got %d", len(m.creates))
	}
	if _, bound, _ := s.store.GetBinding("sub-1"); bound {
		t.Fatal("the crash must leave the account unbound")
	}

	// The start.
	s.log.sync, m.after = durable, nil
	recovered, err := s.log.Recover()
	if err != nil || len(recovered) != 1 || recovered[0].Write != "user.create" || recovered[0].AccountID == "" {
		t.Fatalf("want the create settled unknown at the start, naming its account, got %+v %v", recovered, err)
	}

	if planned := planCharlotte(t, s); planned.Effect != EffectApply || !strings.HasPrefix(planned.Detail, "Adopts charlotte,") || planned.Conflict != nil {
		t.Fatalf("the plan must say what the apply does, got %s %q", planned.Effect, planned.Detail)
	}

	before := len(records(t, s))
	rr := postApply(t, s, applyCall("c2"))
	out := decodeOutcome(t, rr)
	if rr.Code != http.StatusOK || out.Effect != EffectApplied || !strings.HasPrefix(out.Detail, "Adopted charlotte,") ||
		out.AccountID != recovered[0].AccountID || out.Username != "charlotte" {
		t.Fatalf("want the create to adopt and answer as a create, got %d %s", rr.Code, rr.Body.String())
	}
	if len(m.creates) != 1 {
		t.Fatalf("an adoption must not create again, got %d creates", len(m.creates))
	}
	b, bound, _ := s.store.GetBinding("sub-1")
	if !bound || b.Username != "charlotte" || accountID(b.UID) != recovered[0].AccountID {
		t.Fatalf("want sub-1 bound to the account its create made, got %+v", b)
	}
	// The bind is logged, under this call, as every binding change is.
	var bind []Record
	for _, r := range records(t, s)[before:] {
		if r.Write == "store.bind" {
			bind = append(bind, r)
		}
	}
	if len(bind) != 2 || bind[0].Outcome != outcomeIntent || bind[1].Outcome != outcomeSucceeded ||
		bind[1].Intent != bind[0].Seq || bind[0].CallID != "c2" || bind[0].Subject != "sub-1" {
		t.Fatalf("want the adoption's bind logged as an intent and its result, got %+v", bind)
	}

	// The second create: bound, so no derivation and no adoption.
	if rr := postApply(t, s, applyCall("c3")); rr.Code != http.StatusOK || strings.Contains(decodeOutcome(t, rr).Detail, "Adopted") {
		t.Fatalf("want the ordinary bound path, got %d %s", rr.Code, rr.Body.String())
	}

	// Released by an operator: the doubt was settled by the adoption's bind,
	// so the next create finds an unbound account and a person decides.
	if rr := postOperation(t, s, "account.release", `{"call_id":"c4","subject":"sub-1"}`); rr.Code != http.StatusOK {
		t.Fatalf("release: %d %s", rr.Code, rr.Body.String())
	}
	if rr := postApply(t, s, applyCall("c5")); rr.Code != http.StatusConflict || problemCode(t, rr) != CodeBindingConflict {
		t.Fatalf("an adopted create must not be adopted twice, got %d %s", rr.Code, rr.Body.String())
	}
}

// Every case but the one: each keeps BINDING_CONFLICT, plan and apply alike,
// and binds nothing. The first row is the control that does adopt.
func TestOnlyThisSubjectsUnknownCreateOfThatAccountIsAdopted(t *testing.T) {
	const inRange = `[{"username":"charlotte","id":19,"uid":200001,"locked":false,"smb":false,"groups":[42]}]`
	for _, tc := range []struct {
		name, users      string
		subject, account string // the logged create's; empty logs none
		result           string // its result; empty leaves it open
		bind             *Binding
		adopts           bool
		// routedAround: the name is another managed subject's, so the derivation
		// suffixes past it (DeriveUsername) and a new account is made instead.
		routedAround bool
	}{
		{name: "this subject's create, unknown", users: inRange, subject: "sub-1", account: "200001", result: outcomeUnknown, adopts: true},
		{name: "no create logged", users: inRange},
		{name: "the create failed", users: inRange, subject: "sub-1", account: "200001", result: outcomeFailed},
		{name: "the create was never sent", users: inRange, subject: "sub-1", account: "200001", result: outcomeNotSent},
		{name: "the create is still open", users: inRange, subject: "sub-1", account: "200001"},
		{name: "another subject's create of that account", users: inRange, subject: "sub-2", account: "200001", result: outcomeUnknown},
		{name: "this subject's create of another account", users: inRange, subject: "sub-1", account: "200002", result: outcomeUnknown},
		{name: "bound to another subject", users: inRange, subject: "sub-1", account: "200001", result: outcomeUnknown,
			bind: &Binding{SubjectID: "sub-2", Username: "charlotte", UID: 200001}, routedAround: true},
		{name: "bound to another subject under an older name", users: inRange, subject: "sub-1", account: "200001", result: outcomeUnknown,
			bind: &Binding{SubjectID: "sub-2", Username: "lita", UID: 200001}},
		{name: "outside the add-on's range", subject: "sub-1", account: "9001", result: outcomeUnknown,
			users: `[{"username":"charlotte","id":19,"uid":9001,"locked":false,"smb":false,"groups":[42]}]`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, m := applyServer(t, tc.users)
			if tc.bind != nil {
				if err := s.store.PutBinding(*tc.bind); err != nil {
					t.Fatal(err)
				}
			}
			if tc.subject != "" {
				intent, err := s.log.Append(Record{Operation: "apply", Subject: tc.subject, CallID: "c0", Write: "user.create", AccountID: tc.account, Outcome: outcomeIntent})
				if err != nil {
					t.Fatal(err)
				}
				if tc.result != "" {
					if _, err := s.log.Append(Record{Operation: "apply", Subject: tc.subject, CallID: "c0", Write: "user.create", AccountID: tc.account, Intent: intent.Seq, Outcome: tc.result}); err != nil {
						t.Fatal(err)
					}
				}
			}

			planned := planCharlotte(t, s)
			rr := postApply(t, s, applyCall("c1"))
			b, bound, _ := s.store.GetBinding("sub-1")
			if tc.routedAround {
				if rr.Code != http.StatusOK || b.UID == 200001 || b.Username == "charlotte" || len(m.creates) != 1 {
					t.Fatalf("want a new account beside the bound one, got %d %+v %s", rr.Code, b, rr.Body.String())
				}
				return
			}
			if tc.adopts {
				if rr.Code != http.StatusOK || !bound || planned.Effect != EffectApply {
					t.Fatalf("want adopted, got plan %s, apply %d %s", planned.Effect, rr.Code, rr.Body.String())
				}
				return
			}
			if rr.Code != http.StatusConflict || problemCode(t, rr) != CodeBindingConflict || bound {
				t.Fatalf("want BINDING_CONFLICT and nothing bound, got %d %s", rr.Code, rr.Body.String())
			}
			if planned.Effect != EffectBlocked || planned.Conflict == nil {
				t.Fatalf("the plan must block alike, got %s %q", planned.Effect, planned.Detail)
			}
			if len(m.creates) != 0 || len(m.updates) != 0 {
				t.Fatal("nothing may be written")
			}
		})
	}
}

// The create names its account on every write from the uid on, so the log can
// tie a create in doubt to exactly the account it made.
func TestACreateNamesItsAccountInTheLog(t *testing.T) {
	s, _ := applyServer(t, `[]`)
	rr := postApply(t, s, applyCall("c1"))
	out := decodeOutcome(t, rr)
	if rr.Code != http.StatusOK {
		t.Fatalf("want 200, got %d %s", rr.Code, rr.Body.String())
	}
	for _, r := range records(t, s) {
		if (r.Write == "user.create" || r.Write == "store.bind") && r.AccountID != out.AccountID {
			t.Fatalf("want %s on the create's records, got %+v", out.AccountID, r)
		}
	}
	if err := VerifyLog(s.log.dir); err != nil {
		t.Fatal(err)
	}
}
