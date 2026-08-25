package main

import (
	"encoding/json"
	"net/http"
	"testing"
)

// SMB auditing is per share AND per group, and for the whole life of this
// add-on only the first half was read.
//
// `sharing.smb.query` answers `audit` as `{enable, watch_list, ignore_list}`.
// The add-on read `enable`, and while auditing was off everywhere that was
// indistinguishable from correct. The moment an operator switched it on —
// scoped, as the TrueNAS console encourages, to the groups they care about —
// every share became "audited" and members outside those groups became people
// whose empty activity report read as "did nothing" rather than "nobody was
// watching". That is the single distinction the field exists to draw, and it
// inverted silently.
//
// The two questions are kept apart on purpose. Health asks the target-level
// one; the activity report asks about one person; a share can be audited and
// still answer no to the second.

func TestAShareWatchesOnlyThePeopleItsListsAdmit(t *testing.T) {
	member := map[string]bool{"lab_makers": true}

	for _, tc := range []struct {
		name  string
		share shareAudit
		want  bool
	}{
		{"switched off", shareAudit{Enabled: false}, false},
		{"switched off with a list naming them", shareAudit{
			Enabled: false, WatchList: []string{"lab_makers"}}, false},
		{"on, unscoped", shareAudit{Enabled: true}, true},
		{"on, watching a group they are in", shareAudit{
			Enabled: true, WatchList: []string{"staff", "lab_makers"}}, true},
		// The case that motivated all of this.
		{"on, watching only groups they are not in", shareAudit{
			Enabled: true, WatchList: []string{"staff"}}, false},
		{"on, ignoring a group they are in", shareAudit{
			Enabled: true, IgnoreList: []string{"lab_makers"}}, false},
		// A group on both lists is a misconfiguration, and the safe reading of
		// one is the one that does not claim coverage.
		{"on, both lists naming them", shareAudit{
			Enabled: true, WatchList: []string{"lab_makers"}, IgnoreList: []string{"lab_makers"}}, false},
		{"on, ignoring a group they are not in", shareAudit{
			Enabled: true, IgnoreList: []string{"staff"}}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.share.watches(member); got != tc.want {
				t.Errorf("watches = %v, want %v", got, tc.want)
			}
		})
	}
}

func activityReport(t *testing.T, s *server) *ActivityReport {
	t.Helper()
	rr := postOperation(t, s, "activity.get", `{"call_id":"c1","subject":"sub-1"}`)
	if rr.Code != http.StatusOK {
		t.Fatalf("want 200, got %d (%s)", rr.Code, rr.Body.String())
	}
	var out OperationResult
	if err := json.Unmarshal(rr.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if out.Activity == nil {
		t.Fatal("want an activity report")
	}
	return out.Activity
}

// The regression. `ada` is in `lab_makers` (fixture group 42) and nothing else;
// both shares are switched ON, and neither is watching her.
func TestAnEnabledShareScopedPastTheMemberIsReportedAsNotWatchingThem(t *testing.T) {
	s, m := opServer(t)
	m.fakeRPC.audit = `[]`
	m.fakeRPC.shares = `[
		{"name":"gitlab_data","audit":{"enable":true,"watch_list":["staff"],"ignore_list":[]}},
		{"name":"main","audit":{"enable":true,"watch_list":[],"ignore_list":["lab_makers"]}}
	]`

	report := activityReport(t, s)
	if len(report.UncoveredShares) != 2 {
		t.Fatalf("both shares are switched on and neither records this member; "+
			"reading `enable` alone reported them as watched: %v", report.UncoveredShares)
	}
}

func TestAShareWatchingTheMembersOwnGroupIsNotReported(t *testing.T) {
	s, m := opServer(t)
	m.fakeRPC.audit = `[]`
	m.fakeRPC.shares = `[{"name":"main","audit":{"enable":true,"watch_list":["lab_makers"],"ignore_list":[]}}]`

	if report := activityReport(t, s); len(report.UncoveredShares) != 0 {
		t.Fatalf("this share is watching the member's own group: %v", report.UncoveredShares)
	}
}

// The primary group is its own field on `user.query` and is not repeated in
// `groups`. An ignore list naming it excludes the member, and reading `groups`
// alone would have reported them as watched — the direction of wrongness that
// hides a gap rather than inventing one.
func TestTheMembersPrimaryGroupCountsTowardsCoverage(t *testing.T) {
	s, m := applyServer(t,
		`[{"username":"ada","id":11,"uid":3001,"locked":false,"smb":true,"groups":[42],"group":{"id":41}}]`)
	if err := s.store.PutBinding(Binding{SubjectID: "sub-1", Username: "ada", UID: 3001}); err != nil {
		t.Fatal(err)
	}
	m.fakeRPC.audit = `[]`
	m.fakeRPC.shares = `[{"name":"main","audit":{"enable":true,"watch_list":[],"ignore_list":["builtin_users"]}}]`

	report := activityReport(t, s)
	if len(report.UncoveredShares) != 1 || report.UncoveredShares[0] != "main" {
		t.Fatalf("the ignore list names the member's primary group: %v", report.UncoveredShares)
	}
}

// Coverage that could not be worked out is said, not assumed. An empty list
// here means "every share was watching you", which is the most reassuring
// thing this report can say and the last thing it should say by accident.
func TestCoverageIsUnknownRatherThanCompleteWhenTheGroupsCannotBeRead(t *testing.T) {
	s, m := opServer(t)
	m.fakeRPC.audit = `[]`
	m.fakeRPC.shares = `[{"name":"main","audit":{"enable":true,"watch_list":["staff"],"ignore_list":[]}}]`
	m.fakeRPC.refuse = map[string]string{"group.query": "permission denied"}

	report := activityReport(t, s)
	if len(report.UncoveredShares) != 1 {
		t.Fatalf("want one caveat, got %v", report.UncoveredShares)
	}
	if report.UncoveredShares[0] == "main" {
		t.Fatal("a share named here reads as a fact about that share; nothing was determined")
	}
}

// The two questions stay apart. Health asks the target-level one and must not
// inherit the member-level answer: a share scoped to a group somebody is not in
// is still an audited share, and telling an operator its auditing is disabled
// would send them to change a setting that is already on.
func TestHealthStillAsksWhetherAuditingIsOnAtAll(t *testing.T) {
	s, _ := applyServer(t, `[]`)
	m := &mutatingRPC{fakeRPC: fakeRPC{users: `[]`, groups: fixtureGroups,
		shares: `[
			{"name":"scoped","audit":{"enable":true,"watch_list":["staff"],"ignore_list":[]}},
			{"name":"off","audit":{"enable":false,"watch_list":[],"ignore_list":[]}}
		]`}}
	s.nas = newNAS(func() (rpc, error) { return m, nil }, []string{"25.04"})
	s.nas.version, s.nas.probed = "25.04.2", true

	var h Health
	decodeHealth(t, s, &h)
	if !h.SharesReadable {
		t.Fatal("the share list was readable")
	}
	if len(h.UnauditedShares) != 1 || h.UnauditedShares[0] != "off" {
		t.Fatalf("only the switched-off share belongs on the target-level answer: %v", h.UnauditedShares)
	}
}

// The primary group's own shape. Observed as a record carrying an id; the bare
// form is tolerance for a release this add-on has not met, and tolerance
// nothing exercises is indistinguishable from tolerance that does not work.
func TestThePrimaryGroupIsReadFromWhicheverFormArrives(t *testing.T) {
	for _, tc := range []struct {
		name string
		raw  string
		want string
	}{
		{"the record this release answers with", `{"id":41,"bsdgrp_gid":545}`, "41"},
		{"a bare id", `41`, "41"},
		{"a quoted id", `"41"`, "41"},
		{"absent", ``, ""},
		{"null", `null`, ""},
		{"a record with no id", `{"bsdgrp_gid":545}`, ""},
		// Costs the primary group, never the read: the caller reports what it
		// could not determine, and failing here would lose the other groups too.
		{"something else entirely", `["surprise"]`, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			id, ok := primaryGroupID([]byte(tc.raw))
			if !ok {
				if tc.want != "" {
					t.Fatalf("want %q, got nothing", tc.want)
				}
				return
			}
			if tc.want == "" {
				t.Fatalf("want nothing, got %q", id.String())
			}
			if id.String() != tc.want {
				t.Errorf("id = %q, want %q", id.String(), tc.want)
			}
		})
	}
}
