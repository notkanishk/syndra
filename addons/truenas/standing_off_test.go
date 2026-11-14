package main

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	bolt "go.etcd.io/bbolt"
)

// An account that is off is off for one of three reasons, and a standing says only the one that is
// true: Syndra asked for it (AUTHORING: `waiting_on: none`), TrueNAS has it off though Syndra wants it
// on (a difference made outside Syndra, which an operator sorts out in Syndra), or no subject is
// bound to it (Syndra never asked for anything of it).
func TestAnAccountThatIsOffSaysWhoSwitchedItOff(t *testing.T) {
	yes, no := true, false
	subs := func() []Subject {
		return []Subject{
			{Username: "held", UID: 300001, Enabled: false},
			{Username: "by_hand", UID: 300002, Enabled: false},
			{Username: "never_recorded", UID: 300003, Enabled: false},
			{Username: "root", UID: 0, Enabled: false},
		}
	}
	wishes := map[int64]Wish{
		300001: {Enabled: &no},
		300002: {Enabled: &yes},
		300003: {}, // bound before the wish was kept
	}
	got := map[string]*Standing{}
	for _, v := range subjectViews(subs(), nil, wishes) {
		got[v.Username] = v.Standing
	}

	if s := got["held"]; s.Usable || s.WaitingOn != "none" || s.Next != "" {
		t.Errorf("Syndra holds it off: want none and no next of its own, got %+v", s)
	}
	if s := got["never_recorded"]; s.WaitingOn != "none" {
		t.Errorf("a binding with no wish keeps the rule it always had, got %+v", s)
	}
	by := got["by_hand"]
	if by.WaitingOn != "operator" || !strings.Contains(by.Note, "outside Syndra") || !strings.Contains(by.Next, "Makerspace staff can use Check for differences") {
		t.Errorf("off though Syndra wants it on: want operator, outside Syndra, and a Syndra-side next, got %+v", by)
	}
	root := got["root"]
	if root.WaitingOn != "none" || !strings.Contains(root.Note, "No one in Syndra is assigned") || !strings.Contains(root.Next, "assign it to a person") {
		t.Errorf("bound to nobody: want no claim of Syndra's, got %+v", root)
	}
	for name, s := range got {
		if strings.Contains(s.Next, "by hand") || strings.Contains(strings.ToLower(s.Next), "switch it back on") {
			t.Errorf("%s: a next step never sends the operator to change it on TrueNAS: %q", name, s.Next)
		}
	}

	// A wishes map that could not be read says nothing of who switched it off.
	for _, v := range subjectViews(subs(), nil, nil) {
		if v.Standing.WaitingOn != "none" || strings.Contains(v.Standing.Note, "Syndra.") {
			t.Errorf("%s: unread wishes must claim nothing, got %+v", v.Username, v.Standing)
		}
	}
}

// What an apply asks for the account is kept on the binding, in the write that makes it, and a later
// apply that asks otherwise changes it, so the standing follows Syndra's wish.
func TestAnApplyKeepsWhetherItWantedTheAccountOn(t *testing.T) {
	s, _ := applyServer(t, `[]`)
	postApply(t, s, `{"call_id":"c1","subject":"sub-1","email":"charlotte@example.edu",
		"desired":{"group":["lab_makers"],"enabled":true,"smb_enabled":false}}`)
	b, bound, _ := s.store.GetBinding("sub-1")
	if !bound || b.WantsEnabled == nil || !*b.WantsEnabled {
		t.Fatalf("a create wanting the account on keeps that wish: %+v", b)
	}
	w, err := s.store.Wishes()
	if err != nil || w[b.UID].Enabled == nil || !*w[b.UID].Enabled {
		t.Fatalf("Wishes: %v %+v", err, w)
	}
	rr := postApply(t, s, `{"call_id":"c2","subject":"sub-1","email":"charlotte@example.edu",
		"desired":{"group":["lab_makers"],"enabled":false,"smb_enabled":false}}`)
	b, _, _ = s.store.GetBinding("sub-1")
	if b.WantsEnabled == nil || *b.WantsEnabled {
		t.Fatalf("an apply that switches it off keeps that wish (%d): %+v", rr.Code, b)
	}
}

func TestWishesOnlyListsBoundAccounts(t *testing.T) {
	store, err := OpenStore(filepath.Join(t.TempDir(), "state.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { store.Close() })
	if err := store.PutBinding(Binding{SubjectID: "sub-1", Username: "charlotte", UID: 300001}); err != nil {
		t.Fatal(err)
	}
	w, _ := store.Wishes()
	if _, ok := w[300001]; !ok || len(w) != 1 || w[300001].Enabled != nil {
		t.Fatalf("want the one bound account, with no wish yet: %+v", w)
	}
}

// The record of what Syndra wanted is never false against the account: an off wish is written before
// the lock (it accuses nobody), an on wish only once an enable has landed (not blocked, not in doubt).
func TestAWishIsRecordedOnlyWhenItCannotAccuseAnyone(t *testing.T) {
	const off = `[{"username":"charlotte","id":11,"uid":3001,"locked":true,"smb":false,"groups":[42]}]`
	const on = `[{"username":"charlotte","id":11,"uid":3001,"locked":false,"smb":false,"groups":[42]}]`
	wish := func(t *testing.T, s *server) *bool {
		t.Helper()
		b, bound, err := s.store.GetBinding("sub-1")
		if err != nil || !bound {
			t.Fatalf("binding: %v %v", bound, err)
		}
		return b.WantsEnabled
	}
	setup := func(t *testing.T, users string, kept *bool) (*server, *mutatingRPC) {
		s, m := applyServer(t, users)
		if err := s.store.PutBinding(Binding{SubjectID: "sub-1", Username: "charlotte", UID: 3001, WantsEnabled: kept}); err != nil {
			t.Fatal(err)
		}
		return s, m
	}
	no, yes := false, true

	t.Run("a blocked enable records nothing", func(t *testing.T) {
		s, _ := setup(t, off, &no)
		out := decodeOutcome(t, postApply(t, s, `{"call_id":"c1","subject":"sub-1","email":"charlotte@example.edu",
			"desired":{"group":["no_such_group"],"enabled":true,"smb_enabled":true}}`))
		if out.Effect != EffectBlocked {
			t.Fatalf("want blocked, got %+v", out)
		}
		if b, _, _ := s.store.GetBinding("sub-1"); b.WantsSMB != nil {
			t.Fatalf("a blocked apply changed nothing, so it records no SMB wish either, got %v", *b.WantsSMB)
		}
		if w := wish(t, s); w == nil || *w {
			t.Fatalf("a blocked enable left the account off: the wish must still be off, got %v", w)
		}
	})
	t.Run("a lock in doubt keeps the off wish", func(t *testing.T) {
		s, m := setup(t, on, &yes)
		m.failReadAfterUpdate = true
		out := decodeOutcome(t, postApply(t, s, `{"call_id":"c1","subject":"sub-1","email":"charlotte@example.edu",
			"desired":{"group":["lab_makers"],"enabled":false}}`))
		if !out.Unverified {
			t.Fatalf("want an unverified outcome, got %+v", out)
		}
		if w := wish(t, s); w == nil || *w {
			t.Fatalf("the lock may be off on the NAS: the wish must be off, got %v", w)
		}
	})
	t.Run("a lock whose write fails on the wire is already recorded off", func(t *testing.T) {
		s, m := setup(t, on, &yes)
		m.resetOn = "user.update" // the target may have acted on it
		postApply(t, s, `{"call_id":"c1","subject":"sub-1","email":"charlotte@example.edu",
			"desired":{"group":["lab_makers"],"enabled":false}}`)
		if w := wish(t, s); w == nil || *w {
			t.Fatalf("the off wish is written before the lock, so a lock in doubt cannot read as drift, got %v", w)
		}
	})
	t.Run("an enable in doubt records no on wish", func(t *testing.T) {
		s, m := setup(t, off, &no)
		m.failReadAfterUpdate = true
		out := decodeOutcome(t, postApply(t, s, `{"call_id":"c1","subject":"sub-1","email":"charlotte@example.edu",
			"desired":{"group":["lab_makers"],"enabled":true}}`))
		if !out.Unverified {
			t.Fatalf("want an unverified outcome, got %+v", out)
		}
		if w := wish(t, s); w == nil || *w {
			t.Fatalf("an enable nobody confirmed must not record an on wish, got %v", w)
		}
	})
	t.Run("an enable that landed records on", func(t *testing.T) {
		s, _ := setup(t, off, &no)
		out := decodeOutcome(t, postApply(t, s, `{"call_id":"c1","subject":"sub-1","email":"charlotte@example.edu",
			"desired":{"group":["lab_makers"],"enabled":true}}`))
		if out.Effect != EffectApplied || out.Unverified {
			t.Fatalf("want a verified apply, got %+v", out)
		}
		if w := wish(t, s); w == nil || !*w {
			t.Fatalf("want an on wish after an enable that landed, got %v", w)
		}
	})
	t.Run("a lock that landed records off, and before the write", func(t *testing.T) {
		s, m := setup(t, on, &yes)
		out := decodeOutcome(t, postApply(t, s, `{"call_id":"c1","subject":"sub-1","email":"charlotte@example.edu",
			"desired":{"group":["lab_makers"],"enabled":false}}`))
		if out.Effect != EffectApplied {
			t.Fatalf("want applied, got %+v", out)
		}
		if w := wish(t, s); w == nil || *w {
			t.Fatalf("want an off wish, got %v", w)
		}
		if len(m.updates) != 1 {
			t.Fatalf("one lock write, got %d", len(m.updates))
		}
	})
}

// Bindings from before wishes were kept (contract 1's) arrive with none. A hand switch-off of such an
// account is a difference nothing converges, so no apply would ever write its wish: at the first
// complete read each unset wish is set to what the NAS holds, logged like every binding change,
// once; after that a hand switch-off reads as outside Syndra.
func TestLegacyBindingsGetTheirWishesOnceFromTheFirstCompleteRead(t *testing.T) {
	s, _ := applyServer(t, `[]`)
	for _, b := range []Binding{
		{SubjectID: "sub-on", Username: "on_acct", UID: 300001},
		{SubjectID: "sub-off", Username: "off_acct", UID: 300002},
		{SubjectID: "sub-gone", Username: "gone", UID: 300009}, // no longer on the NAS
	} {
		if err := s.store.PutBinding(b); err != nil {
			t.Fatal(err)
		}
	}
	// Legacy shape, written as an older or later binary would: account_id alone, no uid.
	raw, _ := json.Marshal(Binding{SubjectID: "sub-idonly", Username: "idonly", AccountID: accountID(300003)})
	if err := s.store.db.Update(func(tx *bolt.Tx) error { return tx.Bucket(bucketBindings).Put([]byte("sub-idonly"), raw) }); err != nil {
		t.Fatal(err)
	}
	snap := Snapshot{Subjects: []Subject{
		{Username: "on_acct", UID: 300001, Enabled: true, SMBEnabled: true},
		{Username: "off_acct", UID: 300002, Enabled: false},
		{Username: "idonly", UID: 300003, Enabled: true},
	}}
	before := len(records(t, s))

	s.deriveWishes(Snapshot{Subjects: snap.Subjects, Truncated: true})
	if _, done, _ := s.store.GetSetting(settingWishesDerived); done || len(records(t, s)) != before {
		t.Fatal("a truncated read proves nothing and decides nothing")
	}

	s.deriveWishes(snap)
	w, _ := s.store.Wishes()
	if e := w[300001].Enabled; e == nil || !*e || !w[300001].SMB {
		t.Errorf("an account that is on: want an on wish and SMB, got %+v", w[300001])
	}
	if e := w[300002].Enabled; e == nil || *e {
		t.Errorf("an account that is off: want an off wish, got %+v", w[300002])
	}
	if e := w[300003].Enabled; e == nil || !*e {
		t.Errorf("a binding that kept account_id alone is bound and derived, got %+v", w[300003])
	}
	if g, ok := w[300009]; !ok || g.Enabled != nil {
		t.Errorf("an account no longer on the NAS keeps no wish, got %+v", g)
	}
	if _, done, _ := s.store.GetSetting(settingWishesDerived); !done {
		t.Fatal("the marker is recorded")
	}
	logged := 0
	for _, r := range records(t, s)[before:] {
		if r.Operation == "wishes.derive" {
			logged++
		}
	}
	// Three bindings, each an intent and a result, and the marker's.
	if logged != 8 {
		t.Errorf("every derived write goes through the mutation log: want 8 records, got %d", logged)
	}

	// Never again: a changed account, a new read, and nothing is written or rewritten.
	after := len(records(t, s))
	snap.Subjects[0].Enabled = false // switched off by hand on TrueNAS
	s.deriveWishes(snap)
	if len(records(t, s)) != after {
		t.Fatal("the marker stops a second pass")
	}
	standing := map[string]*Standing{}
	for _, v := range subjectViews(snap.Subjects, nil, s.wishes()) {
		standing[v.Username] = v.Standing
	}
	if st := standing["on_acct"]; st.WaitingOn != "operator" || !strings.Contains(st.Note, "outside Syndra") {
		t.Errorf("a later hand switch-off reads as outside Syndra, got %+v", st)
	}
	if st := standing["off_acct"]; st.WaitingOn != "none" {
		t.Errorf("an account Syndra left off stays Syndra's, got %+v", st)
	}
}

// One reading of a binding's account: a binding that kept account_id alone (or whose uid is zero) is
// bound, for the wishes as for every other "is this account bound".
func TestOneMatcherSaysWhichAccountABindingNames(t *testing.T) {
	s, _ := applyServer(t, `[]`)
	raw, _ := json.Marshal(Binding{SubjectID: "sub-old", Username: "old", AccountID: accountID(300003)})
	if err := s.store.db.Update(func(tx *bolt.Tx) error { return tx.Bucket(bucketBindings).Put([]byte("sub-old"), raw) }); err != nil {
		t.Fatal(err)
	}
	if bound, _ := s.store.UIDBound(300003); !bound {
		t.Error("UIDBound reads the legacy binding as bound")
	}
	if w, _ := s.store.Wishes(); len(w) != 1 || func() bool { _, ok := w[300003]; return !ok }() {
		t.Errorf("Wishes reads it the same way, got %+v", w)
	}
	if u, _ := s.store.BoundUIDs(); u[300003] != "sub-old" {
		t.Errorf("BoundUIDs reads it the same way, got %+v", u)
	}
	for _, v := range subjectViews([]Subject{{Username: "old", UID: 300003, Enabled: false}}, nil, mustWishes(t, s)) {
		if strings.Contains(v.Standing.Note, "No one in Syndra") {
			t.Errorf("a bound account must not read as unassigned: %+v", v.Standing)
		}
	}
}

func mustWishes(t *testing.T, s *server) map[int64]Wish {
	t.Helper()
	w, err := s.store.Wishes()
	if err != nil {
		t.Fatal(err)
	}
	return w
}

// The derivation runs from the first live read of the NAS a request makes, after it has answered; the
// next read then carries the wish.
func TestTheFirstLiveSubjectsReadDerivesTheWishes(t *testing.T) {
	s, _ := applyServer(t, `[{"username":"charlotte","id":11,"uid":3001,"locked":true,"smb":false,"groups":[42]}]`)
	if err := s.store.PutBinding(Binding{SubjectID: "sub-1", Username: "charlotte", UID: 3001}); err != nil {
		t.Fatal(err)
	}
	getSubjects(t, s, true)
	if w := mustWishes(t, s)[3001]; w.Enabled == nil || *w.Enabled {
		t.Fatalf("an account left off gets an off wish from the first live read, got %+v", w)
	}
	if _, done, _ := s.store.GetSetting(settingWishesDerived); !done {
		t.Fatal("and the marker")
	}
}
