package handlers

import (
	"crypto/subtle"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strings"

	"mkauth/internal/auth"
)

// NewRouter constructs the global multiplexer for API requests
func NewRouter() http.Handler {
	mux := http.NewServeMux()

	// Health check — no auth, no CORS
	mux.HandleFunc("GET /healthz", handleHealthCheck)

	// Bundle Routes
	mux.HandleFunc("GET /api/v1/bundles", withCORS(withUserAuth(handleGetBundles)))
	mux.HandleFunc("POST /api/v1/bundles", withCORS(withUserAuth(handleCreateBundle)))
	mux.HandleFunc("GET /api/v1/bundles/{id}/roles", withCORS(withUserAuth(handleGetBundleRoles)))
	mux.HandleFunc("GET /api/v1/bundles/{id}/impact", withCORS(withUserAuth(handleGetBundleImpact)))
	mux.HandleFunc("POST /api/v1/bundles/{id}/roles", withCORS(withUserAuth(handleAddRoleToBundle)))

	// Explorer Views
	mux.HandleFunc("GET /api/v1/catalog", withCORS(withUserAuth(handleGetCatalog)))
	mux.HandleFunc("GET /api/v1/users", withCORS(withUserAuth(handleGetUsers)))
	// User-Bundle Assignments
	mux.HandleFunc("GET /api/v1/users/{id}/grants", withCORS(withUserAuth(handleGetUserDirectGrants)))
	mux.HandleFunc("POST /api/v1/users/{id}/grants", withCORS(withUserAuth(handleUpsertUserDirectGrant)))
	mux.HandleFunc("GET /api/v1/users/{id}/bundles", withCORS(withUserAuth(handleGetUserBundles)))
	mux.HandleFunc("POST /api/v1/users/{id}/bundles", withCORS(withUserAuth(handleAssignBundleToUser)))
	mux.HandleFunc("GET /api/v1/users/{id}/access", withCORS(withUserAuth(handleGetUserAccess)))

	// Application Views
	mux.HandleFunc("GET /api/v1/applications", withCORS(withUserAuth(handleGetApplications)))
	mux.HandleFunc("GET /api/v1/applications/{id}/simulate", withCORS(withUserAuth(handleSimulateApplication)))

	// Project Views
	mux.HandleFunc("GET /api/v1/projects", withCORS(withUserAuth(handleGetProjects)))
	mux.HandleFunc("GET /api/v1/topology", withCORS(withUserAuth(handleGetTopology)))

	// Rules Routes
	mux.HandleFunc("GET /api/v1/rules/mapping", withCORS(withUserAuth(handleGetMappingRules)))
	mux.HandleFunc("POST /api/v1/rules/mapping", withCORS(withUserAuth(handleCreateMappingRule)))
	mux.HandleFunc("PUT /api/v1/rules/mapping/{id}", withCORS(withUserAuth(handleUpdateMappingRule)))

	// Audit Logs
	mux.HandleFunc("GET /api/v1/audit", withCORS(withUserAuth(handleGetAuditLogs)))
	mux.HandleFunc("GET /api/v1/requests", withCORS(withUserAuth(handleGetAccessRequests)))
	mux.HandleFunc("POST /api/v1/requests", withCORS(withUserAuth(handleCreateAccessRequest)))
	mux.HandleFunc("POST /api/v1/requests/{id}/decision", withCORS(withUserAuth(handleResolveAccessRequest)))
	mux.HandleFunc("GET /api/v1/governance/summary", withCORS(withUserAuth(handleGetGovernanceSummary)))

	// Operator: event and trigger logs
	mux.HandleFunc("GET /api/v1/onboarding/triggers", withCORS(withUserAuth(handleGetOnboardingTriggers)))
	mux.HandleFunc("GET /api/v1/webhook/events", withCORS(withUserAuth(handleGetWebhookEvents)))

	// Data Plane routes — verified by their own mechanisms (HMAC / Redis)
	mux.HandleFunc("POST /api/webhooks/zitadel", withCORS(HandleZitadelWebhook))
	mux.HandleFunc("POST /api/action/inject", withCORS(HandleActionInject))

	return withMaxBody(withSecurityHeaders(mux))
}

// withUserAuth is the primary authorization middleware for all admin API routes.
//
// Production mode (ZITADEL_DOMAIN set): requires a Zitadel-issued RS256 JWT in
// the Authorization header. Validates signature, issuer, audience, and expiry.
// Stores the extracted admin user ID in the request context for audit attribution.
//
// Local-dev mode (ZITADEL_DOMAIN unset): falls back to shared API key
// (MKAUTH_API_KEY) so existing tooling continues to work without a live Zitadel
// instance. The shared key is never sufficient in production.
func withUserAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		domain := os.Getenv("ZITADEL_DOMAIN")

		if domain == "" {
			// Local-dev fallback: shared API key
			withAPIKeyAuth(next)(w, r)
			return
		}

		// Production: Zitadel-issued user access token required
		audience := os.Getenv("ZITADEL_AUDIENCE")
		if audience == "" {
			log.Printf("[AUTH] ZITADEL_AUDIENCE is not set; rejecting request")
			jsonErrorResponse(w, http.StatusInternalServerError, "SERVER_ERROR", "Server missing auth configuration")
			return
		}

		rawToken := extractBearerToken(r)
		if rawToken == "" {
			log.Printf("[AUTH] Missing bearer token from %s %s", r.Method, r.URL.Path)
			jsonErrorResponse(w, http.StatusUnauthorized, "UNAUTHORIZED", "Missing or invalid authorization token")
			return
		}

		adminUserID, err := auth.ValidateToken(r.Context(), rawToken, domain, audience)
		if err != nil {
			log.Printf("[AUTH] Token validation failed for %s %s: %v", r.Method, r.URL.Path, err)
			jsonErrorResponse(w, http.StatusUnauthorized, "UNAUTHORIZED", "Invalid or expired token")
			return
		}

		log.Printf("[AUTH] Authorized admin=%s for %s %s", adminUserID, r.Method, r.URL.Path)
		ctx := withAdminUserID(r.Context(), adminUserID)
		next(w, r.WithContext(ctx))
	}
}

// withAPIKeyAuth verifies the MKAUTH_API_KEY shared secret.
// Used as the auth mechanism in local-dev mode and as defense-in-depth for
// data-plane routes that have their own verification (HMAC, Redis).
func withAPIKeyAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		expectedKey := os.Getenv("MKAUTH_API_KEY")
		if expectedKey == "" {
			jsonErrorResponse(w, http.StatusInternalServerError, "SERVER_ERROR", "Server missing auth configuration")
			return
		}
		if subtle.ConstantTimeCompare([]byte(extractBearerToken(r)), []byte(expectedKey)) != 1 {
			jsonErrorResponse(w, http.StatusUnauthorized, "UNAUTHORIZED", "Missing or invalid authorization token")
			return
		}
		next(w, r)
	}
}

// extractBearerToken parses the Authorization header and returns the token string,
// or empty string if the header is absent or not in Bearer format.
func extractBearerToken(r *http.Request) string {
	h := r.Header.Get("Authorization")
	if !strings.HasPrefix(h, "Bearer ") {
		return ""
	}
	return strings.TrimPrefix(h, "Bearer ")
}

// withCORS sets CORS headers using the CORS_ORIGIN env var (default http://localhost:3000).
func withCORS(next http.HandlerFunc) http.HandlerFunc {
	origin := os.Getenv("CORS_ORIGIN")
	if origin == "" {
		origin = "http://localhost:3000"
	}
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS, PUT, DELETE")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		w.Header().Set("Access-Control-Allow-Credentials", "true")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next(w, r)
	}
}

// withSecurityHeaders adds standard security response headers to all responses.
func withSecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		next.ServeHTTP(w, r)
	})
}

// withMaxBody limits request body size to 1 MB to prevent abuse.
func withMaxBody(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
		next.ServeHTTP(w, r)
	})
}

// jsonResponse simplifies writing standard struct definitions to HTTP streams
func jsonResponse(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		log.Printf("Failed to encode json: %v", err)
	}
}

// jsonError defines standard API error shape
type ErrorResponse struct {
	Error   string            `json:"error"`
	Message string            `json:"message"`
	Details map[string]string `json:"details,omitempty"`
}

func jsonErrorResponse(w http.ResponseWriter, status int, errStr, msg string) {
	jsonResponse(w, status, ErrorResponse{Error: errStr, Message: msg})
}

func jsonValidationErrorResponse(w http.ResponseWriter, msg string, details map[string]string) {
	jsonResponse(w, http.StatusBadRequest, ErrorResponse{
		Error:   "VALIDATION_FAILED",
		Message: msg,
		Details: details,
	})
}
