package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// What a group opens is what the NAS's folder ACLs give it, read in NFS4's own
// order. The fixture is the live layout, renamed: one share whose root lets
// the makerspace's groups traverse and every account read, and first-level
// folders that are each a dataset granting one or two groups.

// perms spells an NFS4 permission set out the way an unsimplified getacl
// answers it.
func perms(set ...string) map[string]bool {
	all := []string{"READ_DATA", "WRITE_DATA", "APPEND_DATA", "READ_NAMED_ATTRS", "WRITE_NAMED_ATTRS",
		"EXECUTE", "DELETE", "DELETE_CHILD", "READ_ATTRIBUTES", "WRITE_ATTRIBUTES", "READ_ACL",
		"WRITE_ACL", "WRITE_OWNER", "SYNCHRONIZE"}
	on := map[string]bool{}
	for _, p := range set {
		on[p] = true
	}
	out := map[string]bool{}
	for _, p := range all {
		out[p] = on[p]
	}
	return out
}

var (
	permRead     = perms("READ_DATA", "EXECUTE", "READ_ATTRIBUTES", "READ_NAMED_ATTRS", "READ_ACL", "SYNCHRONIZE")
	permModify   = perms("READ_DATA", "WRITE_DATA", "APPEND_DATA", "EXECUTE", "DELETE", "DELETE_CHILD", "READ_ATTRIBUTES", "WRITE_ATTRIBUTES", "READ_NAMED_ATTRS", "WRITE_NAMED_ATTRS", "READ_ACL", "SYNCHRONIZE")
	permTraverse = perms("EXECUTE", "READ_ATTRIBUTES", "READ_NAMED_ATTRS", "READ_ACL", "SYNCHRONIZE")
)

func ace(tag string, id int64, kind string, p map[string]bool, flags ...string) map[string]any {
	f := map[string]bool{"FILE_INHERIT": true, "DIRECTORY_INHERIT": true}
	for _, fl := range flags {
		f[fl] = true
	}
	return map[string]any{"tag": tag, "id": id, "type": kind, "perms": p, "flags": f, "who": nil}
}

func aclJSON(gid int64, entries ...map[string]any) string {
	b, _ := json.Marshal(map[string]any{"path": "", "acltype": "NFS4", "uid": 0, "gid": gid, "trivial": false, "acl": entries})
	return string(b)
}

const reachGroups = `[
	{"id":41,"gid":545,"group":"builtin_users","builtin":true},
	{"id":43,"gid":544,"group":"builtin_administrators","builtin":true},
	{"id":50,"gid":5000,"group":"community","builtin":false},
	{"id":51,"gid":4000,"group":"staff","builtin":false},
	{"id":52,"gid":5001,"group":"interns","builtin":false}
]`

func reachFake() *fakeRPC {
	root := aclJSON(0,
		ace("owner@", -1, "ALLOW", perms("READ_DATA", "WRITE_DATA", "EXECUTE")),
		ace("GROUP", 545, "ALLOW", perms("READ_DATA", "EXECUTE")),
		ace("GROUP", 4000, "ALLOW", permTraverse),
		ace("GROUP", 5000, "ALLOW", permTraverse),
	)
	inherited := func(extra ...map[string]any) string {
		entries := append(extra,
			ace("owner@", -1, "ALLOW", perms("READ_DATA", "WRITE_DATA", "EXECUTE"), "INHERITED"),
			ace("GROUP", 545, "ALLOW", perms("READ_DATA", "EXECUTE"), "INHERITED"),
			ace("GROUP", 4000, "ALLOW", permTraverse, "INHERITED"),
		)
		return aclJSON(0, entries...)
	}
	return &fakeRPC{
		users:  `[]`,
		groups: reachGroups,
		shares: `[{"name":"main","path":"/mnt/tank/main","enabled":true},
		          {"name":"old","path":"/mnt/tank/old","enabled":false},
		          {"name":"open","path":"/mnt/tank/open","enabled":true}]`,
		answers: map[string]string{
			"pool.dataset.query": `[{"name":"tank/main","mountpoint":"/mnt/tank/main"},
				{"name":"tank/main/waddle","mountpoint":"/mnt/tank/main/waddle"},
				{"name":"tank/main/quack","mountpoint":"/mnt/tank/main/quack"},
				{"name":"tank/main/home_dir","mountpoint":"/mnt/tank/main/home_dir"},
				{"name":"tank/open","mountpoint":"/mnt/tank/open"}]`,
			"filesystem.listdir /mnt/tank/main": `[{"name":"waddle","path":"/mnt/tank/main/waddle"},
				{"name":"quack","path":"/mnt/tank/main/quack"},
				{"name":"home_dir","path":"/mnt/tank/main/home_dir"},
				{"name":"notes","path":"/mnt/tank/main/notes"},
				{"name":".ssh","path":"/mnt/tank/main/.ssh"}]`,
			"filesystem.listdir /mnt/tank/open": `[]`,
			"filesystem.getacl /mnt/tank/main":  root,
			"filesystem.getacl /mnt/tank/main/waddle": inherited(
				ace("GROUP", 5000, "ALLOW", permModify),
				ace("GROUP", 4000, "ALLOW", permModify)),
			"filesystem.getacl /mnt/tank/main/quack": inherited(
				ace("GROUP", 4000, "ALLOW", permModify)),
			"filesystem.getacl /mnt/tank/main/home_dir": inherited(),
			"filesystem.getacl /mnt/tank/main/notes":    inherited(),
			"filesystem.getacl /mnt/tank/main/.ssh":     inherited(),
			"filesystem.getacl /mnt/tank/open": aclJSON(0,
				ace("everyone@", -1, "ALLOW", permRead, "NO_PROPAGATE_INHERIT")),
		},
	}
}

func getReach(t *testing.T, s *server) (*httptest.ResponseRecorder, ReachResponse) {
	t.Helper()
	rr := httptest.NewRecorder()
	s.handleReach(rr, httptest.NewRequest(http.MethodGet, "/reach", nil), nil)
	var out ReachResponse
	_ = json.Unmarshal(rr.Body.Bytes(), &out)
	return rr, out
}

func describe(ps []Place) string {
	var parts []string
	for _, p := range ps {
		where := p.Share
		if p.Folder != "" {
			where += "/" + p.Folder
		}
		parts = append(parts, where+":"+p.Access)
	}
	return strings.Join(parts, ",")
}

func TestAGroupOpensWhatTheFolderACLsGiveIt(t *testing.T) {
	s := testServer(t, reachFake())
	rr, out := getReach(t, s)
	if rr.Code != http.StatusOK {
		t.Fatalf("want 200, got %d (%s)", rr.Code, rr.Body.String())
	}
	cases := map[string]string{
		"community": "main/waddle:change",
		"staff":     "main/quack:change,main/waddle:change",
		// No folder grants it anything beyond what every account has.
		"interns": "",
	}
	for group, want := range cases {
		got, ok := out.Groups[group]
		if !ok {
			t.Fatalf("%s is missing: a group that opens nothing is an empty list, not an absent one", group)
		}
		if describe(got) != want {
			t.Errorf("%s opens %q, want %q", group, describe(got), want)
		}
	}
	if _, ok := out.Groups["builtin_users"]; ok {
		t.Error("a built-in group was reported as one of the makerspace's own")
	}
	// What every account opens: main's root through builtin_users, and the
	// open share through everyone@. The disabled share is nowhere.
	if got := describe(out.Everyone); got != "main:read,open:read" {
		t.Errorf("everyone opens %q", got)
	}
	if !out.Current || out.ReadAt.IsZero() {
		t.Errorf("a fresh read must say so: current=%v read_at=%v", out.Current, out.ReadAt)
	}
	for _, p := range out.Groups["staff"] {
		if p.Dataset == "" || !strings.HasSuffix(p.Dataset, p.Folder) {
			t.Errorf("%s is held by dataset %q", p.Path, p.Dataset)
		}
	}
}

// NFS4 reads entries in order, and the first that names a permission decides
// it. A deny before an allow wins, and an inherit-only entry applies to what is
// created inside the folder, not to the folder.
func TestTheFirstEntryThatNamesAPermissionDecidesIt(t *testing.T) {
	staff := principals{4000: true}
	denyFirst := aclPlace{acl: []aclEntry{
		{Tag: "GROUP", ID: 4000, Type: "DENY", Perms: perms("WRITE_DATA")},
		{Tag: "GROUP", ID: 4000, Type: "ALLOW", Perms: permModify},
	}}
	if got := denyFirst.access(staff); got != accessRead {
		t.Errorf("a deny ahead of the allow was ignored: %q", got)
	}
	allowFirst := aclPlace{acl: []aclEntry{
		{Tag: "GROUP", ID: 4000, Type: "ALLOW", Perms: permModify},
		{Tag: "GROUP", ID: 4000, Type: "DENY", Perms: perms("WRITE_DATA")},
	}}
	if got := allowFirst.access(staff); got != accessChange {
		t.Errorf("a deny behind the allow was applied: %q", got)
	}
	inheritOnly := aclPlace{acl: []aclEntry{
		{Tag: "GROUP", ID: 4000, Type: "ALLOW", Perms: permModify, Flags: map[string]bool{"INHERIT_ONLY": true}},
	}}
	if got := inheritOnly.access(staff); got != accessNone {
		t.Errorf("an inherit-only entry opened the folder itself: %q", got)
	}
	owning := aclPlace{ownerGID: 4000, acl: []aclEntry{{Tag: "group@", ID: -1, Type: "ALLOW", Perms: permRead}}}
	if got := owning.access(staff); got != accessRead {
		t.Errorf("group@ did not apply to the folder's own group: %q", got)
	}
	if got := owning.access(principals{5000: true}); got != accessNone {
		t.Errorf("group@ applied to a group that doesn't own the folder: %q", got)
	}
}

func TestAFolderBehindARootTheGroupCantTraverseIsOutOfReach(t *testing.T) {
	read := []aclPlace{
		{share: "main", acl: []aclEntry{{Tag: "GROUP", ID: 4000, Type: "DENY", Perms: perms("EXECUTE")}}},
		{share: "main", folder: "quack", acl: []aclEntry{{Tag: "GROUP", ID: 4000, Type: "ALLOW", Perms: permModify}}},
	}
	if got := placesFor(read, principals{4000: true}, nil); len(got) != 0 {
		t.Fatalf("offered a folder nobody in the group can get to: %s", describe(got))
	}
}

// A member's places come from the groups their account is actually in, with
// their home first, and usage only where they can save.
func TestAMembersPlacesAreTheirOwnGroupsWithHomeFirst(t *testing.T) {
	fake := reachFake()
	fake.users = `[{"username":"ada","id":11,"uid":200001,"locked":false,"smb":true,"password_disabled":false,
		"groups":[41,50],"home":"/mnt/tank/main/home_dir/ada"}]`
	fake.answers["pool.dataset.get_quota"] = `[]`
	fake.answers["pool.dataset.get_quota tank/main/waddle"] = `[{"id":200001,"used_bytes":5000,"quota_type":"USER"}]`
	fake.answers["pool.dataset.get_quota tank/main/home_dir"] = `[{"id":200001,"used_bytes":0,"quota":1000000,"quota_type":"USER"}]`
	s := testServer(t, fake)
	if err := s.store.PutBinding(Binding{SubjectID: "sub-1", Username: "ada", UID: 200001}); err != nil {
		t.Fatal(err)
	}

	res, _, err := s.storageStatus(OperationRequest{Subject: "sub-1"})
	if err != nil {
		t.Fatal(err)
	}
	got := res.Storage
	if !got.PlacesReadable {
		t.Fatal("places were read and reported unreadable")
	}
	if d := describe(got.Places); d != "main/home_dir/ada:change,main:read,main/waddle:change,open:read" {
		t.Fatalf("places = %q", d)
	}
	home := got.Places[0]
	if !home.Home || home.Usage == nil || home.Usage.UsedBytes != 0 || home.Usage.QuotaBytes != 1000000 {
		t.Errorf("home = %+v usage=%+v; an empty home is nothing saved yet, with its limit", home, home.Usage)
	}
	for _, p := range got.Places {
		switch {
		case p.Access == accessRead && p.Usage != nil:
			t.Errorf("%s is read only and carries usage", p.Path)
		case p.Folder == "waddle" && (p.Usage == nil || p.Usage.UsedBytes != 5000):
			t.Errorf("waddle usage = %+v", p.Usage)
		}
	}
}

func TestAGroupThatOpensNothingLeavesTheMemberAnEmptyReadList(t *testing.T) {
	fake := reachFake()
	fake.users = `[{"username":"ida","id":12,"uid":200002,"locked":false,"smb":true,"password_disabled":false,"groups":[52],"home":""}]`
	s := testServer(t, fake)
	if err := s.store.PutBinding(Binding{SubjectID: "sub-2", Username: "ida", UID: 200002}); err != nil {
		t.Fatal(err)
	}
	res, _, err := s.storageStatus(OperationRequest{Subject: "sub-2"})
	if err != nil {
		t.Fatal(err)
	}
	// Not in builtin_users, so only everyone@ reaches them.
	if d := describe(res.Storage.Places); !res.Storage.PlacesReadable || d != "open:read" {
		t.Fatalf("places = %q readable=%v", d, res.Storage.PlacesReadable)
	}
}

// One read serves the page for a while; when the NAS can't be read, the last
// read is served and said to be the last read. With none, that is said too.
func TestReachFallsBackToTheLastReadAndSaysSo(t *testing.T) {
	fake := reachFake()
	s := testServer(t, fake)
	if rr, _ := getReach(t, s); rr.Code != http.StatusOK {
		t.Fatalf("first read: %d", rr.Code)
	}
	reads := strings.Count(strings.Join(fake.calls, " "), "filesystem.getacl")
	getReach(t, s)
	if again := strings.Count(strings.Join(fake.calls, " "), "filesystem.getacl"); again != reads {
		t.Fatalf("a fresh read was not reused: %d ACL reads became %d", reads, again)
	}

	s.reach.readAt = time.Now().Add(-2 * reachMaxAge)
	fake.refuse = map[string]string{"filesystem.getacl": "[EACCES] no"}
	rr, out := getReach(t, s)
	if rr.Code != http.StatusOK || out.Current {
		t.Fatalf("want the last read, not current: %d current=%v", rr.Code, out.Current)
	}
	if describe(out.Groups["community"]) != "main/waddle:change" {
		t.Errorf("the last read was not what was served: %q", describe(out.Groups["community"]))
	}

	never := testServer(t, &fakeRPC{groups: reachGroups, shares: fake.shares,
		answers: fake.answers, refuse: map[string]string{"filesystem.getacl": "[EACCES] no"}})
	if rr, _ := getReach(t, never); rr.Code != http.StatusServiceUnavailable || !strings.Contains(rr.Body.String(), "REACH_NEVER_READ") {
		t.Fatalf("never read: %d %s", rr.Code, rr.Body.String())
	}
}

// Access taken away rotated the password; given back, SMB waits for a new one.
// The member is told the step that opens their folders again, and not that
// they can connect.
func TestAccessGivenBackNeedsANewPassword(t *testing.T) {
	s, _ := applyServer(t, `[{"username":"ada","id":11,"uid":3001,"locked":false,"smb":false,"password_disabled":false,"groups":[]}]`)
	want := true
	if err := s.store.PutBinding(Binding{SubjectID: "sub-1", Username: "ada", UID: 3001, WantsSMB: &want}); err != nil {
		t.Fatal(err)
	}
	res, _, err := s.storageStatus(OperationRequest{Subject: "sub-1"})
	if err != nil {
		t.Fatal(err)
	}
	if !res.Storage.NeedsNewPassword || res.Storage.Usable || res.Storage.NeedsPassword {
		t.Fatalf("got needs_new_password=%v usable=%v needs_password=%v",
			res.Storage.NeedsNewPassword, res.Storage.Usable, res.Storage.NeedsPassword)
	}

	off := false
	if err := s.store.PutBinding(Binding{SubjectID: "sub-1", Username: "ada", UID: 3001, WantsSMB: &off}); err != nil {
		t.Fatal(err)
	}
	if res, _, _ := s.storageStatus(OperationRequest{Subject: "sub-1"}); res.Storage.NeedsNewPassword {
		t.Fatal("SMB that isn't wanted was reported as waiting for a password")
	}
}

// A role's home folder goes in a folder at the top of a share that is a dataset
// of its own, offered the way people say it. The share root, a plain
// directory, a hidden folder and a disabled share are not offered.
func TestHomeValuesAreTheFoldersAtTheTopOfEachShare(t *testing.T) {
	s := testServer(t, reachFake())
	rr, out := valuesOf(t, s, FieldHome)
	if rr.Code != http.StatusOK {
		t.Fatalf("want 200, got %d (%s)", rr.Code, rr.Body.String())
	}
	if got := strings.Join(out.Values, ","); got != "/mnt/tank/main/home_dir,/mnt/tank/main/quack,/mnt/tank/main/waddle" {
		t.Fatalf("values = %s", got)
	}
	if out.Labels["/mnt/tank/main/home_dir"] != "main › home_dir" {
		t.Errorf("labels = %v", out.Labels)
	}
}
