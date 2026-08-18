package addons

import (
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

	// refreshTimeout bounds ONE add-on's manifest read. Per target, never per
	// pass: a shared budget means the first unreachable add-on spends it and
	// every target behind it is cancelled before it is even asked, so one
	// switched-off NAS would suppress the contract check on every other target.
	refreshTimeout = 5 * time.Second

	// callTimeout bounds ONE dispatched operation. Distinct from refreshTimeout
	// because the two have different costs of being wrong: a manifest read that
	// gives up early is retried on the next tick, while an operation that gives
	// up early becomes indeterminate and needs a human.
	callTimeout = 30 * time.Second

	// breakerThreshold is how many consecutive non-deterministic failures open a
	// target's circuit. Five, not one: a single timeout is ordinary, and opening
	// on it would make one slow response look like an outage.
	breakerThreshold = 5
	// breakerCooldown is how long the circuit stays open. Short enough that a
	// restarted add-on is picked up within one drain pass, long enough that a
	// down target is not being asked once per queued row.
	breakerCooldown = 30 * time.Second

	// manifestRetryCooldown bounds the on-demand manifest read a call makes when
	// its target has none. Long enough that a burst of refused calls does not
	// become a stream of capability reads at a target that is still starting;
	// far shorter than the refresh period, because the whole point is not to
	// wait for the next tick.
	manifestRetryCooldown = 30 * time.Second

	dbClaimAddonOperation        = db.ClaimAddonOperation
	dbUpsertTarget               = db.UpsertTarget
	dbDisableUnconfiguredTargets = db.DisableUnconfiguredTargets
)
