package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// The add-on's own service account must never be adoptable or deletable.
//
// It is an ordinary row in `user.query`, so without a guard it appears in the
// unmanaged inventory as something to adopt and, one confirmation later, to
// purge. Deleting it removes no member's access — it deletes the credential
// Syndra reaches the target with, and nothing on either side can restore it.
// The operator is left with a target that reports itself unreachable and an
// add-on that cannot say why.
//
// Seen for real: the live inventory listed `syndra` (uid 3001, the account the
// API key belongs to) beside a genuine unmanaged member account, with no
// distinction between them.

func TestAdoptingTheAddOnsOwnAccountIsRefused(t *testing.T) {
	s, _ := applyServer(t, `[]`)
	s.nas.selfName, s.nas.selfUID = "syndra", 3001

	_, status, err := s.adoptAccount(OperationRequest{
		Subject: "sub-1", Params: map[string]any{"username": "syndra"},
	})
	if err == nil {
		t.Fatal("the add-on offered its own service account for adoption")
	}
	if status != http.StatusUnprocessableEntity {
		t.Errorf("status = %d, want 422", status)
	}
	// The refusal has to explain itself, or it reads as a bug in adoption.
	if !strings.Contains(err.Error(), "authenticates") {
		t.Errorf("the refusal does not say why: %v", err)
	}
}

// Case is not a defence: TrueNAS usernames are case-insensitive enough that
// `SYNDRA` reaches the same account.
func TestTheGuardIsCaseInsensitive(t *testing.T) {
	s, _ := applyServer(t, `[]`)
	s.nas.selfName, s.nas.selfUID = "syndra", 3001
	_, status, err := s.adoptAccount(OperationRequest{
		Subject: "sub-1", Params: map[string]any{"username": "SYNDRA"},
	})
	if err == nil {
		t.Fatal("a differently-cased spelling of the same account was accepted")
	}
	// Asserted specifically: with an empty account list this call also fails as
	// "no such account", and a test that accepted any error would have passed
	// while the guard was absent. It did exactly that once.
	if status != http.StatusUnprocessableEntity || !strings.Contains(err.Error(), "authenticates") {
		t.Fatalf("refused for the wrong reason: %d %v", status, err)
	}
}

// Purge is checked against the BINDING, not the request, because a binding can
// predate the guard or predate a key reissued against another account — and it
// is the one operation that cannot be undone.
func TestPurgingABindingOntoTheOwnAccountIsRefused(t *testing.T) {
	s, _ := applyServer(t, `[{"username":"syndra","id":9,"uid":3001,"locked":false,"smb":false,"groups":[]}]`)
	s.nas.selfName, s.nas.selfUID = "syndra", 3001
	if err := s.store.PutBinding(Binding{SubjectID: "sub-1", Username: "syndra", UID: 3001}); err != nil {
		t.Fatal(err)
	}

	_, status, err := s.purgeAccount(OperationRequest{
		Subject: "sub-1", Params: map[string]any{"elevated_key": "an-elevated-key"},
	})
	if err == nil {
		t.Fatal("a purge deleted the account the add-on authenticates with")
	}
	if status != http.StatusUnprocessableEntity {
		t.Errorf("status = %d, want 422", status)
	}
}

// Not knowing is not "this is me". `auth.me` is a read that can fail, and
// reading that as the add-on's own account would mark every account so.
// account.adopt fails closed on its own, retryably (TestAdoptWaitsUntilTheAddonKnowsItsOwnAccount).
func TestAnUnknownSelfRefusesNothing(t *testing.T) {
	s, _ := applyServer(t, `[{"username":"charlotte","id":11,"uid":3005,"locked":false,"smb":false,"groups":[]}]`)
	s.nas.selfName = "" // auth.me failed

	if err := s.refuseSelfAccount("charlotte"); err != nil {
		t.Fatalf("an unknown identity refused an ordinary account: %v", err)
	}
}

// And an ordinary account is still adoptable, so the guard above is not vacuous.
func TestAnOrdinaryAccountIsStillAdoptable(t *testing.T) {
	s, _ := applyServer(t, `[{"username":"charlotte","id":11,"uid":3005,"locked":false,"smb":false,"groups":[]}]`)
	s.nas.selfName, s.nas.selfUID = "syndra", 3001
	if err := s.refuseSelfAccount("charlotte"); err != nil {
		t.Fatalf("a member account was refused: %v", err)
	}
}

// A subject whose derived name is the add-on's own account is a conflict that
// is NOT adoptable: account.adopt refuses it, and `adoptable` is true exactly
// when adopt would succeed. Plan and apply say so alike, and the next step is
// not "adopt it".
func TestTheServiceAccountIsNeverOfferedAsAdoptable(t *testing.T) {
	s, m := applyServer(t, `[{"username":"syndra","id":5,"uid":3001,"locked":false,"smb":false,"groups":[]}]`)
	// Asked of the target, as every new session does.
	m.fakeRPC.answers = map[string]string{"auth.me": `{"pw_name":"syndra","pw_uid":3001}`}
	s.nas.selfName, s.nas.selfUID = "syndra", 3001
	const subject = `"subject":"sub-1","email":"syndra@example.org","desired":{"enabled":true}`

	rr := httptest.NewRecorder()
	planInto(t, s, rr, `{"subjects":[{`+subject+`}]}`)
	var plan PlanResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &plan); err != nil || len(plan.Outcomes) != 1 {
		t.Fatalf("want one planned outcome, got %d %s", rr.Code, rr.Body.String())
	}
	applied := postApply(t, s, `{"call_id":"c1",`+subject+`}`)
	if applied.Code != http.StatusConflict || problemCode(t, applied) != CodeBindingConflict {
		t.Fatalf("want 409 BINDING_CONFLICT, got %d %s", applied.Code, applied.Body.String())
	}
	for name, out := range map[string]ApplyOutcome{"plan": plan.Outcomes[0], "apply": decodeOutcome(t, applied)} {
		if out.Conflict == nil || out.Conflict.Adoptable || out.Conflict.BoundTo != "" {
			t.Fatalf("%s: want a conflict that is not adoptable, got %+v", name, out.Conflict)
		}
		if !strings.Contains(out.Detail, "signs in to the app with") || strings.Contains(out.Consequence, "Adopt it") {
			t.Errorf("%s: the words must say whose account it is, not offer it: %q %q", name, out.Detail, out.Consequence)
		}
	}
	if _, status, err := s.adoptAccount(OperationRequest{Subject: "sub-1", Params: map[string]any{"username": "syndra"}}); err == nil || status != http.StatusUnprocessableEntity {
		t.Fatalf("adopt must refuse it, which is why it is not adoptable: %d %v", status, err)
	}
}

// auth.me answering with no name is a failed read, not a name: the last known
// one stays.
func TestAnEmptyOwnAccountNameKeepsTheLastKnownOne(t *testing.T) {
	for _, blank := range []string{``, `  `} {
		s, m := applyServer(t, `[]`)
		m.fakeRPC.answers = map[string]string{"auth.me": `{"pw_name":"` + blank + `","pw_uid":0}`}
		s.nas.readSelf()
		if name, uid, known := s.nas.Self(); !known || name != "syndra" || uid != 3001 {
			t.Fatalf("%q: want the last known syndra 3001 kept, got %q %d %t", blank, name, uid, known)
		}
	}
}

// Until auth.me names the add-on's own account, any account might be it, so
// account.adopt (the irreversible path's first step) refuses retryably and binds
// nothing; a conflict says adoptable false with words of its own; and once the
// name is known, adoption works. Other operations are not held back.
func TestAdoptWaitsUntilTheAddonKnowsItsOwnAccount(t *testing.T) {
	s, _ := applyServer(t, `[{"username":"bayley","id":11,"uid":3005,"locked":false,"smb":false,"groups":[]}]`)
	s.nas.selfName, s.nas.selfUID = "", 0 // startup, before the first good auth.me
	adopt := func(callID string) *httptest.ResponseRecorder {
		return postOperation(t, s, "account.adopt", `{"call_id":"`+callID+`","operation":"account.adopt","subject":"sub-1","actor":"op1","params":{"username":"bayley"}}`)
	}

	rr := adopt("adopt-1")
	if rr.Code != http.StatusUnprocessableEntity || problemCode(t, rr) != CodeOperationFailed {
		t.Fatalf("want 422 OPERATION_FAILED, got %d %s", rr.Code, rr.Body.String())
	}
	if _, bound, _ := s.store.GetBinding("sub-1"); bound {
		t.Fatal("a refused adoption bound the account")
	}

	const subject = `"subject":"sub-1","email":"bayley@example.org","desired":{"enabled":true}`
	plan := httptest.NewRecorder()
	planInto(t, s, plan, `{"subjects":[{`+subject+`}]}`)
	var planned PlanResponse
	if err := json.Unmarshal(plan.Body.Bytes(), &planned); err != nil || len(planned.Outcomes) != 1 {
		t.Fatalf("want one planned outcome, got %d %s", plan.Code, plan.Body.String())
	}
	applied := postApply(t, s, `{"call_id":"c1",`+subject+`}`)
	if applied.Code != http.StatusConflict || problemCode(t, applied) != CodeBindingConflict {
		t.Fatalf("want 409 BINDING_CONFLICT, got %d %s", applied.Code, applied.Body.String())
	}
	for name, out := range map[string]ApplyOutcome{"plan": planned.Outcomes[0], "apply": decodeOutcome(t, applied)} {
		if out.Conflict == nil || out.Conflict.Adoptable {
			t.Fatalf("%s: want a conflict that is not adoptable yet, got %+v", name, out.Conflict)
		}
		if !strings.Contains(out.Detail, "can't check yet") || !strings.Contains(out.Consequence, "Try again shortly.") {
			t.Errorf("%s: want the can't-check-yet words and their next step, got %q %q", name, out.Detail, out.Consequence)
		}
	}

	// Other operations are not held back.
	if rr := postOperation(t, s, "account.release", `{"call_id":"rel-1","operation":"account.release","subject":"sub-2","actor":"op1"}`); rr.Code/100 != 2 {
		t.Errorf("account.release must not wait on the own account, got %d %s", rr.Code, rr.Body.String())
	}

	s.nas.selfName, s.nas.selfUID = "syndra", 3001
	if rr := adopt("adopt-2"); rr.Code != http.StatusOK {
		t.Fatalf("once known, adopt must work, got %d %s", rr.Code, rr.Body.String())
	}
}
