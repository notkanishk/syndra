package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"mkauth/internal/db"
)

// ActionRequest outlines the minimal expected payload from a Zitadel Action ping
type ActionRequest struct {
	UserID    string `json:"user_id"`
	ProjectID string `json:"project_id"`
}

// ActionResponse represents the custom claim payload injected into the Zitadel JWT
type ActionResponse struct {
	CustomClaims map[string]interface{} `json:"customClaims"`
}

// HandleActionInject is the DATA PLANE entrypoint. 
// Performance is critical: It hits Redis directly and returns the pre-calculated claims.
func HandleActionInject(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		jsonErrorResponse(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Only POST is supported")
		return
	}

	var req ActionRequest
	if err := decodeJSONStrict(r.Body, &req); err != nil {
		jsonValidationErrorResponse(w, "Invalid JSON payload", map[string]string{"body": err.Error()})
		return
	}
	if !trimmedNonEmpty(req.UserID) || !trimmedNonEmpty(req.ProjectID) {
		jsonValidationErrorResponse(w, "user_id and project_id are required", map[string]string{
			"user_id":    "required",
			"project_id": "required",
		})
		return
	}

	// 1. Construct the exact Redis key for this user querying this specific application
	cacheKey := fmt.Sprintf("mapping:%s:%s", req.UserID, req.ProjectID)

	// 2. Fetch the pre-computed JWT claim mapping directly from Redis
	val, err := db.Redis.Get(context.Background(), cacheKey).Result()
	if err != nil {
		// Log the cache miss, but never fail an active login flow. Return empty custom claims.
		log.Printf("[DATA PLANE WARNING] Cache miss or error for %s: %v", cacheKey, err)
		jsonResponse(w, http.StatusOK, ActionResponse{CustomClaims: map[string]interface{}{}})
		return
	}

	// 3. Unmarshal the cached payload string into a valid claims map
	var claims map[string]interface{}
	if err := json.Unmarshal([]byte(val), &claims); err != nil {
		log.Printf("[DATA PLANE ERROR] Malformed cache data for %s: %v", cacheKey, err)
		jsonResponse(w, http.StatusOK, ActionResponse{CustomClaims: map[string]interface{}{}})
		return
	}

	// 4. Return sub-millisecond JSON payload to Zitadel Actions pipeline
	jsonResponse(w, http.StatusOK, ActionResponse{
		CustomClaims: claims,
	})
}
