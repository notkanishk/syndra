package drift

import (
	"context"
	"testing"

	"syndra/internal/db"
	"syndra/internal/models"
	"syndra/internal/services"
	"syndra/internal/zitadel"
)

// The sweep must account for every source of intent Syndra has.
//
// This is the guard for the defect itself, rather than for one instance of it.
// The sweep classified a live grant as drift unless it recognised Syndra's
// intent behind it — and its list of recognised sources was written separately
// from the webhook's, so when bundles became a source only one side learned
// about it. Every projected bundle role in the deployment was reported as
// unexplained drift, permanently.
//
// The `switch` below has no `default`, and that is deliberate: adding a member
// to services.AllIntentSources makes this test fail to compile until somebody
// says what the sweep should do about it. A test that silently skipped unknown
// members would be the same hole one level up.
func TestSweep_AccountsForEverySourceOfIntent(t *testing.T) {
	const (
		user    = "u1"
		project = "p1"
		role    = "the-role"
	)
	k := services.HolderKey{UserID: user, ProjectID: project, RoleKey: role}

	for _, source := range services.AllIntentSources {
		t.Run(string(source), func(t *testing.T) {
			var (
				direct     []models.DirectGrant
				bundled    []db.BundleDerivedGrant
				rules      []models.MappingRule
				exclusions []models.ExternalGrantExclusion
				// The rule case needs the person to hold the rule's SOURCE, so
				// the live Zitadel read supplies that alongside the grant under
				// test.
				extraZitadel []zitadel.UserGrant
			)

			switch source {
			case services.IntentDirectGrant:
				direct = []models.DirectGrant{{UserID: user, ProjectID: project, RoleKey: role}}
			case services.IntentBundle:
				bundled = []db.BundleDerivedGrant{{UserID: user, ProjectID: project, RoleKey: role}}
			case services.IntentMappingRule:
				rules = []models.MappingRule{{
					SourceProject: "p-src", SourceRole: "src-role",
					TargetProject: project, TargetRole: role,
				}}
				extraZitadel = []zitadel.UserGrant{
					{ID: "g-src", UserID: user, ProjectID: "p-src", RoleKeys: []string{"src-role"}},
				}
			case services.IntentExclusion:
				exclusions = []models.ExternalGrantExclusion{{
					Target: db.TargetZitadel, UserID: user, ProjectID: project, RoleKey: role,
				}}
			}

			stubSweep(t)
			t.Cleanup(swap(&svcAllDirectGrants, func(context.Context) ([]models.DirectGrant, error) {
				return direct, nil
			}))
			t.Cleanup(swap(&svcAllBundleDerivedGrants, func(context.Context) ([]db.BundleDerivedGrant, error) {
				return bundled, nil
			}))
			t.Cleanup(swap(&svcGetActiveMappingRules, func(context.Context) ([]models.MappingRule, error) {
				return rules, nil
			}))
			t.Cleanup(swap(&svcGetExclusions, func(context.Context, string) ([]models.ExternalGrantExclusion, error) {
				return exclusions, nil
			}))
			t.Cleanup(swap(&allObservedGrants, func(context.Context) ([]db.ObservedGrant, error) {
				all := append([]zitadel.UserGrant{
					{ID: "g1", UserID: user, ProjectID: project, RoleKeys: []string{role}},
				}, extraZitadel...)
				out := make([]db.ObservedGrant, len(all))
				for i, g := range all {
					out[i] = db.ObservedGrant{GrantID: g.ID, UserID: g.UserID, ProjectID: g.ProjectID, RoleKeys: g.RoleKeys}
				}
				return out, nil
			}))

			var raised []string
			t.Cleanup(swap(&upsertDriftItem, func(_ context.Context, _, u, p string, roles []string, _, _, _ string, _ db.DriftEvidence) (string, bool, error) {
				if u == user && p == project && len(roles) > 0 && roles[0] == role {
					raised = append(raised, u+"/"+p+"/"+roles[0])
				}
				return "d1", true, nil
			}))

			if _, err := Sweep(context.Background()); err != nil {
				t.Fatalf("sweep: %v", err)
			}
			if len(raised) != 0 {
				t.Fatalf("the sweep does not account for %q: it raised %v as drift even though "+
					"that source explains it", source, raised)
			}

			// And the same judgement, asked directly, must agree — `explained`
			// is what both the raising and the retracting pass consult.
			holder := services.BuildHolderSet(direct, append([]zitadel.UserGrant{
				{ID: "g1", UserID: user, ProjectID: project, RoleKeys: []string{role}},
			}, extraZitadel...))
			directSet := services.BuildHolderSet(direct, nil)
			bundleSet := map[services.HolderKey]bool{}
			for _, g := range bundled {
				bundleSet[services.HolderKey{UserID: g.UserID, ProjectID: g.ProjectID, RoleKey: g.RoleKey}] = true
			}
			if !explained(k, directSet, bundleSet, holder, rules, exclusions, db.TargetZitadel) {
				t.Fatalf("explained() does not recognise %q", source)
			}
		})
	}
}

// A grant NO source accounts for is still drift. Without this, a detector that
// returned true unconditionally would pass the test above.
func TestSweep_AGrantNothingAccountsForIsStillDrift(t *testing.T) {
	stubSweep(t)
	t.Cleanup(swap(&allObservedGrants, func(context.Context) ([]db.ObservedGrant, error) {
		return []db.ObservedGrant{observedGrant("g1", "u1", "p1", "the-role")}, nil
	}))

	raised := 0
	t.Cleanup(swap(&upsertDriftItem, func(context.Context, string, string, string, []string, string, string, string, db.DriftEvidence) (string, bool, error) {
		raised++
		return "d1", true, nil
	}))

	if _, err := Sweep(context.Background()); err != nil {
		t.Fatalf("sweep: %v", err)
	}
	if raised != 1 {
		t.Fatalf("an unexplained grant must be raised exactly once, got %d", raised)
	}
}
