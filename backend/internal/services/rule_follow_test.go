package services

import (
	"context"
	"errors"
	"slices"
	"testing"

	"syndra/internal/access"
	"syndra/internal/db"
	"syndra/internal/models"
)

// ledger is a fake of what followRules reads: the person's direct grants and
// their queued revocations, which the write under test changes.
type ledger struct {
	direct  []models.DirectGrant
	bundle  map[string][]models.BundleRole
	queued  []db.RoleRef
	reads   int
	written bool
}

func (l *ledger) install(t *testing.T, rules []models.MappingRule) {
	t.Helper()
	resetCascadeDeps(t)
	svcGetActiveMappingRules = func(context.Context) ([]models.MappingRule, error) { return rules, nil }
	svcGetDirectGrantsForUser = func(context.Context, string, bool) ([]models.DirectGrant, error) {
		l.reads++
		return slices.Clone(l.direct), nil
	}
	svcGetUserBundleRolesGrouped = func(context.Context, string) (map[string][]models.BundleRole, error) {
		return l.bundle, nil
	}
	svcQueuedRevocations = func(context.Context, string) ([]db.RoleRef, error) { return l.queued, nil }
}

var chain = []models.MappingRule{
	{ID: "r-ab", SourceProject: "p", SourceRole: "a", TargetProject: "q", TargetRole: "b"},
	{ID: "r-bc", SourceProject: "q", SourceRole: "b", TargetProject: "r", TargetRole: "c"},
}

type write struct{ op, project, role, ref string }

func writesOf(ps []db.EnqueueParams) []write {
	var out []write
	for _, p := range ps {
		if p.Source != "rule" || len(p.RoleKeys) != 1 || p.UserID != "u1" || p.GrantedBy != "op" {
			return []write{{op: "malformed"}}
		}
		out = append(out, write{p.OpType, p.ProjectID, p.RoleKeys[0], p.SourceRef})
	}
	return out
}

// A role Syndra gives by hand reaches the end of its chain: A→B→C queues B and
// C as rule rows, attributed to the rule that reaches each. Before this, the
// direct write enqueued A alone and the webhook that would have fired the rule
// was dropped as Syndra's own.
func TestFollowRules_OwnGrantQueuesTheWholeChain(t *testing.T) {
	l := &ledger{}
	l.install(t, chain)

	got, err := followRules(context.Background(), db.EnqueueParams{UserID: "u1", ProjectID: "p", RoleKeys: []string{"a"}, OpType: "add", GrantedBy: "op"},
		func() error {
			l.written = true
			l.direct = append(l.direct, models.DirectGrant{ProjectID: "p", RoleKey: "a"})
			return nil
		})
	if err != nil {
		t.Fatal(err)
	}
	if !l.written {
		t.Fatal("the write never ran")
	}
	want := []write{{"add", "q", "b", "r-ab"}, {"add", "r", "c", "r-bc"}}
	if w := writesOf(got); !slices.Equal(w, want) {
		t.Fatalf("rule writes = %v, want %v (the write's own role A is the caller's row, not the rules')", w, want)
	}
}

// Removal is the mirror and follows the same chain, and keeps a role still
// held another way: C also comes from a bundle, so only B goes.
func TestFollowRules_RemovalFollowsTheChainAndKeepsWhatIsHeldOtherwise(t *testing.T) {
	l := &ledger{
		direct: []models.DirectGrant{{ProjectID: "p", RoleKey: "a"}},
		bundle: map[string][]models.BundleRole{"bun": {{ProjectID: "r", RoleKey: "c"}}},
	}
	l.install(t, chain)

	got, err := followRules(context.Background(), db.EnqueueParams{UserID: "u1", ProjectID: "p", RoleKeys: []string{"a"}, OpType: "revoke", GrantedBy: "op"},
		func() error {
			// A queued revoke is what the base subtracts, as the outbox row would.
			l.queued = append(l.queued, db.RoleRef{ProjectID: "p", RoleKey: "a"})
			return nil
		})
	if err != nil {
		t.Fatal(err)
	}
	if w, want := writesOf(got), []write{{"revoke", "q", "b", "r-ab"}}; !slices.Equal(w, want) {
		t.Fatalf("rule writes = %v, want %v", w, want)
	}
}

// Without the bundle, the whole chain goes.
func TestFollowRules_RemovalTakesTheWholeChainWhenNothingElseHoldsIt(t *testing.T) {
	l := &ledger{direct: []models.DirectGrant{{ProjectID: "p", RoleKey: "a"}}}
	l.install(t, chain)
	got, err := followRules(context.Background(), db.EnqueueParams{UserID: "u1", ProjectID: "p", RoleKeys: []string{"a"}, OpType: "revoke", GrantedBy: "op"},
		func() error {
			l.queued = append(l.queued, db.RoleRef{ProjectID: "p", RoleKey: "a"})
			return nil
		})
	if err != nil {
		t.Fatal(err)
	}
	want := []write{{"revoke", "q", "b", "r-ab"}, {"revoke", "r", "c", "r-bc"}}
	if w := writesOf(got); !slices.Equal(w, want) {
		t.Fatalf("rule writes = %v, want %v", w, want)
	}
}

// A role the person already reaches through a rule, given again by hand, owes
// the rules nothing new.
func TestFollowRules_NothingOwedWhenTheClosureDoesNotMove(t *testing.T) {
	l := &ledger{direct: []models.DirectGrant{{ProjectID: "p", RoleKey: "a"}}}
	l.install(t, chain)
	got, err := followRules(context.Background(), db.EnqueueParams{UserID: "u1", ProjectID: "q", RoleKeys: []string{"b"}, OpType: "add", GrantedBy: "op"},
		func() error {
			l.direct = append(l.direct, models.DirectGrant{ProjectID: "q", RoleKey: "b"})
			return nil
		})
	if err != nil || len(got) != 0 {
		t.Fatalf("got %v, %v; want nothing", got, err)
	}
}

// With no rules (production today) the write goes alone and the person's
// holdings are not read at all.
func TestFollowRules_NoRulesCostsNoReads(t *testing.T) {
	l := &ledger{}
	l.install(t, nil)
	ran := false
	got, err := followRules(context.Background(), db.EnqueueParams{UserID: "u1", ProjectID: "p", RoleKeys: []string{"a"}, OpType: "add"},
		func() error { ran = true; return nil })
	if err != nil || len(got) != 0 || !ran || l.reads != 0 {
		t.Fatalf("got %v err %v ran %v reads %d", got, err, ran, l.reads)
	}
}

// A failed write fails the whole thing: the caller's transaction rolls back
// and no rule row exists for a write that did not happen.
func TestFollowRules_AFailedWriteQueuesNothing(t *testing.T) {
	l := &ledger{}
	l.install(t, chain)
	boom := errors.New("boom")
	got, err := followRules(context.Background(), db.EnqueueParams{UserID: "u1", ProjectID: "p", RoleKeys: []string{"a"}, OpType: "add"},
		func() error { return boom })
	if !errors.Is(err, boom) || got != nil {
		t.Fatalf("got %v, %v", got, err)
	}
}

// The systems a rule-derived role is mapped to converge with it, in the same
// transaction, as they would for any cascade.
func TestFollowRules_ConvergesWhatTheRulesReach(t *testing.T) {
	l := &ledger{}
	l.install(t, chain)
	dbTargetsMappedToRole = func(_ context.Context, project, role string) ([]string, error) {
		if project == "r" && role == "c" {
			return []string{"truenas"}, nil
		}
		return nil, nil
	}
	origResolve := svcResolveEntitlements
	t.Cleanup(func() { svcResolveEntitlements = origResolve })
	svcResolveEntitlements = func(context.Context, string, string) (EntitlementSet, error) { return EntitlementSet{}, nil }
	var queued []string
	dbRecordSystemConvergence = func(_ context.Context, c db.SystemConvergence) (string, string, error) {
		queued = append(queued, c.Target+"/"+c.SubjectID)
		return "", "", nil
	}
	if _, err := followRules(context.Background(), db.EnqueueParams{UserID: "u1", ProjectID: "p", RoleKeys: []string{"a"}, OpType: "add", GrantedBy: "op"},
		func() error {
			l.direct = append(l.direct, models.DirectGrant{ProjectID: "p", RoleKey: "a"})
			return nil
		}); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(queued, []string{"truenas/u1"}) {
		t.Fatalf("convergences = %v", queued)
	}
}

// C1, C2: the preview a give or an adoption shows is what the write queues.
// RulesFollowing and followRules share ruleDelta, so for the same person and
// the same role they name the same rows, the whole chain, and the rule that
// reaches each.
func TestRulesFollowing_NamesWhatTheWriteQueues(t *testing.T) {
	l := &ledger{}
	l.install(t, chain)
	preview, err := RulesFollowing(context.Background(), "u1", []access.Role{{ProjectID: "p", RoleKey: "a"}})
	if err != nil {
		t.Fatal(err)
	}
	queued, err := followRules(context.Background(), db.EnqueueParams{UserID: "u1", ProjectID: "p", RoleKeys: []string{"a"}, OpType: "add", GrantedBy: "op"},
		func() error {
			l.direct = append(l.direct, models.DirectGrant{ProjectID: "p", RoleKey: "a"})
			return nil
		})
	if err != nil {
		t.Fatal(err)
	}
	var said []write
	for _, r := range preview {
		said = append(said, write{"add", r.ProjectID, r.RoleKey, r.RuleID})
	}
	if w := writesOf(queued); !slices.Equal(said, w) {
		t.Fatalf("preview %v, queued %v: the dialog must name exactly what the write queues", said, w)
	}
	if preview[1].Sentence != "Also gives c in r, by the automatic rule from b." {
		t.Fatalf("an unnamed role falls back to its key, got %q", preview[1].Sentence)
	}
}

// Nothing is previewed for a role already held, or with no rules (production
// today), and no rules costs no reads.
func TestRulesFollowing_NothingWhenNothingFollows(t *testing.T) {
	l := &ledger{direct: []models.DirectGrant{{ProjectID: "p", RoleKey: "a"}}}
	l.install(t, chain)
	if got, err := RulesFollowing(context.Background(), "u1", []access.Role{{ProjectID: "p", RoleKey: "a"}}); err != nil || got != nil {
		t.Fatalf("already held: got %v, %v", got, err)
	}
	l = &ledger{}
	l.install(t, nil)
	if got, err := RulesFollowing(context.Background(), "u1", []access.Role{{ProjectID: "p", RoleKey: "a"}}); err != nil || got != nil || l.reads != 0 {
		t.Fatalf("no rules: got %v, %v, reads %d", got, err, l.reads)
	}
}

// It is the hook the database layer runs every direct write through.
func TestFollowRules_IsWiredIntoTheDirectWritePath(t *testing.T) {
	if db.FollowRules == nil {
		t.Fatal("db.FollowRules is unwired: Syndra's own grants would reach no rule target")
	}
}
