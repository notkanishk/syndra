package handlers

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"syndra/internal/directory"
	"syndra/internal/models"
)

// The API took any project and role key; only the pickers kept to the
// catalogue. A role Zitadel doesn't offer is refused before anything is
// asked for, given or approved, and an unread catalogue refuses too.

type taggedSource struct {
	directory.Source
	tag string
}

func (s taggedSource) Tag() string { return s.tag }

func offering(t *testing.T, tag string, projects []models.ProjectCatalog, err error) {
	t.Helper()
	origCheck, origProjects, origSource, origConfigured := checkRoleOffered, directoryProjects, directorySource, systemZitadelConfigured
	t.Cleanup(func() {
		checkRoleOffered, directoryProjects, directorySource, systemZitadelConfigured = origCheck, origProjects, origSource, origConfigured
	})
	checkRoleOffered = roleOfferedInDirectory
	directoryProjects = func(context.Context) ([]models.ProjectCatalog, error) { return projects, err }
	real := origSource()
	directorySource = func() directory.Source { return taggedSource{Source: real, tag: tag} }
	systemZitadelConfigured = func() bool { return true }
}

var laser = []models.ProjectCatalog{{ID: "pLaser", Roles: []models.ProjectRole{{Key: "trained"}}}}

func TestOnlyARoleZitadelOffersPasses(t *testing.T) {
	ctx := context.Background()
	offering(t, "zitadel", laser, nil)
	if err := checkRoleOffered(ctx, "pLaser", "trained"); err != nil {
		t.Fatalf("an offered role passes, got %v", err)
	}
	if err := checkRoleOffered(ctx, "pLaser", "gone"); !errors.Is(err, errRoleNotOffered) {
		t.Fatalf("a role not offered is refused, got %v", err)
	}
	if err := checkRoleOffered(ctx, "pOther", "trained"); !errors.Is(err, errRoleNotOffered) {
		t.Fatalf("the same key in another project is another role, got %v", err)
	}

	offering(t, "zitadel", nil, errors.New("timeout"))
	if err := checkRoleOffered(ctx, "pLaser", "trained"); !errors.Is(err, errCatalogueUnread) {
		t.Fatalf("an unread catalogue fails closed, got %v", err)
	}
	// Serving the demo catalogue while Zitadel is configured is not a reading.
	offering(t, "demo", laser, nil)
	if err := checkRoleOffered(ctx, "pLaser", "trained"); !errors.Is(err, errCatalogueUnread) {
		t.Fatalf("a degraded directory fails closed, got %v", err)
	}
}

func TestARoleNotOfferedIsRefusedWhenAskedGivenOrApproved(t *testing.T) {
	resetAccessDeps(t)
	t.Setenv("ZITADEL_DOMAIN", "example.zitadel.cloud")
	offering(t, "zitadel", laser, nil)
	dbRequestHolding = func(context.Context, string, string, string) (models.RequestHolding, error) {
		return models.RequestHolding{Kind: models.HoldingNone}, nil
	}
	wrote := false
	dbCreateAccessRequest = func(context.Context, string, string, string, string, *int) (string, error) {
		wrote = true
		return "req-1", nil
	}

	rr := httptest.NewRecorder()
	handleCreateAccessRequest(rr, memberContext(httptest.NewRequest(http.MethodPost, "/api/v1/requests",
		strings.NewReader(`{"project_id":"pLaser","role_key":"gone","justification":"x"}`)), "member-1"))
	if rr.Code != http.StatusConflict || !strings.Contains(rr.Body.String(), "ROLE_NOT_OFFERED") || wrote {
		t.Fatalf("asking: want 409 ROLE_NOT_OFFERED and nothing filed, got %d %s", rr.Code, rr.Body.String())
	}

	dbGetAccessRequestByID = func(context.Context, string) (models.AccessRequest, error) {
		return models.AccessRequest{ID: "r1", RequesterID: "u1", ProjectID: "pLaser", RoleKey: "gone", Status: "pending"}, nil
	}
	if err := resolveOneAccessRequest(context.Background(), "r1", "approved", "op", ""); !errors.Is(err, errRoleNotOffered) {
		t.Fatalf("approving: want errRoleNotOffered, got %v", err)
	}
}

func TestTheDecisionPlanAgreesWithTheDecision(t *testing.T) {
	withdrawn := pendingRequest("r2", "u2", "trained")
	withdrawn.Status = "withdrawn"
	stubRequestLookup(t, map[string]models.AccessRequest{
		"r1": pendingRequest("r1", "u1", "gone"),
		"r2": withdrawn,
	})
	offering(t, "zitadel", laser, nil)

	rr := httptest.NewRecorder()
	handleBulkDecideRequests(rr, bulkDecisionReq(t, `{"ids":["r1","r2"],"status":"approved"}`, false))
	plan := decodeBulkPlan(t, rr)
	byID := map[string]string{}
	for _, o := range plan.Outcomes {
		byID[o.Email+string(o.Effect)] = o.Detail
	}
	if plan.Summary.Blocked != 1 || plan.Summary.NoChange != 1 || plan.Summary.Apply != 0 {
		t.Fatalf("a role not offered is blocked and a withdrawn request is no change, got %+v %v", plan.Summary, byID)
	}
}
