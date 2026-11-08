package services

import (
	"slices"
	"strings"
	"testing"
	"time"

	"syndra/internal/db"
	"syndra/internal/models"
)

// The single acts on plain data (task 1.2, decision 10): give, remove a
// direct grant, approve. A bundle's single assign and remove is the bulk
// row's Change (planner_test.go); here only what is the single act's own.

func give(s PlanSnapshot) Change {
	return Plan(Action{Op: OpGive, UserID: "u1", ProjectID: "pLab", RoleKey: "member"}, s)
}

func approve(s PlanSnapshot, r models.AccessRequest) Change {
	r.ID, r.RequesterID, r.ProjectID, r.RoleKey = "req1", "u1", "pLab", "member"
	if r.Status == "" {
		r.Status = "pending"
	}
	s.Request = r
	return Plan(Action{Op: OpApprove, UserID: "u1", ProjectID: "pLab", RoleKey: "member", RequestID: "req1"}, s)
}

func revokeGrant(s PlanSnapshot, grantID string) Change {
	return Plan(Action{Op: OpRemoveGrant, UserID: "u1", GrantID: grantID}, s)
}

// memberGivesTrained is a rule: anyone with Member also gets Trained.
func memberGivesTrained(s *PlanSnapshot) {
	s.Access.Rules = append(s.Access.Rules, models.MappingRule{ID: "r1", SourceProject: "pLab", SourceRole: "member", TargetProject: "pLab", TargetRole: "trained"})
}

func TestPlanAct_GiveIsItsOwnWriteThenTheRulesRows(t *testing.T) {
	s := plannerSnap()
	memberGivesTrained(&s)
	s.Access.RoleWaits[db.RoleRef{ProjectID: "pLab", RoleKey: "trained"}] = true
	ch := give(s)
	if ch.Row.Effect != EffectApply || ch.Refusal != "" {
		t.Fatalf("change = %+v", ch)
	}
	if got := plannedWrites(ch); !slices.Equal(got, []sentWrite{{"add", "member", DeliveryStraightAway}, {"add", "trained", DeliveryWaits}}) {
		t.Fatalf("writes = %+v", got)
	}
	// The act's own row is the direct grant; the rule's is the rule's (decision 7).
	if src := writeSources(ch); src["add member"] != "direct/" || src["add trained"] != "rule/r1" {
		t.Fatalf("sources = %v", src)
	}
	if ch.Writes[1].WaitReason != db.WaitReasonRoleWaits {
		t.Errorf("a write that waits is written waiting: %+v", ch.Writes[1])
	}
	// One source of the rule words (A-21 §4.1), said apart and inside the row.
	wantRule := "Also gives Trained in Laser Lab, by the automatic rule from Member in Laser Lab."
	if !slices.Equal(ch.Rules, []string{wantRule}) || ch.Row.Detail != "Gains Member in Laser Lab." {
		t.Fatalf("rules = %q, detail = %q", ch.Rules, ch.Row.Detail)
	}
	if ch.Row.Consequence != wantRule+" Giving Trained in Laser Lab waits for you to send." {
		t.Errorf("consequence = %q", ch.Row.Consequence)
	}
	if ch.Delivery() != DeliveryWaits || !slices.Equal(ch.Waits, []string{"Trained in Laser Lab"}) {
		t.Errorf("delivery = %q, waits = %v", ch.Delivery(), ch.Waits)
	}
}

func TestPlanAct_GiveRefusesWhatTheyHoldAndWhatZitadelDoesNotOffer(t *testing.T) {
	s := plannerSnap()
	s.Holding = models.RequestHolding{Kind: models.HoldingForGood}
	if ch := give(s); ch.Refusal != "held" || ch.Row.Effect != EffectNoChange || len(ch.Writes) != 0 ||
		ch.Row.Detail != "Already has Member in Laser Lab." {
		t.Fatalf("held: %+v", ch)
	}

	s = plannerSnap()
	s.CatalogueUnread = true
	if ch := give(s); ch.Refusal != "catalogue_unread" || ch.Row.Effect != EffectBlocked {
		t.Fatalf("catalogue unread: %+v", ch)
	}

	// A role a rule follows with that Zitadel no longer offers stops the give,
	// with the step to take.
	s = plannerSnap()
	memberGivesTrained(&s)
	delete(s.Catalogue, trained)
	ch := give(s)
	if ch.Refusal != "not_offered:pLab/trained" || ch.Row.Consequence != "Retire the automatic rule that gives it, then try again." {
		t.Fatalf("rule target gone: %+v", ch.Row)
	}

	s = plannerSnap()
	s.Profiles["u1"] = models.UserProfile{ID: "u1", Status: "Departed"}
	if ch := give(s); ch.Refusal != "departed:Departed" || ch.Row.Effect != EffectBlocked {
		t.Fatalf("departed: %+v", ch)
	}
	// A refusal ends on its step, never on a dead end.
	if ch := give(s); ch.Row.Consequence != "If they still need it, make the account active in Zitadel first." {
		t.Fatalf("departed refusal has no step: %+v", ch.Row)
	}
}

// Giving back a role whose removal is still waiting cancels the removal and
// sends nothing: they never lost it in Zitadel (A-21 §2.4).
func TestPlanAct_GiveThatOnlyCancelsAWaitingRemoval(t *testing.T) {
	s := plannerSnap()
	withQueued(&s, models.QueuedChange{ID: "q1", OpType: "revoke", ProjectID: "pLab", RoleKeys: []string{"member"}, Source: "direct"})
	p := s.Access.People["u1"]
	p.Revoking = []db.RoleRef{{ProjectID: "pLab", RoleKey: "member"}}
	p.Standing = map[db.RoleRef]bool{{ProjectID: "pLab", RoleKey: "member"}: true}
	s.Access.People["u1"] = p

	ch := give(s)
	if got := plannedWrites(ch); !slices.Equal(got, []sentWrite{{"add", "member", DeliveryNone}}) {
		t.Fatalf("writes = %+v", got)
	}
	if !slices.Equal(ch.Row.CancelIDs, []string{"q1"}) || ch.Delivery() != DeliveryNone {
		t.Fatalf("cancels = %v, delivery = %q", ch.Row.CancelIDs, ch.Delivery())
	}
	if ch.Row.Detail != "The removal of Member in Laser Lab that was waiting is cancelled, so nothing is sent to Zitadel. They keep it." {
		t.Errorf("detail = %q", ch.Row.Detail)
	}
	// Said kept apart from the cancels line, which the step says.
	if strings.Contains(strings.Join(ch.Said, " "), "Cancels") || !strings.Contains(ch.Row.Consequence, "Cancels a change waiting") {
		t.Errorf("said = %q, consequence = %q", ch.Said, ch.Row.Consequence)
	}
}

// The fingerprint is the decision: a queued change it would now cancel moves
// it, so a give pressed on an older plan is stale (decisions 3, 4).
func TestPlanAct_FingerprintMovesWithWhatItCancels(t *testing.T) {
	s := plannerSnap()
	before := give(s)
	withQueued(&s, models.QueuedChange{ID: "q9", OpType: "revoke", ProjectID: "pLab", RoleKeys: []string{"member"}, Source: "bundle", SourceRef: "b7"})
	after := give(s)
	if before.Row.Fingerprint == after.Row.Fingerprint {
		t.Fatal("a newly queued change the give cancels must move its fingerprint")
	}
	// A renamed role does not.
	s2 := plannerSnap()
	s2.Catalogue[member] = roleWords{Role: "Member (renamed)", Project: "Laser Lab"}
	if give(s2).Row.Fingerprint != before.Row.Fingerprint {
		t.Error("words are not the decision")
	}
}

func TestPlanAct_ApproveCarriesTheRequestAndItsEnd(t *testing.T) {
	now := time.Date(2026, 11, 8, 12, 0, 0, 0, time.UTC)
	s := plannerSnap()
	s.Now = now
	days := 30
	ch := approve(s, models.AccessRequest{Kind: RequestKindAccess, DurationDays: &days})
	if ch.Row.Effect != EffectApply || ch.Approval.Effect != EffectGive {
		t.Fatalf("change = %+v", ch)
	}
	own := ch.Writes[0]
	if own.Source != "direct" || own.SourceRef != "req1" || own.ExpiresAt == nil || own.Reason != "Approved from access request" {
		t.Fatalf("own write = %+v", own.EnqueueParams)
	}

	// Decided since by somebody else: refused, nothing written.
	if ch := approve(s, models.AccessRequest{Status: "rejected"}); ch.Refusal != "resolved" || len(ch.Writes) != 0 {
		t.Fatalf("resolved: %+v", ch)
	}
	// Come to hold it since asking.
	s.Holding = models.RequestHolding{Kind: models.HoldingForGood}
	if ch := approve(s, models.AccessRequest{Kind: RequestKindAccess}); ch.Refusal != "held" {
		t.Fatalf("held: %+v", ch)
	}
	// An extension moves the end of the grant they have and sends the role again.
	until := now.AddDate(0, 0, 3)
	s.Holding = models.RequestHolding{Kind: models.HoldingUntil, Until: &until}
	withDirect(&s, member)
	ch = approve(s, models.AccessRequest{Kind: RequestKindExtend, DurationDays: &days})
	if ch.Approval.Effect != EffectExtend || ch.Writes[0].EndRule != db.EndSet || !strings.HasPrefix(ch.Row.Detail, "Keeps Member in Laser Lab until ") {
		t.Fatalf("extension: %+v / %q", ch.Approval, ch.Row.Detail)
	}
	// The approval's effect is in the fingerprint: a lapse while it waited
	// (extend becomes give again) is a different decision.
	s.Holding = models.RequestHolding{Kind: models.HoldingNone}
	if again := approve(s, models.AccessRequest{Kind: RequestKindExtend, DurationDays: &days}); again.Row.Fingerprint == ch.Row.Fingerprint {
		t.Error("extended and given again must not share a fingerprint")
	}
}

func TestPlanAct_RemoveGrantTakesBackOnlyWhatNothingElseGives(t *testing.T) {
	s := plannerSnap()
	memberGivesTrained(&s)
	withDirect(&s, member)
	ch := revokeGrant(s, "g-member")
	if got := plannedWrites(ch); !slices.Equal(got, []sentWrite{{"revoke", "member", DeliveryStraightAway}, {"revoke", "trained", DeliveryStraightAway}}) {
		t.Fatalf("writes = %+v", got)
	}
	if src := writeSources(ch); src["revoke member"] != "direct/g-member" || src["revoke trained"] != "rule/r1" {
		t.Fatalf("sources = %v", src)
	}
	if ch.Row.Detail != "Loses Member in Laser Lab." ||
		!slices.Equal(ch.Rules, []string{"Also loses Trained in Laser Lab, which the automatic rule from Member in Laser Lab gave."}) {
		t.Fatalf("detail = %q, rules = %q", ch.Row.Detail, ch.Rules)
	}
	if ch.ProjectID != "pLab" || ch.RoleKey != "member" {
		t.Errorf("the act names the grant's role: %s/%s", ch.ProjectID, ch.RoleKey)
	}

	// Still in a bundle they hold: nothing is revoked, and the row says so.
	holdsBundle(&s)
	ch = revokeGrant(s, "g-member")
	if len(ch.Writes) != 0 || ch.Row.Detail != "Keeps Member in Laser Lab another way, so nothing is sent to Zitadel." {
		t.Fatalf("kept: %+v / %q", ch.Writes, ch.Row.Detail)
	}

	if ch := revokeGrant(plannerSnap(), "g-none"); ch.Refusal != "not_held" || len(ch.Writes) != 0 {
		t.Fatalf("no such grant: %+v", ch)
	}
}

// Revoking a give that never left: the queued give is cancelled and nothing
// is sent ("Cancel the waiting give", A-21 §3.2).
func TestPlanAct_RemoveGrantThatOnlyCancelsTheWaitingGive(t *testing.T) {
	s := plannerSnap()
	withDirect(&s, member)
	withQueued(&s, models.QueuedChange{ID: "q2", OpType: "add", ProjectID: "pLab", RoleKeys: []string{"member"}, Source: "direct"})
	ch := revokeGrant(s, "g-member")
	if got := plannedWrites(ch); !slices.Equal(got, []sentWrite{{"revoke", "member", DeliveryNone}}) {
		t.Fatalf("writes = %+v", got)
	}
	if ch.Delivery() != DeliveryNone || !slices.Equal(ch.Row.CancelIDs, []string{"q2"}) ||
		ch.Row.Detail != "Member in Laser Lab was waiting for you to send. It is cancelled, so nothing is sent to Zitadel." {
		t.Fatalf("delivery = %q, cancels = %v, detail = %q", ch.Delivery(), ch.Row.CancelIDs, ch.Row.Detail)
	}
}
