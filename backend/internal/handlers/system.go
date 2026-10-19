package handlers

import (
	"context"
	"log"
	"net/http"
	"os"
	"time"

	"syndra/internal/db"
	"syndra/internal/demo"
	"syndra/internal/directory"
	"syndra/internal/seed"
)

// SystemModeResponse describes which directory backend is currently active and
// whether the deployment is in an unexpected state. Consumed by the UI to
// render a quiet "Live"/"Demo"/"Degraded" indicator in the chrome.
type SystemModeResponse struct {
	// Directory is the source backing /users, /projects, /applications, /catalog.
	// "zitadel" for the live Management API; "unreachable" when Zitadel is
	// configured but its client couldn't be built; "demo" only when Zitadel
	// isn't configured (local development).
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
	// SYNDRA_SEED_DEMO=false and restarts; the rows it already wrote stay in
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

	// Degraded is true when Zitadel is configured and isn't answering: its
	// client couldn't be built ("unreachable"), or its last read failed.
	Degraded bool `json:"degraded"`

	// LastReadAt is when Zitadel last answered a directory read; null when it
	// never has. While Degraded, pages show that read, labelled with its age,
	// and with none they show nothing. Never sample data (N23).
	LastReadAt *time.Time `json:"last_read_at"`
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
	lastRead, current := directory.FreshnessOf(directorySource())

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
		// Degraded is Zitadel not answering now, whether or not Syndra has
		// a last good read to show in its place. LastReadAt tells the two
		// apart: set, the page shows that read with its age; null, nothing.
		Degraded: configured && (tag != "zitadel" || !current),
	}
	if !lastRead.IsZero() {
		resp.LastReadAt = &lastRead
	}
	jsonResponse(w, http.StatusOK, resp)
}
