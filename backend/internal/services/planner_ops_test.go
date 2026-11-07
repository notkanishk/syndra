package services

import (
	"slices"
	"strings"
	"testing"
	"time"

	"syndra/internal/db"
	"syndra/internal/models"
)

// The rest of the access changes on plain data (task 1.3, decision 10):
// extend, the bulk role ops, a bulk decision's approval, adopting drift, and
// repinning a bundle's holders. Each case is a snapshot and the Change it
// must give: its writes and their delivery, its cancels, its rule rows, its
// refusals and its words.

var opsDay = time.Date(2026, 11, 8, 10, 0, 0, 0, time.UTC)

func withGrantEnding(s *PlanSnapshot, k roleKey, end *time.Time) {
	p := s.Access.People["u1"]
	p.Direct = append(p.Direct, models.DirectGrant{ID: "g-" + k.roleKey, UserID: "u1", ProjectID: k.projectID, RoleKey: k.roleKey, ExpiresAt: end})
	s.Access.People["u1"] = p
}

func extend(s PlanSnapshot, days int) Change {
	return Plan(Action{Op: OpExtend, UserID: "u1", ProjectID: "pLab", RoleKey: "member", Days: days}, s)
}

// Settled 5 Nov: Syndra enforces end dates, Zitadel has none. An extension is
// a ledger edit, so its Change sends nothing.
func TestPlanExtend_IsALedgerEditWithNoZitadelWrite(t *testing.T) {
	s := plannerSnap()
	s.Now = opsDay
	soon := opsDay.Add(5 * 24 * time.Hour)
	withGrantEnding(&s, member, &soon)

	ch := extend(s, 30)
	if ch.Row.Effect != EffectApply || len(ch.Writes) != 1 {
		t.Fatalf("change = %+v", ch)
	}
	w := ch.Writes[0]
	if !w.NoPropagation || w.Delivery != DeliveryNone || w.EndRule != db.EndSet || w.ExpiresAt == nil || !w.ExpiresAt.After(soon) {
		t.Fatalf("an extension moves the ledger's end and sends nothing: %+v", w)
	}
	if ch.Delivery() != DeliveryNone || len(ch.Goes) != 0 || len(ch.Waits) != 0 || ch.Row.Delivery != DeliveryNone {
		t.Fatalf("nothing goes to Zitadel, nothing waits: delivery %q goes %v waits %v", ch.Delivery(), ch.Goes, ch.Waits)
	}
	if !strings.HasPrefix(ch.Row.Detail, "Keeps Member in Laser Lab until ") ||
		ch.Row.Consequence != "Nothing is sent to Zitadel, which has no end dates. Syndra ends the access on the day." {
		t.Fatalf("detail %q consequence %q", ch.Row.Detail, ch.Row.Consequence)
	}
	if !slices.Equal(ch.Row.GrantIDs, []string{"g-member"}) {
		t.Errorf("the row names the grant it extends: %v", ch.Row.GrantIDs)
	}
	// The end is the decision: a different end is a different Change.
	if longer := extend(s, 60); longer.Row.Fingerprint == ch.Row.Fingerprint {
		t.Error("extending to another opsDay must be another fingerprint")
	}
}

func TestPlanExtend_Refusals(t *testing.T) {
	s := plannerSnap()
	s.Now = opsDay
	if ch := extend(s, 30); ch.Refusal != "nothing_to_extend" || ch.Row.Effect != EffectNoChange {
		t.Errorf("nothing held: %+v", ch.Row)
	}
	withGrantEnding(&s, member, nil)
	if ch := extend(s, 30); ch.Refusal != "nothing_to_extend" {
		t.Errorf("a grant with no end is never stamped with one: %+v", ch.Row)
	}
	far := opsDay.Add(200 * 24 * time.Hour)
	s = plannerSnap()
	s.Now = opsDay
	withGrantEnding(&s, member, &far)
	if ch := extend(s, 30); ch.Refusal != "runs_past" || ch.Row.Detail != "Already ends later than this would make it." {
		t.Errorf("an extension never shortens: %+v", ch.Row)
	}
	p := s.Profiles["u1"]
	p.Status = "locked"
	s.Profiles["u1"] = p
	if ch := extend(s, 30); !strings.HasPrefix(ch.Refusal, "departed") || ch.Row.Consequence == "" {
		t.Errorf("a departed account is refused, with its step: %+v", ch.Row)
	}
}

// Expiring access selects grant rows: only the ones ticked are extended.
func TestPlanExtend_OnlyTheTickedGrants(t *testing.T) {
	s := plannerSnap()
	s.Now = opsDay
	soon := opsDay.Add(5 * 24 * time.Hour)
	withGrantEnding(&s, member, &soon)
	withGrantEnding(&s, trained, &soon)
	s.Ticked = map[string]bool{"g-trained": true}
	ch := Plan(Action{Op: OpExtend, UserID: "u1", Days: 30}, s)
	if !slices.Equal(ch.Row.GrantIDs, []string{"g-trained"}) || len(ch.Writes) != 1 || ch.Writes[0].RoleKeys[0] != "trained" {
		t.Fatalf("only the ticked grant: %+v", ch.Row)
	}
	s.Ticked = map[string]bool{"g-someone-else": true}
	if ch := Plan(Action{Op: OpExtend, UserID: "u1", Days: 30}, s); ch.Row.Detail != "None of the selected grants are theirs any more." ||
		ch.Row.Consequence == "" {
		t.Fatalf("a ticked grant that has gone says so, with a step: %+v", ch.Row)
	}
}

// The one case an extension reaches the outbox: its new end answers a
// removal still waiting for you to send. It cancels it, and still sends
// nothing (the role stands in Zitadel).
func TestPlanExtend_CancelsAWaitingRemoval(t *testing.T) {
	s := plannerSnap()
	s.Now = opsDay
	soon := opsDay.Add(2 * 24 * time.Hour)
	withGrantEnding(&s, member, &soon)
	withQueued(&s, models.QueuedChange{ID: "q1", OpType: "revoke", ProjectID: "pLab", RoleKeys: []string{"member"}, Source: "direct"})
	p := s.Access.People["u1"]
	p.Standing = map[db.RoleRef]bool{{ProjectID: "pLab", RoleKey: "member"}: true}
	s.Access.People["u1"] = p

	ch := extend(s, 30)
	if ch.Writes[0].NoPropagation || ch.Writes[0].Delivery != DeliveryNone {
		t.Fatalf("the write goes to the one place cancellations happen, and sends nothing: %+v", ch.Writes[0])
	}
	if !slices.Equal(ch.Row.CancelIDs, []string{"q1"}) || !strings.Contains(ch.Row.Consequence, "Cancels a change waiting") {
		t.Fatalf("it names what it cancels: %v %q", ch.Row.CancelIDs, ch.Row.Consequence)
	}
}

func bulkGive(s PlanSnapshot, days int) Change {
	return Plan(Action{Op: BulkOpAssignRole, UserID: "u1", ProjectID: "pLab", RoleKey: "member", Days: days}, s)
}

// A bulk give is a give: the same own row, rule rows, delivery and cancels,
// judged from the plan's one snapshot, with its end said.
func TestPlanBulkGive_IsTheGiveWithItsEnd(t *testing.T) {
	s := plannerSnap()
	s.Now = opsDay
	memberGivesTrained(&s)
	s.Access.RoleWaits[db.RoleRef{ProjectID: "pLab", RoleKey: "trained"}] = true
	ch := bulkGive(s, 30)
	if got := plannedWrites(ch); !slices.Equal(got, []sentWrite{{"add", "member", DeliveryStraightAway}, {"add", "trained", DeliveryWaits}}) {
		t.Fatalf("writes = %+v", got)
	}
	if ch.Writes[0].ExpiresAt == nil || ch.Writes[0].Source != "direct" || ch.Writes[1].Source != "rule" {
		t.Fatalf("own row ends, rule row is the rule's: %+v", ch.Writes)
	}
	if !strings.HasPrefix(ch.Row.Consequence, "Expires ") ||
		!strings.Contains(ch.Row.Consequence, "Also gives Trained in Laser Lab, by the automatic rule from Member in Laser Lab.") ||
		!strings.HasSuffix(ch.Row.Consequence, "Giving Trained in Laser Lab waits for you to send.") {
		t.Fatalf("consequence = %q", ch.Row.Consequence)
	}
	if ch.Row.Delivery != DeliveryWaits || !slices.Equal(ch.Row.Goes, []string{"Member in Laser Lab"}) ||
		!slices.Equal(ch.Row.Waits, []string{"Trained in Laser Lab"}) {
		t.Fatalf("the row says what goes now and what waits: %q %v %v", ch.Row.Delivery, ch.Row.Goes, ch.Row.Waits)
	}
	if bulkGive(s, 0).Row.Consequence[:len("No expiry.")] != "No expiry." {
		t.Error("no end is said")
	}
}

func TestPlanBulkGive_RefusesWhatTheyHold(t *testing.T) {
	s := plannerSnap()
	s.Now = opsDay
	soon := opsDay.Add(5 * 24 * time.Hour)
	withGrantEnding(&s, member, &soon)
	if ch := bulkGive(s, 30); ch.Refusal != "held" || ch.Row.Consequence != "Extend it instead." {
		t.Errorf("a grant that ends: extend it instead: %+v", ch.Row)
	}
	s = plannerSnap()
	holdsBundle(&s)
	if ch := bulkGive(s, 30); ch.Refusal != "held" || !strings.Contains(ch.Row.Detail, "another way") {
		t.Errorf("held through a bundle: %+v", ch.Row)
	}
	s = plannerSnap()
	p := s.Profiles["u1"]
	p.Status = "inactive"
	s.Profiles["u1"] = p
	if ch := bulkGive(s, 30); ch.Row.Effect != EffectBlocked || ch.Row.Detail != "Account is inactive. Remove it from the selection to continue." {
		t.Errorf("departed: %+v", ch.Row)
	}
}

func TestPlanBulkRemove_FindsTheDirectGrant(t *testing.T) {
	s := plannerSnap()
	withDirect(&s, member)
	ch := Plan(Action{Op: BulkOpRemoveRole, UserID: "u1", ProjectID: "pLab", RoleKey: "member"}, s)
	if got := plannedWrites(ch); !slices.Equal(got, []sentWrite{{"revoke", "member", DeliveryStraightAway}}) ||
		!slices.Equal(ch.Row.GrantIDs, []string{"g-member"}) || ch.Row.Detail != "Loses Member in Laser Lab." {
		t.Fatalf("change = %+v", ch.Row)
	}
	if src := writeSources(ch); src["revoke member"] != "direct/g-member" {
		t.Fatalf("the revoke is the grant's: %v", src)
	}

	s = plannerSnap()
	holdsBundle(&s)
	ch = Plan(Action{Op: BulkOpRemoveRole, UserID: "u1", ProjectID: "pLab", RoleKey: "member"}, s)
	if ch.Refusal != "not_held" || ch.Row.Detail != "Holds no direct grant here." || len(ch.Row.GrantIDs) != 0 {
		t.Fatalf("no direct grant to remove: %+v", ch.Row)
	}
	if ch = Plan(Action{Op: BulkOpRemoveRole, UserID: "u1", ProjectID: "pLab", RoleKey: "advanced"}, s); ch.Row.Detail != "Doesn't hold this role." {
		t.Fatalf("not held at all: %+v", ch.Row)
	}
}

// A bulk decision plans every request from its own read under the lock: one
// that has gone, one already decided, and one approved as a single approval is.
func TestPlanBulkApprove_EachRequestFromTheSnapshot(t *testing.T) {
	s := plannerSnap()
	s.Now = opsDay
	s.Requests = map[string]models.AccessRequest{
		"req1": {ID: "req1", RequesterID: "u1", ProjectID: "pLab", RoleKey: "member", Status: "pending"},
		"req2": {ID: "req2", RequesterID: "u1", ProjectID: "pLab", RoleKey: "trained", Status: "rejected"},
	}
	s.Holdings = map[string]models.RequestHolding{"req1": {Kind: models.HoldingNone}, "req2": {Kind: models.HoldingNone}}

	ch := Plan(Action{Op: OpApprove, RequestID: "req1"}, s)
	if ch.Row.UserID != "req1" || ch.UserID != "u1" || ch.Row.Effect != EffectApply || ch.Row.Detail != "Gains Member in Laser Lab." {
		t.Fatalf("the row is the request, the change the requester's: %+v", ch)
	}
	if ch.Row.Name != "Ada Lovelace" {
		t.Errorf("the row names the person: %q", ch.Row.Name)
	}
	if ch := Plan(Action{Op: OpApprove, RequestID: "req2"}, s); ch.Refusal != "resolved" || ch.Row.Detail != "Already declined." {
		t.Errorf("already decided: %+v", ch.Row)
	}
	if ch := Plan(Action{Op: OpApprove, RequestID: "gone"}, s); ch.Refusal != "absent" || ch.Row.Consequence != "Nothing for you to do." {
		t.Errorf("withdrawn: %+v", ch.Row)
	}
	s.Holdings["req1"] = models.RequestHolding{Kind: models.HoldingForGood}
	if ch := Plan(Action{Op: OpApprove, RequestID: "req1"}, s); ch.Refusal != "held" || ch.Row.Consequence != "Decline it to close the request." {
		t.Errorf("already held: the step is to decline: %+v", ch.Row)
	}
}

func adoptSnap() PlanSnapshot {
	s := plannerSnap()
	s.Drift = map[string]models.DriftItem{"d1": {ID: "d1", UserID: "u1", ProjectID: "pLab", RoleKeys: []string{"member"},
		ZitadelGrantID: "zg1", DriftType: db.DriftTargetOnly, Status: "pending_triage"}}
	return s
}

// Adopting records what Zitadel has (no outbox row for it: an add would be a
// live instruction to re-create it) and the rules follow it, sent as any
// rule row is (decision 11).
func TestPlanner_AdoptRecordsTheRoleAndSendsNothingForIt(t *testing.T) {
	s := adoptSnap()
	memberGivesTrained(&s)
	ch := Plan(Action{Op: OpAdopt, DriftID: "d1"}, s)
	if ch.Row.UserID != "d1" || ch.UserID != "u1" || ch.Row.Effect != EffectApply {
		t.Fatalf("change = %+v", ch)
	}
	own := ch.Writes[0]
	if !own.NoPropagation || own.Delivery != DeliveryNone || own.Source != "external_backfill" || own.SourceRef != "" ||
		own.OpType != "add" || own.ZitadelGrantID != "zg1" {
		t.Fatalf("the adopted role is a ledger row only: %+v", own)
	}
	if got := plannedWrites(ch)[1:]; !slices.Equal(got, []sentWrite{{"add", "trained", DeliveryStraightAway}}) ||
		writeSources(ch)["add trained"] != "rule/r1" {
		t.Fatalf("the rule's row: %+v %v", got, writeSources(ch))
	}
	if ch.Row.Detail != "Adopted into Syndra (Member in Laser Lab)." ||
		!strings.Contains(ch.Row.Consequence, "Nothing is sent to Zitadel for it.") ||
		!strings.Contains(ch.Row.Consequence, "Also gives Trained in Laser Lab, by the automatic rule from Member in Laser Lab.") {
		t.Fatalf("words: %q / %q", ch.Row.Detail, ch.Row.Consequence)
	}
	if ch.Row.Delivery != DeliveryStraightAway || !slices.Equal(ch.Row.Goes, []string{"Trained in Laser Lab"}) {
		t.Errorf("what the rule gives goes now: %q %v", ch.Row.Delivery, ch.Row.Goes)
	}
}

func TestPlanner_AdoptRefusals(t *testing.T) {
	s := adoptSnap()
	if ch := Plan(Action{Op: OpAdopt, DriftID: "gone"}, s); ch.Refusal != "absent" || ch.Row.Consequence == "" {
		t.Errorf("gone: %+v", ch.Row)
	}
	item := s.Drift["d1"]
	item.Status = "attributed"
	s.Drift["d1"] = item
	if ch := Plan(Action{Op: OpAdopt, DriftID: "d1"}, s); ch.Refusal != "resolved" || ch.Row.Detail != "Already resolved as adopted." {
		t.Errorf("resolved: %+v", ch.Row)
	}
	item.Status, item.DriftType = "pending_triage", "syndra_only"
	s.Drift["d1"] = item
	if ch := Plan(Action{Op: OpAdopt, DriftID: "d1"}, s); ch.Refusal != "nothing_to_adopt" || ch.Row.Consequence == "" {
		t.Errorf("Zitadel doesn't have it: %+v", ch.Row)
	}
	s = adoptSnap()
	memberGivesTrained(&s)
	delete(s.Catalogue, trained)
	if ch := Plan(Action{Op: OpAdopt, DriftID: "d1"}, s); !strings.HasPrefix(ch.Refusal, "not_offered") ||
		ch.Row.Consequence != "Retire the automatic rule that gives it, then adopt it again." {
		t.Errorf("a rule's role Zitadel no longer offers: %+v", ch.Row)
	}
}

func moveSnap() PlanSnapshot {
	s := plannerSnap()
	holdsBundle(&s) // on v2: member, trained
	s.Holders = map[string]models.BundleHolder{"u1": {UserID: "u1", BundleID: "b1", VersionID: "v2", Version: 2}}
	s.Version = models.BundleVersion{ID: "v3", Version: 3}
	s.VersionRoles = []models.BundleRole{{BundleID: "b1", ProjectID: "pLab", RoleKey: "member"}, {BundleID: "b1", ProjectID: "pLab", RoleKey: "advanced"}}
	return s
}

func TestPlanMove_GainsAndLosesTheVersionsDifference(t *testing.T) {
	s := moveSnap()
	ch := Plan(Action{Op: OpMove, UserID: "u1", BundleID: "b1"}, s)
	if got := plannedWrites(ch); !slices.Equal(got, []sentWrite{{"add", "advanced", DeliveryStraightAway}, {"revoke", "trained", DeliveryStraightAway}}) {
		t.Fatalf("writes = %+v", got)
	}
	if ch.Row.Detail != "Moves from v2 to v3." || ch.Row.Consequence != "Gains Advanced in Laser Lab. Loses Trained in Laser Lab." {
		t.Fatalf("words: %q / %q", ch.Row.Detail, ch.Row.Consequence)
	}
	if src := writeSources(ch); src["add advanced"] != "bundle/b1" || src["revoke trained"] != "bundle/b1" {
		t.Fatalf("the bundle's rows: %v", src)
	}
	if strings.ContainsAny(ch.Row.Detail+ch.Row.Consequence, "→—") {
		t.Error("no arrows or em dashes in the words")
	}
}

func TestPlanMove_RowsThatDontMoveAnything(t *testing.T) {
	s := moveSnap()
	s.Holders["u1"] = models.BundleHolder{UserID: "u1", VersionID: "v3", Version: 3}
	if ch := Plan(Action{Op: OpMove, UserID: "u1", BundleID: "b1"}, s); ch.Refusal != "held" || ch.Row.Detail != "Already on v3." {
		t.Errorf("already there: %+v", ch.Row)
	}
	s = moveSnap()
	if ch := Plan(Action{Op: OpMove, UserID: "u9", BundleID: "b1"}, s); ch.Row.Effect != EffectBlocked && ch.Refusal != "absent" {
		t.Errorf("not a holder: %+v", ch.Row)
	}
	s.LeaveHolders = true
	if ch := Plan(Action{Op: OpPublish, UserID: "u1", BundleID: "b1"}, s); ch.Refusal != "stays" || ch.Row.Detail != "Stays on v2." {
		t.Errorf("a publish that leaves them: %+v", ch.Row)
	}
	s = moveSnap()
	withDirect(&s, advanced)
	withDirect(&s, trained)
	ch := Plan(Action{Op: OpMove, UserID: "u1", BundleID: "b1"}, s)
	if ch.Row.Effect != EffectNoChange || ch.Refusal != "" || len(ch.Writes) != 0 ||
		ch.Row.Consequence != "Everything in this version they already hold another way, so nothing is sent to Zitadel." {
		t.Errorf("a move that changes nothing they hold is still a move, with nothing sent: %+v", ch)
	}
	s = moveSnap()
	delete(s.Catalogue, advanced)
	if ch := Plan(Action{Op: OpMove, UserID: "u1", BundleID: "b1"}, s); !strings.HasPrefix(ch.Refusal, "not_offered") ||
		ch.Row.Consequence != "Take it out of the bundle and publish again." {
		t.Errorf("a role Zitadel no longer offers: %+v", ch.Row)
	}
}
