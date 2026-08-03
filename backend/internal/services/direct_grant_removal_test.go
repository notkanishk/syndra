package services

import (
	"context"
	"testing"

	"mkauth/internal/db"
	"mkauth/internal/models"
)

// Removing a direct grant must enqueue the EFFECTIVE-ACCESS delta, not an
// unconditional revoke.
//
// The confirmation dialog promises "they will still hold this role via Lab
// Tech" when another source covers it. An unconditional revoke made that
// promise false: the queued write removed the role from the identity provider
// anyway, and the person lost access until the next compile put it back.

// grantRemovalFixture wires the closure-diff injectables and captures whatever
// the removal enqueues.
func grantRemovalFixture(
	t *testing.T,
	directs []models.DirectGrant,
	bundles []models.Bundle,
	bundleRoles map[string][]models.BundleRole,
	rules []models.MappingRule,
) *[]db.EnqueueParams {
	t.Helper()
	resetCascadeDeps(t)

	origDelete := svcDeleteDirectGrantAndEnqueue
	t.Cleanup(func() { svcDeleteDirectGrantAndEnqueue = origDelete })

	svcGetDirectGrantsForUser = func(context.Context, string, bool) ([]models.DirectGrant, error) {
		return directs, nil
	}
	svcGetBundlesForUser = func(context.Context, string) ([]models.Bundle, error) {
		return bundles, nil
	}
	// Keyed by bundle id, exactly as the version-aware lookup returns it: what
	// this person gets from each bundle, through the version they are pinned to.
	svcGetUserBundleRolesGrouped = func(context.Context, string) (map[string][]models.BundleRole, error) {
		return bundleRoles, nil
	}
	svcGetActiveMappingRules = func(context.Context) ([]models.MappingRule, error) {
		return rules, nil
	}

	captured := &[]db.EnqueueParams{}
	svcDeleteDirectGrantAndEnqueue = func(
		_ context.Context, _, _, _ string, params []db.EnqueueParams,
	) ([]string, error) {
		*captured = params
		ids := make([]string, len(params))
		for i := range params {
			ids[i] = "ob"
		}
		return ids, nil
	}
	return captured
}

func revokedRoles(params []db.EnqueueParams) []string {
	out := []string{}
	for _, p := range params {
		if p.OpType == "revoke" {
			for _, r := range p.RoleKeys {
				out = append(out, p.ProjectID+"/"+r)
			}
		}
	}
	return out
}

// The regression this file exists for.
func TestDeleteDirectGrant_RoleAlsoInBundle_EnqueuesNoRevoke(t *testing.T) {
	captured := grantRemovalFixture(t,
		[]models.DirectGrant{{ID: "g_88", UserID: "u1", ProjectID: "pLaser", RoleKey: "trained"}},
		[]models.Bundle{{ID: "b_lab", Name: "Lab Tech"}},
		map[string][]models.BundleRole{
			"b_lab": {{ProjectID: "pLaser", RoleKey: "trained"}},
		},
		nil,
	)

	res, err := DeleteDirectGrant(context.Background(), "u1", "g_88", "priya")
	if err != nil {
		t.Fatalf("DeleteDirectGrant: %v", err)
	}

	if got := revokedRoles(*captured); len(got) != 0 {
		t.Fatalf("the bundle still carries pLaser/trained; nothing may be revoked, got %v", got)
	}
	if len(res.OutboxIDs) != 0 {
		t.Errorf("no write should be queued at all, got %v", res.OutboxIDs)
	}
	// And the response must say so, because that is the dialog's promise.
	if len(res.Retained) != 1 || res.Retained[0] != "pLaser/trained" {
		t.Errorf("expected pLaser/trained reported as retained, got %v", res.Retained)
	}
}

func TestDeleteDirectGrant_RoleAlsoFromRule_EnqueuesNoRevoke(t *testing.T) {
	// The person holds 3D Lab / operator, and a rule turns that into
	// Laser Lab / trained. Removing the direct grant of trained changes nothing.
	captured := grantRemovalFixture(t,
		[]models.DirectGrant{
			{ID: "g_88", UserID: "u1", ProjectID: "pLaser", RoleKey: "trained"},
			{ID: "g_12", UserID: "u1", ProjectID: "p3D", RoleKey: "operator"},
		},
		nil, nil,
		[]models.MappingRule{{
			SourceProject: "p3D", SourceRole: "operator",
			TargetProject: "pLaser", TargetRole: "trained",
		}},
	)

	res, err := DeleteDirectGrant(context.Background(), "u1", "g_88", "priya")
	if err != nil {
		t.Fatalf("DeleteDirectGrant: %v", err)
	}

	if got := revokedRoles(*captured); len(got) != 0 {
		t.Fatalf("rule R still produces pLaser/trained; nothing may be revoked, got %v", got)
	}
	if len(res.Retained) != 1 || res.Retained[0] != "pLaser/trained" {
		t.Errorf("expected pLaser/trained reported as retained, got %v", res.Retained)
	}
}

func TestDeleteDirectGrant_OnlySource_EnqueuesTheRevoke(t *testing.T) {
	captured := grantRemovalFixture(t,
		[]models.DirectGrant{{ID: "g_88", UserID: "u1", ProjectID: "pLaser", RoleKey: "trained"}},
		nil, nil, nil,
	)

	res, err := DeleteDirectGrant(context.Background(), "u1", "g_88", "priya")
	if err != nil {
		t.Fatalf("DeleteDirectGrant: %v", err)
	}

	got := revokedRoles(*captured)
	if len(got) != 1 || got[0] != "pLaser/trained" {
		t.Fatalf("the last source is gone; expected one revoke, got %v", got)
	}
	if len(res.Retained) != 0 {
		t.Errorf("nothing is retained here, got %v", res.Retained)
	}
	if len(res.Revoked) != 1 || res.Revoked[0] != "pLaser/trained" {
		t.Errorf("expected the loss reported, got %v", res.Revoked)
	}
}

// A rule-derived role this grant alone supported also falls out of the closure,
// so it must be revoked too — the same reasoning the bundle cascade uses.
func TestDeleteDirectGrant_RevokesRolesTheGrantAloneSupported(t *testing.T) {
	captured := grantRemovalFixture(t,
		[]models.DirectGrant{{ID: "g_88", UserID: "u1", ProjectID: "pStudio", RoleKey: "door"}},
		nil, nil,
		[]models.MappingRule{{
			SourceProject: "pStudio", SourceRole: "door",
			TargetProject: "pWiki", TargetRole: "wiki-read",
		}},
	)

	if _, err := DeleteDirectGrant(context.Background(), "u1", "g_88", "priya"); err != nil {
		t.Fatalf("DeleteDirectGrant: %v", err)
	}

	got := revokedRoles(*captured)
	if len(got) != 2 {
		t.Fatalf("expected the grant and its derived role revoked, got %v", got)
	}
	found := map[string]bool{}
	for _, r := range got {
		found[r] = true
	}
	if !found["pStudio/door"] || !found["pWiki/wiki-read"] {
		t.Errorf("expected both pStudio/door and pWiki/wiki-read, got %v", got)
	}
}

// Every enqueued row must be attributable: source `direct`, the grant id as the
// reference, and the operator who clicked.
func TestDeleteDirectGrant_AttributesEveryQueuedRow(t *testing.T) {
	captured := grantRemovalFixture(t,
		[]models.DirectGrant{{ID: "g_88", UserID: "u1", ProjectID: "pLaser", RoleKey: "trained"}},
		nil, nil, nil,
	)

	if _, err := DeleteDirectGrant(context.Background(), "u1", "g_88", "priya"); err != nil {
		t.Fatalf("DeleteDirectGrant: %v", err)
	}

	if len(*captured) == 0 {
		t.Fatal("expected at least one queued row")
	}
	for _, p := range *captured {
		if p.Source != "direct" || p.SourceRef != "g_88" || p.GrantedBy != "priya" {
			t.Errorf("unattributable row: %#v", p)
		}
		if p.OpType != "revoke" {
			t.Errorf("a removal must only ever queue revokes, got %q", p.OpType)
		}
	}
}

// The other end of the same fact, and the P1 it caused.
//
// A direct removal's writes must not be cascade-group-visible. Its audit row is stamped with a
// cascade id only when they are, and a stamp puts a trace link into the audit log — pointing at
// Change history, which filters source='direct' out. A real pending revoke rendered as an empty
// page saying its writes had been carried out and cleared.
//
// Asserted here, against db.IsCascadeGroupSource, rather than in either package alone: the params
// are built in services and read in db, and the bug lived in the gap.
//
// This also covers the rule-derived revoke. Removing a direct grant CAN revoke a second role a
// mapping rule derived from it, which is a cascade in every sense but the one that matters here —
// all of it is attributed to the grant, and the grant is the operator's own write.
func TestDeleteDirectGrant_QueuesNothingChangeHistoryWouldGroup(t *testing.T) {
	captured := grantRemovalFixture(t,
		[]models.DirectGrant{{ID: "g_88", UserID: "u1", ProjectID: "pStudio", RoleKey: "door"}},
		nil, nil,
		[]models.MappingRule{{
			SourceProject: "pStudio", SourceRole: "door",
			TargetProject: "pWiki", TargetRole: "wiki-read",
		}},
	)

	if _, err := DeleteDirectGrant(context.Background(), "u1", "g_88", "priya"); err != nil {
		t.Fatalf("DeleteDirectGrant: %v", err)
	}
	if len(*captured) == 0 {
		t.Fatal("expected queued revokes, or this test proves nothing")
	}

	for _, p := range *captured {
		if db.IsCascadeGroupSource(p.Source) {
			t.Errorf("source %q appears in Change history, so this removal's audit row would be "+
				"stamped with a cascade id and linked to a page that excludes the write: %#v",
				p.Source, p)
		}
	}
}

// An expired grant is already out of the effective set, so deleting its row
// changes nothing upstream — the sweep did the revoking.
func TestDeleteDirectGrant_ExpiredGrant_EnqueuesNothing(t *testing.T) {
	captured := grantRemovalFixture(t, nil, nil, nil, nil)

	if _, err := DeleteDirectGrant(context.Background(), "u1", "g_expired", "priya"); err != nil {
		t.Fatalf("DeleteDirectGrant: %v", err)
	}
	if len(*captured) != 0 {
		t.Fatalf("expected no queued writes, got %#v", *captured)
	}
}
