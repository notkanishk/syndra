package main

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	bolt "go.etcd.io/bbolt"
)

// Which account a binding names by number is said by the binding (uid_kind), never inferred from the
// range this add-on hands out: an adopted hand-made account has uid 3051, far below the production
// default range (200000-999999), and is no less a real uid for it.

func putRaw(t *testing.T, s *server, subject string, b Binding) {
	t.Helper()
	raw, _ := json.Marshal(b)
	if err := s.store.db.Update(func(tx *bolt.Tx) error { return tx.Bucket(bucketBindings).Put([]byte(subject), raw) }); err != nil {
		t.Fatal(err)
	}
}

// A number outside the range this add-on hands out can come back after a delete (TrueNAS gives a
// new account the next free uid), so it is never believed on its own: only beside the recorded
// name. In range, numbers are never reused and a rename is followed.

const oneApply = `{"call_id":"c1","subject":"sub-1","email":"bob@example.edu","desired":{"group":["lab_makers"],"enabled":true,"smb_enabled":false}}`

func TestAnOutOfRangeNumberHeldByAnotherNameIsAConflictNeverARename(t *testing.T) {
	for _, tc := range []struct{ name, holder, nas string }{
		{"renamed by hand", "bob_renamed", `[{"username":"bob_renamed","id":11,"uid":3051,"locked":false,"smb":false,"groups":[]}]`},
		{"bob deleted, eve got the number", "eve", `[{"username":"eve","id":12,"uid":3051,"locked":false,"smb":false,"groups":[]}]`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, m := applyServer(t, tc.nas)
			if lo, _ := s.idRange(); lo != defaultIDFloor {
				t.Fatalf("this is about the production default range, got floor %d", lo)
			}
			bound := Binding{SubjectID: "sub-1", Username: "bob", UID: 3051, UIDKind: uidKindAccount}
			if err := s.store.PutBinding(bound); err != nil {
				t.Fatal(err)
			}
			out := decodeOutcome(t, postApply(t, s, oneApply))
			if out.Effect != EffectBlocked || out.Conflict == nil || out.Detail != "The account bob, recorded for this person, is gone, and its number is now held by a different account, "+tc.holder+"." ||
				!out.Conflict.Adoptable ||
				!strings.Contains(out.Consequence, "Adopt "+tc.holder+" if it is theirs. If it is not, stop tracking this person's old account; the next change creates a new one for them.") {
				t.Fatalf("want a conflict an operator decides, got %+v", out)
			}
			if len(m.updates) != 0 || len(m.creates) != 0 {
				t.Fatalf("nothing is written to the account that holds the number, and nothing is created: %d updates, %d creates", len(m.updates), len(m.creates))
			}
			if b, _, _ := s.store.GetBinding("sub-1"); b.Username != "bob" || b.UID != 3051 {
				t.Fatalf("the binding is left as it was, got %+v", b)
			}
			// The plan says the same, and says it to the operator in words.
			rr := httptest.NewRecorder()
			planInto(t, s, rr, `{"subjects":[{"subject":"sub-1","email":"bob@example.edu","desired":{"group":["lab_makers"],"enabled":true}}]}`)
			var plan PlanResponse
			if err := json.Unmarshal(rr.Body.Bytes(), &plan); err != nil || len(plan.Outcomes) != 1 || plan.Outcomes[0].Effect != EffectBlocked ||
				!strings.Contains(plan.Outcomes[0].Detail, "is now held by a different account") {
				t.Fatalf("plan: %v %s", err, rr.Body.String())
			}
			// And the holder is never bob's: not for stats, not for standing, not for adoption.
			subjects := []Subject{{Username: tc.holder, UID: 3051, Enabled: true}}
			if _, ok := s.wishesFor(subjects)[3051]; ok {
				t.Error("standing: the holder of the number is not bob")
			}
			if u, _ := s.boundUIDs(subjects); len(u) != 0 {
				t.Errorf("stats: nobody's usage is the holder's, got %+v", u)
			}
			if b, _ := s.accountBound(&subjects[0]); b {
				t.Error("accountBound: the holder is bound to nobody")
			}
		})
	}
}

func TestAnInRangeRenameIsStillFollowed(t *testing.T) {
	s, m := applyServer(t, `[{"username":"bob_renamed","id":11,"uid":300001,"locked":false,"smb":false,"groups":[]}]`)
	if err := s.store.PutBinding(Binding{SubjectID: "sub-1", Username: "bob", UID: 300001, UIDKind: uidKindAccount}); err != nil {
		t.Fatal(err)
	}
	subjects := []Subject{{Username: "bob_renamed", UID: 300001, Enabled: true}}
	if _, ok := s.wishesFor(subjects)[300001]; !ok {
		t.Error("standing: a renamed in-range account is still bound")
	}
	out := decodeOutcome(t, postApply(t, s, oneApply))
	if out.Effect == EffectBlocked || len(m.creates) != 0 || len(m.updates) != 1 {
		t.Fatalf("a rename of an in-range account is followed: %+v, %d updates, %d creates", out, len(m.updates), len(m.creates))
	}
	if b, _, _ := s.store.GetBinding("sub-1"); b.Username != "bob_renamed" || b.UID != 300001 {
		t.Fatalf("the binding follows the rename, got %+v", b)
	}
}

func TestAnOutOfRangeBindingWhoseNameStillMatchesKeepsWorking(t *testing.T) {
	s, m := applyServer(t, `[{"username":"bob","id":11,"uid":3051,"locked":false,"smb":false,"groups":[]}]`)
	if err := s.store.PutBinding(Binding{SubjectID: "sub-1", Username: "bob", UID: 3051, UIDKind: uidKindAccount}); err != nil {
		t.Fatal(err)
	}
	subjects := []Subject{{Username: "bob", UID: 3051, Enabled: true}}
	if _, ok := s.wishesFor(subjects)[3051]; !ok {
		t.Error("standing: bob is bound")
	}
	if u, _ := s.boundUIDs(subjects); u[3051] != "sub-1" {
		t.Errorf("stats: bob's usage is his, got %+v", u)
	}
	out := decodeOutcome(t, postApply(t, s, oneApply))
	if out.Effect == EffectBlocked || len(m.creates) != 0 || len(m.updates) != 1 {
		t.Fatalf("an adopted account whose name still matches converges: %+v, %d updates, %d creates", out, len(m.updates), len(m.creates))
	}
}

// Name and number both gone: the account was deleted, and the apply re-creates it as it always has.
func TestAnOutOfRangeBindingWhoseAccountIsGoneIsRecreated(t *testing.T) {
	s, m := applyServer(t, `[]`)
	if err := s.store.PutBinding(Binding{SubjectID: "sub-1", Username: "bob", UID: 3051, UIDKind: uidKindAccount}); err != nil {
		t.Fatal(err)
	}
	decodeOutcome(t, postApply(t, s, oneApply))
	if len(m.creates) != 1 {
		t.Fatalf("want today's re-create, got %d creates", len(m.creates))
	}
}

func TestAnAdoptionIsWrittenWithTheMarkerWhateverTheUID(t *testing.T) {
	s, _ := applyServer(t, `[{"username":"becky_lynch","id":11,"uid":3001,"locked":false,"smb":true,"groups":[42]}]`)
	rr := postOperation(t, s, "account.adopt", `{"call_id":"a1","operation":"account.adopt","subject":"sub-1","actor":"op1","params":{"username":"becky_lynch"}}`)
	if rr.Code != 200 {
		t.Fatalf("%d %s", rr.Code, rr.Body.String())
	}
	b, _, _ := s.store.GetBinding("sub-1")
	if b.UID != 3001 || b.UIDKind != uidKindAccount {
		t.Fatalf("an adoption names a real uid and says so, got %+v", b)
	}
}

// A contract-1 binding has no marker, and its uid may be the record key. The apply that finds its
// account by name rewrites it once with the real uid and the marker, and from then on it is found by
// number. A record key is never stamped as account_id.
func TestAContractOneBindingIsResolvedByNameOnceAndMarked(t *testing.T) {
	for _, tc := range []struct {
		name string
		uid  int64 // the account's real uid
		key  int64 // what the contract-1 binding holds
	}{
		{"hand-made, below the range, a record key", 3051, 11},
		{"made here, in the range, a record key", 300001, 11},
		{"hand-made, the real uid already (nothing says so)", 3051, 3051},
		{"made here, the real uid already", 300001, 300001},
	} {
		t.Run(tc.name, func(t *testing.T) {
			users := `[{"username":"charlotte","id":11,"uid":` + itoa(tc.uid) + `,"locked":false,"smb":false,"groups":[]}]`
			s, m := applyServer(t, users)
			putRaw(t, s, "sub-1", Binding{SubjectID: "sub-1", Username: "charlotte", UID: tc.key}) // unmarked: what contract 1 wrote

			postApply(t, s, `{"call_id":"c1","subject":"sub-1","email":"charlotte@example.edu",
				"desired":{"group":["lab_makers"],"enabled":true,"smb_enabled":false}}`)
			if len(m.creates) != 0 {
				t.Fatalf("created %d accounts for a bound subject", len(m.creates))
			}
			b, _, _ := s.store.GetBinding("sub-1")
			if b.UID != tc.uid || b.UIDKind != uidKindAccount {
				t.Fatalf("want the real uid %d, marked, got %+v", tc.uid, b)
			}
			// Stored with the marker, and account_id stamped only now that it is a real uid.
			raw := storedBinding(t, s, "sub-1")
			if raw.UIDKind != uidKindAccount || raw.AccountID != itoa(tc.uid) {
				t.Fatalf("stored %+v", raw)
			}
		})
	}
	// A record key is not stamped as the account's stable id.
	s, _ := applyServer(t, `[]`)
	if err := s.store.PutBinding(Binding{SubjectID: "sub-1", Username: "charlotte", UID: 11}); err != nil {
		t.Fatal(err)
	}
	if raw := storedBinding(t, s, "sub-1"); raw.AccountID != "" || raw.UIDKind != "" {
		t.Fatalf("a record key must not read as a real uid: %+v", raw)
	}
}

// A stranger who holds the number a record key reads as is never matched by it.
func TestAStrangerHoldingTheRecordKeyNumberIsNeverMatched(t *testing.T) {
	s, _ := applyServer(t, `[]`)
	putRaw(t, s, "sub-old", Binding{SubjectID: "sub-old", Username: "charlotte", UID: 11}) // record key; charlotte is gone
	subjects := []Subject{{Username: "stranger", UID: 11, Enabled: true}}
	if _, ok := s.wishesFor(subjects)[11]; ok {
		t.Error("standing: the stranger is not charlotte's")
	}
	if u, _ := s.boundUIDs(subjects); len(u) != 0 {
		t.Errorf("stats: nobody's usage is the stranger's, got %+v", u)
	}
	if bound, _ := s.accountBound(&subjects[0]); bound {
		t.Error("accountBound: the stranger is bound to nobody")
	}
}

func storedBinding(t *testing.T, s *server, subject string) Binding {
	t.Helper()
	var b Binding
	err := s.store.db.View(func(tx *bolt.Tx) error { return json.Unmarshal(tx.Bucket(bucketBindings).Get([]byte(subject)), &b) })
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func itoa(n int64) string { return accountID(n) }
