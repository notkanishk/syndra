package handlers

import (
	"net/http"

	"mkauth/internal/services"
)

func handleGetCatalog(w http.ResponseWriter, r *http.Request) {
	catalog, err := services.Catalog(r.Context())
	if err != nil {
		jsonErrorResponse(w, http.StatusInternalServerError, "VIEW_ERROR", err.Error())
		return
	}
	jsonResponse(w, http.StatusOK, catalog)
}

func handleGetUsers(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	users, err := services.ListUsers(r.Context(), query)
	if err != nil {
		jsonErrorResponse(w, http.StatusInternalServerError, "VIEW_ERROR", err.Error())
		return
	}

	jsonResponse(w, http.StatusOK, users)
}

func handleGetUserAccess(w http.ResponseWriter, r *http.Request) {
	userID := r.PathValue("id")
	view, err := services.ExplainUserAccess(r.Context(), userID)
	if err != nil {
		jsonErrorResponse(w, http.StatusNotFound, "NOT_FOUND", err.Error())
		return
	}

	jsonResponse(w, http.StatusOK, view)
}

func handleGetApplications(w http.ResponseWriter, r *http.Request) {
	apps, err := services.ListApplications(r.Context())
	if err != nil {
		jsonErrorResponse(w, http.StatusInternalServerError, "VIEW_ERROR", err.Error())
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
		jsonErrorResponse(w, http.StatusNotFound, "NOT_FOUND", err.Error())
		return
	}

	jsonResponse(w, http.StatusOK, simulation)
}

func handleGetProjects(w http.ResponseWriter, r *http.Request) {
	projects, err := services.ListProjects(r.Context())
	if err != nil {
		jsonErrorResponse(w, http.StatusInternalServerError, "VIEW_ERROR", err.Error())
		return
	}

	jsonResponse(w, http.StatusOK, projects)
}

func handleGetBundleImpact(w http.ResponseWriter, r *http.Request) {
	bundleID := r.PathValue("id")
	impact, err := services.BundleImpact(r.Context(), bundleID)
	if err != nil {
		jsonErrorResponse(w, http.StatusInternalServerError, "VIEW_ERROR", err.Error())
		return
	}

	jsonResponse(w, http.StatusOK, impact)
}

func handleGetTopology(w http.ResponseWriter, r *http.Request) {
	topology, err := services.Topology(r.Context())
	if err != nil {
		jsonErrorResponse(w, http.StatusInternalServerError, "VIEW_ERROR", err.Error())
		return
	}

	jsonResponse(w, http.StatusOK, topology)
}
