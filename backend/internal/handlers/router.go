package handlers

import (
	"encoding/json"
	"log"
	"net/http"
)

// NewRouter constructs the global multiplexer for API requests
func NewRouter() *http.ServeMux {
	mux := http.NewServeMux()

	// Bundle Routes
	mux.HandleFunc("GET /api/v1/bundles", withCORS(handleGetBundles))
	mux.HandleFunc("POST /api/v1/bundles", withCORS(handleCreateBundle))

	// Rules Routes
	mux.HandleFunc("GET /api/v1/rules/mapping", withCORS(handleGetMappingRules))
	mux.HandleFunc("POST /api/v1/rules/mapping", withCORS(handleCreateMappingRule))

	// Data Plane existing routes
	mux.HandleFunc("POST /api/webhooks/zitadel", withCORS(HandleZitadelWebhook))
	mux.HandleFunc("POST /api/action/inject", withCORS(HandleActionInject))

	return mux
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
