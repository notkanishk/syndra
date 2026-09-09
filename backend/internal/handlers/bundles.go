package handlers

import (
	"errors"
	"net/http"
	"strings"

	"github.com/jackc/pgx/v5"

	"syndra/internal/db"
	"syndra/internal/models"
)

type AddRoleToBundleRequest struct {
	ProjectID string `json:"project_id"`
	RoleKey   string `json:"role_key"`
}

type AssignBundleRequest struct {
	BundleID string `json:"bundle_id"`
}

// CreateBundleRequest creates a bundle AND its first published version.
//
// Roles are required, and that is the change. A bundle is a set of roles handed
// out as one unit; one carrying no roles is not a smaller version of that, it is
// a thing that cannot do the job it names. Creating one empty put the operator
// somewhere with no good exit: assigning it granted nothing, and the roles they
// then added became "unpublished changes" against a v1 that had never described
// anything.
type CreateBundleRequest struct {
	Name             string                   `json:"name"`
	Description      string                   `json:"description"`
	ConfirmationMode string                   `json:"confirmation_mode,omitempty"`
	Roles            []AddRoleToBundleRequest `json:"roles"`
}

// UpdateBundleRequest carries no confirmation_mode: that is changed through
// POST /policies/confirmation-mode, which is where every other rule and bundle changes it, and
// a second way in would be a second thing to keep consistent.
type UpdateBundleRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

func handleGetBundles(w http.ResponseWriter, r *http.Request) {
	bundles, err := dbGetAllBundles(r.Context())
	if err != nil {
		jsonErrorResponse(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}
	if bundles == nil {
		jsonResponse(w, http.StatusOK, []interface{}{})
		return
	}

	// Holder counts are decoration on this list, not its subject: if the count
	// query fails the bundles still render, with zeros, rather than the whole
	// screen turning into an error over a number.
	if counts, err := dbGetBundleHolderCounts(r.Context()); err == nil {
		for i := range bundles {
			bundles[i].HolderCount = counts[bundles[i].ID]
		}
	}
	// Same treatment for the version numbers: a failed count leaves zeros
	// rather than failing the list. Stale holders is the one an operator scans
	// for — "eleven people never got the edit you made last term".
	if stale, err := dbGetStaleHolderCounts(r.Context()); err == nil {
		for i := range bundles {
			bundles[i].StaleHolders = stale[bundles[i].ID]
		}
	}
	for i := range bundles {
		if draft, err := svcBundleDraft(r.Context(), bundles[i].ID); err == nil {
			bundles[i].UnpublishedChanges = len(draft.Added) + len(draft.Removed)
		}
	}

	jsonResponse(w, http.StatusOK, bundles)
}

func handleCreateBundle(w http.ResponseWriter, r *http.Request) {
	var req CreateBundleRequest
	if err := decodeJSONStrict(r.Body, &req); err != nil {
		jsonValidationErrorResponse(w, "Invalid JSON payload", map[string]string{"body": err.Error()})
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	problems := map[string]string{}
	if req.Name == "" {
		problems["name"] = "required"
	}
	roles, roleProblem := normalizeNewBundleRoles(req.Roles)
	if roleProblem != "" {
		problems["roles"] = roleProblem
	}
	if len(problems) > 0 {
		jsonValidationErrorResponse(w, "A bundle needs a name and at least one role", problems)
		return
	}

	// Creating a bundle triggers no cascade — nobody holds it yet, so its roles
	// reach no member. Mode seeds the row for the future add-role/assign
	// cascades. Inherits the global default unless overridden.
	mode, err := resolveConfirmationMode(r.Context(), req.ConfirmationMode)
	if err != nil {
		jsonErrorResponse(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}
	id, err := dbCreateBundle(r.Context(), req.Name, req.Description, mode, roles)
	if err != nil {
		jsonErrorResponse(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}

	actor := getAdminUserID(r.Context())
	if actor == "" {
		actor = "system"
	}
	_ = dbInsertAuditLog(r.Context(), actor, "-", "bundle.created", id)
	jsonResponse(w, http.StatusCreated, map[string]string{"id": id})
}

// normalizeNewBundleRoles validates and de-duplicates the roles a bundle is
// being created with, returning the problem string for the `roles` field when
// they will not do.
//
// De-duplication rather than refusal: the same role named twice is a client
// sending the same tick twice, not an operator asking for something
// contradictory, and both target tables would reject the second row anyway.
func normalizeNewBundleRoles(in []AddRoleToBundleRequest) ([]models.BundleRole, string) {
	if len(in) == 0 {
		return nil, "at least one"
	}
	seen := map[string]bool{}
	out := make([]models.BundleRole, 0, len(in))
	for _, role := range in {
		projectID := strings.TrimSpace(role.ProjectID)
		roleKey := strings.TrimSpace(role.RoleKey)
		if projectID == "" || roleKey == "" {
			return nil, "each role needs a project_id and a role_key"
		}
		key := projectID + "\x00" + roleKey
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, models.BundleRole{ProjectID: projectID, RoleKey: roleKey})
	}
	return out, ""
}

// handleUpdateBundle renames a bundle and rewrites its description. Nothing else: the roles are
// the working copy's business and the holders are publish's, and folding either into a rename
// would make correcting a typo a thing an operator has to think twice about.
// PUT /api/v1/bundles/{id}
func handleUpdateBundle(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !trimmedNonEmpty(id) {
		jsonValidationErrorResponse(w, "id path parameter is required", map[string]string{"id": "required"})
		return
	}

	var req UpdateBundleRequest
	if err := decodeJSONStrict(r.Body, &req); err != nil {
		jsonValidationErrorResponse(w, "Invalid JSON payload", map[string]string{"body": err.Error()})
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		jsonValidationErrorResponse(w, "name is required", map[string]string{"name": "required"})
		return
	}

	if err := dbUpdateBundle(r.Context(), id, req.Name, req.Description); err != nil {
		switch {
		case errors.Is(err, pgx.ErrNoRows):
			jsonErrorResponse(w, http.StatusNotFound, "NOT_FOUND", "Bundle not found")
		case db.IsUniqueViolation(err):
			jsonErrorResponse(w, http.StatusConflict, "CONFLICT", "Another bundle already has that name")
		default:
			jsonErrorResponse(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		}
		return
	}

	actor := getAdminUserID(r.Context())
	if actor == "" {
		actor = "system"
	}
	_ = dbInsertAuditLog(r.Context(), actor, "-", "bundle.updated", id)
	jsonResponse(w, http.StatusOK, map[string]string{"message": "Bundle updated"})
}

// handleDeleteBundle retires a bundle and revokes what only it was granting, in one transaction.
//
// The welcome flag is reported back rather than guarded against. Refusing to delete the welcome
// bundle would be a rule an operator could not satisfy — the only way to clear the flag is to
// promote a different bundle, which a makerspace with one bundle cannot do. So the deletion goes
// through and the response says what else went with it, and the screen says it before the click.
// DELETE /api/v1/bundles/{id}
func handleDeleteBundle(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !trimmedNonEmpty(id) {
		jsonValidationErrorResponse(w, "id path parameter is required", map[string]string{"id": "required"})
		return
	}

	// Read before the delete: afterwards there is nothing left to ask whether it was the
	// welcome bundle, and the caller needs to be told.
	bundle, err := dbGetBundleByID(r.Context(), id)
	if err != nil {
		jsonErrorResponse(w, http.StatusNotFound, "NOT_FOUND", "Bundle not found")
		return
	}

	actor := getAdminUserID(r.Context())
	if actor == "" {
		actor = "system"
	}
	cascade, err := svcCascadeBundleDeleted(r.Context(), actor, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			jsonErrorResponse(w, http.StatusNotFound, "NOT_FOUND", "Bundle not found")
			return
		}
		jsonErrorResponse(w, http.StatusInternalServerError, "CASCADE_ERROR", err.Error())
		return
	}

	jsonResponse(w, http.StatusOK, map[string]any{
		"message":     "Bundle deleted",
		"was_welcome": bundle.IsWelcome,
		"cascade":     cascade,
	})
}

// handleGetBundleRoles answers one of two different questions, and the caller
// has to say which.
//
// The default is the WORKING COPY — what the next version will contain. That is
// what the bundle editor is editing, and it is the honest answer there.
//
// `?published=true` is what the bundle GRANTS today: the latest published
// version. Every surface that previews an assignment must ask for this one,
// because an assignment pins the published version (db.LatestVersionRoles) and
// nothing else. The assign dialog asked the default and listed roles the apply
// would never hand out — it promised an operator the two unpublished roles they
// had just added, beside a bundle screen calling those same roles unpublished.
func handleGetBundleRoles(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !trimmedNonEmpty(id) {
		jsonValidationErrorResponse(w, "id path parameter is required", map[string]string{"id": "required"})
		return
	}

	var roles []models.BundleRole
	var err error
	if r.URL.Query().Get("published") == "true" {
		// The version is discarded: the caller asked what the bundle grants, and
		// LatestVersionRoles reads both in one go precisely so nobody resolves
		// "latest" twice and pins one version's number to another's roles.
		_, roles, err = dbLatestVersionRoles(r.Context(), id)
	} else {
		roles, err = dbGetRolesForBundle(r.Context(), id)
	}
	if err != nil {
		jsonErrorResponse(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}
	// Never `null`. A Go nil slice marshals to JSON null, and the clients call
	// `.length` and `.map` on this — the same boundary that took the bundles
	// screen down once already.
	if roles == nil {
		roles = []models.BundleRole{}
	}
	jsonResponse(w, http.StatusOK, roles)
}

func handleAddRoleToBundle(w http.ResponseWriter, r *http.Request) {
	bundleID := r.PathValue("id")
	if strings.TrimSpace(bundleID) == "" {
		jsonValidationErrorResponse(w, "id path parameter is required", map[string]string{"id": "required"})
		return
	}

	var req AddRoleToBundleRequest
	if err := decodeJSONStrict(r.Body, &req); err != nil {
		jsonValidationErrorResponse(w, "Invalid JSON payload", map[string]string{"body": err.Error()})
		return
	}
	if !trimmedNonEmpty(req.ProjectID) || !trimmedNonEmpty(req.RoleKey) {
		jsonValidationErrorResponse(w, "project_id and role_key are required", map[string]string{
			"project_id": "required",
			"role_key":   "required",
		})
		return
	}

	actor := getAdminUserID(r.Context())
	if actor == "" {
		actor = "system"
	}
	// This edits the working copy and reaches nobody. The consequence is a
	// separate, rehearsed step — POST /bundles/{id}/publish — so that adding six
	// roles is one decision about fourteen people rather than six.
	if err := svcEditBundleWorkingCopy(r.Context(), actor, bundleID, req.ProjectID, req.RoleKey, true); err != nil {
		jsonErrorResponse(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}
	draft, err := svcBundleDraft(r.Context(), bundleID)
	if err != nil {
		jsonErrorResponse(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}
	jsonResponse(w, http.StatusOK, map[string]any{
		"message": "Role added to the bundle's working copy. Publish a version to apply it.",
		"draft":   draft,
	})
}

func handleGetUserBundles(w http.ResponseWriter, r *http.Request) {
	userID := r.PathValue("id")
	bundles, err := dbGetBundlesForUser(r.Context(), userID)
	if err != nil {
		jsonErrorResponse(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}
	jsonResponse(w, http.StatusOK, bundles)
}

func handleAssignBundleToUser(w http.ResponseWriter, r *http.Request) {
	userID := r.PathValue("id")
	if strings.TrimSpace(userID) == "" {
		jsonValidationErrorResponse(w, "id path parameter is required", map[string]string{"id": "required"})
		return
	}
	var req AssignBundleRequest
	if err := decodeJSONStrict(r.Body, &req); err != nil {
		jsonValidationErrorResponse(w, "Invalid JSON payload", map[string]string{"body": err.Error()})
		return
	}
	if !trimmedNonEmpty(req.BundleID) {
		jsonValidationErrorResponse(w, "bundle_id is required", map[string]string{"bundle_id": "required"})
		return
	}

	actor := getAdminUserID(r.Context())
	if actor == "" {
		actor = "system"
	}
	// The cascade owns the mutation now (assign + per-role outbox rows commit in one tx), then
	// (auto mode) drains those rows. Enqueue failure rolls back the mutation → 500; a drain
	// failure rides in cascade.drain and is not fatal.
	cascade, err := svcCascadeBundleAssigned(r.Context(), actor, userID, req.BundleID)
	if err != nil {
		jsonErrorResponse(w, http.StatusInternalServerError, "CASCADE_ERROR", err.Error())
		return
	}
	// Idempotent, and it says which of the two happened. "Bundle assigned"
	// after a no-op reads as a change that was made, which is what the caller
	// will tell the operator.
	message := "Bundle assigned to user"
	if cascade.NoOp {
		message = "Already holds this bundle — nothing changed."
	}
	jsonResponse(w, http.StatusOK, map[string]any{
		"message": message,
		"cascade": cascade,
	})
}

// handleSetWelcomeBundle marks a bundle as the welcome bundle. Clears any
// previous welcome flag in the same transaction (see db.SetWelcomeBundle).
// PUT /api/v1/bundles/{id}/welcome
func handleSetWelcomeBundle(w http.ResponseWriter, r *http.Request) {
	bundleID := r.PathValue("id")
	if strings.TrimSpace(bundleID) == "" {
		jsonValidationErrorResponse(w, "id path parameter is required", map[string]string{"id": "required"})
		return
	}

	if err := dbSetWelcomeBundle(r.Context(), bundleID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			jsonErrorResponse(w, http.StatusNotFound, "NOT_FOUND", "Bundle not found")
			return
		}
		jsonErrorResponse(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}

	actor := getAdminUserID(r.Context())
	if actor == "" {
		actor = "system"
	}
	_ = dbInsertAuditLog(r.Context(), actor, "-", "bundle.welcome_set", bundleID)
	jsonResponse(w, http.StatusOK, map[string]string{"message": "Welcome bundle set"})
}
