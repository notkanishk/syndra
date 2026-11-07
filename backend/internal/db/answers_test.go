package db

import (
	"context"
	"testing"

	"syndra/internal/models"
)

// Answers is the one statement of which queued change a write cancels: the
// rules the SQL predicate used to state, held here on plain data.
func TestAnswers(t *testing.T) {
	q := func(op, source, ref string) models.QueuedChange {
		return models.QueuedChange{ID: "q", OpType: op, UserID: "u1", ProjectID: "p", RoleKeys: []string{"r"}, Source: source, SourceRef: ref}
	}
	w := func(op, source, ref string) EnqueueParams {
		return EnqueueParams{UserID: "u1", ProjectID: "p", RoleKeys: []string{"r"}, OpType: op, Source: source, SourceRef: ref}
	}
	for _, tc := range []struct {
		name string
		p    EnqueueParams
		c    models.QueuedChange
		want bool
	}{
		{"an add answers a removal from any source", w("add", "bundle", "b1"), q("revoke", "direct", ""), true},
		{"an add does not answer an add", w("add", "bundle", "b1"), q("add", "bundle", "b1"), false},
		{"a removal answers its own bundle's add", w("revoke", "bundle", "b1"), q("add", "bundle", "b1"), true},
		{"a removal answers its own bundle's replace", w("revoke", "bundle", "b1"), q("replace", "bundle", "b1"), true},
		{"not another bundle's add", w("revoke", "bundle", "b1"), q("add", "bundle", "b2"), false},
		{"not a direct add", w("revoke", "bundle", "b1"), q("add", "direct", ""), false},
		{"a direct removal answers any direct add", w("revoke", "", "grant-1"), q("add", "direct", "request-9"), true},
		{"not a removal", w("revoke", "bundle", "b1"), q("revoke", "bundle", "b1"), false},
		{"another person", w("add", "", ""), models.QueuedChange{OpType: "revoke", UserID: "u2", ProjectID: "p", RoleKeys: []string{"r"}}, false},
		{"another role", w("add", "", ""), models.QueuedChange{OpType: "revoke", UserID: "u1", ProjectID: "p", RoleKeys: []string{"s"}}, false},
		{"a multi-role write answers nothing", EnqueueParams{UserID: "u1", ProjectID: "p", RoleKeys: []string{"r", "s"}, OpType: "add"}, q("revoke", "direct", ""), false},
		{"a replace answers nothing", w("replace", "", ""), q("revoke", "direct", ""), false},
		// Rule rows (task 1.6): source 'rule', source_ref the rule that reaches the role.
		{"a rule's removal answers the rule's add", w("revoke", "rule", "r1"), q("add", "rule", "r1"), true},
		{"a rule's removal answers another rule's add of the role", w("revoke", "rule", "r2"), q("add", "rule", "r1"), true},
		{"a rule's removal is not a bundle's", w("revoke", "rule", "r1"), q("add", "bundle", "b1"), false},
		{"a rule's removal is not a direct grant's", w("revoke", "rule", "r1"), q("add", "direct", ""), false},
		{"a bundle's removal does not answer a rule's add", w("revoke", "bundle", "b1"), q("add", "rule", "r1"), false},
		{"a direct removal does not answer a rule's add", w("revoke", "", "grant-1"), q("add", "rule", "r1"), false},
		{"a direct give answers a rule's removal", w("add", "", ""), q("revoke", "rule", "r1"), true},
		{"a bundle's give answers a rule's removal", w("add", "bundle", "b1"), q("revoke", "rule", "r1"), true},
		{"a rule's give answers any removal", w("add", "rule", "r1"), q("revoke", "bundle", "b1"), true},
	} {
		if got := Answers(tc.p, tc.c); got != tc.want {
			t.Errorf("%s: Answers = %v, want %v", tc.name, got, tc.want)
		}
	}
}

// A plan apply admits exactly the cancellations its rows named, and refuses
// any other (architecture deepening, decision 4).
func TestCancellationsNamed_AdmitsOnlyThoseNamed(t *testing.T) {
	ctx, gate := GuardCancellations(context.Background(), nil, func(context.Context) bool { return true })
	CancellationsNamed(ctx, []string{"q1"})
	if !AdmitCancellations(ctx, []models.QueuedChange{{ID: "q1"}}) {
		t.Fatal("a named cancellation is admitted")
	}
	if AdmitCancellations(ctx, []models.QueuedChange{{ID: "q1"}, {ID: "q2"}}) {
		t.Fatal("a cancellation the plan didn't name is refused")
	}
	if refused := gate.Refused(); len(refused) == 0 {
		t.Fatal("the refusal is recorded for the response")
	}
}
