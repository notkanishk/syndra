package services

import (
	"testing"

	"syndra/internal/models"
)

// What a publish was approved against, digested. Every field in here is a fact
// an operator read; a change to any of them means the approval describes
// something else, and the apply must be refused rather than quietly widened.

func roles(pairs ...[2]string) []models.BundleRole {
	out := make([]models.BundleRole, 0, len(pairs))
	for _, p := range pairs {
		out = append(out, models.BundleRole{ProjectID: p[0], RoleKey: p[1]})
	}
	return out
}

func holders(pairs ...[2]any) []models.BundleHolder {
	out := make([]models.BundleHolder, 0, len(pairs))
	for _, p := range pairs {
		out = append(out, models.BundleHolder{UserID: p[0].(string), Version: p[1].(int)})
	}
	return out
}

func TestFingerprintBundlePublish_IsBlindToReadOrder(t *testing.T) {
	a := FingerprintBundlePublish("b1", true,
		roles([2]string{"p1", "laser"}, [2]string{"p2", "cnc"}),
		holders([2]any{"u1", 1}, [2]any{"u2", 2}))
	b := FingerprintBundlePublish("b1", true,
		roles([2]string{"p2", "cnc"}, [2]string{"p1", "laser"}),
		holders([2]any{"u2", 2}, [2]any{"u1", 1}))

	if a != b {
		t.Fatal("a change in row order is not a change in state")
	}
}

// The case per-subject verification cannot see: u1's own row is identical, and
// somebody new now holds the bundle and would be moved under an approval that
// never mentioned them.
func TestFingerprintBundlePublish_ACohortThatGrewIsADifferentRequest(t *testing.T) {
	one := FingerprintBundlePublish("b1", true, roles([2]string{"p1", "laser"}), holders([2]any{"u1", 1}))
	two := FingerprintBundlePublish("b1", true, roles([2]string{"p1", "laser"}),
		holders([2]any{"u1", 1}, [2]any{"u2", 1}))

	if one == two {
		t.Fatal("a holder joining between the review and the apply must invalidate the approval")
	}
}

// Somebody moved from v2 to v4 by another operator is standing somewhere else,
// so the row that said "v2 → v3" is describing a move that no longer exists.
func TestFingerprintBundlePublish_AHolderOnADifferentVersionIsADifferentCohort(t *testing.T) {
	before := FingerprintBundlePublish("b1", true, roles([2]string{"p1", "laser"}), holders([2]any{"u1", 2}))
	after := FingerprintBundlePublish("b1", true, roles([2]string{"p1", "laser"}), holders([2]any{"u1", 4}))

	if before == after {
		t.Fatal("the version a holder stands on is part of what was reviewed")
	}
}

// A working-copy edit can change what v_next contains without moving anybody:
// adding a role every holder already has from a rule leaves every delta
// identical. The version being published is still not the one that was approved.
func TestFingerprintBundlePublish_ChangedContentsInvalidateEvenWhenNobodyMoves(t *testing.T) {
	before := FingerprintBundlePublish("b1", true, roles([2]string{"p1", "laser"}), holders([2]any{"u1", 1}))
	after := FingerprintBundlePublish("b1", true,
		roles([2]string{"p1", "laser"}, [2]string{"p1", "already-held"}), holders([2]any{"u1", 1}))

	if before == after {
		t.Fatal("what the new version contains is part of what was approved")
	}
}

// Both answers to "do the holders come along" are legitimate, and they are
// different acts. An approval for one must not spend on the other.
func TestFingerprintBundlePublish_MigrateIsPartOfTheRequest(t *testing.T) {
	move := FingerprintBundlePublish("b1", true, roles([2]string{"p1", "laser"}), holders([2]any{"u1", 1}))
	leave := FingerprintBundlePublish("b1", false, roles([2]string{"p1", "laser"}), holders([2]any{"u1", 1}))

	if move == leave {
		t.Fatal("moving fourteen people and leaving them alone cannot share an approval")
	}
}

func TestFingerprintMoveHolders_BindsTheVersionAndTheNamedPeople(t *testing.T) {
	base := FingerprintMoveHolders("b1", "v2", roles([2]string{"p1", "laser"}), []string{"u1", "u2"})

	if same := FingerprintMoveHolders("b1", "v2", roles([2]string{"p1", "laser"}), []string{"u2", "u1"}); same != base {
		t.Fatal("the order the people were named in is not part of the request")
	}
	if other := FingerprintMoveHolders("b1", "v3", roles([2]string{"p1", "laser"}), []string{"u1", "u2"}); other == base {
		t.Fatal("a different target version is a different act")
	}
	if fewer := FingerprintMoveHolders("b1", "v2", roles([2]string{"p1", "laser"}), []string{"u1"}); fewer == base {
		t.Fatal("a different set of people is a different act")
	}
}
