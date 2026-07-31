package claims

import (
	"reflect"
	"testing"
)

func TestFormatRoles(t *testing.T) {
	roles := []string{"admin", "viewer"}

	got := FormatRoles(roles, FormatArray)
	list, ok := got.([]string)
	if !ok || !reflect.DeepEqual(list, roles) {
		t.Fatalf("array format: got %#v", got)
	}

	if got := FormatRoles(roles, FormatCSV); got != "admin,viewer" {
		t.Fatalf("csv format: got %v", got)
	}
	if got := FormatRoles(roles, FormatSpaceDelimited); got != "admin viewer" {
		t.Fatalf("space_delimited format: got %v", got)
	}
	// An unknown format degrades to an array: roles in the wrong shape are
	// recoverable, roles missing entirely are a locked door.
	if got := FormatRoles(roles, "wat"); !reflect.DeepEqual(got, roles) {
		t.Fatalf("unknown format should fall back to array, got %#v", got)
	}
	// An empty role set must serialise as [] and never as null — the silence
	// is the answer people are looking for on the preview screen.
	empty, ok := FormatRoles(nil, FormatArray).([]string)
	if !ok || empty == nil || len(empty) != 0 {
		t.Fatalf("nil roles should render as an empty array, got %#v", empty)
	}
}

func TestEmit_RolesAttributesAndStatics(t *testing.T) {
	p := Profile{
		ProjectID:       "pLaser",
		ClaimName:       "mkauth.laser.roles",
		FormatType:      FormatArray,
		AttributeClaims: map[string]string{"mkauth.laser.email": AttrEmail, "mkauth.laser.count": AttrRoleCount},
		StaticClaims:    map[string]any{"mkauth.tenant": "makerspace"},
	}
	got := p.Emit(Facts{Roles: []string{"trained"}, Email: "t@x.edu", UserID: "u1"})

	if !reflect.DeepEqual(got["mkauth.laser.roles"], []string{"trained"}) {
		t.Errorf("roles claim: got %#v", got["mkauth.laser.roles"])
	}
	if got["mkauth.laser.email"] != "t@x.edu" {
		t.Errorf("attribute claim: got %v", got["mkauth.laser.email"])
	}
	if got["mkauth.laser.count"] != 1 {
		t.Errorf("role_count attribute: got %v", got["mkauth.laser.count"])
	}
	if got["mkauth.tenant"] != "makerspace" {
		t.Errorf("static claim: got %v", got["mkauth.tenant"])
	}
}

// An attribute the facts cannot supply is omitted, not emitted as null. A
// claim reading "email": null tells an application the user has no email,
// which is a different and worse lie than the claim being absent.
func TestEmit_MissingAttributeIsOmitted(t *testing.T) {
	p := Profile{
		ClaimName:       "roles",
		FormatType:      FormatArray,
		AttributeClaims: map[string]string{"email": AttrEmail},
	}
	got := p.Emit(Facts{Roles: []string{"a"}})
	if _, present := got["email"]; present {
		t.Fatalf("expected the email claim to be omitted when unknown, got %#v", got)
	}
}

func TestShape_DefaultPlusOverrides(t *testing.T) {
	profiles := []Profile{
		{ApplicationID: "app_badge", ClaimName: "badge.roles", FormatType: FormatCSV},
		{ClaimName: "mkauth.laser.roles", FormatType: FormatArray},
	}
	got := Shape(profiles, Facts{Roles: []string{"trained", "maintainer"}})

	if len(got) != 2 {
		t.Fatalf("expected the project default AND the override key, got %#v", got)
	}
	if got["badge.roles"] != "trained,maintainer" {
		t.Errorf("override format not applied: %v", got["badge.roles"])
	}
	if !reflect.DeepEqual(got["mkauth.laser.roles"], []string{"trained", "maintainer"}) {
		t.Errorf("default profile not applied: %v", got["mkauth.laser.roles"])
	}
}

func TestValidateProfile(t *testing.T) {
	valid := Profile{ClaimName: "mkauth.laser.roles", FormatType: FormatArray}
	if err := ValidateProfile(valid); err != nil {
		t.Fatalf("expected a dotted namespace key to validate, got %v", err)
	}

	cases := map[string]Profile{
		"empty claim name":  {ClaimName: "  ", FormatType: FormatArray},
		"unknown format":    {ClaimName: "roles", FormatType: "yaml"},
		"trailing dot":      {ClaimName: "mkauth.laser.", FormatType: FormatArray},
		"illegal character": {ClaimName: "mkauth laser roles", FormatType: FormatArray},
		"unknown attribute": {ClaimName: "roles", FormatType: FormatArray,
			AttributeClaims: map[string]string{"x": "shoe_size"}},
		"key used twice": {ClaimName: "roles", FormatType: FormatArray,
			AttributeClaims: map[string]string{"roles": AttrEmail}},
		"static shadows attribute": {ClaimName: "roles", FormatType: FormatArray,
			AttributeClaims: map[string]string{"team": AttrTeam},
			StaticClaims:    map[string]any{"team": "fixed"}},
	}
	for name, p := range cases {
		if err := ValidateProfile(p); err == nil {
			t.Errorf("%s: expected rejection, got nil", name)
		}
	}
}

// Two applications on one project cannot both claim the same key: a flat JWT
// holds one value per name, so the second would silently read the first's.
func TestConflicts(t *testing.T) {
	profiles := []Profile{
		{ClaimName: "roles", FormatType: FormatArray},
		{ApplicationID: "app_badge", ApplicationName: "Badge Reader", ClaimName: "roles", FormatType: FormatCSV},
	}
	conflicts := Conflicts(profiles)
	if len(conflicts) != 1 || conflicts[0].ClaimKey != "roles" {
		t.Fatalf("expected one conflict on the roles key, got %#v", conflicts)
	}
	if conflicts[0].Other != "the project default" {
		t.Errorf("conflict should name the existing owner, got %q", conflicts[0].Other)
	}

	clean := []Profile{
		{ClaimName: "mkauth.laser.roles", FormatType: FormatArray},
		{ApplicationID: "app_badge", ClaimName: "badge.roles", FormatType: FormatCSV},
	}
	if got := Conflicts(clean); len(got) != 0 {
		t.Fatalf("distinct keys must not conflict, got %#v", got)
	}
}

func TestKeys(t *testing.T) {
	p := Profile{
		ClaimName:       "roles",
		FormatType:      FormatArray,
		AttributeClaims: map[string]string{"email": AttrEmail},
		StaticClaims:    map[string]any{"tenant": "makerspace"},
	}
	want := []string{"email", "roles", "tenant"}
	if got := p.Keys(); !reflect.DeepEqual(got, want) {
		t.Fatalf("Keys(): got %v want %v", got, want)
	}
}
