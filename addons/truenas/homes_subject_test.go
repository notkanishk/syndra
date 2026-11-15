package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// 7t-3: /homes says whose each home is by the same attribution /subjects
// serves (attribute), so Syndra matches a home to a person by subject, never by
// the name its own copy recorded. A home whose account no binding names carries
// no subject, whatever it is called.
func TestHomesCarryTheSubjectTheBindingNames(t *testing.T) {
	fake := reachFake()
	fake.users = `[{"username":"bob_renamed","id":11,"uid":3051,"locked":false,"smb":true,"groups":[],"home":"/mnt/tank/main/bob"},
		{"username":"bob","id":12,"uid":3099,"locked":false,"smb":true,"groups":[],"home":"/mnt/tank/main/stranger"}]`
	s := testServer(t, fake)
	// bob's account was renamed by hand; a stranger now holds the name. The
	// binding is in range of nothing, so the name finds the stranger: a
	// conflict, and no home is bob's.
	if err := s.store.PutBinding(Binding{SubjectID: "sub-1", Username: "bob", UID: 3051, UIDKind: uidKindAccount}); err != nil {
		t.Fatal(err)
	}
	// A second subject whose account is found as recorded.
	if err := s.store.PutBinding(Binding{SubjectID: "sub-2", Username: "bob_renamed", UID: 3051, UIDKind: uidKindAccount}); err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	s.handleHomes(rr, httptest.NewRequest(http.MethodGet, "/homes?fresh=true", nil), nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("homes: %d %s", rr.Code, rr.Body)
	}
	assertSchema(t, "profiles/storage/homes.json", rr.Body.Bytes(), "/homes")
	var got HomesResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	subjects := map[string]string{}
	for _, h := range got.Homes {
		subjects[h.Username] = h.Subject
	}
	// The same verdict /subjects gives for this read.
	_, view := getSubjects(t, s, false)
	want := map[string]string{}
	for _, v := range view.Subjects {
		want[v.Username] = v.Subject
	}
	if len(subjects) != 2 || subjects["bob_renamed"] != want["bob_renamed"] || subjects["bob"] != want["bob"] {
		t.Fatalf("homes %v, /subjects %v: one attribution", subjects, want)
	}
	if subjects["bob_renamed"] != "sub-2" || subjects["bob"] != "" {
		t.Fatalf("want bob_renamed's home sub-2's and the stranger's nobody's, got %v", subjects)
	}
}

// The bindings each home is attributed by could not be read: the add-on's
// records failed, so 500 STORE_UNREADABLE, never TARGET_UNREADABLE blaming the
// NAS and never homes with nobody's subject.
func TestHomesWithAnUnreadableBindingAreStoreUnreadable(t *testing.T) {
	fake := reachFake()
	fake.users = `[{"username":"bob","id":11,"uid":3051,"locked":false,"smb":true,"groups":[],"home":"/mnt/tank/main/bob"}]`
	s := testServer(t, fake)
	if err := s.store.PutBinding(Binding{SubjectID: "sub-1", Username: "bob", UID: 3051, UIDKind: uidKindAccount}); err != nil {
		t.Fatal(err)
	}
	corrupt(t, s, bucketBindings, "sub-1")
	rr := httptest.NewRecorder()
	s.handleHomes(rr, httptest.NewRequest(http.MethodGet, "/homes?fresh=true", nil), nil)
	if rr.Code != http.StatusInternalServerError || problemCode(t, rr) != CodeStoreUnreadable {
		t.Fatalf("want 500 %s, got %d %s", CodeStoreUnreadable, rr.Code, rr.Body)
	}
}
