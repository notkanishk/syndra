package handlers

import (
	"context"
	"log"
	"net/http"
	"time"

	"syndra/internal/db"
)

func handleHealthCheck(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()

	if err := db.PG.Ping(ctx); err != nil {
		log.Printf("[HTTP] database ping failed: %v", err)
		jsonResponse(w, http.StatusServiceUnavailable, map[string]string{"status": "unhealthy", "db": "unreachable"})
		return
	}
	jsonResponse(w, http.StatusOK, map[string]string{"status": "ok"})
}
