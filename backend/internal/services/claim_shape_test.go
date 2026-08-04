package services

import (
	"context"
	"strings"
	"testing"

	"syndra/internal/claims"
	"syndra/internal/db"
	"syndra/internal/models"
)

// stubClaimShapeDeps neutralises every claim-shaping injectable and returns a
// pointer to the last profile written, so tests can assert what was persisted.
func stubClaimShapeDeps(t *testing.T) (*db.ClaimProfileRow, *db.AppClaimOverrideRow) {
	t.Helper()
	origGet, origList, origUpsert := svcGetClaimProfile, svcListClaimProfiles, svcUpsertClaimProfile
	origListOv, origUpsertOv, origDelOv := svcListAppClaimOverrides, svcUpsertAppClaimOverride, svcDeleteAppClaimOverride
	t.Cleanup(func() {
		svcGetClaimProfile, svcListClaimProfiles, svcUpsertClaimProfile = origGet, origList, origUpsert
		svcListAppClaimOverrides, svcUpsertAppClaimOverride, svcDeleteAppClaimOverride = origListOv, origUpsertOv, origDelOv
	})

	savedProfile := &db.ClaimProfileRow{}
	savedOverride := &db.AppClaimOverrideRow{}

	svcGetClaimProfile = func(context.Context, string) (db.ClaimProfileRow, bool, error) {
		return db.ClaimProfileRow{}, false, nil
	}
	svcListClaimProfiles = func(context.Context) ([]db.ClaimProfileRow, error) { return nil, nil }
	svcListAppClaimOverrides = func(context.Context) ([]db.AppClaimOverrideRow, error) { return nil, nil }
	svcUpsertClaimProfile = func(_ context.Context, r db.ClaimProfileRow) error {
		*savedProfile = r
		return nil
	}
	svcUpsertAppClaimOverride = func(_ context.Context, r db.AppClaimOverrideRow) error {
		*savedOverride = r
		return nil
	}
	svcDeleteAppClaimOverride = func(context.Context, string) error { return nil }
	return savedProfile, savedOverride
}

// A project nobody has configured still resolves to a usable profile — the
// data plane must never be handed an empty set, which would emit a token with
// no roles at all.
func TestResolveClaimProfiles_FallsBackToBuiltInDefault(t *testing.T) {
	stubClaimShapeDeps(t)

	got, err := ResolveClaimProfiles(context.Background(), "pLaser")
	if err != nil {
		t.Fatalf("ResolveClaimProfiles: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected exactly the default profile, got %#v", got)
	}
	if got[0].ClaimName != claims.DefaultClaimName || got[0].FormatType != claims.DefaultFormat {
		t.Fatalf("expected the built-in default shape, got %#v", got[0])
	}
}

func TestResolveClaimProfiles_DefaultThenOverridesForThisProjectOnly(t *testing.T) {
	stubClaimShapeDeps(t)

	svcGetClaimProfile = func(_ context.Context, projectID string) (db.ClaimProfileRow, bool, error) {
		return db.ClaimProfileRow{ProjectID: projectID, ClaimName: "syndra.laser.roles", FormatType: claims.FormatArray}, true, nil
	}
	svcListAppClaimOverrides = func(context.Context) ([]db.AppClaimOverrideRow, error) {
		return []db.AppClaimOverrideRow{
			{ApplicationID: "app_badge", ProjectID: "pLaser", ClaimName: "badge.roles", FormatType: claims.FormatCSV},
			{ApplicationID: "app_other", ProjectID: "pStudio", ClaimName: "studio.roles", FormatType: claims.FormatCSV},
		}, nil
	}

	got, err := ResolveClaimProfiles(context.Background(), "pLaser")
	if err != nil {
		t.Fatalf("ResolveClaimProfiles: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected the default plus this project's one override, got %#v", got)
	}
	if got[0].ApplicationID != "" || got[0].ClaimName != "syndra.laser.roles" {
		t.Errorf("the project default must come first, got %#v", got[0])
	}
	if got[1].ApplicationID != "app_badge" {
		t.Errorf("another project's override leaked in: %#v", got[1])
	}
}

// A flat JWT holds one value per name, so a key another project already emits
// must be rejected at save time — otherwise one application silently reads
// another's roles.
func TestSaveProjectClaimProfile_RejectsKeyOwnedByAnotherProject(t *testing.T) {
	stubClaimShapeDeps(t)

	svcListClaimProfiles = func(context.Context) ([]db.ClaimProfileRow, error) {
		return []db.ClaimProfileRow{
			{ProjectID: "pStudio", ClaimName: "shared.roles", FormatType: claims.FormatArray},
		}, nil
	}

	err := SaveProjectClaimProfile(context.Background(), "pLaser", claims.Profile{
		ClaimName: "shared.roles", FormatType: claims.FormatArray,
	})
	if err == nil {
		t.Fatal("expected a cross-project claim key collision to be rejected")
	}
	if !strings.Contains(err.Error(), "shared.roles") {
		t.Errorf("the error must name the colliding key, got %q", err)
	}
}

// Re-saving a project's own profile is not a collision with itself.
func TestSaveProjectClaimProfile_AllowsReSaveOfOwnKey(t *testing.T) {
	saved, _ := stubClaimShapeDeps(t)

	svcListClaimProfiles = func(context.Context) ([]db.ClaimProfileRow, error) {
		return []db.ClaimProfileRow{
			{ProjectID: "pLaser", ClaimName: "syndra.laser.roles", FormatType: claims.FormatArray},
		}, nil
	}

	err := SaveProjectClaimProfile(context.Background(), "pLaser", claims.Profile{
		ClaimName: "syndra.laser.roles", FormatType: claims.FormatCSV,
	})
	if err != nil {
		t.Fatalf("re-saving the same project's own key must be allowed, got %v", err)
	}
	if saved.FormatType != claims.FormatCSV {
		t.Errorf("expected the new format to be persisted, got %q", saved.FormatType)
	}
}

func TestSaveProjectClaimProfile_RejectsInvalidProfile(t *testing.T) {
	stubClaimShapeDeps(t)

	err := SaveProjectClaimProfile(context.Background(), "pLaser", claims.Profile{
		ClaimName: "syndra laser roles", FormatType: claims.FormatArray,
	})
	if err == nil {
		t.Fatal("expected an invalid claim key to be rejected before it reaches a signed token")
	}
}

// The override's project comes from the application, never from the request:
// an override filed against the wrong project would shape a token the app
// never receives.
func TestSaveAppClaimOverride_DerivesProjectFromApplication(t *testing.T) {
	_, savedOverride := stubClaimShapeDeps(t)
	setupSnapshotTestFixtures(t, 1, 1, 1) // app a0 lives on project p0

	err := SaveAppClaimOverride(context.Background(), "a0", claims.Profile{
		ProjectID: "spoofed-project", ClaimName: "badge.roles", FormatType: claims.FormatCSV,
	})
	if err != nil {
		t.Fatalf("SaveAppClaimOverride: %v", err)
	}
	if savedOverride.ProjectID != "p0" {
		t.Errorf("expected the project from the directory, got %q", savedOverride.ProjectID)
	}
	if savedOverride.ApplicationID != "a0" {
		t.Errorf("expected the application id from the path, got %q", savedOverride.ApplicationID)
	}
}

func TestSaveAppClaimOverride_UnknownApplication(t *testing.T) {
	stubClaimShapeDeps(t)
	setupSnapshotTestFixtures(t, 1, 1, 1)

	err := SaveAppClaimOverride(context.Background(), "nope", claims.Profile{
		ClaimName: "x.roles", FormatType: claims.FormatCSV,
	})
	if err == nil {
		t.Fatal("expected an override for an unknown application to be rejected")
	}
}

// The operator view has to say which application owns each key, because a
// token carries every key on the project and a sibling's key otherwise reads
// as an unexplained extra claim.
func TestProjectClaimShape_AttributesEveryEmittedKey(t *testing.T) {
	stubClaimShapeDeps(t)
	setupSnapshotTestFixtures(t, 1, 1, 1)

	svcGetClaimProfile = func(_ context.Context, projectID string) (db.ClaimProfileRow, bool, error) {
		return db.ClaimProfileRow{
			ProjectID: projectID, ClaimName: "syndra.p0.roles", FormatType: claims.FormatArray,
			AttributeClaims: map[string]string{"syndra.p0.email": claims.AttrEmail},
		}, true, nil
	}
	svcListAppClaimOverrides = func(context.Context) ([]db.AppClaimOverrideRow, error) {
		return []db.AppClaimOverrideRow{
			{ApplicationID: "a0", ProjectID: "p0", ClaimName: "badge.roles", FormatType: claims.FormatCSV},
		}, nil
	}

	shape, err := ProjectClaimShape(context.Background(), "p0")
	if err != nil {
		t.Fatalf("ProjectClaimShape: %v", err)
	}
	if len(shape.Conflicts) != 0 {
		t.Fatalf("distinct keys must not conflict, got %#v", shape.Conflicts)
	}

	owners := map[string]models.ClaimKeyOwner{}
	for _, k := range shape.EmittedKeys {
		owners[k.Key] = k
	}
	if len(owners) != 3 {
		t.Fatalf("expected roles + attribute + override keys, got %#v", shape.EmittedKeys)
	}
	if owners["badge.roles"].ApplicationID != "a0" || owners["badge.roles"].OwnerLabel != "App 0" {
		t.Errorf("override key must name its application, got %#v", owners["badge.roles"])
	}
	if owners["syndra.p0.email"].Kind != "attribute" || owners["syndra.p0.email"].Source != claims.AttrEmail {
		t.Errorf("attribute key must name its source, got %#v", owners["syndra.p0.email"])
	}
	if owners["syndra.p0.roles"].OwnerLabel != "Project default" {
		t.Errorf("default key must be labelled as the project default, got %#v", owners["syndra.p0.roles"])
	}
}
