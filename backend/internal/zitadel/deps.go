package zitadel

import (
	"net/http"
	"time"

	"syndra/internal/db"
)

// Injectable vars for testing. Production code uses real implementations.
var (
	httpDo = func(client *http.Client, req *http.Request) (*http.Response, error) {
		return client.Do(req)
	}
	timeNow                 = time.Now
	tokenHTTPClient         = &http.Client{Timeout: 10 * time.Second}
	dbGetActiveMappingRules = db.GetActiveMappingRules
)
