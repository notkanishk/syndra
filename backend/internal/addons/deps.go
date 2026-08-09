package addons

import (
	"net/http"
	"os"
	"time"

	"syndra/internal/db"
)

// Injectable dependencies. Mirrors the save-swap-restore pattern used across
// the backend (services/deps.go, cache/deps.go, zitadel/deps.go). Tests drive
// registration and manifest resolution without a live add-on by swapping these.
var (
	getenv             = os.Getenv
	timeNow            = time.Now
	fetchAddonManifest = httpFetchManifest

	dbUpsertTarget               = db.UpsertTarget
	dbDisableUnconfiguredTargets = db.DisableUnconfiguredTargets

	// manifestHTTPClient is the plain client used to read /capabilities. The
	// mutually-authenticated client that carries plan ids and operation ids on
	// mutating calls arrives with the transport work (task 2.5/2.35); reading a
	// manifest over it changes nothing here but the RoundTripper.
	manifestHTTPClient = &http.Client{Timeout: 10 * time.Second}
)
