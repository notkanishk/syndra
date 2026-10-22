package services

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"syndra/internal/db"
	"syndra/internal/models"
)

// stubServiceMarks swaps the stored-marks read for the given rows. Tests that
// never mention service accounts get an empty table, never the live pool.
func stubServiceMarks(t *testing.T, rows ...db.ServiceAccountRow) {
	t.Helper()
	orig := svcListServiceAccountRows
	t.Cleanup(func() { svcListServiceAccountRows = orig })
	svcListServiceAccountRows = func(context.Context) ([]db.ServiceAccountRow, error) { return rows, nil }
}

// A human-type account the operator marked is a service account: it is not
// onboarded, exactly like a Zitadel machine user.
func TestTriggerOnboarding_MarkedHumanGetsNothing(t *testing.T) {
	resetOnboardingDeps(t)
	happyPathDeps()
	stubServiceMarks(t, db.ServiceAccountRow{UserID: "ingest", Kind: "device"})
	svcFindUser = func(context.Context, string) (models.UserProfile, bool, error) {
		return models.UserProfile{ID: "ingest", Kind: models.UserKindHuman}, true, nil
	}
	svcInsertOnboardingTrigger = func(context.Context, string, string, string) (string, bool, error) {
		t.Error("a marked account must not record an onboarding trigger")
		return "", false, nil
	}
	svcCascadeWelcomeBundle = func(context.Context, string, string, string) (CascadeResult, error) {
		t.Error("a marked account must not be handed the welcome bundle")
		return CascadeResult{}, nil
	}
	if err := TriggerOnboarding(context.Background(), "ingest", "webhook", "k"); err != nil {
		t.Fatalf("a marked account is skipped, not failed: %v", err)
	}
}

// An unreadable marks table is a failure, not a guess that the account is a
// person: Zitadel redelivers and the welcome bundle is not handed out blind.
func TestTriggerOnboarding_UnreadableMarksFailTheTrigger(t *testing.T) {
	resetOnboardingDeps(t)
	happyPathDeps()
	svcFindUser = func(context.Context, string) (models.UserProfile, bool, error) {
		return models.UserProfile{ID: "u1", Kind: models.UserKindHuman}, true, nil
	}
	svcListServiceAccountRows = func(context.Context) ([]db.ServiceAccountRow, error) {
		return nil, errors.New("db down")
	}
	svcCascadeWelcomeBundle = func(context.Context, string, string, string) (CascadeResult, error) {
		t.Error("nothing may be assigned when the marks could not be read")
		return CascadeResult{}, nil
	}
	if err := TriggerOnboarding(context.Background(), "u1", "webhook", "k"); err == nil {
		t.Fatal("an unreadable marks table must fail the trigger")
	}
}

// The no-access gap lists people: a marked human holding no welcome bundle is
// not in it.
func TestFindMissedOnboarding_ExcludesMarkedHuman(t *testing.T) {
	resetReconcileDeps(t)
	stubServiceMarks(t, db.ServiceAccountRow{UserID: "u-kiosk", Kind: "device"})
	svcGetWelcomeBundle = func(context.Context) (string, error) { return "bundle-welcome", nil }
	svcGetUsersForBundle = func(context.Context, string) ([]string, error) { return nil, nil }
	svcDirectoryUsers = func(context.Context) ([]models.UserProfile, error) {
		return []models.UserProfile{
			{ID: "u-person", Name: "Person", Status: "active"},
			{ID: "u-kiosk", Name: "Kiosk", Status: "active", Kind: models.UserKindHuman},
		}, nil
	}
	got, err := FindMissedOnboarding(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Missed) != 1 || got.Missed[0].UserID != "u-person" {
		t.Fatalf("want only the person, got %+v", got.Missed)
	}
}

// A marked human's drift stays in the queue, flagged as a service account.
func TestDriftTriageQueue_KeepsMarkedHumanAsServiceAccount(t *testing.T) {
	withTriageDeps(t,
		[]models.DriftItem{{ID: "x", Target: db.TargetZitadel, UserID: "u-kiosk", ProjectID: "p_wiki", RoleKeys: []string{"read"}, DetectedAt: time.Now()}},
		map[string]models.UserProfile{"u-kiosk": {ID: "u-kiosk", Name: "Kiosk", Status: "active", Kind: models.UserKindHuman}})
	stubServiceMarks(t, db.ServiceAccountRow{UserID: "u-kiosk", Kind: "device"})
	got, err := DriftTriageQueue(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || !got[0].UserIsServiceAccount {
		t.Fatalf("a marked human's drift stays, as a service account: %+v", got)
	}
}

func marksAt(rows ...db.ServiceAccountRow) map[string]db.ServiceAccountRow {
	m := map[string]db.ServiceAccountRow{}
	for _, r := range rows {
		m[r.UserID] = r
	}
	return m
}

// The list is every machine user plus every marked human, by name, each with
// what it holds as Zitadel observes it and its stored kind, purpose and owner.
// A machine with no row reads as an Integration with no purpose and no owner.
func TestServiceAccountViews_MachinesAndMarkedHumansByName(t *testing.T) {
	k1, r1 := role("p_wiki", "read", "Wiki")
	k2, r2 := role("p_wiki", "write", "Wiki")
	users := []models.UserProfile{
		{ID: "u-person", Name: "Person"},
		{ID: "u-zed", Name: "Zed kiosk", Kind: models.UserKindHuman},
		{ID: "u-netbird", Name: "netbird", Kind: models.UserKindMachine},
	}
	snap := snapshotWith(users, map[string]map[roleKey]*models.EffectiveRole{
		"u-person": {k1: r1}, "u-zed": {k1: r1, k2: r2}, "u-netbird": {k1: r1},
	}, nil)
	marked := time.Date(2026, 10, 20, 9, 0, 0, 0, time.UTC)
	marks := marksAt(db.ServiceAccountRow{UserID: "u-zed", Kind: "device", Purpose: "Front-door panel",
		OwnerID: "u-person", MarkedBy: "admin-1", MarkedAt: marked})

	got, err := serviceAccountViews(snap, marks)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].Name != "netbird" || got[1].Name != "Zed kiosk" {
		t.Fatalf("want netbird then Zed kiosk, got %+v", got)
	}
	nb, zed := got[0], got[1]
	if nb.Kind != "integration" || nb.ZitadelType != "machine" || nb.Purpose != "" || nb.OwnerID != "" || nb.MarkedAt != nil || nb.RoleCount != 1 {
		t.Errorf("a machine with no row: %+v", nb)
	}
	if zed.Kind != "device" || zed.ZitadelType != "human" || zed.Purpose != "Front-door panel" ||
		zed.OwnerID != "u-person" || zed.MarkedBy != "admin-1" || zed.MarkedAt == nil || !zed.MarkedAt.Equal(marked) || zed.RoleCount != 2 {
		t.Errorf("a marked human: %+v", zed)
	}
}

func saveDeps(t *testing.T, users ...models.UserProfile) (upserts *[]db.ServiceAccountRow) {
	t.Helper()
	oFind, oUp, oSnap := svcFindUser, svcUpsertServiceAccount, svcSnapshotOf
	t.Cleanup(func() { svcFindUser, svcUpsertServiceAccount, svcSnapshotOf = oFind, oUp, oSnap })
	svcSnapshotOf = func(_ context.Context, us []models.UserProfile) *accessSnapshot { return snapshotWith(us, nil, nil) }
	svcFindUser = func(_ context.Context, id string) (models.UserProfile, bool, error) {
		for _, u := range users {
			if u.ID == id {
				return u, true, nil
			}
		}
		return models.UserProfile{}, false, nil
	}
	var rows []db.ServiceAccountRow
	svcUpsertServiceAccount = func(_ context.Context, id, kind, purpose, owner, actor string) (db.ServiceAccountRow, error) {
		r := db.ServiceAccountRow{UserID: id, Kind: kind, Purpose: purpose, OwnerID: owner, MarkedBy: actor, MarkedAt: time.Now(), UpdatedAt: time.Now()}
		rows = append(rows, r)
		return r, nil
	}
	return &rows
}

func TestSaveServiceAccount_Validation(t *testing.T) {
	stubServiceMarks(t, db.ServiceAccountRow{UserID: "u-other-sa", Kind: "agent"})
	rows := saveDeps(t,
		models.UserProfile{ID: "u-kiosk", Name: "Kiosk", Kind: models.UserKindHuman},
		models.UserProfile{ID: "u-owner", Name: "Owner", Kind: models.UserKindHuman},
		models.UserProfile{ID: "u-other-sa", Name: "Other", Kind: models.UserKindHuman},
		models.UserProfile{ID: "u-machine", Name: "bot", Kind: models.UserKindMachine},
	)
	ctx := context.Background()
	bad := func(field string, in ServiceAccountInput, id string) {
		t.Helper()
		_, err := SaveServiceAccount(ctx, id, in, "admin")
		var inv *ServiceAccountInvalid
		if !errors.As(err, &inv) || inv.Field != field {
			t.Errorf("%+v: want invalid %q, got %v", in, field, err)
		}
	}
	bad("kind", ServiceAccountInput{Kind: "robot"}, "u-kiosk")
	bad("kind", ServiceAccountInput{}, "u-kiosk")
	bad("purpose", ServiceAccountInput{Kind: "device", Purpose: strings.Repeat("x", 201)}, "u-kiosk")
	bad("owner_id", ServiceAccountInput{Kind: "device", OwnerID: "u-other-sa"}, "u-kiosk") // a service account
	bad("owner_id", ServiceAccountInput{Kind: "device", OwnerID: "u-machine"}, "u-kiosk")  // a machine
	bad("owner_id", ServiceAccountInput{Kind: "device", OwnerID: "u-nobody"}, "u-kiosk")   // unknown
	bad("owner_id", ServiceAccountInput{Kind: "device", OwnerID: "u-kiosk"}, "u-kiosk")    // itself
	if len(*rows) != 0 {
		t.Fatalf("nothing is written for a refused request: %+v", *rows)
	}
	if _, err := SaveServiceAccount(ctx, "u-nobody", ServiceAccountInput{Kind: "device"}, "admin"); !errors.Is(err, ErrServiceAccountNotFound) {
		t.Errorf("an id the directory does not know is not found, got %v", err)
	}

	got, err := SaveServiceAccount(ctx, "u-kiosk", ServiceAccountInput{Kind: "device", Purpose: "  Front panel  ", OwnerID: "u-owner"}, "admin")
	if err != nil {
		t.Fatal(err)
	}
	if len(*rows) != 1 || (*rows)[0].Purpose != "Front panel" || (*rows)[0].MarkedBy != "admin" {
		t.Fatalf("purpose is trimmed, actor recorded: %+v", *rows)
	}
	if got.ID != "u-kiosk" || got.Name != "Kiosk" || got.Kind != "device" || got.ZitadelType != "human" || got.OwnerID != "u-owner" {
		t.Errorf("returned account: %+v", got)
	}
}

func TestUnmarkServiceAccount(t *testing.T) {
	saveDeps(t,
		models.UserProfile{ID: "u-kiosk", Kind: models.UserKindHuman},
		models.UserProfile{ID: "u-plain", Kind: models.UserKindHuman},
		models.UserProfile{ID: "u-machine", Kind: models.UserKindMachine},
	)
	oDel := svcDeleteServiceAccount
	t.Cleanup(func() { svcDeleteServiceAccount = oDel })
	var deleted []string
	svcDeleteServiceAccount = func(_ context.Context, id string) (bool, error) {
		deleted = append(deleted, id)
		return id == "u-kiosk", nil
	}
	ctx := context.Background()
	if err := UnmarkServiceAccount(ctx, "u-machine"); !errors.Is(err, ErrMachineAccount) {
		t.Errorf("a machine cannot be marked as a person, got %v", err)
	}
	if err := UnmarkServiceAccount(ctx, "u-plain"); !errors.Is(err, ErrServiceAccountNotFound) {
		t.Errorf("an unmarked person is not a service account, got %v", err)
	}
	if err := UnmarkServiceAccount(ctx, "u-nobody"); !errors.Is(err, ErrServiceAccountNotFound) {
		t.Errorf("unknown id, got %v", err)
	}
	if err := UnmarkServiceAccount(ctx, "u-kiosk"); err != nil {
		t.Errorf("marked human is unmarked: %v", err)
	}
	if len(deleted) != 2 { // u-plain and u-kiosk reach the delete; machine and unknown do not
		t.Errorf("deleted calls: %v", deleted)
	}
}

// Each user on the People list says whether it is effectively a service
// account, so a surface can drop it from a people flow without a second read.
func TestListUsers_CarriesEffectiveServiceAccountFlag(t *testing.T) {
	users := []models.UserProfile{
		{ID: "u-person", Name: "Person"},
		{ID: "u-zed", Name: "Zed", Kind: models.UserKindHuman},
		{ID: "u-netbird", Name: "netbird", Kind: models.UserKindMachine},
	}
	flagged := flagServiceAccounts(users, marksAt(db.ServiceAccountRow{UserID: "u-zed"}))
	if users[1].ServiceAccount {
		t.Fatal("the shared slice must not be written to")
	}
	items, err := listUsersFromSnapshot(snapshotWith(flagged, nil, nil), "", attentionIndex{})
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]bool{}
	for _, it := range items {
		got[it.User.ID] = it.User.ServiceAccount
	}
	if got["u-person"] || !got["u-zed"] || !got["u-netbird"] {
		t.Fatalf("person false, marked human and machine true: %v", got)
	}
}
