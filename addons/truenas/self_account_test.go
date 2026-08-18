package main

import (
	"net/http"
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

// Not knowing must not block everything. `auth.me` is a read that can fail, and
// treating "I could not find out" as "this is me" would refuse adoption of
// every account on the target.
func TestAnUnknownSelfRefusesNothing(t *testing.T) {
	s, _ := applyServer(t, `[{"username":"ada","id":11,"uid":3005,"locked":false,"smb":false,"groups":[]}]`)
	s.nas.selfName = "" // auth.me failed

	if err := s.refuseSelfAccount("ada"); err != nil {
		t.Fatalf("an unknown identity refused an ordinary account: %v", err)
	}
}

// And an ordinary account is still adoptable, so the guard above is not vacuous.
func TestAnOrdinaryAccountIsStillAdoptable(t *testing.T) {
	s, _ := applyServer(t, `[{"username":"ada","id":11,"uid":3005,"locked":false,"smb":false,"groups":[]}]`)
	s.nas.selfName, s.nas.selfUID = "syndra", 3001
	if err := s.refuseSelfAccount("ada"); err != nil {
		t.Fatalf("a member account was refused: %v", err)
	}
}
