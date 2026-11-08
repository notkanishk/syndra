package services

import (
	"strings"
	"testing"

	"syndra/internal/db"
	"syndra/internal/models"
)

// System work on plain data (task 1.9): the planner decides each write, its
// delivery and what it cancels, with no database.

func systemWrites(ch Change) string {
	var out []string
	for _, w := range ch.Writes {
		out = append(out, w.OpType+" "+w.ProjectID+"/"+w.RoleKeys[0]+" "+w.Source+":"+w.SourceRef+" "+w.Delivery)
	}
	return strings.Join(out, " | ")
}

// The welcome bundle: its roles are the bundle's rows, a role a rule follows
// with is the rule's, and a role that waits waits. It cancels a removal still
// waiting for the role it gives (any give answers a queued removal), and that
// is the one queued change it names.
func TestSystemChange_WelcomeBundle(t *testing.T) {
	rules := []models.MappingRule{{ID: "r1", SourceProject: "pLab", SourceRole: "member", TargetProject: "pWiki", TargetRole: "editor"}}
	person := db.PersonAccess{Queued: []models.QueuedChange{
		{ID: "q-rm", UserID: "u1", OpType: "revoke", ProjectID: "pLab", RoleKeys: []string{"member"}, Source: "direct"},
		{ID: "q-other", UserID: "u1", OpType: "revoke", ProjectID: "pShop", RoleKeys: []string{"x"}, Source: "direct"},
	}}
	s := PlanSnapshot{Access: db.AccessSnapshot{Rules: rules, RoleWaits: map[db.RoleRef]bool{{ProjectID: "pLab", RoleKey: "trained"}: true}}}
	after := map[roleKey]bool{member: true, trained: true}

	ch := systemChange("u1", person, s, map[roleKey]bool{}, after, rules, rules, "bundle", "b1")
	want := "add pLab/member bundle:b1 straight_away | add pLab/trained bundle:b1 waits_for_you | add pWiki/editor rule:r1 straight_away"
	if got := systemWrites(ch); got != want {
		t.Fatalf("writes\n got %s\nwant %s", got, want)
	}
	if strings.Join(ch.Row.CancelIDs, ",") != "q-rm" {
		t.Fatalf("cancels only the waiting removal of what it gives: %v", ch.Row.CancelIDs)
	}
	// Where the role still stands in Zitadel, cancelling that removal is all
	// that is owed: nothing is sent for it.
	person.Standing = map[db.RoleRef]bool{{ProjectID: "pLab", RoleKey: "member"}: true}
	if ch := systemChange("u1", person, s, map[roleKey]bool{}, after, rules, rules, "bundle", "b1"); ch.Writes[0].Delivery != DeliveryNone {
		t.Fatalf("a give that only cancels sends nothing: %s", systemWrites(ch))
	}
}

// A rule's own edit: a one-step rule's row is the edited rule's (a new one's
// id is stamped by its insert), only the people whose closure moves get any,
// and a role still reached another way stays.
func TestSystemChange_RuleEdit(t *testing.T) {
	old := models.MappingRule{ID: "r1", SourceProject: "pLab", SourceRole: "member", TargetProject: "pWiki", TargetRole: "editor"}
	base := map[roleKey]bool{member: true}
	s := PlanSnapshot{}

	fresh := old
	fresh.ID = ""
	created := systemChange("u1", db.PersonAccess{}, s, base, base, nil, []models.MappingRule{fresh}, "rule", "")
	if got := systemWrites(created); got != "add pWiki/editor rule: straight_away" {
		t.Fatalf("create: %s", got)
	}
	retired := systemChange("u1", db.PersonAccess{}, s, base, base, []models.MappingRule{old}, nil, "rule", "r1")
	if got := systemWrites(retired); got != "revoke pWiki/editor rule:r1 straight_away" {
		t.Fatalf("retire: %s", got)
	}
	// Someone who doesn't hold the source gets nothing.
	if ch := systemChange("u2", db.PersonAccess{}, s, map[roleKey]bool{}, map[roleKey]bool{}, []models.MappingRule{old}, nil, "rule", "r1"); len(ch.Writes) != 0 {
		t.Fatalf("no source, no rows: %s", systemWrites(ch))
	}
	// A role they hold directly too is never revoked by the rule going.
	both := map[roleKey]bool{member: true, {projectID: "pWiki", roleKey: "editor"}: true}
	if ch := systemChange("u1", db.PersonAccess{}, s, both, both, []models.MappingRule{old}, nil, "rule", "r1"); len(ch.Writes) != 0 {
		t.Fatalf("still held another way: %s", systemWrites(ch))
	}
}

// chainWrites is each write with the rule its row names and the role kept on
// it as the one that set it off: "add pLab/trained r1<member".
func chainWrites(ch Change) string {
	var out []string
	for _, w := range ch.Writes {
		from := "-"
		if w.RuleFrom != nil {
			from = w.RuleFrom.RoleKey
		}
		out = append(out, w.OpType+" "+w.ProjectID+"/"+w.RoleKeys[0]+" "+w.Source+":"+w.SourceRef+"<"+from)
	}
	return strings.Join(out, " | ")
}

// A rule's own edit in a chain (task 1.6a): each row names the rule that
// reaches its role, the hop one step before it, so Pending changes says
// "Rule from" the role before it, never the edited rule's first role for a
// role two or three steps down.
func TestSystemChange_RuleEditNamesEachHop(t *testing.T) {
	supervisor, visitor := roleKey{projectID: "pLab", roleKey: "supervisor"}, roleKey{projectID: "pLab", roleKey: "visitor"}
	rule := func(id string, from, to roleKey) models.MappingRule {
		return models.MappingRule{ID: id, SourceProject: from.projectID, SourceRole: from.roleKey, TargetProject: to.projectID, TargetRole: to.roleKey}
	}
	r1, r2, r3 := rule("r1", member, trained), rule("r2", trained, advanced), rule("r3", advanced, supervisor)
	fresh := r1
	fresh.ID = "" // not yet created: the insert stamps its id
	base := map[roleKey]bool{member: true}
	plan := func(before, after []models.MappingRule) string {
		return chainWrites(systemChange("u1", db.PersonAccess{}, PlanSnapshot{}, base, base, before, after, "rule", ""))
	}

	for _, c := range []struct {
		name          string
		before, after []models.MappingRule
		want          string
	}{
		{"create, two hops", []models.MappingRule{r2}, []models.MappingRule{r2, fresh},
			"add pLab/advanced rule:r2<trained | add pLab/trained rule:<member"},
		{"create, three hops", []models.MappingRule{r2, r3}, []models.MappingRule{r2, r3, fresh},
			"add pLab/advanced rule:r2<trained | add pLab/supervisor rule:r3<advanced | add pLab/trained rule:<member"},
		{"retire, two hops", []models.MappingRule{r1, r2}, []models.MappingRule{r2},
			"revoke pLab/advanced rule:r2<trained | revoke pLab/trained rule:r1<member"},
		{"retire, three hops", []models.MappingRule{r1, r2, r3}, []models.MappingRule{r2, r3},
			"revoke pLab/advanced rule:r2<trained | revoke pLab/supervisor rule:r3<advanced | revoke pLab/trained rule:r1<member"},
		// An edit of r1's target: what it gave goes, named by r1 as it was
		// (from member); what it gives now comes, named by r1 as it is.
		{"edit, two hops", []models.MappingRule{r1, r2}, []models.MappingRule{rule("r1", member, supervisor), r2},
			"add pLab/supervisor rule:r1<member | revoke pLab/advanced rule:r2<trained | revoke pLab/trained rule:r1<member"},
		{"edit, three hops", []models.MappingRule{r1, r2, r3}, []models.MappingRule{rule("r1", member, visitor), r2, r3},
			"add pLab/visitor rule:r1<member | revoke pLab/advanced rule:r2<trained | revoke pLab/supervisor rule:r3<advanced | revoke pLab/trained rule:r1<member"},
		// An edit of r1's first role, to one they don't hold: r1 as it was
		// names the rows it took back, each later hop its own rule.
		{"edit the first role", []models.MappingRule{r1, r2}, []models.MappingRule{rule("r1", visitor, trained), r2},
			"revoke pLab/advanced rule:r2<trained | revoke pLab/trained rule:r1<member"},
	} {
		if got := plan(c.before, c.after); got != c.want {
			t.Errorf("%s\n got %s\nwant %s", c.name, got, c.want)
		}
	}
}

// A bundle retired: each holder loses what only it gave; a role a direct
// grant also gives stays; a role its rule followed with is the rule's row.
func TestSystemChange_BundleRetired(t *testing.T) {
	rules := []models.MappingRule{{ID: "r1", SourceProject: "pLab", SourceRole: "trained", TargetProject: "pWiki", TargetRole: "editor"}}
	person := db.PersonAccess{
		Direct:   []models.DirectGrant{{ID: "g1", ProjectID: "pLab", RoleKey: "member"}},
		ByBundle: map[string][]models.BundleRole{"b1": {{ProjectID: "pLab", RoleKey: "member"}, {ProjectID: "pLab", RoleKey: "trained"}}},
	}
	s := PlanSnapshot{Access: db.AccessSnapshot{Rules: rules}}
	ch := systemChange("u1", person, s, baseFrom(person, ""), baseFrom(person, "b1"), rules, rules, "bundle", "b1")
	if got := systemWrites(ch); got != "revoke pLab/trained bundle:b1 straight_away | revoke pWiki/editor rule:r1 straight_away" {
		t.Fatalf("retire: %s", got)
	}
}

// Drift › Revoke: every role on the row is revoked as the operator decided,
// one write each, with the row's Zitadel grant id; each waits as its role
// does. Zitadel › Users: a role Syndra still gives is kept, read under the
// lock.
func TestPlanRevoke(t *testing.T) {
	s := PlanSnapshot{Profiles: map[string]models.UserProfile{"u1": {ID: "u1"}}, Catalogue: map[roleKey]roleWords{member: {Role: "Member", Project: "Laser Lab"}, trained: {Role: "Trained", Project: "Laser Lab"}},
		Drift: map[string]models.DriftItem{"d1": {ID: "d1", UserID: "u1", ProjectID: "pLab", RoleKeys: []string{"member", "trained"},
			ZitadelGrantID: "zg1", Status: "pending_triage"}},
		Access: db.AccessSnapshot{People: map[string]db.PersonAccess{"u1": {Direct: []models.DirectGrant{{ID: "g1", ProjectID: "pLab", RoleKey: "member"}}}},
			RoleWaits: map[db.RoleRef]bool{{ProjectID: "pLab", RoleKey: "trained"}: true}}}

	ch := Plan(Action{Op: OpRevoke, DriftID: "d1"}, s)
	if got := systemWrites(ch); got != "revoke pLab/member direct: straight_away | revoke pLab/trained direct: waits_for_you" {
		t.Fatalf("drift revoke: %s", got)
	}
	if ch.Writes[0].ZitadelGrantID != "zg1" || ch.Row.Detail != "Loses Member in Laser Lab and Trained in Laser Lab." {
		t.Fatalf("grant id and words: %+v", ch.Row)
	}
	if ch.Row.UserID != "d1" || ch.Delivery() != DeliveryWaits {
		t.Fatalf("the row is the drift row, and it waits: %+v", ch.Row)
	}

	// The Zitadel › Users page: member is a direct grant of theirs (a removal,
	// not a revoke, picks it), so a revoke of it keeps it.
	kept := Plan(Action{Op: OpRevoke, UserID: "u1", ProjectID: "pLab", RoleKey: "member"}, s)
	if kept.Refusal != "held" || len(kept.Writes) != 0 {
		t.Fatalf("a role Syndra gives is kept: %+v", kept)
	}
	gone := Plan(Action{Op: OpRevoke, UserID: "u1", ProjectID: "pLab", RoleKey: "stray"}, s)
	if got := systemWrites(gone); got != "revoke pLab/stray direct: straight_away" {
		t.Fatalf("a role nothing gives is revoked: %s", got)
	}

	// A drift row resolved since is refused before anything is planned.
	s.Drift["d1"] = models.DriftItem{ID: "d1", UserID: "u1", ProjectID: "pLab", Status: "attributed"}
	if ch := Plan(Action{Op: OpRevoke, DriftID: "d1"}, s); ch.Refusal != "resolved" {
		t.Fatalf("resolved: %+v", ch)
	}
}
