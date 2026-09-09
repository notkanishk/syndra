package zitadel

import (
	"context"
	"testing"

	"syndra/internal/models"
)

// A rule losing its source must not take away access another source still gives.
//
// `RevokeMappingRules` runs when a source role is removed, finds the grant the
// rule derived, and deletes it. It asked one question — does the target grant
// exist upstream — and never the one that matters: does anything ELSE still
// give this person that role.
//
// So somebody holding laser/operator from a bundle AND from the rule
// training:certified → laser:operator lost it from Zitadel the moment an
// operator removed training:certified by hand. Nothing repaired it. No outbox
// row was written, the ledger did not change, and the sweep's syndra_only half
// does not conclude absence for bundle-derived roles, so no finding was raised
// either. Access destroyed silently by a routine edit.
//
// The closure path has always had this check — services/cascade.go states it as
// "a role still covered by another source stays in `after` and is never
// revoked". This is the pre-closure path that never got it.

func stubRevokeWorld(t *testing.T, rules []models.MappingRule, grants []UserGrant) *fakeMgmt {
	t.Helper()

	origRules := dbGetActiveMappingRules
	origClient := MgmtClient
	origExpected := StillExpected
	t.Cleanup(func() {
		dbGetActiveMappingRules = origRules
		MgmtClient = origClient
		StillExpected = origExpected
	})

	dbGetActiveMappingRules = func(context.Context) ([]models.MappingRule, error) { return rules, nil }
	fake := &fakeMgmt{grants: grants}
	MgmtClient = fake
	return fake
}

func TestRevokeMappingRules_KeepsARoleAnotherSourceStillGives(t *testing.T) {
	rules := []models.MappingRule{{
		SourceProject: "p-training", SourceRole: "certified",
		TargetProject: "p-laser", TargetRole: "operator",
	}}
	fake := stubRevokeWorld(t, rules, []UserGrant{
		{ID: "g-laser", UserID: "u1", ProjectID: "p-laser", RoleKeys: []string{"operator"}},
	})

	// A bundle still gives laser/operator.
	StillExpected = func(context.Context, string, string, string) (bool, error) { return true, nil }

	if err := RevokeMappingRules(context.Background(), "u1", "p-training", "certified"); err != nil {
		t.Fatalf("RevokeMappingRules: %v", err)
	}
	if len(fake.removed) != 0 || len(fake.updated) != 0 {
		t.Fatalf("revoked a role another source still gives: removed=%v updated=%v",
			fake.removed, fake.updated)
	}
}

func TestRevokeMappingRules_RevokesWhenNothingElseGivesIt(t *testing.T) {
	rules := []models.MappingRule{{
		SourceProject: "p-training", SourceRole: "certified",
		TargetProject: "p-laser", TargetRole: "operator",
	}}
	fake := stubRevokeWorld(t, rules, []UserGrant{
		{ID: "g-laser", UserID: "u1", ProjectID: "p-laser", RoleKeys: []string{"operator"}},
	})

	StillExpected = func(context.Context, string, string, string) (bool, error) { return false, nil }

	if err := RevokeMappingRules(context.Background(), "u1", "p-training", "certified"); err != nil {
		t.Fatalf("RevokeMappingRules: %v", err)
	}
	if len(fake.removed) != 1 {
		t.Fatalf("the rule was the only source, so the grant must go: removed=%v", fake.removed)
	}
}

// Unwired and unreadable both skip. The two mistakes are not equal: a
// revocation withheld leaves a grant the sweep raises for a human, and one
// performed wrongly leaves nothing at all.
func TestRevokeMappingRules_SkipsWhenItCannotCheckCoverage(t *testing.T) {
	rules := []models.MappingRule{{
		SourceProject: "p-training", SourceRole: "certified",
		TargetProject: "p-laser", TargetRole: "operator",
	}}

	t.Run("unwired", func(t *testing.T) {
		fake := stubRevokeWorld(t, rules, []UserGrant{
			{ID: "g-laser", UserID: "u1", ProjectID: "p-laser", RoleKeys: []string{"operator"}},
		})
		StillExpected = nil

		if err := RevokeMappingRules(context.Background(), "u1", "p-training", "certified"); err != nil {
			t.Fatalf("RevokeMappingRules: %v", err)
		}
		if len(fake.removed) != 0 {
			t.Fatal("revoked without being able to check coverage")
		}
	})

	t.Run("read failed", func(t *testing.T) {
		fake := stubRevokeWorld(t, rules, []UserGrant{
			{ID: "g-laser", UserID: "u1", ProjectID: "p-laser", RoleKeys: []string{"operator"}},
		})
		StillExpected = func(context.Context, string, string, string) (bool, error) {
			return false, context.DeadlineExceeded
		}

		if err := RevokeMappingRules(context.Background(), "u1", "p-training", "certified"); err != nil {
			t.Fatalf("RevokeMappingRules: %v", err)
		}
		if len(fake.removed) != 0 {
			t.Fatal("revoked on an unreadable coverage check")
		}
	})
}

// fakeMgmt implements only the three calls this path makes. The interface is
// embedded so any OTHER call panics loudly rather than returning a zero value
// that would let a test pass by accident.
type fakeMgmt struct {
	ZitadelClient
	grants  []UserGrant
	removed []string
	updated []string
}

func (f *fakeMgmt) ListUserGrants(_ context.Context, _ string, _ SearchParams) (*SearchResult[UserGrant], error) {
	return &SearchResult[UserGrant]{Items: f.grants, Total: len(f.grants)}, nil
}

func (f *fakeMgmt) RemoveUserGrant(_ context.Context, _, grantID string) error {
	f.removed = append(f.removed, grantID)
	return nil
}

func (f *fakeMgmt) UpdateUserGrant(_ context.Context, _, grantID string, _ []string) error {
	f.updated = append(f.updated, grantID)
	return nil
}
