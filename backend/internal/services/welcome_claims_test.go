package services

import (
	"context"
	"errors"
	"reflect"
	"sort"
	"testing"
	"time"

	"syndra/internal/claimserve"
	"syndra/internal/db"
	"syndra/internal/models"
	"syndra/internal/services/propagation"
)

// welcomeWorld stubs every read WelcomeClaims makes: the switch on, bundle b1
// (auto) giving p1:maker and p1:safety, a rule p1:maker → p2:visitor, apps in
// p1, p2 and p3, and settings passed in for the rest.
type welcomeWorld struct {
	switchValue string
	model       string
	waits       []db.RoleWait
	retired     bool
	noBundle    bool
	appsErr     error
}

func stubWelcome(t *testing.T, w welcomeWorld) {
	t.Helper()
	resetCascadeDeps(t)
	origSetting, origBundle, origWaits, origApps := svcGetConfigSetting, svcGetWelcomeBundle, svcListRoleWaits, svcApplications
	t.Cleanup(func() {
		svcGetConfigSetting, svcGetWelcomeBundle, svcListRoleWaits, svcApplications = origSetting, origBundle, origWaits, origApps
	})
	svcGetConfigSetting = func(_ context.Context, key string) (string, error) {
		switch key {
		case ConfigKeyWelcomeClaims:
			return w.switchValue, nil
		case db.ConfigKeyConfirmationModel:
			return w.model, nil
		}
		return "", nil
	}
	svcGetWelcomeBundle = func(context.Context) (string, error) {
		if w.noBundle {
			return "", db.ErrNoWelcomeBundleConfigured
		}
		return "b1", nil
	}
	svcGetBundleByID = func(_ context.Context, id string) (models.Bundle, error) {
		b := models.Bundle{ID: id, Name: "Welcome"}
		if w.retired {
			at := time.Now()
			b.RetiredAt = &at
		}
		return b, nil
	}
	svcLatestVersionRoles = func(context.Context, string) (models.BundleVersion, []models.BundleRole, error) {
		return models.BundleVersion{ID: "v1", Version: 1}, []models.BundleRole{
			{BundleID: "b1", ProjectID: "p1", RoleKey: "maker"},
			{BundleID: "b1", ProjectID: "p1", RoleKey: "safety"},
		}, nil
	}
	svcGetActiveMappingRules = func(context.Context) ([]models.MappingRule, error) {
		return []models.MappingRule{{ID: "r1", SourceProject: "p1", SourceRole: "maker", TargetProject: "p2", TargetRole: "visitor"}}, nil
	}
	svcListRoleWaits = func(context.Context) ([]db.RoleWait, error) { return w.waits, nil }
	svcApplications = func(context.Context) ([]models.ApplicationCatalog, error) {
		return []models.ApplicationCatalog{
			{ID: "a1", ProjectID: "p1", ClientID: "c1"},
			{ID: "a2", ProjectID: "p2", ClientID: "c2"},
			{ID: "a3", ProjectID: "p3", ClientID: "c3"},
			{ID: "a4", ProjectID: "p1"}, // no client id: never a key
		}, w.appsErr
	}
}

func rolesByProject(w claimserve.Welcome) map[string][]string {
	out := map[string][]string{}
	for p, f := range w.Facts {
		out[p] = f.Roles
	}
	return out
}

func TestWelcomeClaims_Off(t *testing.T) {
	stubWelcome(t, welcomeWorld{switchValue: ""})
	svcGetWelcomeBundle = func(context.Context) (string, error) {
		t.Fatal("off reads nothing else")
		return "", nil
	}
	w, err := WelcomeClaims(context.Background())
	if err != nil || !reflect.DeepEqual(w, claimserve.Welcome{}) {
		t.Fatalf("got %+v, %v", w, err)
	}
}

func TestWelcomeClaims_OnSendsWhatTheBundleGivesWithoutApproval(t *testing.T) {
	stubWelcome(t, welcomeWorld{switchValue: "true",
		waits: []db.RoleWait{{ProjectID: "p1", RoleKey: "safety"}}})
	w, err := WelcomeClaims(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	want := map[string][]string{"p1": {"maker"}, "p2": {"visitor"}}
	if !w.On || !reflect.DeepEqual(rolesByProject(w), want) {
		t.Fatalf("roles %v, want %v (safety waits for approval; visitor is the rule's)", rolesByProject(w), want)
	}
	if !reflect.DeepEqual(w.Facts["p1"].Bundles, []string{"Welcome"}) || w.Facts["p2"].Bundles != nil {
		t.Fatalf("bundles name the welcome bundle only where it gives a role itself: %+v", w.Facts)
	}
	if !reflect.DeepEqual(w.Clients, map[string]string{"c1": "p1", "c2": "p2"}) {
		t.Fatalf("clients %v: only apps of projects with welcome roles", w.Clients)
	}
}

func TestWelcomeClaims_NothingToSend(t *testing.T) {
	everyRole := []db.RoleWait{{ProjectID: "p1", RoleKey: "maker"}, {ProjectID: "p1", RoleKey: "safety"}, {ProjectID: "p2", RoleKey: "visitor"}}
	for name, w := range map[string]welcomeWorld{
		"every role waits":            {waits: everyRole},
		"no welcome bundle":           {noBundle: true},
		"the welcome bundle, retired": {retired: true},
	} {
		t.Run(name, func(t *testing.T) {
			w.switchValue = "true"
			stubWelcome(t, w)
			got, err := WelcomeClaims(context.Background())
			if err != nil || !reflect.DeepEqual(got, claimserve.Welcome{On: true}) {
				t.Fatalf("got %+v, %v", got, err)
			}
		})
	}
}

func TestWelcomeClaims_AReadFailureIsAnError(t *testing.T) {
	stubWelcome(t, welcomeWorld{switchValue: "true", appsErr: errors.New("zitadel down")})
	if _, err := WelcomeClaims(context.Background()); err == nil {
		t.Fatal("a failed read must keep the last publish, not publish a guess")
	}
}

// The welcome roles are exactly what onboarding's cascade sends a person who
// holds nothing: the same closure, the same latest published version.
func TestWelcomeClaims_EqualsWhatOnboardingWouldSend(t *testing.T) {
	stubWelcome(t, welcomeWorld{switchValue: "true"})
	svcGetDirectGrantsForUser = noDirects
	svcGetUserBundleRolesGrouped = noBundleRoles
	var sent []string
	svcAssignBundleAndEnqueue = func(_ context.Context, _, _, _, _, _ string, ps []db.EnqueueParams) ([]string, bool, error) {
		for _, p := range ps {
			for _, r := range p.RoleKeys {
				sent = append(sent, p.ProjectID+":"+r)
			}
		}
		return []string{"o1"}, true, nil
	}
	svcDrainBatch = func(context.Context, []string) (propagation.DrainResult, error) {
		return propagation.DrainResult{}, nil
	}
	if _, err := CascadeBundleAssignedToUser(context.Background(), db.WelcomeActor, "new", "b1"); err != nil {
		t.Fatal(err)
	}
	w, err := WelcomeClaims(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	var welcome []string
	for p, roles := range rolesByProject(w) {
		for _, r := range roles {
			welcome = append(welcome, p+":"+r)
		}
	}
	sort.Strings(sent)
	sort.Strings(welcome)
	if !reflect.DeepEqual(sent, welcome) {
		t.Fatalf("onboarding sends %v, the welcome switch %v", sent, welcome)
	}
}
