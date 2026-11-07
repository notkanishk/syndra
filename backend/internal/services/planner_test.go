package services

import (
	"maps"
	"strings"
	"testing"
	"time"

	"syndra/internal/db"
	"syndra/internal/models"
)

// The planner on plain data (architecture deepening, decision 10): no
// database, no directory, no stubs. Each case is a snapshot and the Change it
// must give.

var (
	member   = roleKey{projectID: "pLab", roleKey: "member"}
	trained  = roleKey{projectID: "pLab", roleKey: "trained"}
	advanced = roleKey{projectID: "pLab", roleKey: "advanced"}
)

func plannerSnap() PlanSnapshot {
	return PlanSnapshot{
		Bundle:       models.Bundle{ID: "b1", Name: "Safety"},
		Version:      models.BundleVersion{ID: "v2", Version: 2},
		VersionRoles: []models.BundleRole{{BundleID: "b1", ProjectID: "pLab", RoleKey: "member"}, {BundleID: "b1", ProjectID: "pLab", RoleKey: "trained"}},
		Catalogue: map[roleKey]roleWords{
			member: {Role: "Member", Project: "Laser Lab"}, trained: {Role: "Trained", Project: "Laser Lab"},
			advanced: {Role: "Advanced", Project: "Laser Lab"},
		},
		Profiles: map[string]models.UserProfile{"u1": {ID: "u1", Name: "Ada Lovelace", Status: "active"}},
		Holding:  models.RequestHolding{Kind: models.HoldingNone},
		Access: db.AccessSnapshot{
			People:    map[string]db.PersonAccess{"u1": {}},
			RoleWaits: map[db.RoleRef]bool{},
		},
	}
}

// holdsBundle pins u1 to the bundle, giving them its roles.
func holdsBundle(s *PlanSnapshot) {
	p := s.Access.People["u1"]
	p.Bundles = map[string]bool{"b1": true}
	p.ByBundle = map[string][]models.BundleRole{"b1": s.VersionRoles}
	s.Access.People["u1"] = p
}

func withDirect(s *PlanSnapshot, k roleKey) {
	p := s.Access.People["u1"]
	p.Direct = append(p.Direct, models.DirectGrant{ID: "g-" + k.roleKey, UserID: "u1", ProjectID: k.projectID, RoleKey: k.roleKey})
	s.Access.People["u1"] = p
}

func withQueued(s *PlanSnapshot, c models.QueuedChange) {
	p := s.Access.People["u1"]
	c.UserID = "u1"
	p.Queued = append(p.Queued, c)
	s.Access.People["u1"] = p
}

func assign(s PlanSnapshot) Change {
	return Plan(Action{Op: BulkOpAssignBundle, UserID: "u1", BundleID: "b1"}, s)
}

func remove(s PlanSnapshot) Change {
	return Plan(Action{Op: BulkOpRemoveBundle, UserID: "u1", BundleID: "b1"}, s)
}

type sentWrite struct{ op, role, delivery string }

func plannedWrites(ch Change) []sentWrite {
	var out []sentWrite
	for _, w := range ch.Writes {
		out = append(out, sentWrite{w.OpType, w.RoleKeys[0], w.Delivery})
	}
	return out
}

func sameWrites(t *testing.T, ch Change, want ...sentWrite) {
	t.Helper()
	got := plannedWrites(ch)
	if len(got) != len(want) {
		t.Fatalf("writes = %+v, want %+v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("writes = %+v, want %+v", got, want)
		}
	}
	for _, w := range ch.Writes {
		if w.UserID != "u1" || (w.Source != "rule" && (w.Source != "bundle" || w.SourceRef != "b1")) {
			t.Fatalf("a bundle's write is attributed to the bundle, a rule's to the rule: %+v", w)
		}
	}
}

// writeSources is each write's role with its source and ref.
func writeSources(ch Change) map[string]string {
	out := map[string]string{}
	for _, w := range ch.Writes {
		out[w.OpType+" "+w.RoleKeys[0]] = w.Source + "/" + w.SourceRef
	}
	return out
}

func TestPlan_AssignGivesWhatTheyLackStraightAway(t *testing.T) {
	ch := assign(plannerSnap())
	if ch.Refusal != "" || ch.Row.Effect != EffectApply || ch.VersionID != "v2" {
		t.Fatalf("change = %+v", ch)
	}
	sameWrites(t, ch, sentWrite{"add", "member", DeliveryStraightAway}, sentWrite{"add", "trained", DeliveryStraightAway})
	if ch.Row.Detail != "Joins the Safety bundle." || ch.Row.Consequence != "Gains Member in Laser Lab and Trained in Laser Lab." {
		t.Errorf("row = %q / %q", ch.Row.Detail, ch.Row.Consequence)
	}
}

func TestPlan_AssignLeavesOutWhatTheyAlreadyHold(t *testing.T) {
	s := plannerSnap()
	withDirect(&s, member)
	ch := assign(s)
	sameWrites(t, ch, sentWrite{"add", "trained", DeliveryStraightAway})
	if ch.Row.Consequence != "Gains Trained in Laser Lab." {
		t.Errorf("consequence = %q", ch.Row.Consequence)
	}

	withDirect(&s, trained)
	ch = assign(s)
	if ch.Row.Effect != EffectApply || len(ch.Writes) != 0 ||
		ch.Row.Consequence != "Already holds every role it gives, so nothing is sent to Zitadel." {
		t.Errorf("joining with everything held: %+v", ch)
	}
}

// A queued revocation is already decided: the role is not counted as held, so
// the bundle gives it again.
func TestPlan_AQueuedRevocationIsNotHolding(t *testing.T) {
	s := plannerSnap()
	withDirect(&s, member)
	p := s.Access.People["u1"]
	p.Revoking = []db.RoleRef{{ProjectID: "pLab", RoleKey: "member"}}
	s.Access.People["u1"] = p
	sameWrites(t, assign(s), sentWrite{"add", "member", DeliveryStraightAway}, sentWrite{"add", "trained", DeliveryStraightAway})
}

// Already held: the bundle itself. No change, with the reason (decision 6).
func TestPlan_AlreadyInTheBundleIsNoChange(t *testing.T) {
	s := plannerSnap()
	holdsBundle(&s)
	ch := assign(s)
	if ch.Row.Effect != EffectNoChange || ch.Refusal != "held" || ch.Row.Detail != "Already in the Safety bundle." || len(ch.Writes) != 0 {
		t.Fatalf("change = %+v", ch)
	}
	if ch := remove(plannerSnap()); ch.Row.Effect != EffectNoChange || ch.Row.Detail != "Not in the Safety bundle." {
		t.Fatalf("leaving a bundle they don't hold: %+v", ch)
	}
}

// A role Zitadel no longer offers: the row is No change with why and what to
// do, and nothing is written for it; an unreadable catalogue refuses a give.
func TestPlan_RoleNoLongerOffered(t *testing.T) {
	s := plannerSnap()
	delete(s.Catalogue, trained)
	ch := assign(s)
	if ch.Row.Effect != EffectNoChange || !strings.HasPrefix(ch.Refusal, "not_offered:") || len(ch.Writes) != 0 {
		t.Fatalf("change = %+v", ch)
	}
	// The role is gone but its project is still listed: named by the project.
	if ch.Row.Detail != "This would give trained in Laser Lab, which no longer exists in Zitadel." ||
		ch.Row.Consequence != "Take it out of the Safety bundle and publish, then give the bundle again." {
		t.Errorf("row = %q / %q", ch.Row.Detail, ch.Row.Consequence)
	}

	// The whole project is gone: never its raw id, and one name said once.
	s = plannerSnap()
	delete(s.Catalogue, member)
	delete(s.Catalogue, trained)
	delete(s.Catalogue, advanced)
	ch = assign(s)
	if ch.Row.Detail != "This would give member in a project Zitadel no longer lists and trained in a project Zitadel no longer lists, which no longer exist in Zitadel." {
		t.Errorf("unlisted project: %q", ch.Row.Detail)
	}
	if ch.Row.Consequence != "Take them out of the Safety bundle and publish, then give the bundle again." {
		t.Errorf("plural step: %q", ch.Row.Consequence)
	}
	if strings.Contains(ch.Row.Detail, "pLab") {
		t.Errorf("a raw project id reached the words: %q", ch.Row.Detail)
	}

	s = plannerSnap()
	s.Catalogue, s.CatalogueUnread = nil, true
	if ch := assign(s); ch.Row.Effect != EffectBlocked || ch.Refusal != "catalogue_unread" || !strings.Contains(ch.Row.Detail, "Try again shortly") {
		t.Fatalf("unread catalogue: %+v", ch)
	}
	// Taking a role away needs no offer: a removal goes on.
	s = plannerSnap()
	holdsBundle(&s)
	s.Catalogue, s.CatalogueUnread = nil, true
	if ch := remove(s); ch.Row.Effect != EffectApply || len(ch.Writes) != 2 {
		t.Fatalf("removal with an unread catalogue: %+v", ch)
	}
}

func TestPlan_RefusesWhoCantChange(t *testing.T) {
	s := plannerSnap()
	s.Profiles["u1"] = models.UserProfile{ID: "u1", Name: "Ada", Status: "locked"}
	if ch := assign(s); ch.Row.Effect != EffectBlocked || ch.Row.Detail != "Account is locked. Remove it from the selection to continue." {
		t.Fatalf("departed: %+v", ch)
	}
	s = plannerSnap()
	delete(s.Profiles, "u1")
	if ch := assign(s); ch.Row.Effect != EffectBlocked || ch.Refusal != "absent" {
		t.Fatalf("unknown: %+v", ch)
	}
	s = plannerSnap()
	retired := time.Date(2026, 11, 3, 12, 0, 0, 0, time.UTC)
	s.Bundle.RetiredAt = &retired
	if ch := assign(s); ch.Row.Effect != EffectBlocked || ch.Row.Detail != "The Safety bundle was retired on 3 Nov 2026, so nobody can join it." {
		t.Fatalf("retired: %+v", ch)
	}
}

// Removal takes away only what the bundle was their only source for.
func TestPlan_RemoveOnlySourceVersusOtherSources(t *testing.T) {
	s := plannerSnap()
	holdsBundle(&s)
	ch := remove(s)
	sameWrites(t, ch, sentWrite{"revoke", "member", DeliveryStraightAway}, sentWrite{"revoke", "trained", DeliveryStraightAway})
	if ch.Row.Detail != "Leaves the Safety bundle." || ch.Row.Consequence != "Loses Member in Laser Lab and Trained in Laser Lab." {
		t.Errorf("row = %q / %q", ch.Row.Detail, ch.Row.Consequence)
	}

	withDirect(&s, member)
	ch = remove(s)
	sameWrites(t, ch, sentWrite{"revoke", "trained", DeliveryStraightAway})
	if ch.Row.Consequence != "Loses Trained in Laser Lab. Keeps Member in Laser Lab another way." {
		t.Errorf("consequence = %q", ch.Row.Consequence)
	}

	withDirect(&s, trained)
	ch = remove(s)
	if ch.Row.Effect != EffectApply || len(ch.Writes) != 0 ||
		ch.Row.Consequence != "Keeps every role it gave another way, so nothing is sent to Zitadel." {
		t.Errorf("every role kept: %+v", ch)
	}
}

// Decision 11: a role marked to wait waits for you; the rest go straight away.
func TestPlan_WaitsForYouVersusStraightAway(t *testing.T) {
	s := plannerSnap()
	s.Access.RoleWaits[db.RoleRef{ProjectID: "pLab", RoleKey: "trained"}] = true
	ch := assign(s)
	sameWrites(t, ch, sentWrite{"add", "member", DeliveryStraightAway}, sentWrite{"add", "trained", DeliveryWaits})
	if ch.Writes[1].WaitReason != db.WaitReasonRoleWaits || ch.Writes[0].WaitReason != "" {
		t.Errorf("wait reasons = %q, %q", ch.Writes[0].WaitReason, ch.Writes[1].WaitReason)
	}
	if !strings.HasSuffix(ch.Row.Consequence, "Giving Trained in Laser Lab waits for you to send.") {
		t.Errorf("consequence = %q", ch.Row.Consequence)
	}
	holdsBundle(&s)
	if ch := remove(s); !strings.HasSuffix(ch.Row.Consequence, "Removing Trained in Laser Lab waits for you to send.") {
		t.Errorf("removal consequence = %q", ch.Row.Consequence)
	}
}

// The queued changes a plan cancels are named on the row and carried as ids:
// an add cancels a queued removal from anywhere; a removal cancels only adds
// from this bundle. Whether the write is still sent depends on whether the
// role stands in Zitadel.
func TestPlan_CancelIDs(t *testing.T) {
	queued := time.Date(2026, 11, 2, 9, 0, 0, 0, time.UTC)
	s := plannerSnap()
	withQueued(&s, models.QueuedChange{ID: "q-rev", OpType: "revoke", ProjectID: "pLab", RoleKeys: []string{"member"}, Source: "direct", QueuedAt: queued})
	ch := assign(s)
	if len(ch.Row.CancelIDs) != 1 || ch.Row.CancelIDs[0] != "q-rev" {
		t.Fatalf("cancel ids = %v", ch.Row.CancelIDs)
	}
	// Not standing in Zitadel: the add is still owed, and goes.
	sameWrites(t, ch, sentWrite{"add", "member", DeliveryStraightAway}, sentWrite{"add", "trained", DeliveryStraightAway})
	if !strings.HasSuffix(ch.Row.Consequence, "Cancels a change waiting for you to send: the removal of member queued 2 Nov 2026.") {
		t.Errorf("consequence = %q", ch.Row.Consequence)
	}
	// Standing: cancelling the removal is all it takes, so nothing is sent.
	p := s.Access.People["u1"]
	p.Standing = map[db.RoleRef]bool{{ProjectID: "pLab", RoleKey: "member"}: true}
	s.Access.People["u1"] = p
	sameWrites(t, assign(s), sentWrite{"add", "member", DeliveryNone}, sentWrite{"add", "trained", DeliveryStraightAway})

	s = plannerSnap()
	holdsBundle(&s)
	withQueued(&s, models.QueuedChange{ID: "q-own", OpType: "add", ProjectID: "pLab", RoleKeys: []string{"member"}, Source: "bundle", SourceRef: "b1", QueuedAt: queued})
	withQueued(&s, models.QueuedChange{ID: "q-other", OpType: "add", ProjectID: "pLab", RoleKeys: []string{"trained"}, Source: "bundle", SourceRef: "b9", QueuedAt: queued})
	if ids := remove(s).Row.CancelIDs; len(ids) != 1 || ids[0] != "q-own" {
		t.Fatalf("a removal cancels only its own bundle's add: %v", ids)
	}
}

// Rules inside the planner (decision 7): the whole closure, named on the row.
func TestPlan_RulesFollowTheBundle(t *testing.T) {
	s := plannerSnap()
	s.Access.Rules = []models.MappingRule{{ID: "r1", SourceProject: "pLab", SourceRole: "trained", TargetProject: "pLab", TargetRole: "advanced"}}
	ch := assign(s)
	sameWrites(t, ch, sentWrite{"add", "advanced", DeliveryStraightAway}, sentWrite{"add", "member", DeliveryStraightAway}, sentWrite{"add", "trained", DeliveryStraightAway})
	if !strings.Contains(ch.Row.Consequence, "Also gives Advanced in Laser Lab, by the automatic rule from Trained in Laser Lab.") {
		t.Errorf("consequence = %q", ch.Row.Consequence)
	}
	// The bundle's roles are its rows; the role the rule follows with is the
	// rule's row, with the rule (task 1.6): the shape FollowRules writes.
	want := map[string]string{"add member": "bundle/b1", "add trained": "bundle/b1", "add advanced": "rule/r1"}
	if got := writeSources(ch); !maps.Equal(got, want) {
		t.Errorf("sources = %v, want %v", got, want)
	}
	holdsBundle(&s)
	ch = remove(s)
	if !strings.Contains(ch.Row.Consequence, "Also loses Advanced in Laser Lab, which the automatic rule from Trained in Laser Lab gave.") {
		t.Errorf("removal consequence = %q", ch.Row.Consequence)
	}
	want = map[string]string{"revoke member": "bundle/b1", "revoke trained": "bundle/b1", "revoke advanced": "rule/r1"}
	if got := writeSources(ch); !maps.Equal(got, want) {
		t.Errorf("removal sources = %v, want %v", got, want)
	}
}

// Two hops: each rule row names the rule that reaches it, whose first role is
// the hop before it, as the words do.
func TestPlan_RuleChainNamesEachHop(t *testing.T) {
	s := plannerSnap()
	s.Access.Rules = []models.MappingRule{
		{ID: "r1", SourceProject: "pLab", SourceRole: "trained", TargetProject: "pLab", TargetRole: "advanced"},
		{ID: "r2", SourceProject: "pLab", SourceRole: "advanced", TargetProject: "pLab", TargetRole: "supervisor"},
	}
	s.Catalogue[roleKey{projectID: "pLab", roleKey: "supervisor"}] = roleWords{Role: "Supervisor", Project: "Laser Lab"}
	ch := assign(s)
	if got := writeSources(ch); got["add advanced"] != "rule/r1" || got["add supervisor"] != "rule/r2" {
		t.Errorf("sources = %v", got)
	}
	if !strings.Contains(ch.Row.Consequence, "Also gives Supervisor in Laser Lab, by the automatic rule from Advanced in Laser Lab.") {
		t.Errorf("consequence = %q", ch.Row.Consequence)
	}
}

// The fingerprint is the decision: the same snapshot gives the same one, a
// renamed role or an unrelated holding leaves it, and anything that changes
// what is written or cancelled moves it (decision 3).
func TestPlan_FingerprintStabilityAndChange(t *testing.T) {
	base := assign(plannerSnap()).Row.Fingerprint
	if again := assign(plannerSnap()).Row.Fingerprint; again != base {
		t.Fatal("the same snapshot must give the same fingerprint")
	}

	s := plannerSnap()
	s.Catalogue[member] = roleWords{Role: "Member (renamed)", Project: "Laser Lab"}
	withDirect(&s, roleKey{projectID: "pElsewhere", roleKey: "x"})
	if got := assign(s).Row.Fingerprint; got != base {
		t.Error("words and unrelated holdings are not what the plan decided")
	}

	for name, mutate := range map[string]func(*PlanSnapshot){
		"a queued change it would cancel": func(s *PlanSnapshot) {
			withQueued(s, models.QueuedChange{ID: "q-new", OpType: "revoke", ProjectID: "pLab", RoleKeys: []string{"trained"}, Source: "direct"})
		},
		"a role now waits":      func(s *PlanSnapshot) { s.Access.RoleWaits[db.RoleRef{ProjectID: "pLab", RoleKey: "member"}] = true },
		"a role now held":       func(s *PlanSnapshot) { withDirect(s, trained) },
		"a newer version":       func(s *PlanSnapshot) { s.Version.ID = "v3" },
		"they joined meanwhile": holdsBundle,
		"a rule now follows": func(s *PlanSnapshot) {
			s.Access.Rules = []models.MappingRule{{ID: "r1", SourceProject: "pLab", SourceRole: "trained", TargetProject: "pLab", TargetRole: "advanced"}}
		},
	} {
		s := plannerSnap()
		mutate(&s)
		if assign(s).Row.Fingerprint == base {
			t.Errorf("%s must move the fingerprint", name)
		}
	}
}
