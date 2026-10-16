package handlers

import (
	"errors"
	"net/http"

	"syndra/internal/directory"
	"syndra/internal/services"
)

// viewError answers a failed read. A directory Zitadel has never answered is
// 503 DIRECTORY_UNREACHABLE with no rows, so the page shows its Notice and
// nothing else (N23); anything else keeps the status its handler gives it.
func viewError(w http.ResponseWriter, err error, status int, code string) {
	if errors.Is(err, directory.ErrUnreachable) {
		jsonErrorResponse(w, http.StatusServiceUnavailable, "DIRECTORY_UNREACHABLE",
			"Zitadel isn't answering, and Syndra hasn't read it yet.")
		return
	}
	jsonErrorResponse(w, status, code, err.Error())
}

func handleGetCatalog(w http.ResponseWriter, r *http.Request) {
	catalog, err := services.Catalog(r.Context())
	if err != nil {
		viewError(w, err, http.StatusInternalServerError, "VIEW_ERROR")
		return
	}
	jsonResponse(w, http.StatusOK, catalog)
}

func handleGetUsers(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	users, err := services.ListUsers(r.Context(), query)
	if err != nil {
		viewError(w, err, http.StatusInternalServerError, "VIEW_ERROR")
		return
	}

	jsonResponse(w, http.StatusOK, users)
}

func handleGetUserAccess(w http.ResponseWriter, r *http.Request) {
	userID := r.PathValue("id")
	view, err := services.ExplainUserAccess(r.Context(), userID)
	if err != nil {
		viewError(w, err, http.StatusNotFound, "NOT_FOUND")
		return
	}

	jsonResponse(w, http.StatusOK, view)
}

func handleGetApplications(w http.ResponseWriter, r *http.Request) {
	apps, err := services.ListApplications(r.Context())
	if err != nil {
		viewError(w, err, http.StatusInternalServerError, "VIEW_ERROR")
		return
	}

	jsonResponse(w, http.StatusOK, apps)
}

func handleSimulateApplication(w http.ResponseWriter, r *http.Request) {
	appID := r.PathValue("id")
	userID := r.URL.Query().Get("user_id")
	if userID == "" {
		jsonErrorResponse(w, http.StatusBadRequest, "VALIDATION_FAILED", "user_id query parameter is required")
		return
	}

	simulation, err := services.SimulateApplication(r.Context(), appID, userID)
	if err != nil {
		viewError(w, err, http.StatusNotFound, "NOT_FOUND")
		return
	}

	jsonResponse(w, http.StatusOK, simulation)
}

func handleGetProjects(w http.ResponseWriter, r *http.Request) {
	projects, err := services.ListProjects(r.Context())
	if err != nil {
		viewError(w, err, http.StatusInternalServerError, "VIEW_ERROR")
		return
	}

	jsonResponse(w, http.StatusOK, projects)
}

func handleGetBundleImpact(w http.ResponseWriter, r *http.Request) {
	bundleID := r.PathValue("id")
	impact, err := services.BundleImpact(r.Context(), bundleID)
	if err != nil {
		viewError(w, err, http.StatusInternalServerError, "VIEW_ERROR")
		return
	}

	jsonResponse(w, http.StatusOK, impact)
}

func handleGetTopology(w http.ResponseWriter, r *http.Request) {
	topology, err := services.Topology(r.Context())
	if err != nil {
		viewError(w, err, http.StatusInternalServerError, "VIEW_ERROR")
		return
	}

	jsonResponse(w, http.StatusOK, topology)
}
