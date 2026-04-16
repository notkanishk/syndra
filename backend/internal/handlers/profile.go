package handlers

import (
	"net/http"

	"mkauth/internal/demo"
	"mkauth/internal/zitadel"
)

// handleGetUserProfile returns a user's display name and email.
// Used by the sync service to provision LLDAP user attributes.
// GET /api/v1/users/{uid}/profile
func handleGetUserProfile(w http.ResponseWriter, r *http.Request) {
	uid := r.PathValue("uid")
	if uid == "" {
		jsonValidationErrorResponse(w, "Missing user ID", map[string]string{"uid": "required"})
		return
	}

	// Production mode: Zitadel is authoritative — errors are surfaced, not masked.
	if zitadel.MgmtClient != nil {
		user, err := zitadel.MgmtClient.GetUser(r.Context(), uid)
		if err != nil {
			jsonErrorResponse(w, http.StatusBadGateway, "UPSTREAM_ERROR", "Failed to fetch user from Zitadel: "+err.Error())
			return
		}
		if user == nil {
			jsonErrorResponse(w, http.StatusNotFound, "NOT_FOUND", "User not found in Zitadel")
			return
		}
		jsonResponse(w, http.StatusOK, map[string]string{
			"user_id":      user.ID,
			"display_name": user.DisplayName,
			"email":        user.Email,
		})
		return
	}

	// Dev mode only: no Zitadel client — use demo catalog.
	if user, found := demo.FindUser(uid); found {
		jsonResponse(w, http.StatusOK, map[string]string{
			"user_id":      user.ID,
			"display_name": user.Name,
			"email":        user.Email,
		})
		return
	}

	jsonErrorResponse(w, http.StatusNotFound, "NOT_FOUND", "User not found")
}
