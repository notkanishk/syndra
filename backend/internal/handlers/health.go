package handlers

import (
	"context"
	"net/http"
	"time"

	"syndra/internal/db"
)

func handleHealthCheck(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()

	if err := db.PG.Ping(ctx); err != nil {
		jsonResponse(w, http.StatusServiceUnavailable, map[string]string{"status": "unhealthy", "db": err.Error()})
		return
	}
	jsonResponse(w, http.StatusOK, map[string]string{"status": "ok"})
}
