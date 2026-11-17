package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// "It's theirs: adopt it" on a binding conflict (addon-contract 7t-7, owner ruling 17 Nov 2026):
// account.adopt moves a person's binding to the account now holding its recorded name or number,
// when the add-on says that account is adoptable, and judges it all again when it runs. A stale
// adoptable is refused in words, and the binding is left exactly as it was.

// bob's recorded number 3051 (outside the range, so it can be handed out again) is now eve's.
const eveHoldsBobsNumber = `[{"username":"eve","id":12,"uid":3051,"locked":false,"smb":false,"groups":[]}]`

func movedServer(t *testing.T, users string) (*server, *mutatingRPC) {
	t.Helper()
	s, m := applyServer(t, users)
	if err := s.store.PutBinding(Binding{SubjectID: "sub-1", Username: "bob", UID: 3051, UIDKind: uidKindAccount}); err != nil {
		t.Fatal(err)
	}
	return s, m
}

func adoptBody(callID, username, accountID string) string {
	params := `"username":"` + username + `"`
	if accountID != "" {
		params += `,"account_id":"` + accountID + `"`
	}
	return `{"call_id":"` + callID + `","operation":"account.adopt","subject":"sub-1","actor":"op_1","params":{` + params + `}}`
}

func TestAdoptingTheHolderMovesTheBindingAndTouchesNoAccount(t *testing.T) {
	s, m := movedServer(t, eveHoldsBobsNumber)
	rr := postOperation(t, s, "account.adopt", adoptBody("m1", "eve", "3051"))
	var res OperationResult
	if err := json.Unmarshal(rr.Body.Bytes(), &res); err != nil || rr.Code != http.StatusOK || res.AccountID != "3051" {
		t.Fatalf("want 200 with eve's account, got %d %s", rr.Code, rr.Body.String())
	}
	b, _, _ := s.store.GetBinding("sub-1")
	if b.Username != "eve" || b.UID != 3051 || b.UIDKind != uidKindAccount || b.BoundBy != "op_1" {
		t.Fatalf("the binding names eve, marked, by who decided it: %+v", b)
	}
	if w := writesTo(m, &deleteRecorder{}); len(w) != 0 {
		t.Fatalf("an adoption writes to no account: %v", w)
	}
	var bound bool
	for _, r := range loggedWrites(t, s) {
		bound = bound || r.Write == "store.bind"
	}
	if !bound {
		t.Fatal("the move is logged as the binding it writes")
	}
	// The conflict is over: /subjects gives eve's row to sub-1.
	at, err := s.attribute([]Subject{{Username: "eve", UID: 3051}})
	if err != nil || len(at.conflicts) != 0 || at.bySubject["sub-1"].Account == nil {
		t.Fatalf("after the move sub-1 names eve with no conflict: %v %+v", err, at.conflicts)
	}
	// And a repeat is done, not a second move.
	if rr := postOperation(t, s, "account.adopt", adoptBody("m2", "eve", "3051")); rr.Code != http.StatusOK ||
		!strings.Contains(rr.Body.String(), "already bound") {
		t.Fatalf("a repeat answers done: %d %s", rr.Code, rr.Body.String())
	}
}

// What the operator was shown is no longer what TrueNAS holds, each way it can stop being so. The
// add-on judges it again at execution and refuses; nothing is written to the store or any account.
func TestAStaleAdoptableIsRefusedWhenItRuns(t *testing.T) {
	for _, tc := range []struct {
		name, users, username, accountID, says string
		otherBinding                           *Binding
	}{
		// The operator was shown eve as 3050; the eve holding bob's number now is 3051 (made again
		// since, or another account given the name). The name alone cannot tell: the shown id does.
		{"the name went to another account since", `[{"username":"eve","id":13,"uid":3051,"locked":false,"smb":false,"groups":[]}]`,
			"eve", "3050", "is no longer the account you were shown", nil},
		{"someone else adopted it since", eveHoldsBobsNumber, "eve", "3051", "already bound to another subject",
			&Binding{SubjectID: "sub-2", Username: "eve", UID: 3051, UIDKind: uidKindAccount}},
		{"it was renamed since", `[{"username":"eve_2","id":12,"uid":3051,"locked":false,"smb":false,"groups":[]}]`,
			"eve", "3051", "already bound to bob", nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, m := movedServer(t, tc.users)
			if tc.otherBinding != nil {
				if err := s.store.PutBinding(*tc.otherBinding); err != nil {
					t.Fatal(err)
				}
			}
			rr := postOperation(t, s, "account.adopt", adoptBody("s1", tc.username, tc.accountID))
			if rr.Code != http.StatusConflict || !strings.Contains(rr.Body.String(), tc.says) {
				t.Fatalf("want 409 saying %q, got %d %s", tc.says, rr.Code, rr.Body.String())
			}
			if b, _, _ := s.store.GetBinding("sub-1"); b.Username != "bob" || b.UID != 3051 {
				t.Fatalf("a refused move leaves the binding as it was: %+v", b)
			}
			if w := writesTo(m, &deleteRecorder{}); len(w) != 0 {
				t.Fatalf("nothing is written: %v", w)
			}
			for _, r := range loggedWrites(t, s) {
				if r.Write == "store.bind" && r.Subject == "sub-1" {
					t.Fatalf("a refused move logs no binding: %+v", r)
				}
			}
		})
	}
}

// The move is judged again over the one read it is written from, not on the first look: TrueNAS
// changes between the live lookup (eve holds bob's number) and the whole read (an account named
// bob is back, so bob's conflict is now about that one). eve is still bound to nobody, so only the
// re-judgement refuses it.
func TestAMoveIsJudgedAgainOverTheReadItIsWrittenFrom(t *testing.T) {
	s, m := movedServer(t, eveHoldsBobsNumber)
	changed := false
	m.after = map[string]func(){"user.query": func() {
		last, _ := json.Marshal(m.fakeRPC.params[len(m.fakeRPC.params)-1])
		if !changed && strings.Contains(string(last), `["uid","=",3051]`) {
			changed = true
			m.fakeRPC.users = strings.Replace(m.fakeRPC.users, `[`, `[{"username":"bob","id":14,"uid":3099,"locked":false,"smb":false,"groups":[]},`, 1)
		}
	}}
	rr := postOperation(t, s, "account.adopt", adoptBody("w1", "eve", "3051"))
	if !changed || rr.Code != http.StatusConflict || !strings.Contains(rr.Body.String(), "can no longer be adopted for this person") {
		t.Fatalf("want the stale move refused, got changed=%v %d %s", changed, rr.Code, rr.Body.String())
	}
	if b, _, _ := s.store.GetBinding("sub-1"); b.Username != "bob" || b.UID != 3051 {
		t.Fatalf("nothing moved: %+v", b)
	}
}

// adoptable is account.adopt's own rule: never the add-on's own account, and never while the
// add-on cannot yet tell which account that is.
func TestTheHolderIsNeverAdoptableWhenItIsTheAddOnsOwn(t *testing.T) {
	for _, tc := range []struct {
		name, self string
	}{{"its own account", "eve"}, {"its own account not known yet", ""}} {
		s, _ := movedServer(t, eveHoldsBobsNumber)
		s.nas.selfName, s.nas.selfUID = tc.self, 3051
		if tc.self == "" {
			s.nas.selfUID = 0
		}
		at, err := s.attribute([]Subject{{Username: "eve", UID: 3051}})
		if err != nil || len(at.conflicts) != 1 || at.conflicts[0].Adoptable {
			t.Fatalf("%s: want one conflict, not adoptable: %v %+v", tc.name, err, at.conflicts)
		}
	}
}

// A move is never made from the name alone: without the account the operator was shown it is a 409
// in words, and nothing moves (a name can go to another account between the look and the click).
func TestAMoveWithoutTheShownAccountIsRefused(t *testing.T) {
	s, m := movedServer(t, eveHoldsBobsNumber)
	rr := postOperation(t, s, "account.adopt", adoptBody("n1", "eve", ""))
	if rr.Code != http.StatusConflict || !strings.Contains(rr.Body.String(), "needs the account you were shown") {
		t.Fatalf("want 409 in words, got %d %s", rr.Code, rr.Body.String())
	}
	if b, _, _ := s.store.GetBinding("sub-1"); b.Username != "bob" || b.UID != 3051 {
		t.Fatalf("nothing moved: %+v", b)
	}
	if w := writesTo(m, &deleteRecorder{}); len(w) != 0 {
		t.Fatalf("nothing is written: %v", w)
	}
}

// Two people's records point to one holder (one by its number, one by its name): it is adoptable for
// neither, the words say why and what to do, and a move for either is refused.
func TestAHolderTwoConflictsNameIsAdoptableForNeither(t *testing.T) {
	s, _ := movedServer(t, eveHoldsBobsNumber)
	// sub-2's recorded name eve (in range, its own number gone) is held by the same account.
	if err := s.store.PutBinding(Binding{SubjectID: "sub-2", Username: "eve", UID: 300009, UIDKind: uidKindAccount}); err != nil {
		t.Fatal(err)
	}
	at, err := s.attribute([]Subject{{Username: "eve", UID: 3051}})
	if err != nil || len(at.conflicts) != 2 {
		t.Fatalf("want two conflicts: %v %+v", err, at.conflicts)
	}
	for _, c := range at.conflicts {
		if c.Adoptable || !strings.Contains(c.Next, "Another person's record points to eve too") {
			t.Errorf("%s: adoptable for neither, and said: %+v", c.Subject, c.BindingConflict)
		}
	}
	if rr := postOperation(t, s, "account.adopt", adoptBody("s2", "eve", "3051")); rr.Code != http.StatusConflict {
		t.Fatalf("a move to a holder two people claim is refused, got %d %s", rr.Code, rr.Body.String())
	}
	if b, _, _ := s.store.GetBinding("sub-1"); b.Username != "bob" {
		t.Fatalf("nothing moved: %+v", b)
	}
}

// account.release with account_id releases only while the binding still names that account: a person
// released and bound again since has a new account, and a stale release must not drop it.
func TestAReleaseForAnAccountTheBindingNoLongerNamesIsRefused(t *testing.T) {
	s, _ := movedServer(t, eveHoldsBobsNumber)
	rel := func(callID, accountID string) *httptest.ResponseRecorder {
		return postOperation(t, s, "account.release", `{"call_id":"`+callID+`","operation":"account.release","subject":"sub-1","actor":"op_1","params":{"account_id":"`+accountID+`"}}`)
	}
	if rr := rel("r1", "3077"); rr.Code != http.StatusConflict || !strings.Contains(rr.Body.String(), "now names a different account") {
		t.Fatalf("stale: want 409 in words, got %d %s", rr.Code, rr.Body.String())
	}
	if _, bound, _ := s.store.GetBinding("sub-1"); !bound {
		t.Fatal("a refused release left the binding")
	}
	if rr := rel("r2", "3051"); rr.Code != http.StatusOK {
		t.Fatalf("the account it names: want 200, got %d %s", rr.Code, rr.Body.String())
	}
	if _, bound, _ := s.store.GetBinding("sub-1"); bound {
		t.Fatal("released")
	}
}

// A contract-1 binding (no marker, uid maybe a record key) is matched by the account the module finds.
func TestAReleasePreconditionOnALegacyBindingAsksTheModule(t *testing.T) {
	s, _ := applyServer(t, `[{"username":"bob","id":11,"uid":3051,"locked":false,"smb":false,"groups":[]}]`)
	if err := s.store.PutBinding(Binding{SubjectID: "sub-1", Username: "bob", UID: 11}); err != nil {
		t.Fatal(err)
	}
	if _, status, err := s.releaseAccount(OperationRequest{Subject: "sub-1", Params: map[string]any{"account_id": "11"}}); status != http.StatusConflict || err == nil {
		t.Fatalf("the record key is not the account: want 409, got %d %v", status, err)
	}
	if _, status, err := s.releaseAccount(OperationRequest{Subject: "sub-1", Params: map[string]any{"account_id": "3051"}}); err != nil || status != http.StatusOK {
		t.Fatalf("the account its name finds: want 200, got %d %v", status, err)
	}
}

// account_id is a precondition only when it is a string; anything else is a bad request, never ignored.
func TestAnAccountIDThatIsNotAStringIsABadRequest(t *testing.T) {
	s, _ := movedServer(t, eveHoldsBobsNumber)
	rr := postOperation(t, s, "account.adopt",
		`{"call_id":"b1","operation":"account.adopt","subject":"sub-1","actor":"op_1","params":{"username":"eve","account_id":3051}}`)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d %s", rr.Code, rr.Body.String())
	}
	if b, _, _ := s.store.GetBinding("sub-1"); b.Username != "bob" {
		t.Fatalf("nothing moved: %+v", b)
	}
}
