package services

import (
	"context"
	"testing"
	"time"

	"syndra/internal/addons"
	"syndra/internal/db"
	"syndra/internal/models"
)

// §29 — the one rule this surface rests on, and the one distinction it must not
// collapse.

func stubDormant(t *testing.T) {
	t.Helper()
	subjects, bindings, resolve := dormantSubjects, dormantBindings, dormantResolve
	status, roles, mapped := dormantSubjectStatus, dormantSubjectRoles, dormantTargetMapped
	t.Cleanup(func() {
		dormantSubjects, dormantBindings, dormantResolve = subjects, bindings, resolve
		dormantSubjectStatus, dormantSubjectRoles, dormantTargetMapped = status, roles, mapped
	})

	dormantSubjects = func(context.Context, string) addons.SubjectsResult {
		return addons.SubjectsResult{
			Outcome:  addons.OutcomeSucceeded,
			Current:  true,
			TakenAt:  time.Now(),
			Accounts: []addons.TargetAccount{{Username: "ada", UID: 3001}, {Username: "leo", UID: 3002}},
		}
	}
	dormantBindings = func(context.Context, string) ([]db.TargetBinding, error) {
		return []db.TargetBinding{
			{Target: "truenas", SubjectID: "u1", Username: "ada"},
			{Target: "truenas", SubjectID: "u2", Username: "leo"},
		}, nil
	}
	dormantResolve = func(_ context.Context, subjectID, _ string) (EntitlementSet, error) {
		if subjectID == "u1" {
			// Still entitled: a role reaches this target for them.
			return EntitlementSet{
				Fields:    map[string][]string{"group": {"lab_makers"}},
				Lifecycle: LifecycleState{Enabled: true},
			}, nil
		}
		return EntitlementSet{Fields: map[string][]string{}}, nil
	}
	dormantSubjectStatus = func(_ context.Context, subjectID string) (string, bool) {
		return "Leo Marsh", true
	}
	dormantSubjectRoles = func(context.Context, string) ([]models.DirectGrant, error) {
		return []models.DirectGrant{{ID: "g1"}}, nil
	}
	dormantTargetMapped = func(context.Context, string) (bool, error) { return true, nil }
}

// The rule: anything an active role still grants never appears here. It is
// resolved through the same resolver as every other entitlement question,
// because a second implementation of it would be a second answer — and the
// first time the two disagreed, somebody would lose an account a role confers.
func TestAnAccountAnActiveRoleStillGrantsIsNeverDormant(t *testing.T) {
	stubDormant(t)

	report, err := DormantAccounts(context.Background(), "truenas")
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Accounts) != 1 || report.Accounts[0].Account != "leo" {
		t.Fatalf("only the unentitled account is dormant, got %+v", report.Accounts)
	}
}

// A subject who has left and a subject who is still a member are the same
// dormancy and the opposite action: one is housekeeping, the other is somebody
// quietly locked out.
func TestStillBeingAMemberIsReportedAndDrivesTheCause(t *testing.T) {
	stubDormant(t)

	report, _ := DormantAccounts(context.Background(), "truenas")
	if !report.Accounts[0].SubjectStillMember {
		t.Fatal("a subject the directory still knows is still a member")
	}
	if report.Accounts[0].Reason != DormantRoleDeleted {
		t.Errorf("a member whose roles no longer reach here is %q, got %q", DormantRoleDeleted, report.Accounts[0].Reason)
	}

	dormantSubjectStatus = func(context.Context, string) (string, bool) { return "", false }
	report, _ = DormantAccounts(context.Background(), "truenas")
	if report.Accounts[0].SubjectStillMember {
		t.Error("a subject the directory cannot place is not a member")
	}
	if report.Accounts[0].Reason != DormantMembershipEnded {
		t.Errorf("want %q, got %q", DormantMembershipEnded, report.Accounts[0].Reason)
	}
}

// Failing to classify must make an account HARDER to remove, not easier:
// `role_deleted` is the cause whose rows the surface refuses to select.
func TestAnUnclassifiableAccountFallsToTheUnselectableCause(t *testing.T) {
	stubDormant(t)
	dormantSubjectRoles = func(context.Context, string) ([]models.DirectGrant, error) {
		return nil, context.DeadlineExceeded
	}

	report, _ := DormantAccounts(context.Background(), "truenas")
	if report.Accounts[0].Reason != DormantRoleDeleted {
		t.Errorf("an unknown cause must be the conservative one, got %q", report.Accounts[0].Reason)
	}
}

// Every row here is a candidate for removal, and "this account has no reason to
// exist" is a conclusion about absence. A read that did not happen, or that
// stopped at a cap, cannot support one.
func TestAnUnusableReadIsRefusedRatherThanAnsweredEmpty(t *testing.T) {
	for _, tc := range []struct {
		name string
		read addons.SubjectsResult
	}{
		{"unreachable", addons.SubjectsResult{Outcome: addons.OutcomeUnreached}},
		{"stale mirror", addons.SubjectsResult{Outcome: addons.OutcomeSucceeded, Current: false}},
		{"truncated", addons.SubjectsResult{Outcome: addons.OutcomeSucceeded, Current: true, Truncated: true}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			stubDormant(t)
			dormantSubjects = func(context.Context, string) addons.SubjectsResult { return tc.read }

			if _, err := DormantAccounts(context.Background(), "truenas"); err == nil {
				t.Fatal("want a refusal — an empty list here is a statement nobody may make")
			}
		})
	}
}

// An account bound to something that is not on the target is not dormant: there
// is nothing to remove, and the reconcile pass is what repairs the binding.
func TestABindingToAnAbsentAccountIsNotDormant(t *testing.T) {
	stubDormant(t)
	dormantSubjects = func(context.Context, string) addons.SubjectsResult {
		return addons.SubjectsResult{
			Outcome: addons.OutcomeSucceeded, Current: true, TakenAt: time.Now(),
			Accounts: []addons.TargetAccount{{Username: "ada", UID: 3001}},
		}
	}

	report, _ := DormantAccounts(context.Background(), "truenas")
	if len(report.Accounts) != 0 {
		t.Fatalf("nothing to remove is not dormant, got %+v", report.Accounts)
	}
}
