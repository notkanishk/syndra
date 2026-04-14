package zitadel

import (
	"context"
	"net/http"
	"time"

	"mkauth/internal/db"
	"mkauth/internal/models"
)

// Injectable vars for testing. Production code uses real implementations.
var (
	httpDo = func(client *http.Client, req *http.Request) (*http.Response, error) {
		return client.Do(req)
	}
	timeNow             = time.Now
	tokenHTTPClient     = &http.Client{Timeout: 10 * time.Second}
	dbGetActiveMappingRules = func(ctx context.Context) ([]models.MappingRule, error) {
		return db.GetActiveMappingRules(ctx)
	}
)
