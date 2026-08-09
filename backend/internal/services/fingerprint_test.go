package services

import (
	"testing"
	"time"

	"syndra/internal/models"
)

// 3.2 — the fingerprint covers the object that was REVIEWED, not merely the
// subject's identity. Each case below is a change an operator would want an
// apply to refuse, and a fingerprint blind to it is a plan that verifies
// vacuously.

func TestUserAccessFingerprintMovesWithEveryReviewedFact(t *testing.T) {
	base := func() (string, string, []string, []models.DirectGrant) {
		expires := time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)
		return "u1", "active",
			[]string{"pLaser/trained", "pPrint/member"},
			[]models.DirectGrant{{ID: "g1", ProjectID: "pLaser", RoleKey: "trained", Source: "direct", ExpiresAt: &expires}}
	}
	uid, status, roles, grants := base()
	original := FingerprintUserAccess(uid, status, roles, grants)

	later := time.Date(2027, 9, 1, 12, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		name string
		got  string
		why  string
	}{
		{"a role gained elsewhere",
			FingerprintUserAccess(uid, status, append(append([]string{}, roles...), "pDoor/entry"), grants),
			"a bundle landing between review and apply changes whether a removal leaves them holding the role"},
		{"a role lost elsewhere",
			FingerprintUserAccess(uid, status, roles[:1], grants),
			"the same, in the direction that makes a 'no change' row actionable"},
		{"the grant replaced by an identical-looking one",
			FingerprintUserAccess(uid, status, roles, []models.DirectGrant{{ID: "g2", ProjectID: "pLaser", RoleKey: "trained", Source: "direct", ExpiresAt: grants[0].ExpiresAt}}),
			"GrantIDs on the plan row is what the apply mutates, so an id that moved is a row nobody reviewed"},
		{"the grant re-sourced",
			FingerprintUserAccess(uid, status, roles, []models.DirectGrant{{ID: "g1", ProjectID: "pLaser", RoleKey: "trained", Source: "bundle", ExpiresAt: grants[0].ExpiresAt}}),
			"provenance decides what a removal actually removes"},
		{"the grant renewed",
			FingerprintUserAccess(uid, status, roles, []models.DirectGrant{{ID: "g1", ProjectID: "pLaser", RoleKey: "trained", Source: "direct", ExpiresAt: &later}}),
			"the reviewed fact for extend is the date, and the same row with a later one is a different approval"},
		{"the account departed",
			FingerprintUserAccess(uid, "departed", roles, grants),
			"granting to somebody who has left is the commonest way a bulk selection goes wrong"},
		{"a different person entirely",
			FingerprintUserAccess("u2", status, roles, grants),
			"a fingerprint that ignores the subject verifies one person's state against another's row"},
	} {
		if tc.got == original {
			t.Errorf("%s must invalidate the approval — %s", tc.name, tc.why)
		}
	}

	// Stability, in both directions that matter: read order is not state, and
	// re-reading an unchanged world must still verify.
	shuffled := FingerprintUserAccess(uid, status, []string{"pPrint/member", "pLaser/trained"}, grants)
	if shuffled != original {
		t.Error("a change in read order is not a change in state")
	}
}

func TestDriftFingerprintCarriesTheRowsOwnStatus(t *testing.T) {
	item := models.DriftItem{
		ID: "d1", Target: "zitadel", Status: "pending_triage",
		UserID: "u1", ProjectID: "p1", RoleKeys: []string{"viewer", "editor"},
	}
	pending := FingerprintDriftItem(item)

	resolved := item
	resolved.Status = "marked_external"
	if FingerprintDriftItem(resolved) == pending {
		// The concrete case design §8 names: a shared queue, a second operator
		// resolving the row first. No fingerprint over the access alone can see
		// it — the grants have not moved and neither has the person.
		t.Fatal("a row somebody else resolved must fail verification")
	}

	onAnotherTarget := item
	onAnotherTarget.Target = "truenas"
	if FingerprintDriftItem(onAnotherTarget) == pending {
		t.Error("a role key means nothing without the target whose catalogue it belongs to")
	}

	reordered := item
	reordered.RoleKeys = []string{"editor", "viewer"}
	if FingerprintDriftItem(reordered) != pending {
		t.Error("role order is not state")
	}
}

func TestRequestFingerprintCarriesStatusAndDuration(t *testing.T) {
	thirty, ninety := 30, 90
	req := models.AccessRequest{ID: "r1", Status: "pending", RequesterID: "u1", ProjectID: "p", RoleKey: "trained", DurationDays: &thirty}
	pending := FingerprintAccessRequest(req)

	decided := req
	decided.Status = "approved"
	if FingerprintAccessRequest(decided) == pending {
		t.Error("a request a second reviewer already decided must fail verification")
	}

	longer := req
	longer.DurationDays = &ninety
	if FingerprintAccessRequest(longer) == pending {
		t.Error("approving mints a grant for the duration, so an edited window is an approval nobody gave")
	}
}

// Only what changes the EFFECT is bound. An annotation an operator writes at
// apply time must not cost a re-plan, or the stale-plan dialog becomes
// something to click through.
func TestRequestBindingCoversTheDiffAndNotTheAnnotation(t *testing.T) {
	base := BulkRequest{Op: BulkOpAssignRole, UserIDs: []string{"u1", "u2"}, ProjectID: "p", RoleKey: "r", Reason: "cohort", DurationDays: 30}
	original := FingerprintBulkRequest(base)

	for _, tc := range []struct {
		name string
		edit func(*BulkRequest)
	}{
		{"a wider cohort", func(r *BulkRequest) { r.UserIDs = append(r.UserIDs, "u3") }},
		{"a narrower cohort", func(r *BulkRequest) { r.UserIDs = r.UserIDs[:1] }},
		{"a different role", func(r *BulkRequest) { r.RoleKey = "admin" }},
		{"a different project", func(r *BulkRequest) { r.ProjectID = "pOther" }},
		{"a different operation", func(r *BulkRequest) { r.Op = BulkOpRemoveRole }},
		{"a longer window", func(r *BulkRequest) { r.DurationDays = 3650 }},
		{"a permanent grant", func(r *BulkRequest) { r.DurationDays = 0 }},
		{"a different bundle", func(r *BulkRequest) { r.BundleID = "b9" }},
		{"different ticked grants", func(r *BulkRequest) { r.GrantIDs = []string{"g1"} }},
	} {
		edited := base
		edited.UserIDs = append([]string(nil), base.UserIDs...)
		tc.edit(&edited)
		if FingerprintBulkRequest(edited) == original {
			t.Errorf("%s must not be applicable under the original approval", tc.name)
		}
	}

	annotated := base
	annotated.Reason = "corrected typo"
	if FingerprintBulkRequest(annotated) != original {
		t.Error("the reason is recorded beside the write and changes nothing about who gets what")
	}

	reordered := base
	reordered.UserIDs = []string{"u2", "u1"}
	if FingerprintBulkRequest(reordered) != original {
		t.Error("selection order is not selection")
	}
}

// The encoding is injective. Joined on a separator, two different field lists
// collide on a boundary an attacker picks rather than finds.
func TestFingerprintFieldsCannotBeReassociated(t *testing.T) {
	if Fingerprint("ab", "c") == Fingerprint("a", "bc") {
		t.Fatal("field boundaries must be part of what is hashed")
	}
	if Fingerprint("a:b") == Fingerprint("a", "b") {
		t.Fatal("a separator appearing inside a value must not split it")
	}
	if Fingerprint("a", "") == Fingerprint("a") {
		t.Fatal("an empty field is a field")
	}
}

func TestIDCohortBindingIgnoresOrderAndBlanksButNotParameters(t *testing.T) {
	base := FingerprintIDCohort("adopt", []string{"d1", "d2"}, "source", "external_backfill")

	if FingerprintIDCohort("adopt", []string{" d2 ", "d1", "", "d1"}, "source", "external_backfill") != base {
		t.Error("order, padding, blanks and duplicates are not the cohort")
	}
	if FingerprintIDCohort("adopt", []string{"d1", "d2", "d3"}, "source", "external_backfill") == base {
		t.Error("a widened cohort must not be applicable under the original approval")
	}
	if FingerprintIDCohort("adopt", []string{"d1", "d2"}, "source", "direct") == base {
		t.Error("a parameter that changes what the operation does must be bound")
	}
	if FingerprintIDCohort("mark_external", []string{"d1", "d2"}, "source", "external_backfill") == base {
		t.Error("two operations over one cohort are two different approvals")
	}
}
