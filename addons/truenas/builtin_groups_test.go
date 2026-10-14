package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// Built-in groups are the machine's, not the makerspace's. Syndra never offers,
// adds or removes one: offering builtin_administrators is handing NAS
// administration to everyone who holds a role, and removing builtin_users
// (SMB's BUILTIN\Users) takes away every share whose ACL grants it.

const groupsWithAdmin = `[
	{"id":41,"gid":545,"group":"builtin_users","builtin":true},
	{"id":43,"gid":544,"group":"builtin_administrators","builtin":true},
	{"id":42,"gid":900,"group":"lab_makers"}
]`

func valuesOf(t *testing.T, s *server, field string) (*httptest.ResponseRecorder, ValuesResponse) {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/values/"+field, nil)
	req.SetPathValue("field", field)
	rr := httptest.NewRecorder()
	s.handleValues(rr, req, nil)
	var out ValuesResponse
	_ = json.Unmarshal(rr.Body.Bytes(), &out)
	return rr, out
}

func TestBuiltinGroupsAreNeverOffered(t *testing.T) {
	s := testServer(t, &fakeRPC{users: `[]`, groups: groupsWithAdmin})
	rr, out := valuesOf(t, s, FieldGroup)
	if rr.Code != http.StatusOK {
		t.Fatalf("want 200, got %d (%s)", rr.Code, rr.Body.String())
	}
	if strings.Join(out.Values, ",") != "lab_makers" {
		t.Fatalf("only the makerspace's own groups may be offered, got %v", out.Values)
	}
}

func TestBuiltinGroupIsNotReportedOnASubject(t *testing.T) {
	s := testServer(t, &fakeRPC{users: fixtureUsers, groups: groupsWithAdmin})
	snap, err := s.readSubjects()
	if err != nil {
		t.Fatal(err)
	}
	for _, sub := range snap.Subjects {
		for _, g := range sub.Groups {
			if g == "builtin_users" {
				t.Fatalf("%s reports a built-in group, which no mapping can explain: %v", sub.Username, sub.Groups)
			}
		}
	}
}

func TestConvergeCarriesBuiltinMembershipsThrough(t *testing.T) {
	s, m := applyServer(t, `[{"username":"ada","id":11,"uid":3001,"locked":false,"smb":true,"groups":[41]}]`)
	m.fakeRPC.groups = groupsWithAdmin
	if err := s.store.PutBinding(Binding{SubjectID: "sub-1", Username: "ada", UID: 3001}); err != nil {
		t.Fatal(err)
	}

	rr := postApply(t, s, `{"call_id":"c1","subject":"sub-1","email":"ada@example.edu","desired":{"group":["lab_makers"]}}`)
	if decodeOutcome(t, rr).Effect != EffectApplied {
		t.Fatalf("want applied: %s", rr.Body.String())
	}
	groups, _ := m.updates[0]["groups"].([]apiID)
	var ids []string
	for _, g := range groups {
		ids = append(ids, g.String())
	}
	if strings.Join(ids, ",") != "42,41" {
		t.Fatalf("want lab_makers added and builtin_users kept, got %v", ids)
	}
}

func TestConvergeIsANoOpWhenOnlyBuiltinGroupsDiffer(t *testing.T) {
	s, m := applyServer(t, `[{"username":"ada","id":11,"uid":3001,"locked":false,"smb":true,"groups":[41,42]}]`)
	m.fakeRPC.groups = groupsWithAdmin
	if err := s.store.PutBinding(Binding{SubjectID: "sub-1", Username: "ada", UID: 3001}); err != nil {
		t.Fatal(err)
	}

	rr := postApply(t, s, `{"call_id":"c1","subject":"sub-1","email":"ada@example.edu","desired":{"group":["lab_makers"]}}`)
	if out := decodeOutcome(t, rr); out.Effect != EffectNoChange {
		t.Fatalf("builtin_users is not Syndra's to reconcile, want no change, got %s (%s)", out.Effect, rr.Body.String())
	}
	if len(m.updates) != 0 {
		t.Fatalf("nothing to write, got %v", m.updates)
	}
}

func TestDesiringABuiltinGroupIsRefused(t *testing.T) {
	s, m := applyServer(t, `[{"username":"ada","id":11,"uid":3001,"locked":false,"smb":true,"groups":[42]}]`)
	m.fakeRPC.groups = groupsWithAdmin
	if err := s.store.PutBinding(Binding{SubjectID: "sub-1", Username: "ada", UID: 3001}); err != nil {
		t.Fatal(err)
	}

	rr := postApply(t, s, `{"call_id":"c1","subject":"sub-1","email":"ada@example.edu","desired":{"group":["builtin_administrators"]}}`)
	if rr.Code != http.StatusUnprocessableEntity {
		t.Fatalf("a mapping to a built-in group must fail loudly, got %d (%s)", rr.Code, rr.Body.String())
	}
	if len(m.updates) != 0 {
		t.Fatal("nothing may be written")
	}
}

func TestATruncatedGroupReadIsAnError(t *testing.T) {
	rows := make([]string, 0, subjectReadCap+1)
	for i := 0; i <= subjectReadCap; i++ {
		rows = append(rows, fmt.Sprintf(`{"id":%d,"gid":%d,"group":"g%d"}`, 1000+i, 10000+i, i))
	}
	s := testServer(t, &fakeRPC{users: fixtureUsers, groups: "[" + strings.Join(rows, ",") + "]"})

	if rr, _ := valuesOf(t, s, FieldGroup); rr.Code == http.StatusOK {
		t.Fatal("half a list offered as the whole one")
	}
	if _, err := s.readSubjects(); !errors.Is(err, errGroupsTruncated) {
		t.Fatalf("convergence must refuse a partial group index, got %v", err)
	}
}

// A NAS whose groups are all built in offers nothing, and says what it left
// out, so the picker can explain the empty list instead of looking broken.
func TestAnAllBuiltinNASSaysWhatItLeftOut(t *testing.T) {
	s := testServer(t, &fakeRPC{users: `[]`, groups: `[
		{"id":41,"gid":545,"group":"builtin_users","builtin":true},
		{"id":43,"gid":544,"group":"builtin_administrators","builtin":true},
		{"id":44,"gid":546,"group":"builtin_guests","builtin":true}]`})
	rr, out := valuesOf(t, s, FieldGroup)
	if rr.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", rr.Code)
	}
	if len(out.Values) != 0 || out.Values == nil {
		t.Fatalf("values = %#v, want an empty list", out.Values)
	}
	if out.BuiltinCount != 3 || strings.Join(out.BuiltinExamples, ",") != "builtin_users,builtin_administrators" {
		t.Fatalf("count=%d examples=%v; the first two in read order", out.BuiltinCount, out.BuiltinExamples)
	}
}
