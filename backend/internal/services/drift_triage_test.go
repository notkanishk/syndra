package services

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"syndra/internal/db"
	"syndra/internal/models"
)

func withTriageDeps(t *testing.T, items []models.DriftItem, users map[string]models.UserProfile) {
	t.Helper()
	origDrift := svcGetPendingDriftItems
	origLocal := svcDbGetAllLocalRoles
	origUsage := svcDbGetRoleUsageCounts
	origAssigned := svcDbGetAssignedUserCounts
	origRefs := svcDbGetAllReferencedRoleKeys
	origFind := directoryFindUser
	origGrants, origBases := svcGetAllDirectGrants, svcMergeBases
	t.Cleanup(func() {
		svcGetAllDirectGrants, svcMergeBases = origGrants, origBases
	})
	// No ledger and no observations by default: a row with no history is still
	// a row, and every test written before provenance existed asserts what one
	// looks like.
	svcGetAllDirectGrants = func(context.Context, bool) ([]models.DirectGrant, error) { return nil, nil }
	svcMergeBases = func(context.Context, string) (map[string]db.MergeBase, error) {
		return map[string]db.MergeBase{}, nil
	}
	t.Cleanup(func() {
		svcGetPendingDriftItems = origDrift
		svcDbGetAllLocalRoles = origLocal
		svcDbGetRoleUsageCounts = origUsage
		svcDbGetAssignedUserCounts = origAssigned
		svcDbGetAllReferencedRoleKeys = origRefs
		directoryFindUser = origFind
	})

	svcGetPendingDriftItems = func(context.Context) ([]models.DriftItem, error) { return items, nil }
	svcDbGetAllLocalRoles = func(context.Context) ([]models.Role, error) {
		return []models.Role{
			{ProjectID: "p_laser", RoleKey: "operator", DisplayName: "Bay operator", Group: "Safety-gated"},
			{ProjectID: "p_wiki", RoleKey: "read", DisplayName: "Wiki reader", Group: "Open bench"},
		}, nil
	}
	svcDbGetRoleUsageCounts = func(context.Context) (map[string]db.RoleUsage, error) {
		return map[string]db.RoleUsage{}, nil
	}
	svcDbGetAssignedUserCounts = func(context.Context) (map[string]int, error) { return map[string]int{}, nil }
	svcDbGetAllReferencedRoleKeys = func(context.Context) ([][2]string, error) { return nil, nil }
	directoryFindUser = func(_ context.Context, id string) (models.UserProfile, bool, error) {
		u, ok := users[id]
		return u, ok, nil
	}
}

// A safety-gated role found yesterday must outrank a wiki role found last week.
// Age alone is the wrong order for a queue where one row is a laser cutter.
func TestDriftTriageQueue_OrdersByRiskThenAge(t *testing.T) {
	old := time.Now().Add(-7 * 24 * time.Hour)
	recent := time.Now().Add(-1 * 24 * time.Hour)
	withTriageDeps(t, []models.DriftItem{
		{ID: "wiki", Target: db.TargetZitadel, UserID: "u1", ProjectID: "p_wiki", RoleKeys: []string{"read"}, DetectedAt: old},
		{ID: "laser", Target: db.TargetZitadel, UserID: "u2", ProjectID: "p_laser", RoleKeys: []string{"operator"}, DetectedAt: recent},
	}, nil)

	got, err := DriftTriageQueue(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("want 2 rows, got %d", len(got))
	}
	if got[0].ID != "laser" {
		t.Fatalf("safety-gated row must sort first, got %q", got[0].ID)
	}
	if got[0].RoleGroup != "Safety-gated" {
		t.Fatalf("risk pill needs the role group, got %q", got[0].RoleGroup)
	}
}

// Within the same risk tier, the oldest item comes first: it has been
// unexplained longest.
func TestDriftTriageQueue_OldestFirstWithinTier(t *testing.T) {
	older := time.Now().Add(-9 * 24 * time.Hour)
	newer := time.Now().Add(-2 * 24 * time.Hour)
	withTriageDeps(t, []models.DriftItem{
		{ID: "newer", Target: db.TargetZitadel, UserID: "u1", ProjectID: "p_wiki", RoleKeys: []string{"read"}, DetectedAt: newer},
		{ID: "older", Target: db.TargetZitadel, UserID: "u2", ProjectID: "p_wiki", RoleKeys: []string{"read"}, DetectedAt: older},
	}, nil)

	got, err := DriftTriageQueue(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got[0].ID != "older" {
		t.Fatalf("oldest must lead within a tier, got %q", got[0].ID)
	}
}

// A role Syndra no longer knows about outranks routine drift: adopting it would
// recreate something somebody deliberately retired.
func TestDriftTriageQueue_UncataloguedRoleOutranksKnownRoutineRole(t *testing.T) {
	same := time.Now().Add(-3 * 24 * time.Hour)
	withTriageDeps(t, []models.DriftItem{
		{ID: "known", Target: db.TargetZitadel, UserID: "u1", ProjectID: "p_wiki", RoleKeys: []string{"read"}, DetectedAt: same},
		{ID: "retired", Target: db.TargetZitadel, UserID: "u2", ProjectID: "p_wiki", RoleKeys: []string{"legacy-op"}, DetectedAt: same},
	}, nil)

	got, err := DriftTriageQueue(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got[0].ID != "retired" {
		t.Fatalf("uncatalogued role must outrank a known routine one, got %q", got[0].ID)
	}
	if got[0].RoleInCatalogue {
		t.Error("legacy-op is not in the catalogue and must not be reported as such")
	}
}

// "2 more items for this person" is the context that changes a revoke decision,
// and it counts OTHER items — never the row you are looking at.
func TestDriftTriageQueue_CountsOtherItemsForTheSamePerson(t *testing.T) {
	now := time.Now()
	withTriageDeps(t, []models.DriftItem{
		{ID: "a", Target: db.TargetZitadel, UserID: "marta", ProjectID: "p_wiki", RoleKeys: []string{"read"}, DetectedAt: now},
		{ID: "b", Target: db.TargetZitadel, UserID: "marta", ProjectID: "p_wiki", RoleKeys: []string{"read2"}, DetectedAt: now},
		{ID: "c", Target: db.TargetZitadel, UserID: "marta", ProjectID: "p_wiki", RoleKeys: []string{"read3"}, DetectedAt: now},
		{ID: "d", Target: db.TargetZitadel, UserID: "solo", ProjectID: "p_wiki", RoleKeys: []string{"read"}, DetectedAt: now},
	}, nil)

	got, err := DriftTriageQueue(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range got {
		want := 2
		if item.UserID == "solo" {
			want = 0
		}
		if item.OtherItemsForUser != want {
			t.Errorf("%s: other items = %d, want %d", item.ID, item.OtherItemsForUser, want)
		}
	}
}

// A machine account is not a person: "adopt" is the wrong verb for something an
// integration re-creates on every deploy, so the row has to say what it is.
func TestDriftTriageQueue_MarksServiceAccounts(t *testing.T) {
	withTriageDeps(t,
		[]models.DriftItem{{ID: "x", Target: db.TargetZitadel, UserID: "svc1", ProjectID: "p_wiki", RoleKeys: []string{"read"}, DetectedAt: time.Now()}},
		map[string]models.UserProfile{
			"svc1": {ID: "svc1", Name: "svc-bookings", Email: "svc-bookings@makerspace.local", Status: "active"},
		})

	got, err := DriftTriageQueue(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !got[0].UserIsServiceAccount {
		t.Error("svc-bookings must be recognised as a service account")
	}
}

// An empty queue returns an empty slice, not nil: "everything is explained" is
// an answer, and the caller must not have to distinguish it from a failure.
func TestDriftTriageQueue_EmptyReturnsEmptySlice(t *testing.T) {
	withTriageDeps(t, nil, nil)
	got, err := DriftTriageQueue(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got == nil || len(got) != 0 {
		t.Fatalf("want empty slice, got %#v", got)
	}
}

// 1.13 — the catalogue is built from Zitadel projects and roles. A TrueNAS
// dataset permission is not absent from it; it is not the kind of thing it
// lists. Judged against it anyway, every add-on row would rank as a retired
// role and the queue would sort by which system found the drift rather than by
// how much it matters.
func TestDriftTriageQueue_DoesNotJudgeAnAddOnRowAgainstZitadelsCatalogue(t *testing.T) {
	older := time.Now().Add(-9 * 24 * time.Hour)
	newer := time.Now().Add(-1 * 24 * time.Hour)
	withTriageDeps(t, []models.DriftItem{
		{ID: "zitadel-known", Target: db.TargetZitadel, UserID: "u1", ProjectID: "p_wiki", RoleKeys: []string{"read"}, DetectedAt: older},
		{ID: "addon", Target: "truenas", UserID: "u2", ProjectID: "", RoleKeys: []string{"tank/projects:rw"}, DetectedAt: newer},
	}, nil)

	got, err := DriftTriageQueue(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	var addon models.DriftTriageItem
	for _, item := range got {
		if item.ID == "addon" {
			addon = item
		}
	}
	if addon.RoleCatalogueApplies {
		t.Error("a target with no role catalogue must say so, or the UI reads role_in_catalogue=false as a retired role")
	}
	if addon.RoleInCatalogue || addon.RoleGroup != "" {
		t.Errorf("an add-on row must not be enriched from Zitadel's catalogue, got in_catalogue=%v group=%q", addon.RoleInCatalogue, addon.RoleGroup)
	}
	// Both rows rank routine, so the older one leads. Ranked as retired, the
	// newer add-on row would have jumped it.
	if got[0].ID != "zitadel-known" {
		t.Fatalf("an add-on row must not outrank routine drift; order was %q first", got[0].ID)
	}
}

// The same row on a target that does have a catalogue is still judged by it —
// the gate narrows the claim, it does not withdraw it.
func TestDriftTriageQueue_StillJudgesZitadelRowsAgainstTheCatalogue(t *testing.T) {
	same := time.Now().Add(-3 * 24 * time.Hour)
	withTriageDeps(t, []models.DriftItem{
		{ID: "known", Target: db.TargetZitadel, UserID: "u1", ProjectID: "p_wiki", RoleKeys: []string{"read"}, DetectedAt: same},
	}, nil)

	got, err := DriftTriageQueue(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !got[0].RoleCatalogueApplies || !got[0].RoleInCatalogue || got[0].RoleGroup != "Open bench" {
		t.Fatalf("a Zitadel row must still be enriched from the catalogue, got %+v", got[0])
	}
}

// A filtered listing is a smaller answer, not a differently-shaped one. The
// surface reads role_in_catalogue and role_catalogue_applies off every row, and
// an absent field is indistinguishable from a false one — so a raw filtered
// response silently withdrew the "role not in catalogue" warning from rows that
// had earned it.
func TestDriftTriageRows_EnrichesAFilteredSubset(t *testing.T) {
	same := time.Now().Add(-3 * 24 * time.Hour)
	pending := []models.DriftItem{
		{ID: "known", Target: db.TargetZitadel, UserID: "u1", ProjectID: "p_wiki", RoleKeys: []string{"read"}, DetectedAt: same},
		{ID: "retired", Target: db.TargetZitadel, UserID: "u1", ProjectID: "p_wiki", RoleKeys: []string{"legacy-op"}, DetectedAt: same},
		{ID: "laser", Target: db.TargetZitadel, UserID: "u2", ProjectID: "p_laser", RoleKeys: []string{"operator"}, DetectedAt: same},
	}
	withTriageDeps(t, pending, nil)

	got, err := DriftTriageRows(context.Background(), pending[1:2]) // the "retired" row alone
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("want the one filtered row, got %d", len(got))
	}
	if !got[0].RoleCatalogueApplies || got[0].RoleInCatalogue {
		t.Errorf("a filtered Zitadel row must still be judged against the catalogue, got applies=%v in_catalogue=%v",
			got[0].RoleCatalogueApplies, got[0].RoleInCatalogue)
	}
}

// "Marta has 2 more items" is a fact about Marta, not about the query. Counted
// within the filter it would shrink to match whatever the operator happened to
// be looking at, and read as reassurance.
func TestDriftTriageRows_CountsOtherItemsOverTheWholeQueue(t *testing.T) {
	now := time.Now()
	pending := []models.DriftItem{
		{ID: "a", Target: db.TargetZitadel, UserID: "marta", ProjectID: "p_wiki", RoleKeys: []string{"read"}, DetectedAt: now},
		{ID: "b", Target: db.TargetZitadel, UserID: "marta", ProjectID: "p_wiki", RoleKeys: []string{"read2"}, DetectedAt: now},
		{ID: "c", Target: db.TargetZitadel, UserID: "marta", ProjectID: "p_wiki", RoleKeys: []string{"read3"}, DetectedAt: now},
	}
	withTriageDeps(t, pending, nil)

	got, err := DriftTriageRows(context.Background(), pending[:1])
	if err != nil {
		t.Fatal(err)
	}
	if got[0].OtherItemsForUser != 2 {
		t.Fatalf("the count must span the whole pending queue, not the filtered slice; got %d", got[0].OtherItemsForUser)
	}
}

// A row outside the counted population — one a status filter pulled up after it
// was resolved — must not report one fewer item than the person actually has.
func TestDriftTriageRows_DoesNotDiscountARowOutsideThePendingQueue(t *testing.T) {
	now := time.Now()
	pending := []models.DriftItem{
		{ID: "a", Target: db.TargetZitadel, UserID: "marta", ProjectID: "p_wiki", RoleKeys: []string{"read"}, DetectedAt: now},
		{ID: "b", Target: db.TargetZitadel, UserID: "marta", ProjectID: "p_wiki", RoleKeys: []string{"read2"}, DetectedAt: now},
	}
	withTriageDeps(t, pending, nil)

	resolved := models.DriftItem{ID: "old", Target: db.TargetZitadel, UserID: "marta", ProjectID: "p_wiki",
		RoleKeys: []string{"read9"}, Status: "revoked", DetectedAt: now}
	got, err := DriftTriageRows(context.Background(), []models.DriftItem{resolved})
	if err != nil {
		t.Fatal(err)
	}
	if got[0].OtherItemsForUser != 2 {
		t.Fatalf("a row absent from the pending queue must not subtract itself from it; got %d", got[0].OtherItemsForUser)
	}
}

// Nothing to enrich means nothing to load: an empty subset must not go asking
// for the population it would count over.
func TestDriftTriageRows_EmptySubsetLoadsNothing(t *testing.T) {
	withTriageDeps(t, nil, nil)
	orig := svcGetPendingDriftItems
	t.Cleanup(func() { svcGetPendingDriftItems = orig })
	svcGetPendingDriftItems = func(context.Context) ([]models.DriftItem, error) {
		t.Fatal("an empty subset must not load the pending queue")
		return nil, nil
	}
	got, err := DriftTriageRows(context.Background(), nil)
	if err != nil || got == nil || len(got) != 0 {
		t.Fatalf("want an empty slice and no error, got %#v / %v", got, err)
	}
}

// A removal Syndra can recognise (change `reconciliation-as-merge`).
//
// A `syndra_only` row is produced by comparing two sets, and on the row it reads
// exactly like an unexplained absence — an operator triaging it has no way to
// know that Syndra granted this deliberately, who did, why, or that the target
// was holding it as recently as this morning. Every one of those changes what
// they should do about it.

func TestDriftTriage_ARemovedGrantCarriesItsOwnHistory(t *testing.T) {
	granted := time.Date(2026, 8, 3, 9, 0, 0, 0, time.UTC)
	observed := time.Date(2026, 8, 19, 3, 0, 0, 0, time.UTC)

	withTriageDeps(t, []models.DriftItem{{
		ID: "d1", Target: db.TargetZitadel, UserID: "u1", ProjectID: "p_laser",
		RoleKeys: []string{"operator"}, DriftType: db.DriftSyndraOnly,
		Status: "pending_triage", DetectedAt: observed.Add(time.Hour),
	}}, nil)
	svcGetAllDirectGrants = func(context.Context, bool) ([]models.DirectGrant, error) {
		return []models.DirectGrant{{
			UserID: "u1", ProjectID: "p_laser", RoleKey: "operator",
			GrantedBy: "op-ada", Reason: "inducted on the laser", CreatedAt: granted,
			Source: "direct",
		}}, nil
	}
	svcMergeBases = func(context.Context, string) (map[string]db.MergeBase, error) {
		return map[string]db.MergeBase{"u1": {
			Target: db.TargetZitadel, SubjectID: "u1", ObservedAt: observed,
			Base: map[string]json.RawMessage{"p_laser/operator": json.RawMessage(`true`)},
		}}, nil
	}

	rows, err := DriftTriageQueue(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatalf("want one row, got %d", len(rows))
	}
	p := rows[0].Provenance
	if p == nil {
		t.Fatal("a removal of something Syndra applied must not read as a stranger")
	}
	if p.GrantedBy != "op-ada" || p.Reason != "inducted on the laser" {
		t.Fatalf("the decision behind the access must travel: %+v", p)
	}
	if p.GrantedAt == nil || !p.GrantedAt.Equal(granted) {
		t.Fatalf("when it was granted: %+v", p)
	}
	// The half that makes a removal legible: it was live at 03:00 and is gone
	// now, rather than a row that appeared from nowhere.
	if p.LastObservedAt == nil || !p.LastObservedAt.Equal(observed) {
		t.Fatalf("when the target was last seen holding it: %+v", p)
	}
}

// Access Syndra has no record of gets no history, because any history attached
// to it would be somebody else's.
func TestDriftTriage_UnexplainedAccessCarriesNoProvenance(t *testing.T) {
	withTriageDeps(t, []models.DriftItem{{
		ID: "d1", Target: db.TargetZitadel, UserID: "u1", ProjectID: "p_laser",
		RoleKeys: []string{"operator"}, DriftType: db.DriftTargetOnly,
		Status: "pending_triage", DetectedAt: time.Now(),
	}}, nil)
	svcGetAllDirectGrants = func(context.Context, bool) ([]models.DirectGrant, error) {
		return []models.DirectGrant{{
			UserID: "u1", ProjectID: "p_laser", RoleKey: "operator", GrantedBy: "op-ada",
		}}, nil
	}

	rows, err := DriftTriageQueue(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if rows[0].Provenance != nil {
		t.Fatalf("a target_only row is access Syndra never applied: %+v", rows[0].Provenance)
	}
}

// A grant the last complete read did not see is dated with nothing rather than
// with the subject's observation time, which would be a claim about a grant
// nobody observed.
func TestDriftTriage_AnUnobservedGrantIsNotDated(t *testing.T) {
	withTriageDeps(t, []models.DriftItem{{
		ID: "d1", Target: db.TargetZitadel, UserID: "u1", ProjectID: "p_laser",
		RoleKeys: []string{"operator"}, DriftType: db.DriftSyndraOnly,
		Status: "pending_triage", DetectedAt: time.Now(),
	}}, nil)
	svcGetAllDirectGrants = func(context.Context, bool) ([]models.DirectGrant, error) {
		return []models.DirectGrant{{UserID: "u1", ProjectID: "p_laser", RoleKey: "operator", GrantedBy: "op-ada"}}, nil
	}
	svcMergeBases = func(context.Context, string) (map[string]db.MergeBase, error) {
		return map[string]db.MergeBase{"u1": {
			Target: db.TargetZitadel, SubjectID: "u1", ObservedAt: time.Now(),
			Base: map[string]json.RawMessage{"p_wiki/read": json.RawMessage(`true`)},
		}}, nil
	}

	rows, err := DriftTriageQueue(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if rows[0].Provenance == nil || rows[0].Provenance.LastObservedAt != nil {
		t.Fatalf("nobody saw the target holding this one: %+v", rows[0].Provenance)
	}
}
