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

// logWrite appends one write's intent and, unless result is empty, its result,
// as mutate writes them.
func logWrite(t *testing.T, s *server, subject, write, account, result string) {
	t.Helper()
	r := Record{Operation: "apply", Subject: subject, CallID: "c0-" + subject, Write: write, AccountID: account}
	intent := r
	intent.Outcome = outcomeIntent
	logged, err := s.log.Append(intent)
	if err != nil {
		t.Fatal(err)
	}
	if result == "" {
		return
	}
	r.Intent, r.Outcome = logged.Seq, result
	if _, err := s.log.Append(r); err != nil {
		t.Fatal(err)
	}
}

// Every case but the ones that adopt: each keeps BINDING_CONFLICT, plan and
// apply alike, and binds nothing.
func TestOnlyThisSubjectsUnknownCreateOfThatAccountIsAdopted(t *testing.T) {
	const inRange = `[{"username":"charlotte","id":19,"uid":200001,"locked":false,"smb":false,"password_disabled":true,"groups":[42]}]`
	unknownCreate := func(t *testing.T, s *server) { logWrite(t, s, "sub-1", "user.create", "200001", outcomeUnknown) }
	for _, tc := range []struct {
		name, users string
		setup       func(*testing.T, *server)
		adopts      bool
		// routedAround: the name is another managed subject's, so the derivation
		// suffixes past it (DeriveUsername) and a new account is made instead.
		routedAround bool
	}{
		{name: "this subject's create, unknown", users: inRange, setup: unknownCreate, adopts: true},
		{name: "another subject's create of it failed", users: inRange, adopts: true, setup: func(t *testing.T, s *server) {
			logWrite(t, s, "sub-2", "user.create", "200001", outcomeFailed)
			unknownCreate(t, s)
		}},
		{name: "no create logged", users: inRange, setup: func(*testing.T, *server) {}},
		{name: "the create failed", users: inRange, setup: func(t *testing.T, s *server) { logWrite(t, s, "sub-1", "user.create", "200001", outcomeFailed) }},
		{name: "the create was never sent", users: inRange, setup: func(t *testing.T, s *server) { logWrite(t, s, "sub-1", "user.create", "200001", outcomeNotSent) }},
		{name: "the create is still open", users: inRange, setup: func(t *testing.T, s *server) { logWrite(t, s, "sub-1", "user.create", "200001", "") }},
		{name: "another subject's create of that account", users: inRange, setup: func(t *testing.T, s *server) { logWrite(t, s, "sub-2", "user.create", "200001", outcomeUnknown) }},
		{name: "this subject's create of another account", users: inRange, setup: func(t *testing.T, s *server) { logWrite(t, s, "sub-1", "user.create", "200002", outcomeUnknown) }},
		// Review (b): an orphan group's gid reused, two creates of one uid in doubt.
		{name: "two subjects' creates of the uid, both unknown", users: inRange, setup: func(t *testing.T, s *server) {
			unknownCreate(t, s)
			logWrite(t, s, "sub-2", "user.create", "200001", outcomeUnknown)
		}},
		{name: "the account was bound since, by another subject, and the bind's result is unknown", users: inRange, setup: func(t *testing.T, s *server) {
			unknownCreate(t, s)
			logWrite(t, s, "sub-2", "store.bind", "200001", outcomeUnknown)
		}},
		{name: "the account was released since", users: inRange, setup: func(t *testing.T, s *server) {
			unknownCreate(t, s)
			logWrite(t, s, "sub-2", "store.unbind", "200001", outcomeSucceeded)
		}},
		{name: "this subject was bound since, before records carried an id", users: inRange, setup: func(t *testing.T, s *server) {
			unknownCreate(t, s)
			logWrite(t, s, "sub-1", "store.bind", "", outcomeSucceeded)
		}},
		// This subject's own bind ends its own doubt, never another subject's
		// claim on the uid: that one still makes two candidates.
		{name: "another subject's create survives this subject's own bind", users: inRange, setup: func(t *testing.T, s *server) {
			logWrite(t, s, "sub-2", "user.create", "200001", outcomeUnknown)
			logWrite(t, s, "sub-1", "store.bind", "", outcomeSucceeded)
			unknownCreate(t, s)
		}},
		{name: "bound to another subject", users: inRange, routedAround: true, setup: func(t *testing.T, s *server) {
			unknownCreate(t, s)
			if err := s.store.PutBinding(Binding{SubjectID: "sub-2", Username: "charlotte", UID: 200001}); err != nil {
				t.Fatal(err)
			}
		}},
		{name: "bound to another subject under an older name", users: inRange, setup: func(t *testing.T, s *server) {
			unknownCreate(t, s)
			if err := s.store.PutBinding(Binding{SubjectID: "sub-2", Username: "lita", UID: 200001}); err != nil {
				t.Fatal(err)
			}
		}},
		{name: "the add-on's own account", users: inRange, setup: func(t *testing.T, s *server) {
			unknownCreate(t, s)
			s.nas.selfName, s.nas.selfUID = "charlotte", 200001
		}},
		// Review round 3: a whole-volume restore brings back an old create in
		// doubt with a log that agrees, after the account was used. Only an
		// account still as a create leaves it is adopted.
		{name: "a password set since, with a perfect log", setup: unknownCreate,
			users: `[{"username":"charlotte","id":19,"uid":200001,"locked":false,"smb":false,"password_disabled":false,"groups":[42]}]`},
		{name: "SMB on since, with a perfect log", setup: unknownCreate,
			users: `[{"username":"charlotte","id":19,"uid":200001,"locked":false,"smb":true,"password_disabled":true,"groups":[42]}]`},
		{name: "below the add-on's range", setup: func(t *testing.T, s *server) { logWrite(t, s, "sub-1", "user.create", "9001", outcomeUnknown) },
			users: `[{"username":"charlotte","id":19,"uid":9001,"locked":false,"smb":false,"password_disabled":true,"groups":[42]}]`},
		{name: "above the add-on's range", setup: func(t *testing.T, s *server) { logWrite(t, s, "sub-1", "user.create", "1000000", outcomeUnknown) },
			users: `[{"username":"charlotte","id":19,"uid":1000000,"locked":false,"smb":false,"password_disabled":true,"groups":[42]}]`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, m := applyServer(t, tc.users)
			tc.setup(t, s)

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
				if rr.Code != http.StatusOK || !bound || b.UID != 200001 || planned.Effect != EffectApply {
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

// Review (a), end to end: this subject's create is in doubt, an operator
// adopts the account to someone else, who releases it. The next create must
// not take that person's former account.
func TestAnAccountBoundElsewhereSinceIsNotAdoptedByAnOldDoubt(t *testing.T) {
	s, _ := applyServer(t, `[{"username":"charlotte","id":19,"uid":200001,"locked":false,"smb":false,"password_disabled":true,"groups":[42]}]`)
	logWrite(t, s, "sub-1", "user.create", "200001", outcomeUnknown)
	if rr := postOperation(t, s, "account.adopt", `{"call_id":"c2","operation":"account.adopt","subject":"sub-2","actor":"op-lita","params":{"username":"charlotte"}}`); rr.Code != http.StatusOK {
		t.Fatalf("adopt: %d %s", rr.Code, rr.Body.String())
	}
	if rr := postOperation(t, s, "account.release", `{"call_id":"c3","subject":"sub-2"}`); rr.Code != http.StatusOK {
		t.Fatalf("release: %d %s", rr.Code, rr.Body.String())
	}
	for _, r := range records(t, s) {
		if (r.Write == "store.bind" || r.Write == "store.unbind") && r.AccountID != "200001" {
			t.Fatalf("a binding write must name its account: %+v", r)
		}
	}
	if rr := postApply(t, s, applyCall("c4")); rr.Code != http.StatusConflict || problemCode(t, rr) != CodeBindingConflict {
		t.Fatalf("want BINDING_CONFLICT, got %d %s", rr.Code, rr.Body.String())
	}
	if _, bound, _ := s.store.GetBinding("sub-1"); bound {
		t.Fatal("sub-1 took an account another subject held since")
	}
}

// An adoption whose converge would block binds nothing, so the blocked
// outcome's "Nothing was changed." is true; the plan blocks alike.
func TestAnAdoptionThatWouldBlockBindsNothing(t *testing.T) {
	s, m := applyServer(t, `[{"username":"charlotte","id":19,"uid":200001,"locked":false,"smb":false,"password_disabled":true,"groups":[42]}]`)
	logWrite(t, s, "sub-1", "user.create", "200001", outcomeUnknown)
	body := `{"call_id":"c1","subject":"sub-1","email":"charlotte@example.org","desired":{"group":["no_such_group"],"enabled":true}}`
	rr := postApply(t, s, body)
	out := decodeOutcome(t, rr)
	if rr.Code != http.StatusUnprocessableEntity || problemCode(t, rr) != CodeApplyBlocked || out.Consequence != "Nothing was changed." {
		t.Fatalf("want 422 APPLY_BLOCKED saying nothing changed, got %d %s", rr.Code, rr.Body.String())
	}
	if _, bound, _ := s.store.GetBinding("sub-1"); bound || len(m.updates) != 0 {
		t.Fatal("a blocked adoption bound the account")
	}
}

// The licence is read across rotated segments: a create in an old segment
// still counts, and a bind of its account in a later one still cancels it.
func TestCreateInDoubtReadsEveryRotatedSegment(t *testing.T) {
	l, err := OpenMutationLog(t.TempDir(), 1, 16) // rotates before every append
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	app := func(r Record) Record {
		t.Helper()
		got, err := l.Append(r)
		if err != nil {
			t.Fatal(err)
		}
		return got
	}
	in := app(Record{Subject: "sub-1", Write: "user.create", AccountID: "200001", Outcome: outcomeIntent})
	app(Record{Subject: "sub-1", Write: "user.create", AccountID: "200001", Intent: in.Seq, Outcome: outcomeUnknown})
	app(Record{Subject: "sub-3", Write: "user.update", Outcome: outcomeIntent})
	if ok, err := l.CreateInDoubt("sub-1", "200001"); err != nil || !ok {
		t.Fatalf("a create in an older segment must count: %t %v", ok, err)
	}
	bind := app(Record{Subject: "sub-2", Write: "store.bind", AccountID: "200001", Outcome: outcomeIntent})
	app(Record{Subject: "sub-2", Write: "store.bind", AccountID: "200001", Intent: bind.Seq, Outcome: outcomeSucceeded})
	if ok, err := l.CreateInDoubt("sub-1", "200001"); err != nil || ok {
		t.Fatalf("a bind in a later segment must cancel it: %t %v", ok, err)
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
