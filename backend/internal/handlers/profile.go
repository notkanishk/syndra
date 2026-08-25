package handlers

import (
	"net/http"

	"syndra/internal/demo"
	"syndra/internal/directory"
	"syndra/internal/zitadel"
)

// handleGetUserProfile returns a user's display name and email.
// Read by an add-on's convergence to derive an account name; the email
// localpart is what a username comes from (design §11).
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

// handleGetMyProfile returns the requester's full UserProfile — the same
// shape directory.Default.FindUser produces, with title/team
// overlaid from Zitadel metadata. Used by the Next.js OIDC callback to
// populate the session cookie so OIDC and demo sessions render identically
// (May 2026 audit C2/D5).
// GET /api/v1/me/profile  (auth: withUserAuth)
func handleGetMyProfile(w http.ResponseWriter, r *http.Request) {
	uid := getAdminUserID(r.Context())
	if uid == "" {
		jsonErrorResponse(w, http.StatusUnauthorized, "UNAUTHORIZED", "No actor in request context")
		return
	}

	profile, found, err := directory.Default.FindUser(r.Context(), uid)
	if err != nil {
		jsonErrorResponse(w, http.StatusBadGateway, "UPSTREAM_ERROR", "Failed to resolve profile: "+err.Error())
		return
	}
	if !found {
		jsonErrorResponse(w, http.StatusNotFound, "NOT_FOUND", "Profile not found for current actor")
		return
	}
	jsonResponse(w, http.StatusOK, profile)
}
