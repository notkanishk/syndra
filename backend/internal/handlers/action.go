package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"
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

// redisTimeout is the maximum time the data plane will wait for a Redis response.
// Zitadel Actions v2 have hard latency budgets; this prevents cascade failures.
const redisTimeout = 50 * time.Millisecond

// HandleActionInject is the DATA PLANE entrypoint.
// Performance is critical: it hits Redis directly and returns the pre-calculated claims.
//
// Degraded behavior is explicit and per-project:
//   - fail_closed (default): return empty claims and log a warning
//   - minimal_safe: return a configured minimal claim set from the database
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

	// 2. Fetch the pre-computed JWT claim mapping from Redis with a strict timeout
	redisCtx, cancel := context.WithTimeout(r.Context(), redisTimeout)
	defer cancel()

	val, err := redisGetClaims(redisCtx, cacheKey)
	if err != nil {
		if redisCtx.Err() != nil {
			log.Printf("[DATA PLANE] Redis timeout for key=%s after %v", cacheKey, redisTimeout)
		} else {
			log.Printf("[DATA PLANE] Cache miss for key=%s: %v", cacheKey, err)
		}
		jsonResponse(w, http.StatusOK, degradedResponse(r.Context(), req.ProjectID))
		return
	}

	// 3. Unmarshal the cached payload string into a valid claims map
	var claims map[string]interface{}
	if err := json.Unmarshal([]byte(val), &claims); err != nil {
		log.Printf("[DATA PLANE] Malformed cache data for key=%s: %v", cacheKey, err)
		jsonResponse(w, http.StatusOK, degradedResponse(r.Context(), req.ProjectID))
		return
	}

	log.Printf("[DATA PLANE] Cache hit for key=%s", cacheKey)

	// 4. Return sub-millisecond JSON payload to Zitadel Actions pipeline
	jsonResponse(w, http.StatusOK, ActionResponse{CustomClaims: claims})
}

// degradedResponse returns the appropriate claim set when the cache is unavailable
// or contains malformed data, based on the project's configured failure mode.
//
// fail_closed (default): empty claims — applications must handle absent custom claims
// minimal_safe: a small configured set of claims that are always safe to emit
func degradedResponse(ctx context.Context, projectID string) ActionResponse {
	mode, minimalClaims, err := dbGetClaimFailureMode(ctx, projectID)
	if err != nil {
		// Cannot determine configured mode — default to fail_closed
		log.Printf("[DATA PLANE] Could not load failure mode for project=%s (defaulting to fail_closed): %v", projectID, err)
		return ActionResponse{CustomClaims: map[string]interface{}{}}
	}

	switch mode {
	case "minimal_safe":
		if minimalClaims == nil {
			minimalClaims = map[string]interface{}{}
		}
		log.Printf("[DATA PLANE] Degraded mode=minimal_safe for project=%s", projectID)
		return ActionResponse{CustomClaims: minimalClaims}
	default: // fail_closed
		log.Printf("[DATA PLANE] Degraded mode=fail_closed for project=%s", projectID)
		return ActionResponse{CustomClaims: map[string]interface{}{}}
	}
}
