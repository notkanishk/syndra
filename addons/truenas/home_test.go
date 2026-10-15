package main

import (
	"net/http"
	"strings"
	"testing"
)

// Home folders by role: a role says only whether it gives one, the system has
// one place they go, and each is made private by TrueNAS itself, never moved,
// never removed, never someone else's.

const homePlace = "/mnt/pool/main/home_dir"

func homeServer(t *testing.T, users string, answers map[string]string, refuse map[string]string) (*server, *mutatingRPC) {
	t.Helper()
	s, m := applyServer(t, users)
	m.fakeRPC.answers = answers
	m.fakeRPC.refuse = refuse
	if err := s.store.PutBinding(Binding{SubjectID: "sub-1", Username: "ada", UID: 3001}); err != nil {
		t.Fatal(err)
	}
	if err := s.store.PutSetting(settingHomeRoot, Setting{Value: homePlace}); err != nil {
		t.Fatal(err)
	}
	return s, m
}

var noSuchPath = map[string]string{"filesystem.stat": "[ENOENT] No such file or directory"}

func applyHome(t *testing.T, s *server, desired string) ApplyOutcome {
	t.Helper()
	rr := postApply(t, s, `{"call_id":"c1","subject":"sub-1","email":"ada@example.edu","desired":`+desired+`}`)
	if rr.Code != http.StatusOK {
		t.Fatalf("want 200, got %d (%s)", rr.Code, rr.Body.String())
	}
	return decodeOutcome(t, rr)
}

func TestARoleMakesAPrivateHomeFolder(t *testing.T) {
	s, m := homeServer(t, `[{"username":"ada","id":11,"uid":3001,"locked":false,"smb":true,"groups":[42],"home":"/var/empty"}]`, nil, noSuchPath)
	out := applyHome(t, s, `{"group":["lab_makers"],"home":["true"]}`)
	if out.Effect != EffectApplied {
		t.Fatalf("want applied, got %s (%s)", out.Effect, out.Detail)
	}
	u := m.updates[0]
	if u["home"] != homePlace+"/ada" || u["home_create"] != true || u["home_mode"] != "700" {
		t.Fatalf("want TrueNAS's own private create at %s/ada, got %v", homePlace, u)
	}
}

// The location changed after this home was made, or it was made by hand. It
// stays where it is, and that is not news: Syndra never moves a home folder.
func TestAHomeElsewhereStaysWhereItIs(t *testing.T) {
	s, m := homeServer(t, `[{"username":"ada","id":11,"uid":3001,"locked":false,"smb":true,"groups":[42],"home":"/mnt/pool/old/ada"}]`, nil, nil)
	out := applyHome(t, s, `{"group":["lab_makers"],"home":["true"]}`)
	if len(m.updates) != 0 || out.Effect != EffectNoChange {
		t.Fatalf("a home elsewhere must not be moved: %s %v", out.Effect, m.updates)
	}
}

// A role that gives a home folder on a system with no place chosen for them
// makes none, says so, and everything else still lands.
func TestNoHomeIsMadeUntilTheSystemHasAPlaceForThem(t *testing.T) {
	s, m := homeServer(t, `[{"username":"ada","id":11,"uid":3001,"locked":false,"smb":true,"groups":[],"home":"/var/empty"}]`, nil, noSuchPath)
	if err := s.store.PutSetting(settingHomeRoot, Setting{}); err != nil {
		t.Fatal(err)
	}
	out := applyHome(t, s, `{"group":["lab_makers"],"home":["true"]}`)
	if len(m.updates) != 1 {
		t.Fatalf("the groups still land, got %v", m.updates)
	}
	if _, set := m.updates[0]["home"]; set {
		t.Fatalf("a home was made with nowhere chosen for it: %v", m.updates[0])
	}
	if !strings.Contains(out.Detail, "no place for home folders chosen") {
		t.Fatalf("the outcome must say why, got %q", out.Detail)
	}
}

func TestAnotherPersonsFolderIsNeverAdopted(t *testing.T) {
	s, m := homeServer(t, `[{"username":"ada","id":11,"uid":3001,"locked":false,"smb":true,"groups":[42],"home":"/var/empty"}]`,
		map[string]string{"filesystem.stat": `{"uid":3004,"gid":3001,"mode":16872}`}, nil)
	out := applyHome(t, s, `{"group":["lab_makers"],"home":["true"]}`)
	if len(m.updates) != 0 {
		t.Fatalf("a folder owned by someone else must not become this home: %v", m.updates)
	}
	if !strings.Contains(out.Detail, "belongs to someone else") {
		t.Fatalf("the outcome must say so, got %q", out.Detail)
	}
}

// Two roles that both give a home folder give one home: there is one place.
func TestTwoRolesGivingAHomeGiveOne(t *testing.T) {
	s, m := homeServer(t, `[{"username":"ada","id":11,"uid":3001,"locked":false,"smb":true,"groups":[42],"home":"/var/empty"}]`, nil, noSuchPath)
	applyHome(t, s, `{"group":["lab_makers"],"home":["true","true"]}`)
	if len(m.updates) != 1 || m.updates[0]["home"] != homePlace+"/ada" {
		t.Fatalf("want one home at %s/ada, got %v", homePlace, m.updates)
	}
}

func TestARoleNamingAnythingButTrueIsRefused(t *testing.T) {
	s, m := homeServer(t, `[{"username":"ada","id":11,"uid":3001,"locked":false,"smb":true,"groups":[42],"home":"/var/empty"}]`, nil, noSuchPath)
	rr := postApply(t, s, `{"call_id":"c1","subject":"sub-1","email":"ada@example.edu","desired":{"home":["/mnt/pool/main/home_dir"]}}`)
	if rr.Code == http.StatusOK || len(m.updates) != 0 {
		t.Fatalf("a place sent per role must be refused now that there is one place: %d %v", rr.Code, m.updates)
	}
}

// A person with two roles that give a home folder, with different limits,
// gets the higher one (owner, 15 Oct).
func TestTheLargestLimitIsSetOnTheHomesDataset(t *testing.T) {
	s, m := homeServer(t, `[{"username":"ada","id":11,"uid":3001,"locked":false,"smb":true,"groups":[42],"home":"`+homePlace+`/ada"}]`,
		map[string]string{"pool.dataset.get_quota": `[]`}, nil)
	out := applyHome(t, s, `{"group":["lab_makers"],"home":["true"],"home_quota":["10737418240","53687091200"]}`)
	if out.Effect != EffectApplied || !strings.Contains(out.Detail, "50 GiB") {
		t.Fatalf("want the 50 GiB limit set, got %s %q", out.Effect, out.Detail)
	}
	args, _ := lastCallParams(m, "pool.dataset.set_quota").([]any)
	if len(args) != 2 || args[0] != "pool/main/home_dir" {
		t.Fatalf("the limit goes on the home's dataset, got %v", args)
	}
}

func TestAnUnchangedLimitIsNotRewritten(t *testing.T) {
	s, m := homeServer(t, `[{"username":"ada","id":11,"uid":3001,"locked":false,"smb":true,"groups":[42],"home":"`+homePlace+`/ada"}]`,
		map[string]string{"pool.dataset.get_quota": `[{"id":3001,"quota":53687091200,"used_bytes":10}]`}, nil)
	out := applyHome(t, s, `{"group":["lab_makers"],"home":["true"],"home_quota":["53687091200"]}`)
	if out.Effect != EffectNoChange {
		t.Fatalf("want no change, got %s %q", out.Effect, out.Detail)
	}
	for _, c := range m.fakeRPC.calls {
		if c == "pool.dataset.set_quota" {
			t.Fatal("an unchanged limit must not be written")
		}
	}
}

// "No limit" is the largest limit there is: a role or a person's own giving it
// lifts the folder's limit, even beside a role that gives a size.
func TestNoLimitIsTheLargestAndLiftsTheLimit(t *testing.T) {
	s, m := homeServer(t, `[{"username":"ada","id":11,"uid":3001,"locked":false,"smb":true,"groups":[42],"home":"`+homePlace+`/ada"}]`,
		map[string]string{"pool.dataset.get_quota": `[{"id":3001,"quota":21474836480,"used_bytes":10}]`}, nil)
	out := applyHome(t, s, `{"group":["lab_makers"],"home":["true"],"home_quota":["21474836480","0"]}`)
	if !strings.Contains(out.Detail, "limit lifted") {
		t.Fatalf("want the limit lifted, got %s %q", out.Effect, out.Detail)
	}
	args, _ := lastCallParams(m, "pool.dataset.set_quota").([]any)
	entry := args[1].([]any)[0].(map[string]any)
	if entry["quota_value"] != int64(0) {
		t.Fatalf("quota_value = %v", entry["quota_value"])
	}
}
