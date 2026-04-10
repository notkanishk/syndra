package services

import (
	"reflect"
	"testing"

	"mkauth/internal/models"
)

func TestFormatRolesContract(t *testing.T) {
	roles := []string{"admin", "viewer"}

	gotDefault := formatRoles(roles, "array")
	defaultRoles, ok := gotDefault.([]string)
	if !ok {
		t.Fatalf("expected []string for default format, got %T", gotDefault)
	}
	if !reflect.DeepEqual(defaultRoles, roles) {
		t.Fatalf("default format mismatch: got %#v want %#v", defaultRoles, roles)
	}

	gotCSV := formatRoles(roles, "csv")
	if gotCSV != "admin,viewer" {
		t.Fatalf("csv mismatch: got %v", gotCSV)
	}

	gotSpace := formatRoles(roles, "space_delimited")
	if gotSpace != "admin viewer" {
		t.Fatalf("space_delimited mismatch: got %v", gotSpace)
	}
}

func TestReadRolesFromClaims(t *testing.T) {
	input := []interface{}{"admin", 12, "viewer", true}
	got := readRoles(input)
	want := []string{"admin", "viewer"}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("readRoles mismatch: got %#v want %#v", got, want)
	}
}

func TestUpsertRoleDeduplicatesReason(t *testing.T) {
	roleMap := map[roleKey]*models.EffectiveRole{}
	key := roleKey{projectID: "p1", roleKey: "admin"}
	reason := models.RoleReason{
		Kind:        "bundle",
		Description: "Granted by bundle Engineering",
		BundleID:    "b1",
		BundleName:  "Engineering",
	}

	firstAdded := upsertRole(roleMap, key, true, reason)
	secondAdded := upsertRole(roleMap, key, true, reason)

	if !firstAdded {
		t.Fatalf("expected first insert to add role")
	}
	if secondAdded {
		t.Fatalf("expected duplicate reason to be ignored")
	}

	current := roleMap[key]
	if current == nil {
		t.Fatalf("expected role to exist after insert")
	}
	if len(current.Reasons) != 1 {
		t.Fatalf("expected exactly one reason, got %d", len(current.Reasons))
	}
	if !current.IsSource {
		t.Fatalf("expected source flag to remain true")
	}
}
