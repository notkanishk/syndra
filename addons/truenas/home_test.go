package main

import (
	"net/http"
	"strings"
	"testing"
)

// Home folders by role: made private by TrueNAS itself, never moved, never
// removed, never someone else's.

const homePlace = "/mnt/pool/main/home_dir"

func homeServer(t *testing.T, users string, answers map[string]string, refuse map[string]string) (*server, *mutatingRPC) {
	t.Helper()
	s, m := applyServer(t, users)
	m.fakeRPC.answers = answers
	m.fakeRPC.refuse = refuse
	if err := s.store.PutBinding(Binding{SubjectID: "sub-1", Username: "ada", UID: 3001}); err != nil {
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
	out := applyHome(t, s, `{"group":["lab_makers"],"home":["`+homePlace+`"]}`)
	if out.Effect != EffectApplied {
		t.Fatalf("want applied, got %s (%s)", out.Effect, out.Detail)
	}
	u := m.updates[0]
	if u["home"] != homePlace+"/ada" || u["home_create"] != true || u["home_mode"] != "700" {
		t.Fatalf("want TrueNAS's own private create at %s/ada, got %v", homePlace, u)
	}
}

func TestAHomeIsNeverMoved(t *testing.T) {
	s, m := homeServer(t, `[{"username":"ada","id":11,"uid":3001,"locked":false,"smb":true,"groups":[42],"home":"/mnt/pool/old/ada"}]`, nil, nil)
	out := applyHome(t, s, `{"group":["lab_makers"],"home":["`+homePlace+`"]}`)
	if len(m.updates) != 0 {
		t.Fatalf("a home elsewhere must not be moved: %v", m.updates)
	}
	if !strings.Contains(out.Detail, "never moves a home folder") {
		t.Fatalf("the outcome must say why, got %q", out.Detail)
	}
}

func TestAnotherPersonsFolderIsNeverAdopted(t *testing.T) {
	s, m := homeServer(t, `[{"username":"ada","id":11,"uid":3001,"locked":false,"smb":true,"groups":[42],"home":"/var/empty"}]`,
		map[string]string{"filesystem.stat": `{"uid":3004,"gid":3001,"mode":16872}`}, nil)
	out := applyHome(t, s, `{"group":["lab_makers"],"home":["`+homePlace+`"]}`)
	if len(m.updates) != 0 {
		t.Fatalf("a folder owned by someone else must not become this home: %v", m.updates)
	}
	if !strings.Contains(out.Detail, "belongs to someone else") {
		t.Fatalf("the outcome must say so, got %q", out.Detail)
	}
}

func TestTwoRolesDisagreeingLeaveTheHomeAlone(t *testing.T) {
	s, m := homeServer(t, `[{"username":"ada","id":11,"uid":3001,"locked":false,"smb":true,"groups":[],"home":"/var/empty"}]`, nil, noSuchPath)
	out := applyHome(t, s, `{"group":["lab_makers"],"home":["`+homePlace+`","/mnt/pool/main/staff_homes"]}`)
	if len(m.updates) != 1 {
		t.Fatalf("the groups still land, got %v", m.updates)
	}
	if _, set := m.updates[0]["home"]; set {
		t.Fatalf("no home may be chosen between two: %v", m.updates[0])
	}
	if !strings.Contains(out.Detail, "different home folders") {
		t.Fatalf("the outcome must name the disagreement, got %q", out.Detail)
	}
}

func TestTheLargestLimitIsSetOnTheHomesDataset(t *testing.T) {
	s, m := homeServer(t, `[{"username":"ada","id":11,"uid":3001,"locked":false,"smb":true,"groups":[42],"home":"`+homePlace+`/ada"}]`,
		map[string]string{"pool.dataset.get_quota": `[]`}, nil)
	out := applyHome(t, s, `{"group":["lab_makers"],"home":["`+homePlace+`"],"home_quota":["10737418240","53687091200"]}`)
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
	out := applyHome(t, s, `{"group":["lab_makers"],"home":["`+homePlace+`"],"home_quota":["53687091200"]}`)
	if out.Effect != EffectNoChange {
		t.Fatalf("want no change, got %s %q", out.Effect, out.Detail)
	}
	for _, c := range m.fakeRPC.calls {
		if c == "pool.dataset.set_quota" {
			t.Fatal("an unchanged limit must not be written")
		}
	}
}
