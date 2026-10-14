package handlers

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"

	"syndra/internal/db"
)

// A connected system serves one Zitadel project (truenas-reconcile D3–D5).
//
// The project is needed to GIVE access, never to take it away: "take away",
// holds that deny, the dormant sweep, releasing a binding and lifecycle stay
// available on a system with no project, because a safety action that waits on
// configuration is a person keeping access nobody can remove.

// writeProjectRequired is the one refusal every giving write returns.
func writeProjectRequired(w http.ResponseWriter, target string) {
	jsonErrorResponse(w, http.StatusConflict, "PROJECT_REQUIRED",
		fmt.Sprintf("Choose %s's Zitadel project first. Until then nothing can give access there.", target))
}

// requireProject answers a giving write's gate for the target it names,
// writing the refusal itself; true means go on.
func requireProject(w http.ResponseWriter, r *http.Request, target string) bool {
	err := dbRequireTargetProject(r.Context(), target)
	switch {
	case err == nil:
		return true
	case errors.Is(err, db.ErrProjectRequired):
		writeProjectRequired(w, target)
	default:
		jsonErrorResponse(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
	}
	return false
}

// withTargetProject gates a path-keyed giving write: `/targets/{target}/…`.
func withTargetProject(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if requireProject(w, r, r.PathValue("target")) {
			next(w, r)
		}
	}
}

// mappingProject is the project a new mapping on target belongs to: the
// target's own. A caller may still name one (older clients do); it must be the
// same, because a mapping from another project would be refused by the
// database's composite key with a constraint message instead of a sentence.
func mappingProject(ctx context.Context, target, requested string) (string, error) {
	if err := dbRequireTargetProject(ctx, target); err != nil {
		return "", err
	}
	project, err := dbTargetProject(ctx, target)
	if err != nil {
		return "", err
	}
	if requested != "" && requested != project {
		return "", fmt.Errorf("%w: that role belongs to another Zitadel project; %s serves %s", db.ErrMappingInvalid, target, project)
	}
	return project, nil
}

type targetProjectRequest struct {
	ProjectID string `json:"project_id"`
}

// handleSetTargetProject sets the Zitadel project a system serves. A
// Syndra-side write that changes nobody's access by itself.
func handleSetTargetProject(w http.ResponseWriter, r *http.Request) {
	target := r.PathValue("target")
	var req targetProjectRequest
	if err := decodeJSONStrict(r.Body, &req); err != nil {
		jsonValidationErrorResponse(w, "Invalid JSON payload", map[string]string{"body": err.Error()})
		return
	}
	req.ProjectID = strings.TrimSpace(req.ProjectID)
	if req.ProjectID == "" {
		jsonValidationErrorResponse(w, "project_id is required", map[string]string{"project_id": "required"})
		return
	}
	if target == db.TargetZitadel {
		jsonErrorResponse(w, http.StatusConflict, "PROJECT_NOT_APPLICABLE",
			"Zitadel is where projects come from, so it doesn't serve one.")
		return
	}
	if _, err := addonsGet(target); err != nil {
		jsonErrorResponse(w, http.StatusNotFound, "TARGET_NOT_FOUND", target+" is not a connected system.")
		return
	}

	// Named from the directory, so a project that does not exist is refused
	// here rather than stored as an id nothing resolves.
	projects, err := directoryProjects(r.Context())
	if err != nil {
		jsonErrorResponse(w, http.StatusServiceUnavailable, "DIRECTORY_UNAVAILABLE",
			"Zitadel's projects can't be read right now, so the project can't be checked. Nothing was changed.")
		return
	}
	name := ""
	for _, p := range projects {
		if p.ID == req.ProjectID {
			name = p.Name
		}
	}
	if name == "" {
		jsonValidationErrorResponse(w, "There is no Zitadel project with that id.", map[string]string{"project_id": "unknown"})
		return
	}

	var taken db.ErrProjectTaken
	var exist db.ErrMappingsExist
	switch err := dbSetTargetProject(r.Context(), target, req.ProjectID); {
	case errors.As(err, &taken):
		jsonResponse(w, http.StatusConflict, ErrorResponse{Error: "PROJECT_TAKEN",
			Message: fmt.Sprintf("%s already serves %s. A project serves one system.", name, taken.Target),
			Details: map[string]string{"target": taken.Target}})
		return
	case errors.As(err, &exist):
		jsonResponse(w, http.StatusConflict, ErrorResponse{Error: "MAPPINGS_EXIST",
			Message: fmt.Sprintf("%s has %d role mapping(s). Remove them first: their roles belong to the current project.", target, exist.Count),
			Details: map[string]string{"count": strconv.Itoa(exist.Count)}})
		return
	case err != nil:
		jsonErrorResponse(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}
	log.Printf("[TARGETS] %s now serves Zitadel project %s (%s), set by %s", target, name, req.ProjectID, resolveActor(r, ""))
	jsonResponse(w, http.StatusOK, map[string]any{"target": target, "project": map[string]string{"id": req.ProjectID, "name": name}})
}
