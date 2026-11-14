package handlers

import (
	"context"
	"log"
	"net/http"
	"time"

	"syndra/internal/clock"
	"syndra/internal/db"
	"syndra/internal/locale"
)

// healthBody is the answer's fields: the deployment's zone, and its display
// locale when one is set (omitted, not empty, when LOCALE is unset).
func healthBody(fields map[string]string) map[string]string {
	fields["time_zone"] = clock.Name
	if locale.Tag != "" {
		fields["locale"] = locale.Tag
	}
	return fields
}

func handleHealthCheck(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()

	if err := db.PG.Ping(ctx); err != nil {
		log.Printf("[HTTP] database ping failed: %v", err)
		jsonResponse(w, http.StatusServiceUnavailable, healthBody(map[string]string{"status": "unhealthy", "db": "unreachable"}))
		return
	}
	jsonResponse(w, http.StatusOK, healthBody(map[string]string{"status": "ok"}))
}
