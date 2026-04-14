package services

import "testing"

func TestFlattenLLDAPGroup_BasicCase(t *testing.T) {
	got := FlattenLLDAPGroup("printing", "member")
	if got != "printing_member" {
		t.Errorf("expected printing_member, got %s", got)
	}
}

func TestFlattenLLDAPGroup_AlreadyLowercase(t *testing.T) {
	got := FlattenLLDAPGroup("doors", "3d_lab_pin")
	if got != "doors_3d_lab_pin" {
		t.Errorf("expected doors_3d_lab_pin, got %s", got)
	}
}

func TestFlattenLLDAPGroup_MixedCaseProjectID(t *testing.T) {
	// Project IDs should be lowercased for safety even if they arrive mixed-case.
	got := FlattenLLDAPGroup("Platform", "Admin")
	if got != "platform_admin" {
		t.Errorf("expected platform_admin, got %s", got)
	}
}

func TestFlattenLLDAPGroup_UnderscoresPreserved(t *testing.T) {
	got := FlattenLLDAPGroup("samba", "share_admin")
	if got != "samba_share_admin" {
		t.Errorf("expected samba_share_admin, got %s", got)
	}
}

func TestFlattenLLDAPGroup_ImmutableKeyStability(t *testing.T) {
	// Two different project IDs must produce different groups even if display names
	// would collide. This is the P1 fix — using stable IDs, not mutable names.
	g1 := FlattenLLDAPGroup("proj_a", "admin")
	g2 := FlattenLLDAPGroup("proj_b", "admin")
	if g1 == g2 {
		t.Error("different project IDs must produce different LLDAP groups")
	}
}
