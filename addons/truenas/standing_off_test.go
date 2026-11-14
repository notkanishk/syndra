package main

import (
	"path/filepath"
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
	if root.WaitingOn != "none" || !strings.Contains(root.Note, "not by Syndra") || !strings.Contains(root.Next, "assign it to a person") {
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
