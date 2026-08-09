// Package propagation contains the operator-triggered drain that flushes the
// propagation_outbox outbox to Zitadel. It mirrors services/expiry:
// a small package whose external effects are injectable function vars so the
// drain logic is testable without a live Zitadel or DB.
//
// Doctrine: every Syndra-mediated Zitadel grant mutation is first written to the
// durable ledger + outbox (db.EnqueueDirectGrantPropagation), then propagated
// here. `applied` (synchronous 2xx, or an idempotent 409 AlreadyExists) is
// terminal success — there is no `confirmed` state, because the self-mutation
// guard drops Syndra's own grant webhook events (design Decision 1).
package propagation

import (
	"context"
	"errors"
	"os"
	"strconv"

	"syndra/internal/addons"
	"syndra/internal/db"
	"syndra/internal/zitadel"
)

// Injectable dependencies — save/swap/restore in tests (see expiry/deps.go).
var (
	claimPending = db.ClaimPendingPropagations
	// claimRevocations is the background runner's claim. A separate seam, not a
	// parameter on claimPending: the two differ in what they are ALLOWED to
	// return, and a boolean would let a caller ask the operator claim for
	// revocations or — the direction that matters — the runner's claim for
	// everything.
	claimRevocations = db.ClaimPendingRevocations
	claimOne         = db.ClaimPropagationByID
	undispatchable   = db.UndispatchableTarget
	awaitingDispatch = db.TargetsAwaitingDispatch
	markApplied      = db.MarkPropagationApplied
	markFailed       = db.MarkPropagationFailed
	requeue          = db.RequeuePropagation
	// release returns a row to pending without spending a retry, for the one
	// case where nothing was attempted at all.
	release = db.ReleasePropagation

	// reconcileLedger prunes direct_role_grants to match the desired state an
	// applied revoke/replace established in Zitadel (revoke removes the named
	// rows; replace removes superseded direct rows). add is a no-op. Runs before
	// markApplied so a failure leaves the outbox row non-terminal for retry.
	reconcileLedger = db.ReconcileLedgerOnApplied

	// acquireDrainLock serializes concurrent drains via a session-level advisory
	// lock. Returns a release closure + acquired flag; acquired=false means
	// another drain holds it and this one must skip. Serialization is what keeps
	// the in_flight reclaim safe (only crash-orphaned rows are ever reclaimed).
	acquireDrainLock = db.TryAcquireDrainLock

	// Reachability pre-flight: a cheap real call (limit-1 grant list) doubles as
	// a probe. nil client (local-policy-only mode) is treated as offline so the
	// drain halts cleanly rather than nil-panicking.
	zitadelReachable = func(ctx context.Context) bool {
		if zitadel.MgmtClient == nil {
			return false
		}
		_, err := zitadel.MgmtClient.ListAllGrants(ctx, zitadel.SearchParams{Limit: 1})
		return err == nil
	}

	zitadelAddUserGrant = func(ctx context.Context, userID, projectID string, roleKeys []string) error {
		return zitadel.MgmtClient.AddUserGrant(ctx, userID, projectID, roleKeys)
	}
	zitadelUpdateUserGrant = func(ctx context.Context, userID, grantID string, roleKeys []string) error {
		return zitadel.MgmtClient.UpdateUserGrant(ctx, userID, grantID, roleKeys)
	}
	zitadelRemoveUserGrant = func(ctx context.Context, userID, grantID string) error {
		return zitadel.MgmtClient.RemoveUserGrant(ctx, userID, grantID)
	}

	// already-exists check (latency optimization only — see alreadyExists):
	// webhook index first; on miss, ONE live grant list per row (not per role).
	grantIndexHasRole  = db.GrantIndexHasRole
	liveUserGrantRoles = func(ctx context.Context, userID, projectID string) (map[string]bool, error) {
		res, err := zitadel.MgmtClient.ListUserGrants(ctx, userID, zitadel.SearchParams{Limit: 100})
		if err != nil {
			return nil, err
		}
		out := map[string]bool{}
		for _, g := range res.Items {
			if g.ProjectID != projectID {
				continue
			}
			for _, rk := range g.RoleKeys {
				out[rk] = true
			}
		}
		return out, nil
	}

	pruneTerminal = db.PruneTerminalPropagations

	// The add-on dispatcher's two seams. Separate from the Zitadel ones because
	// they are a different leg of the contract, and a test has to be able to
	// fail either: a row dispatched without its approved snapshot, and an
	// outcome recorded as something the target did not say.
	readIntent       = db.ReadEntitlementIntent
	applyEntitlement = addons.Apply

	maxRetries    = outboxMaxRetries()    // OUTBOX_MAX_RETRIES (default 5)
	retentionDays = outboxRetentionDays() // OUTBOX_RETENTION_DAYS (default 30)
)

type ackClass int

const (
	ackApplied   ackClass = iota
	ackFailed             // terminal — operator must inspect
	ackTransient          // 5xx / timeout / network / 429 / 408 — retry
)

// classifyZitadelError maps a Zitadel client error to an ACK class by HTTP
// status, NOT by string-sniffing (design Decision 1, review finding C):
//   - 409 AlreadyExists on add/replace  → ackApplied (idempotent success)
//   - 429 Too Many Requests, 408 Timeout → ackTransient (despite being 4xx)
//   - all other 4xx                      → ackFailed (terminal)
//   - 5xx / network / no status          → ackTransient
func classifyZitadelError(err error) ackClass {
	code := zitadelStatusCode(err)
	switch {
	case code == 409:
		return ackApplied
	case code == 429 || code == 408:
		return ackTransient
	case code >= 400 && code < 500:
		return ackFailed
	default: // 5xx, network errors, or unknown
		return ackTransient
	}
}

// zitadelStatusCode extracts the HTTP status from a *zitadel.StatusError, or 0
// if the error does not carry one (network error, context cancel, etc.).
func zitadelStatusCode(err error) int {
	var se *zitadel.StatusError
	if errors.As(err, &se) {
		return se.Code
	}
	return 0
}

func outboxMaxRetries() int    { return envInt("OUTBOX_MAX_RETRIES", 5) }
func outboxRetentionDays() int { return envInt("OUTBOX_RETENTION_DAYS", 30) }

func envInt(name string, def int) int {
	v := os.Getenv(name)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil || n < 0 {
		return def
	}
	return n
}
