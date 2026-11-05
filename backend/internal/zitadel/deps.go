package zitadel

import (
	"net/http"
	"time"
)

// Injectable vars for testing. Production code uses real implementations.
var (
	httpDo = func(client *http.Client, req *http.Request) (*http.Response, error) {
		return client.Do(req)
	}
	timeNow         = time.Now
	tokenHTTPClient = &http.Client{Timeout: 10 * time.Second}
)
