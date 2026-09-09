package handlers

import (
	"net/http"
	"strings"

	"syndra/internal/models"
	"syndra/internal/services"
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

// PublishBundleVersionRequest is the body of a publish.
type PublishBundleVersionRequest struct {
	Note string `json:"note"`
	// Migrate moves everyone currently holding the bundle onto the new version.
	// Absent means false: leaving people where they are is the answer that
	// changes nothing, and it is the safe default for a request that forgot to
	// say.
	Migrate bool `json:"migrate"`
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
		BundleID: bundleID,
		Note:     strings.TrimSpace(req.Note),
		Migrate:  req.Migrate,
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

	// A citation is required exactly when the rehearsal produced one, and
	// issuePlan produces one exactly when the rehearsal reached somebody. A
	// bundle nobody holds is the case: publishing it is a real act with no
	// subject to approve, and demanding an approval that cannot exist would put
	// the operator back on the dead end from the other side.
	if err := claimPublishPlan(r, planSurfaceBundlePublish, actor, req.PlanID, call); err != nil {
		writePlanCitationError(w, err)
		return
	}

	plan, version, err := svcPublishBundleVersion(r.Context(), actor, call)
	if err != nil {
		// "nothing to publish" is a conflict with the world, not a server
		// fault: somebody else published while this dialog was open, or the
		// draft was discarded.
		if strings.HasPrefix(err.Error(), "nothing to publish") {
			jsonErrorResponse(w, http.StatusConflict, "NOTHING_TO_PUBLISH", err.Error())
			return
		}
		jsonErrorResponse(w, http.StatusInternalServerError, "APPLY_ERROR", err.Error())
		return
	}
	jsonResponse(w, http.StatusOK, map[string]any{"plan": plan, "version": version})
}

// claimPublishPlan spends the approval a publish cites, or reports why it
// cannot be spent.
//
// The apply is a single atomic operation over a cohort, not a row-by-row walk,
// so the claimed subject rows are not what the write iterates — PublishBundleVersion
// recomputes under the access lock and moves the holders it finds. What the
// claim contributes is the guarantee that the world behind the approval has not
// moved: the request fingerprint binds the version contents and the cohort, and
// the per-subject fingerprints bind each holder's reviewed delta. If either
// moved, the apply is refused before the lock is taken and the operator reads a
// fresh plan instead.
func claimPublishPlan(
	r *http.Request,
	surface, actor, planID string,
	call services.PublishRequest,
) error {
	// Rehearsed once here, for the fingerprints the citation is checked against
	// and for the request fingerprint the approval was bound to. The verdicts
	// are not re-decided from it.
	live, _, err := svcRehearsePublish(r.Context(), call)
	if err != nil {
		return err
	}
	// Nobody holds it, so the rehearsal approved nobody and there is nothing to
	// verify. Publishing is still an act — it cuts the version future
	// assignments will pin — and it takes the same path a mapping definition
	// does.
	if len(live.Outcomes) == 0 {
		return nil
	}
	if strings.TrimSpace(planID) == "" {
		return errPlanCitationMissing
	}

	_, err = claimPlan(r.Context(), surface, actor, live.RequestFingerprint, planID,
		func() map[string]services.BulkOutcome { return indexOutcomes(live.Outcomes) })
	return err
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

	// A move NAMES its cohort, so unlike a publish it can never rehearse to
	// nobody with work still to do: no user ids is a validation failure above.
	live, err := svcRehearseMoveHolders(r.Context(), call)
	if err != nil {
		jsonErrorResponse(w, http.StatusInternalServerError, "PLAN_ERROR", err.Error())
		return
	}
	if strings.TrimSpace(req.PlanID) == "" {
		missingPlanCitation(w)
		return
	}
	if _, err := claimPlan(r.Context(), planSurfaceBundleMove, actor, live.RequestFingerprint, req.PlanID,
		func() map[string]services.BulkOutcome { return indexOutcomes(live.Outcomes) }); err != nil {
		writePlanCitationError(w, err)
		return
	}

	plan, err := svcMoveHolders(r.Context(), actor, call)
	if err != nil {
		jsonErrorResponse(w, http.StatusInternalServerError, "APPLY_ERROR", err.Error())
		return
	}
	jsonResponse(w, http.StatusOK, plan)
}
