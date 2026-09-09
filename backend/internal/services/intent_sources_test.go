package services

import (
	"context"
	"testing"

	"syndra/internal/models"
)

// UserExpectsRole must account for every source of intent that is its business.
//
// The sibling of the sweep's own exhaustiveness test, and together they are the
// guard for the defect rather than for one instance of it. Two detectors answer
// "does Syndra expect this grant?" — this one for the webhook, the sweep's
// `explained` for reconciliation — and they were written separately. Bundles
// were added to Syndra's model, this side learned about them, the sweep did
// not, and every projected bundle role became permanent false drift.
//
// The `switch` has no `default`: adding a member to AllIntentSources makes this
// fail to compile until somebody decides what this detector does about it.
//
// IntentExclusion is deliberately NOT this function's business, and saying so
// here is the point. An exclusion is an operator's statement about one TARGET
// ("this grant is legitimately external on Zitadel"), and UserExpectsRole
// answers a target-independent question about Syndra's own intent. The webhook
// path applies exclusions separately. If that ever changes, this case is where
// the change gets noticed.
func TestUserExpectsRole_AccountsForEverySourceOfIntent(t *testing.T) {
	const (
		user    = "u1"
		project = "p1"
		role    = "the-role"
	)

	for _, source := range AllIntentSources {
		t.Run(string(source), func(t *testing.T) {
			var (
				direct      []models.DirectGrant
				bundles     []models.Bundle
				bundleRoles map[string][]models.BundleRole
				rules       []models.MappingRule
				// For the rule case the person must hold the rule's source, and
				// here that has to come from Syndra's own reads.
				expectAccounted = true
			)

			switch source {
			case IntentDirectGrant:
				direct = []models.DirectGrant{{UserID: user, ProjectID: project, RoleKey: role}}
			case IntentBundle:
				bundles = []models.Bundle{{ID: "b1", Name: "Admin Ops", PinnedVersion: 2}}
				bundleRoles = map[string][]models.BundleRole{
					"b1": {{BundleID: "b1", ProjectID: project, RoleKey: role}},
				}
			case IntentMappingRule:
				direct = []models.DirectGrant{{UserID: user, ProjectID: "p-src", RoleKey: "src-role"}}
				rules = []models.MappingRule{{
					SourceProject: "p-src", SourceRole: "src-role",
					TargetProject: project, TargetRole: role,
				}}
			case IntentExclusion:
				// Not this detector's question — see the doc comment. Asserting
				// the negative keeps the omission deliberate rather than
				// forgotten.
				expectAccounted = false
			}

			restore := stubExpectationReads(t, direct, bundles, bundleRoles, rules)
			defer restore()

			got, err := UserExpectsRole(context.Background(), user, project, role)
			if err != nil {
				t.Fatalf("UserExpectsRole: %v", err)
			}
			if got != expectAccounted {
				t.Fatalf("UserExpectsRole accounted=%v for source %q, want %v — this detector "+
					"has not been taught about it", got, source, expectAccounted)
			}
		})
	}
}

// A role no source accounts for is not expected. Without this, a detector that
// returned true unconditionally would satisfy the test above.
func TestUserExpectsRole_AccountsForNothingWhenNothingGivesIt(t *testing.T) {
	restore := stubExpectationReads(t, nil, nil, nil, nil)
	defer restore()

	got, err := UserExpectsRole(context.Background(), "u1", "p1", "the-role")
	if err != nil {
		t.Fatalf("UserExpectsRole: %v", err)
	}
	if got {
		t.Fatal("a role nothing gives must not be reported as expected")
	}
}

// stubExpectationReads points collectUserRoles' reads at fixtures.
func stubExpectationReads(
	t *testing.T,
	direct []models.DirectGrant,
	bundles []models.Bundle,
	bundleRoles map[string][]models.BundleRole,
	rules []models.MappingRule,
) func() {
	t.Helper()

	origDirect := svcGetDirectGrantsForUser
	origBundles := svcGetBundlesForUser
	origGrouped := svcGetUserBundleRolesGrouped
	origRules := svcGetActiveMappingRules

	svcGetDirectGrantsForUser = func(context.Context, string, bool) ([]models.DirectGrant, error) {
		return direct, nil
	}
	svcGetBundlesForUser = func(context.Context, string) ([]models.Bundle, error) {
		return bundles, nil
	}
	svcGetUserBundleRolesGrouped = func(context.Context, string) (map[string][]models.BundleRole, error) {
		return bundleRoles, nil
	}
	svcGetActiveMappingRules = func(context.Context) ([]models.MappingRule, error) {
		return rules, nil
	}

	return func() {
		svcGetDirectGrantsForUser = origDirect
		svcGetBundlesForUser = origBundles
		svcGetUserBundleRolesGrouped = origGrouped
		svcGetActiveMappingRules = origRules
	}
}
