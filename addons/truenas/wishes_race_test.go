package main

import (
	"sync"
	"testing"
	"time"
)

// deriveWishes races the writers of a binding at cutover (the first drift pass's applies, a release,
// a purge). Each binding is settled alone, under its subject's lock, by a compare-and-set; these hold
// the subject's lock the way an apply does, let the derivation reach it, change the binding under it,
// and release it.

func legacySnapshot() Snapshot {
	return Snapshot{Subjects: []Subject{
		{Username: "charlotte", UID: 300001, Enabled: true, SMBEnabled: true},
	}}
}

// derivingAround runs deriveWishes on a goroutine that has to wait for sub-1's lock, which the test
// holds while it changes the binding, and returns once the derivation has finished.
func derivingAround(t *testing.T, s *server, snap Snapshot, change func()) {
	t.Helper()
	unlock := s.subjects.lock("sub-1")
	var done sync.WaitGroup
	done.Add(1)
	go func() { defer done.Done(); s.deriveWishes(snap) }()
	// It has begun (counted in flight) and is read or reading the bindings; whichever side of the
	// change it reads on, a correct derivation ends with the binding the change left.
	for i := 0; i < 200 && s.life.InFlight() == 0; i++ {
		time.Sleep(time.Millisecond)
	}
	time.Sleep(50 * time.Millisecond)
	change()
	unlock()
	done.Wait()
}

func legacy(t *testing.T, s *server) Binding {
	t.Helper()
	b := Binding{SubjectID: "sub-1", Username: "charlotte", UID: 300001}
	if err := s.store.PutBinding(b); err != nil {
		t.Fatal(err)
	}
	return b
}

func TestADerivationNeverResurrectsAReleasedBinding(t *testing.T) {
	s, _ := applyServer(t, `[]`)
	legacy(t, s)
	derivingAround(t, s, legacySnapshot(), func() {
		if err := s.store.DeleteBinding("sub-1"); err != nil { // a purge or a release
			t.Fatal(err)
		}
	})
	if _, bound, _ := s.store.GetBinding("sub-1"); bound {
		t.Fatal("the derivation wrote a deleted binding back: the next apply would re-create a deleted account")
	}
}

func TestADerivationNeverOverwritesAWishAnApplyWroteFirst(t *testing.T) {
	s, _ := applyServer(t, `[]`)
	b := legacy(t, s)
	off := false
	derivingAround(t, s, legacySnapshot(), func() { // the account is on in the read; the apply's pre-lock wish is off
		b.WantsEnabled = &off
		if err := s.store.PutBinding(b); err != nil {
			t.Fatal(err)
		}
	})
	got, _, _ := s.store.GetBinding("sub-1")
	if got.WantsEnabled == nil || *got.WantsEnabled {
		t.Fatalf("an off wish an apply wrote was overwritten with the account's state: %v", got.WantsEnabled)
	}
	if got.WantsSMB == nil || !*got.WantsSMB {
		t.Fatalf("what was still unset is derived, got %v", got.WantsSMB)
	}
}

func TestADerivationNeverRevertsARenamedBindingAndTriesAgain(t *testing.T) {
	s, _ := applyServer(t, `[]`)
	b := legacy(t, s)
	derivingAround(t, s, legacySnapshot(), func() { // locate followed a rename
		b.Username = "charlotte_renamed"
		if err := s.store.PutBinding(b); err != nil {
			t.Fatal(err)
		}
	})
	got, _, _ := s.store.GetBinding("sub-1")
	if got.Username != "charlotte_renamed" || got.WantsEnabled != nil {
		t.Fatalf("the rename was reverted or overwritten: %+v", got)
	}
	if _, done, _ := s.store.GetSetting(settingWishesDerived); done {
		t.Fatal("a binding that changed under the read is not settled: the marker waits for the next read")
	}
	s.deriveWishes(Snapshot{Subjects: []Subject{{Username: "charlotte_renamed", UID: 300001, Enabled: false}}})
	got, _, _ = s.store.GetBinding("sub-1")
	if got.WantsEnabled == nil || *got.WantsEnabled || got.Username != "charlotte_renamed" {
		t.Fatalf("the next read finishes it: %+v", got)
	}
	if _, done, _ := s.store.GetSetting(settingWishesDerived); !done {
		t.Fatal("and records the marker")
	}
}

// Only an active add-on derives, and it counts as in flight while it does.
func TestADerivationWaitsForAnActiveAddOn(t *testing.T) {
	for _, state := range []string{LifecycleReadOnly, LifecycleDraining} {
		t.Run(state, func(t *testing.T) {
			s, _ := applyServer(t, `[]`)
			legacy(t, s)
			if err := s.life.Set(state, "maintenance"); err != nil {
				t.Fatal(err)
			}
			s.deriveWishes(legacySnapshot())
			if b, _, _ := s.store.GetBinding("sub-1"); b.WantsEnabled != nil {
				t.Fatalf("%s: a bookkeeping write was made", state)
			}
			if _, done, _ := s.store.GetSetting(settingWishesDerived); done {
				t.Fatalf("%s: the marker was set", state)
			}
		})
	}
	s, _ := applyServer(t, `[]`)
	legacy(t, s)
	s.deriveWishes(legacySnapshot())
	if _, done, _ := s.store.GetSetting(settingWishesDerived); !done {
		t.Fatal("an active add-on derives")
	}
	if s.life.InFlight() != 0 {
		t.Fatalf("the derivation left %d in flight", s.life.InFlight())
	}
}

// A read a write began during is never kept (the fresher's generation), and what was read then is
// not acted on.
func TestOnlyAKeptReadIsActedOn(t *testing.T) {
	var f fresher
	at := time.Date(2026, 11, 15, 9, 0, 0, 0, time.UTC)
	_, _, _ = f.read("subjects", true, func() (any, time.Time, error) { return 1, at, nil })
	if !f.keeps("subjects", at) {
		t.Fatal("a read nothing interrupted is kept")
	}
	later := at.Add(time.Hour)
	_, _, _ = f.read("subjects", true, func() (any, time.Time, error) {
		f.forget() // a write begins during the read
		return 2, later, nil
	})
	if f.keeps("subjects", later) {
		t.Fatal("a read a write began during is a read of the past and is not kept")
	}
}

// A contract-1 binding holds a record key, not a uid. One rule says which account it names, boundAccount's:
// an uid outside this add-on's range names none by number, only the account its name finds, and a
// stranger who holds that number is never matched.
func TestARecordKeyBindingNamesTheAccountItsNameFinds(t *testing.T) {
	s, _ := applyServer(t, `[]`)
	if err := s.store.PutBinding(Binding{SubjectID: "sub-old", Username: "charlotte", UID: 11}); err != nil { // 11: a record key
		t.Fatal(err)
	}
	snap := Snapshot{Subjects: []Subject{
		{Username: "stranger", UID: 11, Enabled: true}, // holds the number the record key reads as
		{Username: "charlotte", UID: 300001, Enabled: false},
	}}

	w := s.wishesFor(snap.Subjects)
	if _, ok := w[300001]; !ok {
		t.Errorf("wishes: the account the name finds is bound, got %+v", w)
	}
	if _, ok := w[11]; ok {
		t.Errorf("wishes: a stranger holding the record key's number is not bound, got %+v", w)
	}
	if u, _ := s.boundUIDs(snap.Subjects); u[300001] != "sub-old" || len(u) != 1 {
		t.Errorf("boundUIDs: %+v", u)
	}
	if bound, _ := s.accountBound(&snap.Subjects[0]); bound {
		t.Error("accountBound: the stranger is not bound")
	}
	if bound, _ := s.accountBound(&snap.Subjects[1]); !bound {
		t.Error("accountBound: the account the name finds is bound")
	}

	s.deriveWishes(snap)
	b, _, _ := s.store.GetBinding("sub-old")
	if b.WantsEnabled == nil || *b.WantsEnabled {
		t.Fatalf("derived from the account its name finds (off), not the stranger (on): %+v", b.WantsEnabled)
	}
}

// DeriveWish is the compare-and-set itself: identity, then unset, then one write.
func TestDeriveWishChecksBeforeItWrites(t *testing.T) {
	s, _ := applyServer(t, `[]`)
	b := legacy(t, s)
	if out, err := s.store.DeriveWish(Binding{SubjectID: "sub-1", Username: "charlotte", UID: 300002}, true, true); err != nil || out != wishChanged {
		t.Fatalf("another account: %v %v", out, err)
	}
	if out, err := s.store.DeriveWish(Binding{SubjectID: "nobody", Username: "x", UID: 1}, true, true); err != nil || out != wishGone {
		t.Fatalf("gone: %v %v", out, err)
	}
	if out, err := s.store.DeriveWish(b, true, false); err != nil || out != wishWritten {
		t.Fatalf("written: %v %v", out, err)
	}
	if out, err := s.store.DeriveWish(b, false, true); err != nil || out != wishSet {
		t.Fatalf("set: %v %v", out, err)
	}
	got, _, _ := s.store.GetBinding("sub-1")
	if got.WantsEnabled == nil || !*got.WantsEnabled || got.WantsSMB == nil || *got.WantsSMB {
		t.Fatalf("the second call overwrote the first: %+v", got)
	}
}
