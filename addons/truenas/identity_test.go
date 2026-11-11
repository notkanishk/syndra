package main

import (
	"net/http"
	"path/filepath"
	"testing"
)

// Identity numbers come from the add-on's own range and are never handed out
// twice: a new account must never own an old member's files.

func TestAnIdentityIsNeverHandedOutTwice(t *testing.T) {
	store, err := OpenStore(filepath.Join(t.TempDir(), "state.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { store.Close() })

	first, _ := store.ClaimID(200000, 999999, 0)
	second, _ := store.ClaimID(200000, 999999, 0)
	if first != 200000 || second != 200001 {
		t.Fatalf("want 200000 then 200001, got %d %d", first, second)
	}
	// Something on the NAS already sits higher in the range: go above it.
	third, _ := store.ClaimID(200000, 999999, 250000)
	if third != 250001 {
		t.Fatalf("want above the highest number the target reports, got %d", third)
	}
	// And never back down, even if that account disappears.
	fourth, _ := store.ClaimID(200000, 999999, 0)
	if fourth != 250002 {
		t.Fatalf("a number once handed out stays spent, got %d", fourth)
	}
	if _, err := store.ClaimID(200000, 250002, 0); err == nil {
		t.Fatal("a used-up range must refuse, never wrap around")
	}
}

func TestANewAccountGetsItsOwnNumberAndPersonalGroup(t *testing.T) {
	s, m := applyServer(t, `[]`)
	rr := postApply(t, s, `{"call_id":"c1","subject":"sub-9","email":"zoe@example.edu","desired":{"group":["lab_makers"],"enabled":true,"smb_enabled":true}}`)
	if out := decodeOutcome(t, rr); out.Effect != EffectApplied {
		t.Fatalf("want applied, got %s (%s)", out.Effect, rr.Body.String())
	}
	c := m.creates[0]
	if c["uid"] != int64(200000) || c["group_create"] != false || m.groupCreates != 1 {
		t.Fatalf("want uid 200000 in its own personal group, got %v (groups made %d)", c, m.groupCreates)
	}
	args, _ := lastCallParams(m, "group.create").([]any)
	g, _ := args[0].(map[string]any)
	if g["gid"] != int64(200000) || g["name"] != "zoe" {
		t.Fatalf("the personal group shares the number and the name, got %v", g)
	}
}

func TestARefusedAccountTakesItsPersonalGroupWithIt(t *testing.T) {
	s, m := applyServer(t, `[]`)
	m.refuseUserCreate = true
	rr := postApply(t, s, `{"call_id":"c1","subject":"sub-9","email":"zoe@example.edu","desired":{"group":["lab_makers"],"enabled":true}}`)
	if rr.Code == http.StatusOK && decodeOutcome(t, rr).Effect == EffectApplied {
		t.Fatal("a refused create must not report success")
	}
	if m.groupCreates != 1 || m.groupDeletes != 1 {
		t.Fatalf("the personal group must be undone, made %d removed %d", m.groupCreates, m.groupDeletes)
	}
}

// The cleanup deletes only the group it made, and only while it is empty: an
// account created in it since would otherwise lose its primary group.
func TestAGroupIsUndoneOnlyWhileItIsStillEmptyAndStillOurs(t *testing.T) {
	made := apiID{raw: []byte(`92`)}
	for _, tc := range []struct {
		name, groups string
		deleted      bool
	}{
		{"empty", `[{"id":92,"gid":200000,"group":"zoe","builtin":false,"users":[]}]`, true},
		{"an account in it since", `[{"id":92,"gid":200000,"group":"zoe","builtin":false,"users":[71]}]`, false},
		{"the name now another group's", `[{"id":95,"gid":200001,"group":"zoe","builtin":false,"users":[]}]`, false},
		{"gone already", `[]`, false},
	} {
		s, m := applyServer(t, `[]`)
		m.fakeRPC.groups = tc.groups
		s.undoGroup("zoe", made)
		if got := m.groupDeletes == 1; got != tc.deleted {
			t.Errorf("%s: deleted = %t, want %t", tc.name, got, tc.deleted)
		}
	}
}
