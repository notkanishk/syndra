package handlers

import (
	"context"
	"log"
	"net/http"
	"os"

	"mkauth/internal/db"
	"mkauth/internal/demo"
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

	// SeedResidue counts operator rows still referencing a demo fixture,
	// regardless of which process wrote them or whether seeding is currently
	// enabled. SeedActive goes false the moment an operator sets
	// MKAUTH_SEED_DEMO=false and restarts; the rows it already wrote stay in
	// the database and keep being served. This is the number that says so.
	//
	// Negative is impossible; zero means clean. The count is not exposed as a
	// per-table breakdown on purpose — the UI needs one sentence, and an
	// operator who wants the detail runs the reset script, which prints it.
	SeedResidue int `json:"seed_residue"`

	// ResetCommand is the verbatim shell command that clears SeedResidue.
	// Returned by the backend rather than hardcoded in the UI so the two can
	// never drift apart, matching the rotate_command convention on
	// /zitadel/action-rotation-status.
	ResetCommand string `json:"reset_command"`

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

// countDemoResidue is injectable for tests.
var countDemoResidue = func(ctx context.Context) (int, error) {
	return db.CountDemoResidue(ctx, demo.ProjectIDs(), demo.UserIDs())
}

// resetCommand is what an operator runs to clear demo residue. Kept next to
// the count it resolves so a rename of the make target updates both.
const resetCommand = "make reset-demo-data"

func handleSystemMode(w http.ResponseWriter, r *http.Request) {
	tag := directorySource().Tag()
	configured := systemZitadelConfigured()

	// A failed count must not read as a clean database. Log it and report
	// zero — the banner then stays quiet rather than announcing residue that
	// may not exist, and the operator still has the reset command on the
	// Identity provider page.
	residue, err := countDemoResidue(r.Context())
	if err != nil {
		log.Printf("[SYSTEM] demo residue count failed: %v", err)
		residue = 0
	}

	resp := SystemModeResponse{
		Directory:         tag,
		SeedActive:        seedActive(),
		SeedResidue:       residue,
		ResetCommand:      resetCommand,
		ZitadelConfigured: configured,
		Degraded:          configured && tag != "zitadel",
	}
	jsonResponse(w, http.StatusOK, resp)
}
