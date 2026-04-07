package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
)

// NewRouter constructs the global multiplexer for API requests
func NewRouter() *http.ServeMux {
	mux := http.NewServeMux()

	// Bundle Routes
	mux.HandleFunc("GET /api/v1/bundles", withCORS(withAuth(handleGetBundles)))
	mux.HandleFunc("POST /api/v1/bundles", withCORS(withAuth(handleCreateBundle)))
	mux.HandleFunc("GET /api/v1/bundles/{id}/roles", withCORS(withAuth(handleGetBundleRoles)))
	mux.HandleFunc("GET /api/v1/bundles/{id}/impact", withCORS(withAuth(handleGetBundleImpact)))
	mux.HandleFunc("POST /api/v1/bundles/{id}/roles", withCORS(withAuth(handleAddRoleToBundle)))

	// Explorer Views
	mux.HandleFunc("GET /api/v1/catalog", withCORS(withAuth(handleGetCatalog)))
	mux.HandleFunc("GET /api/v1/users", withCORS(withAuth(handleGetUsers)))
	// User-Bundle Assignments
	mux.HandleFunc("GET /api/v1/users/{id}/grants", withCORS(withAuth(handleGetUserDirectGrants)))
	mux.HandleFunc("POST /api/v1/users/{id}/grants", withCORS(withAuth(handleUpsertUserDirectGrant)))
	mux.HandleFunc("GET /api/v1/users/{id}/bundles", withCORS(withAuth(handleGetUserBundles)))
	mux.HandleFunc("POST /api/v1/users/{id}/bundles", withCORS(withAuth(handleAssignBundleToUser)))
	mux.HandleFunc("GET /api/v1/users/{id}/access", withCORS(withAuth(handleGetUserAccess)))

	// Application Views
	mux.HandleFunc("GET /api/v1/applications", withCORS(withAuth(handleGetApplications)))
	mux.HandleFunc("GET /api/v1/applications/{id}/simulate", withCORS(withAuth(handleSimulateApplication)))

	// Project Views
	mux.HandleFunc("GET /api/v1/projects", withCORS(withAuth(handleGetProjects)))

	// Rules Routes
	mux.HandleFunc("GET /api/v1/rules/mapping", withCORS(withAuth(handleGetMappingRules)))
	mux.HandleFunc("POST /api/v1/rules/mapping", withCORS(withAuth(handleCreateMappingRule)))
	mux.HandleFunc("PUT /api/v1/rules/mapping/{id}", withCORS(withAuth(handleUpdateMappingRule)))

	// Audit Logs
	mux.HandleFunc("GET /api/v1/audit", withCORS(withAuth(handleGetAuditLogs)))
	mux.HandleFunc("GET /api/v1/requests", withCORS(withAuth(handleGetAccessRequests)))
	mux.HandleFunc("POST /api/v1/requests", withCORS(withAuth(handleCreateAccessRequest)))
	mux.HandleFunc("POST /api/v1/requests/{id}/decision", withCORS(withAuth(handleResolveAccessRequest)))
	mux.HandleFunc("GET /api/v1/governance/summary", withCORS(withAuth(handleGetGovernanceSummary)))

	// Data Plane existing routes
	mux.HandleFunc("POST /api/webhooks/zitadel", withCORS(HandleZitadelWebhook)) // Webhook verifies its own payload
	mux.HandleFunc("POST /api/action/inject", withCORS(HandleActionInject))      // Action inject verifies itself or uses a specific different mechanism if needed, but typically wide open locally

	return mux
}

// withAuth verifies the MKAUTH_API_KEY for protected endpoints
func withAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		expectedKey := os.Getenv("MKAUTH_API_KEY")
		if expectedKey == "" {
			// If not set, allow dev access (or default to blocked, but for this dev setup let's block if missing to enforce it)
			jsonErrorResponse(w, http.StatusInternalServerError, "SERVER_ERROR", "Server missing auth configuration")
			return
		}

		authHeader := r.Header.Get("Authorization")
		if authHeader != "Bearer "+expectedKey {
			jsonErrorResponse(w, http.StatusUnauthorized, "UNAUTHORIZED", "Missing or invalid authorization token")
			return
		}

		next(w, r)
	}
}

// withCORS is a basic CORS middleware ensuring Next.js UI binds seamlessly
func withCORS(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS, PUT, DELETE")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}
		
		next(w, r)
	}
}

// jsonResponse simplifies writing standard struct definitions to HTTP streams
func jsonResponse(w http.ResponseWriter, status int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		log.Printf("Failed to encode json: %v", err)
	}
}

// jsonError defines standard API error shape
type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message"`
}

func jsonErrorResponse(w http.ResponseWriter, status int, errStr, msg string) {
	jsonResponse(w, status, ErrorResponse{Error: errStr, Message: msg})
}
