package handlers

import (
	"net/http"
	"os"

	"mkauth/internal/directory"
	"mkauth/internal/seed"
)

// SystemModeResponse describes which directory backend is currently active and
// whether the deployment is in an unexpected state. Consumed by the UI to
// render a quiet "Live"/"Demo"/"Degraded" indicator in the chrome.
type SystemModeResponse struct {
	// Directory is the source backing /users, /projects, /applications, /catalog.
	// "zitadel" when the live Management API is reachable; "demo" when the
	// hardcoded fallback catalog is being served.
	Directory string `json:"directory"`

	// SeedActive reports whether demo seed data (bundles, mapping rules, audit
	// logs) was populated in the current process. True in pure local-dev,
	// false in production deployments.
	SeedActive bool `json:"seed_active"`

	// ZitadelConfigured reports whether ZITADEL_DOMAIN is set in the environment.
	// A true value here with Directory == "demo" indicates an unexpected
	// fallback (typically a missing/unreadable machine key path).
	ZitadelConfigured bool `json:"zitadel_configured"`

	// Degraded is true iff ZitadelConfigured is true but Directory is not
	// "zitadel" — a configured-but-unreachable Zitadel client. The UI surfaces
	// this prominently so admins notice a silent fallback.
	Degraded bool `json:"degraded"`
}

// directorySource is injectable for tests.
var directorySource = func() directory.Source { return directory.Default }

// seedActive is injectable for tests.
var seedActive = seed.DemoSeedActive

// systemZitadelConfigured is injectable for tests.
var systemZitadelConfigured = func() bool {
	return os.Getenv("ZITADEL_DOMAIN") != ""
}

func handleSystemMode(w http.ResponseWriter, _ *http.Request) {
	tag := directorySource().Tag()
	configured := systemZitadelConfigured()

	resp := SystemModeResponse{
		Directory:         tag,
		SeedActive:        seedActive(),
		ZitadelConfigured: configured,
		Degraded:          configured && tag != "zitadel",
	}
	jsonResponse(w, http.StatusOK, resp)
}
