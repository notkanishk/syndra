package main

import (
	"strings"
	"testing"
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

	if s := got["held"]; s.Usable || s.WaitingOn != "none" || !strings.Contains(s.Next, "Syndra keeps it off") {
		t.Errorf("Syndra asked for it off: want none and a next that says so, got %+v", s)
	}
	// A bound account whose wish is unknown (a legacy binding never applied since wishes were kept):
	// no claim either way, and the step is Syndra's own comparison.
	if s := got["never_recorded"]; s.Usable || s.WaitingOn != "none" || s.Note != "Switched off on TrueNAS." ||
		s.Next != "If no one in Syndra asked for this, use Check for differences on the system's Mappings page." {
		t.Errorf("an unknown wish makes no claim, got %+v", s)
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
	if w := s.wishesFor([]Subject{{Username: b.Username, UID: b.UID}}); w[b.UID].Enabled == nil || !*w[b.UID].Enabled {
		t.Fatalf("wishesFor: %+v", w)
	}
	rr := postApply(t, s, `{"call_id":"c2","subject":"sub-1","email":"charlotte@example.edu",
		"desired":{"group":["lab_makers"],"enabled":false,"smb_enabled":false}}`)
	b, _, _ = s.store.GetBinding("sub-1")
	if b.WantsEnabled == nil || *b.WantsEnabled {
		t.Fatalf("an apply that switches it off keeps that wish (%d): %+v", rr.Code, b)
	}
}

func TestWishesOnlyListsBoundAccounts(t *testing.T) {
	s, _ := applyServer(t, `[]`)
	if err := s.store.PutBinding(Binding{SubjectID: "sub-1", Username: "charlotte", UID: 300001, UIDKind: uidKindAccount}); err != nil {
		t.Fatal(err)
	}
	w := s.wishesFor([]Subject{{Username: "charlotte", UID: 300001}, {Username: "root", UID: 0}})
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
		if err := s.store.PutBinding(Binding{SubjectID: "sub-1", Username: "charlotte", UID: 3001, UIDKind: uidKindAccount, WantsEnabled: kept}); err != nil {
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
