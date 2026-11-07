package handlers

import (
	"errors"
	"net/http"
	"strings"

	"syndra/internal/models"
	"syndra/internal/services"

	"github.com/jackc/pgx/v5"
)

// Bundle version surfaces.
//
// Publishing and moving holders both follow the rehearsal contract the rest of
// the product uses: POST returns a plan, POST?apply=true returns the same plan
// with the writes done. There is no `apply` body field — applying is a property
// of the request, not of the payload, so a plan cannot be replayed into an
// accidental write.

// GET /api/v1/bundles/{id}/versions
func handleGetBundleVersions(w http.ResponseWriter, r *http.Request) {
	bundleID := r.PathValue("id")
	if !trimmedNonEmpty(bundleID) {
		jsonValidationErrorResponse(w, "id path parameter is required", map[string]string{"id": "required"})
		return
	}

	versions, err := svcListBundleVersions(r.Context(), bundleID)
	if err != nil {
		jsonErrorResponse(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}

	// Contents come with the list. A version is only meaningful as what it
	// contained, and a UI that had to fetch each one to say so would either
	// fan out N requests or show version numbers with nothing behind them.
	for i := range versions {
		roles, err := svcGetRolesForVersion(r.Context(), versions[i].ID)
		if err != nil {
			jsonErrorResponse(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
			return
		}
		if roles == nil {
			roles = []models.BundleRole{}
		}
		versions[i].Roles = roles
	}
	if versions == nil {
		versions = []models.BundleVersion{}
	}
	jsonResponse(w, http.StatusOK, versions)
}

// GET /api/v1/bundles/{id}/holders — who is on which version.
func handleGetBundleHolders(w http.ResponseWriter, r *http.Request) {
	bundleID := r.PathValue("id")
	if !trimmedNonEmpty(bundleID) {
		jsonValidationErrorResponse(w, "id path parameter is required", map[string]string{"id": "required"})
		return
	}
	holders, err := svcBundleHolders(r.Context(), bundleID)
	if err != nil {
		jsonErrorResponse(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}
	if holders == nil {
		holders = []models.BundleHolder{}
	}
	jsonResponse(w, http.StatusOK, holders)
}

// GET /api/v1/bundles/{id}/draft — the unpublished difference.
func handleGetBundleDraft(w http.ResponseWriter, r *http.Request) {
	bundleID := r.PathValue("id")
	if !trimmedNonEmpty(bundleID) {
		jsonValidationErrorResponse(w, "id path parameter is required", map[string]string{"id": "required"})
		return
	}
	draft, err := svcBundleDraft(r.Context(), bundleID)
	if err != nil {
		jsonErrorResponse(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}
	jsonResponse(w, http.StatusOK, draft)
}

// DELETE /api/v1/bundles/{id}/draft — "Discard changes": the working copy goes
// back to exactly the latest published version. Nobody's access is touched, so
// there is no rehearsal and no cascade. Returns the fresh (empty) draft; with
// nothing unpublished it is the same answer, still 200.
func handleDiscardBundleDraft(w http.ResponseWriter, r *http.Request) {
	bundleID := r.PathValue("id")
	if !trimmedNonEmpty(bundleID) {
		jsonValidationErrorResponse(w, "id path parameter is required", map[string]string{"id": "required"})
		return
	}
	actor := getAdminUserID(r.Context())
	if actor == "" {
		actor = "system"
	}
	if err := svcDiscardBundleDraft(r.Context(), actor, bundleID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			jsonErrorResponse(w, http.StatusNotFound, "NOT_FOUND", "bundle not found")
			return
		}
		jsonErrorResponse(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}
	handleGetBundleDraft(w, r)
}

// PublishBundleVersionRequest is the body of a publish.
type PublishBundleVersionRequest struct {
	Note string `json:"note"`
	// Migrate moves everyone currently holding the bundle onto the new version.
	// Absent means false: leaving people where they are is the answer that
	// changes nothing, and it is the safe default for a request that forgot to
	// say.
	Migrate bool `json:"migrate"`
	// Includers answers, for every bundle whose working copy includes this
	// one, "update" or "leave" (B8). Required for each, no default.
	Includers []services.IncluderChoice `json:"includers,omitempty"`
	// PlanID cites the rehearsal being applied. Required with ?apply=true
	// whenever the rehearsal reached anybody.
	PlanID string `json:"plan_id,omitempty"`
	// AcknowledgeScope is the operator saying the affected-holder count out
	// loud, required only above the configured limit.
	AcknowledgeScope bool `json:"acknowledge_scope,omitempty"`
}

// POST /api/v1/bundles/{id}/publish[?apply=true]
//
// Publishing joins the plan gate (design §8) the same way every other rehearsed
// surface has. It was the last mutation in the product where the apply
// recomputed its own diff and wrote from it — which was two separate problems.
//
// The one an operator hit: the shared dialog disables Apply without a
// `plan_id`, correctly, and this endpoint issued none, so every publish that
// reached anybody was a dead end on screen. Exactly the failure the mapping
// rollback hit and was fixed for; this surface was missed.
//
// The one nobody hit yet: an operator read what publishing would do to fourteen
// people, and the apply recomputed it against a world free to have moved in
// between. The lock inside PublishBundleVersion makes the write atomic, which is
// not the same as making it the write that was approved.
func handlePublishBundleVersion(w http.ResponseWriter, r *http.Request) {
	bundleID := r.PathValue("id")
	if !trimmedNonEmpty(bundleID) {
		jsonValidationErrorResponse(w, "id path parameter is required", map[string]string{"id": "required"})
		return
	}

	var req PublishBundleVersionRequest
	if err := decodeJSONStrict(r.Body, &req); err != nil {
		jsonValidationErrorResponse(w, "Invalid JSON payload", map[string]string{"body": err.Error()})
		return
	}

	call := services.PublishRequest{
		BundleID:  bundleID,
		Note:      strings.TrimSpace(req.Note),
		Migrate:   req.Migrate,
		Includers: req.Includers,
	}
	actor := getAdminUserID(r.Context())
	if actor == "" {
		actor = "system"
	}

	if r.URL.Query().Get("apply") != "true" {
		plan, draft, err := svcRehearsePublish(r.Context(), call)
		if err != nil {
			jsonErrorResponse(w, http.StatusInternalServerError, "PLAN_ERROR", err.Error())
			return
		}
		if err := issuePlan(r.Context(), planSurfaceBundlePublish, actor,
			plan.RequestFingerprint, req.AcknowledgeScope, &plan); err != nil {
			writePlanIssueError(w, err)
			return
		}
		jsonResponse(w, http.StatusOK, map[string]any{"plan": plan, "draft": draft})
		return
	}

	// Through the planner (task 1.3): under the lock it reads the version and
	// the holders again, checks them against the approval (the request
	// fingerprint binds the contents and the cohort), plans every holder again
	// and writes only the reviewed Change. A bundle nobody holds has no plan
	// to cite: publishing it is still an act.
	plan, ok := applyPlanned(w, r, services.BulkRequest{Op: services.OpPublish, BundleID: bundleID, Note: call.Note,
		Migrate: call.Migrate, Includers: call.Includers, Actor: actor}, planCitation(planSurfaceBundlePublish, actor, "", req.PlanID))
	if ok {
		jsonResponse(w, http.StatusOK, map[string]any{"plan": plan, "version": plan.Version})
	}
}

// MoveHoldersRequest repins named holders onto one version.
type MoveHoldersRequest struct {
	VersionID string   `json:"version_id"`
	UserIDs   []string `json:"user_ids"`
	// PlanID cites the rehearsal being applied. Required with ?apply=true.
	PlanID string `json:"plan_id,omitempty"`
	// AcknowledgeScope is the operator saying the affected count out loud.
	AcknowledgeScope bool `json:"acknowledge_scope,omitempty"`
}

// POST /api/v1/bundles/{id}/holders/move[?apply=true]
func handleMoveBundleHolders(w http.ResponseWriter, r *http.Request) {
	bundleID := r.PathValue("id")
	if !trimmedNonEmpty(bundleID) {
		jsonValidationErrorResponse(w, "id path parameter is required", map[string]string{"id": "required"})
		return
	}

	var req MoveHoldersRequest
	if err := decodeJSONStrict(r.Body, &req); err != nil {
		jsonValidationErrorResponse(w, "Invalid JSON payload", map[string]string{"body": err.Error()})
		return
	}
	problems := map[string]string{}
	if !trimmedNonEmpty(req.VersionID) {
		problems["version_id"] = "required"
	}
	if len(req.UserIDs) == 0 {
		problems["user_ids"] = "at least one"
	}
	if len(problems) > 0 {
		jsonValidationErrorResponse(w, "version_id and user_ids are required", problems)
		return
	}

	call := services.MoveHoldersRequest{
		BundleID:  bundleID,
		VersionID: req.VersionID,
		UserIDs:   req.UserIDs,
	}
	actor := getAdminUserID(r.Context())
	if actor == "" {
		actor = "system"
	}

	if r.URL.Query().Get("apply") != "true" {
		plan, err := svcRehearseMoveHolders(r.Context(), call)
		if err != nil {
			jsonErrorResponse(w, http.StatusInternalServerError, "PLAN_ERROR", err.Error())
			return
		}
		if err := issuePlan(r.Context(), planSurfaceBundleMove, actor,
			plan.RequestFingerprint, req.AcknowledgeScope, &plan); err != nil {
			writePlanIssueError(w, err)
			return
		}
		jsonResponse(w, http.StatusOK, plan)
		return
	}
	if strings.TrimSpace(req.PlanID) == "" {
		missingPlanCitation(w)
		return
	}
	applyPlannedBulk(w, r, services.BulkRequest{Op: services.OpMove, BundleID: bundleID, VersionID: req.VersionID,
		UserIDs: req.UserIDs, Actor: actor}, planCitation(planSurfaceBundleMove, actor, "", req.PlanID), http.StatusOK)
}
