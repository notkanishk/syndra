package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
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
	// What every account opens: the open share, through everyone@. main's
	// root opens to builtin_users, which Syndra's accounts aren't in, so it is
	// not everyone's. The disabled share is nowhere.
	if got := describe(out.Everyone); got != "open:read" {
		t.Errorf("everyone opens %q", got)
	}
	if !out.Current || out.TakenAt == "" || out.Stale != nil {
		t.Errorf("a fresh read must say so: current=%v taken_at=%v stale=%v", out.Current, out.TakenAt, out.Stale)
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
	fake.users = `[{"username":"charlotte","id":11,"uid":200001,"locked":false,"smb":true,"password_disabled":false,
		"groups":[41,50],"home":"/mnt/tank/main/home_dir/charlotte"}]`
	fake.answers["pool.dataset.get_quota"] = `[]`
	fake.answers["pool.dataset.get_quota tank/main/waddle"] = `[{"id":200001,"used_bytes":5000,"quota_type":"USER"}]`
	fake.answers["pool.dataset.get_quota tank/main/home_dir"] = `[{"id":200001,"used_bytes":0,"quota":1000000,"quota_type":"USER"}]`
	s := testServer(t, fake)
	if err := s.store.PutBinding(Binding{SubjectID: "sub-1", Username: "charlotte", UID: 200001}); err != nil {
		t.Fatal(err)
	}

	res, _, err := s.storageStatus(OperationRequest{Subject: "sub-1"})
	if err != nil {
		t.Fatal(err)
	}
	got := res.Result.(*StorageStatus)
	if !got.PlacesReadable {
		t.Fatal("places were read and reported unreadable")
	}
	if d := describe(got.Places); d != "main/home_dir/charlotte:change,main:read,main/waddle:change,open:read" {
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
	got := res.Result.(*StorageStatus)
	if d := describe(got.Places); !got.PlacesReadable || d != "open:read" {
		t.Fatalf("places = %q readable=%v", d, got.PlacesReadable)
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
	crr, cached := getReach(t, s)
	assertSchema(t, "profiles/storage/reach.json", crr.Body.Bytes(), "a cached /reach")
	if again := strings.Count(strings.Join(fake.calls, " "), "filesystem.getacl"); again != reads {
		t.Fatalf("a fresh read was not reused: %d ACL reads became %d", reads, again)
	}
	// The cache served on purpose is not current (design §4: current is a
	// read that began after the request arrived), and says it is cached.
	if cached.Current || cached.Stale == nil || cached.Stale.Reason != "cached" {
		t.Fatalf("a cached answer must be current false, stale cached: %+v", cached.Stale)
	}

	ageCopies(s, 2*freshMaxAge)
	fake.refuse = map[string]string{"filesystem.getacl": "[EACCES] no"}
	rr, out := getReach(t, s)
	if rr.Code != http.StatusOK || out.Current || out.Stale == nil || out.Stale.Reason != "unreachable" {
		t.Fatalf("want the last read, not current, stale unreachable: %d current=%v %+v", rr.Code, out.Current, out.Stale)
	}
	assertSchema(t, "profiles/storage/reach.json", rr.Body.Bytes(), "a /reach from the last read")
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
	s, _ := applyServer(t, `[{"username":"charlotte","id":11,"uid":3001,"locked":false,"smb":false,"password_disabled":false,"groups":[]}]`)
	want := true
	if err := s.store.PutBinding(Binding{SubjectID: "sub-1", Username: "charlotte", UID: 3001, WantsSMB: &want}); err != nil {
		t.Fatal(err)
	}
	res, _, err := s.storageStatus(OperationRequest{Subject: "sub-1"})
	if err != nil {
		t.Fatal(err)
	}
	if !res.Result.(*StorageStatus).NeedsNewPassword || res.Result.(*StorageStatus).Usable || res.Result.(*StorageStatus).NeedsPassword {
		t.Fatalf("got needs_new_password=%v usable=%v needs_password=%v",
			res.Result.(*StorageStatus).NeedsNewPassword, res.Result.(*StorageStatus).Usable, res.Result.(*StorageStatus).NeedsPassword)
	}
	// Its standing is the same fact (7l): not usable, waiting on the member
	// for a new password, never "usable, waiting on none" beside usable:false.
	if st := res.Result.(*StorageStatus).Standing; st == nil || st.Usable || st.WaitingOn != "member" ||
		!strings.Contains(st.Note, "set their password again") || !strings.Contains(st.Next, "new storage password") {
		t.Fatalf("the standing contradicts the status: %+v", st)
	}

	off := false
	if err := s.store.PutBinding(Binding{SubjectID: "sub-1", Username: "charlotte", UID: 3001, WantsSMB: &off}); err != nil {
		t.Fatal(err)
	}
	res, _, _ = s.storageStatus(OperationRequest{Subject: "sub-1"})
	if st := res.Result.(*StorageStatus); st.NeedsNewPassword || !st.Usable || st.Standing == nil || !st.Standing.Usable || st.Standing.WaitingOn != "none" {
		t.Fatalf("SMB that isn't wanted was reported as waiting for a password: %+v", st)
	}
}

// TRUENAS_SHARES narrows what Syndra considers: a share outside it is not
// read, offered or counted, even where everyone@ opens it.
func TestAShareOutsideTheScopeIsNotConsidered(t *testing.T) {
	fake := reachFake()
	s := testServer(t, fake)
	s.shares = shareScope(splitList(" main , "))

	_, out := getReach(t, s)
	if got := describe(out.Everyone); got != "" {
		t.Fatalf("everyone opens %q; the open share is outside the scope", got)
	}
	for _, p := range fake.params {
		if args, ok := p.([]any); ok && len(args) > 0 && args[0] == "/mnt/tank/open" {
			t.Fatalf("a share outside the scope was read: %v", args)
		}
	}
	if describe(out.Groups["staff"]) != "main/quack:change,main/waddle:change" {
		t.Errorf("staff = %q", describe(out.Groups["staff"]))
	}
}
