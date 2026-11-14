package main

import (
	"encoding/json"
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

func TestAnAdoptedAccountOutsideTheRangeIsStillFoundByNumberAfterARename(t *testing.T) {
	// Adopted: account.adopt writes a marked binding, whatever the uid.
	s, m := applyServer(t, `[{"username":"rey_renamed","id":11,"uid":3051,"locked":false,"smb":false,"groups":[]}]`)
	if lo, _ := s.idRange(); lo != defaultIDFloor {
		t.Fatalf("this test is about the production default range, got floor %d", lo)
	}
	if err := s.store.PutBinding(Binding{SubjectID: "sub-rey", Username: "rey_old_name", UID: 3051, UIDKind: uidKindAccount}); err != nil {
		t.Fatal(err)
	}
	subjects := []Subject{{Username: "rey_renamed", UID: 3051, Enabled: true}}

	if _, ok := s.wishesFor(subjects)[3051]; !ok {
		t.Error("standing: the renamed account is bound")
	}
	if u, _ := s.boundUIDs(subjects); u[3051] != "sub-rey" {
		t.Errorf("stats: the renamed account is its subject's, got %+v", u)
	}
	if bound, _ := s.accountBound(&subjects[0]); !bound {
		t.Error("accountBound: the renamed account is bound")
	}

	// And an apply finds it by number: no second account, the member's data stays where it is.
	out := decodeOutcome(t, postApply(t, s, `{"call_id":"c1","subject":"sub-rey","email":"rey@example.edu",
		"desired":{"group":["lab_makers"],"enabled":true,"smb_enabled":false}}`))
	if len(m.creates) != 0 {
		t.Fatalf("a renamed adopted account was not found: the apply created %d new account(s), %+v", len(m.creates), out)
	}
	if b, _, _ := s.store.GetBinding("sub-rey"); b.Username != "rey_renamed" || b.UID != 3051 {
		t.Fatalf("the binding follows the rename, got %+v", b)
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

// A binding a later binary wrote with account_id alone holds a real uid: it reads as bound, for the
// wishes and the standing alike.
func TestABindingWithAccountIDAloneReadsAsBound(t *testing.T) {
	s, _ := applyServer(t, `[]`)
	putRaw(t, s, "sub-old", Binding{SubjectID: "sub-old", Username: "old", AccountID: accountID(3051)})
	subjects := []Subject{{Username: "old_renamed", UID: 3051, Enabled: false}}
	if _, ok := s.wishesFor(subjects)[3051]; !ok {
		t.Fatal("wishesFor: account_id alone is a real uid, matched by number")
	}
	for _, v := range subjectViews(subjects, nil, s.wishesFor(subjects)) {
		if strings.Contains(v.Standing.Note, "No one in Syndra is assigned") || v.Standing.Note != "Switched off on TrueNAS." {
			t.Errorf("a bound account with no known wish makes no claim, and is not unassigned: %+v", v.Standing)
		}
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
