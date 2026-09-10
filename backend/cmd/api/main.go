package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"syndra/internal/addons"
	"syndra/internal/db"
	"syndra/internal/directory"
	"syndra/internal/handlers"
	"syndra/internal/observe"
	"syndra/internal/seed"
	"syndra/internal/services/drift"
	"syndra/internal/services/expiry"
	"syndra/internal/services/periodic"
	"syndra/internal/services/propagation"
	"syndra/internal/zitadel"
)

// requireProductionSigningKeys aborts startup if ZITADEL_DOMAIN is set but
// either signing-key env is empty. Production deployments without these keys
// would silently accept unverified webhook/action payloads — an unacceptable
// trust posture flagged by the May 2026 audit (C1).
func requireProductionSigningKeys() {
	if os.Getenv("ZITADEL_DOMAIN") == "" {
		return // dev mode — the action-signature middleware allows passthrough
	}
	missing := []string{}
	if os.Getenv("ZITADEL_EVENT_SIGNING_KEY") == "" {
		missing = append(missing, "ZITADEL_EVENT_SIGNING_KEY")
	}
	if os.Getenv("ZITADEL_ACTION_SIGNING_KEY") == "" {
		missing = append(missing, "ZITADEL_ACTION_SIGNING_KEY")
	}
	if len(missing) > 0 {
		log.Fatalf("[STARTUP] Production refusing to start: ZITADEL_DOMAIN is set but %s is empty. Configure signing keys before deploying.", strings.Join(missing, ", "))
	}
}

// warnIfWelcomeBundleMissing emits an operator-visible warning at startup when
// no bundle has is_welcome=TRUE. Onboarding triggers that fire in this state
// will fail with "no welcome bundle configured" until an operator sets one via
// PUT /api/v1/bundles/{id}/welcome (May 2026 audit D1 — explicit-only contract,
// no autopromote on migration).
// awaitFirstManifests re-reads the add-ons that have not answered yet, quickly
// at first, until they have.
//
// The single start-up read races the add-on's own listener and loses often
// enough that it is the normal experience of a deploy: both containers are
// recreated together, the backend asks for /capabilities a second or two
// before the add-on is listening, and the answer is `connection refused`.
// Nothing retried. The next attempt was a full refresh interval away — up to
// fifteen minutes — while Connected systems said "this usually clears by
// itself within a minute or two", which was the one thing it could not do.
//
// Bounded, and it stops at the first success. After this the periodic refresh
// owns the question; this exists only to close the window a restart opens.
// observeInterval is how often Zitadel is asked what is true. Overridable
// because a deployment with a much larger directory pays more for each pass
// and may want it slower — at the cost of a looser bound on staleness, which
// is the trade being made and should be made deliberately.
func observeInterval() time.Duration {
	v := os.Getenv("OBSERVE_SWEEP_INTERVAL")
	if v == "" {
		return 5 * time.Minute
	}
	d, err := time.ParseDuration(v)
	if err != nil || d <= 0 {
		log.Printf("[OBSERVE] Invalid OBSERVE_SWEEP_INTERVAL=%q, defaulting to 5m", v)
		return 5 * time.Minute
	}
	return d
}

func awaitFirstManifests(ctx context.Context) {
	for _, wait := range []time.Duration{2, 3, 5, 10, 15, 30, 45} {
		if len(addons.PendingManifests()) == 0 {
			return
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(wait * time.Second):
		}
		_ = addons.RefreshAll(ctx)
	}
	if pending := addons.PendingManifests(); len(pending) > 0 {
		// Said once, plainly. Past this point the wait is the refresh interval,
		// and an operator reading the start-up log deserves to know which of
		// the two they are in.
		log.Printf("[ADDON] %v has not served a manifest yet; retrying on the refresh interval", pending)
	}
}

// checkObservation reports whether the store holds an org observation yet.
// A seam so awaitFirstObservation is testable without a database.
var checkObservation = func(ctx context.Context) error {
	_, err := db.LatestOrgObservation(ctx)
	return err
}

// awaitFirstObservation blocks, briefly and boundedly, until the observation
// sweep has written something — so drift's first run reads a store that has
// actually been asked, not one that is empty because nobody has looked yet.
//
// Same shape as awaitFirstManifests and the same reason: the observer's own
// run-on-boot usually wins this race in a couple of seconds, and this only
// exists to close the window where drift's run-on-boot could win it instead.
// Bounded, and gives up onto the schedule rather than blocking forever — a
// deployment with no Zitadel configured would otherwise never start drift at
// all, when Sweep already knows how to say "not checked yet" about exactly
// that case.
func awaitFirstObservation(ctx context.Context) {
	for _, wait := range []time.Duration{2, 3, 5, 10, 15, 30, 45} {
		if checkObservation(ctx) == nil {
			return
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(wait * time.Second):
		}
	}
	if checkObservation(ctx) != nil {
		log.Printf("[DRIFT] no observation yet after startup wait; starting on schedule regardless — findings will read \"not checked yet\" until the sweep catches up")
	}
}

func warnIfWelcomeBundleMissing(ctx context.Context) {
	_, err := db.GetWelcomeBundle(ctx)
	if err == nil {
		return
	}
	if errors.Is(err, db.ErrNoWelcomeBundleConfigured) {
		log.Println("[STARTUP] WARNING: no welcome bundle configured — onboarding triggers will fail until an operator sets one (PUT /api/v1/bundles/{id}/welcome).")
		return
	}
	log.Printf("[STARTUP] WARNING: welcome-bundle check failed (%v); onboarding may not work until resolved.", err)
}

func main() {
	requireProductionSigningKeys()
	fmt.Println("Syndra Backend Starting...")

	// Initialize connections safely (They read ENVs and connect)
	db.ConnectPostgres()
	db.ConnectRedis()

	// Initialize Zitadel service account auth before seeding so the seed path
	// can see whether we're in live mode and skip itself accordingly.
	if err := zitadel.InitClient(); err != nil {
		log.Printf("Zitadel Init Warning: %v", err)
	}

	// Select the directory source (live Zitadel when MgmtClient is initialized,
	// demo fallback otherwise). Emits the [DIRECTORY] Source=... log line.
	directory.Init()

	// Read the add-on registry from deployment configuration and reconcile the
	// database's targets registry to match. No add-on is contacted here: one
	// that is switched off must not delay startup, and one that is unreachable
	// must still be registered, because operator navigation derives from the
	// deployment rather than from what happens to be answering. A failure here
	// is a database failure, not an add-on failure, so it is fatal — every
	// target-carrying table resolves its foreign key against that registry.
	if err := addons.Init(context.Background()); err != nil {
		log.Fatalf("[STARTUP] Add-on registry initialisation failed: %v", err)
	}

	// First manifest read, synchronously, before the server accepts anything. A
	// contract-version mismatch is a registration refusal and belongs in the
	// startup log where an operator looks after a deploy, not discovered
	// minutes later on a tick. It is deliberately NOT fatal: refusing to boot
	// the backend that governs every other target because one NAS add-on
	// shipped ahead of it would be the fail-open rule inverted. RefreshAll
	// bounds each add-on individually and runs them concurrently, so a
	// switched-off one costs one timeout rather than the whole pass.
	_ = addons.RefreshAll(context.Background())

	if err := seed.EnsureDemoData(context.Background()); err != nil {
		log.Fatalf("Demo seed failed: %v", err)
	}

	warnIfWelcomeBundleMissing(context.Background())

	mux := handlers.NewRouter()

	server := &http.Server{
		Addr:    ":8080",
		Handler: mux,
	}

	// Root context cancelled on SIGINT/SIGTERM. Created before background
	// workers so they participate in the same graceful-shutdown signal.
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Grant-expiry scheduler: periodic sweep of expired direct grants to
	// emit LLDAP removal intents, hard-delete rows, invalidate cache, and
	// cascade-revoke derived Zitadel grants. sched is nil when disabled;
	// shutdown joins on sched.Done() (if non-nil) before closing shared
	// DB/Redis clients so an in-flight sweep cannot race teardown.
	var sched *periodic.Runner
	if schedulerEnabled() {
		batch := schedulerBatchSize()
		sched = periodic.New("SCHEDULER", schedulerInterval(), 5*time.Minute, func(ctx context.Context) error {
			expiry.Sweep(ctx, batch)
			// A separate pass on the same tick, not a second stage of the
			// first: grant expiry removes access and this restores it, so a
			// batch that aborted halfway through one must not skip the other.
			expiry.SweepAllowances(ctx, batch)
			return nil
		})
		go sched.Start(ctx)
	} else {
		log.Println("[SCHEDULER] Disabled via EXPIRY_SCHEDULER_ENABLED=false")
	}

	// Drift reconciliation scheduler: periodic Zitadel↔Syndra sweep (B2/C6).
	//
	// one-truth-many-checks, "The last two readers": drift no longer pages
	// Zitadel itself — it diffs internal/observe's store, which the OBSERVE
	// sweep below already refreshes every five minutes. Paying for a second
	// listing bought nothing drift ever used for a different purpose, so its
	// cadence is now the observer's: an out-of-band grant sits undetected for
	// as long as the store is stale, never longer, and this is a database diff
	// against data already in memory rather than a network round trip — the
	// six-hour interval it used to run on was the cost of ITS OWN read, which
	// is the cost this change removes.
	var driftSched *periodic.Runner
	if driftSchedulerEnabled() {
		driftSched = periodic.New("DRIFT", observeInterval(), 5*time.Minute, func(ctx context.Context) error {
			_, err := drift.Sweep(ctx)
			return err
		})
		// Never before the store holds an answer: a periodic.Runner's first run
		// is immediate, and reading the store before the observer has written
		// anything to it is indistinguishable from reading it after Zitadel
		// went quiet forever — both are ErrNoObservation. Sweep already refuses
		// to turn that into a clean bill, but there is no reason to manufacture
		// the race when awaitFirstManifests is the exact pattern for waiting on
		// the thing this depends on.
		go func(r *periodic.Runner) {
			awaitFirstObservation(ctx)
			r.Start(ctx)
		}(driftSched)
	} else {
		log.Println("[DRIFT] Disabled via DRIFT_SCHEDULER_ENABLED=false")
	}

	// Revocation drain: the one background drain in the system. Grants stay
	// operator-gated — the drain rule protects a consent property, and nobody
	// gains access without a human — but a queued revoke is retained access, so
	// it must not wait on somebody opening the right page. Shares the operator
	// drain's advisory lock, so the two never dispatch concurrently; a pass that
	// cannot take it simply returns and the next tick tries again.
	var revokeSched *periodic.Runner
	if revocationDrainEnabled() {
		revokeSched = periodic.New("REVOKE", revocationDrainInterval(), 5*time.Minute, func(ctx context.Context) error {
			res, err := propagation.DrainRevocations(ctx)
			if err != nil {
				return err
			}
			if res.Applied+res.Failed+res.Requeued+res.Abandoned+res.Errored > 0 || res.Halted {
				log.Printf("[REVOKE] applied=%d failed=%d requeued=%d abandoned=%d errored=%d halted=%v %s",
					res.Applied, res.Failed, res.Requeued, res.Abandoned, res.Errored, res.Halted, res.Reason)
			}
			return nil
		})
		go revokeSched.Start(ctx)
	} else {
		log.Println("[REVOKE] Disabled via REVOCATION_DRAIN_ENABLED=false")
	}

	// Add-on reconciliation: one pass per registered add-on target, resolving
	// current state and converging what has drifted (design §15). Deliberately
	// not a drift SWEEP — an operator-initiated change applies the snapshot that
	// was approved, and this resolves what policy says now, because a reconcile
	// that replayed snapshots would fight every legitimate edit.
	//
	// It converges nothing by itself: it queues, and the rows wait for the drain
	// like every other add-on row. What it does immediately is notice — an
	// account somebody changed on the NAS by hand, and every account on the
	// target that Syndra never provisioned.
	var reconcileSched *periodic.Runner
	if targets := addons.Registered(); len(targets) > 0 && driftSchedulerEnabled() {
		reconcileSched = periodic.New("ADDON-RECONCILE", driftInterval(), 6*time.Hour, func(ctx context.Context) error {
			for _, reg := range addons.Registered() {
				res, err := drift.ReconcileAddon(ctx, reg.Target)
				if err != nil {
					// One target's failure must not stop the others: they are
					// separate deployments with separate outages.
					log.Printf("[ADDON-RECONCILE] %s: %v", reg.Target, err)
					continue
				}
				if res.Queued > 0 || len(res.Unmanaged) > 0 || res.Halted {
					log.Printf("[ADDON-RECONCILE] %s bound=%d queued=%d unmanaged=%d halted=%v %s",
						reg.Target, res.Bound, res.Queued, len(res.Unmanaged), res.Halted, res.Reason)
				}
			}
			return nil
		})
		// Started once the manifests are in, not immediately. A reconcile
		// without a manifest cannot do anything but halt, and the halted pass
		// it records is what Home renders — for the six hours until the next
		// one, a deployment that came up perfectly reports a system Syndra
		// could not check.
		go func(r *periodic.Runner) {
			awaitFirstManifests(ctx)
			r.Start(ctx)
		}(reconcileSched)
	}

	// Add-on manifest refresh: reads each registered add-on's /capabilities,
	// checks the contract version, and resolves the effective operation set
	// against backend policy. Registration alone makes nothing callable — this
	// loop is what turns a configured add-on into a capable one, and what
	// notices when an operation stops being available on its target. Runs once
	// immediately, then on each tick.
	var addonSched *periodic.Runner
	if len(addons.Registered()) > 0 {
		addonSched = periodic.New("ADDON", addonRefreshInterval(), 15*time.Minute, addons.RefreshAll)
		go addonSched.Start(ctx)
		go awaitFirstManifests(ctx)
	}

	// The observation sweep.
	//
	// Its interval is the bound on how stale any surface's answer can be, and
	// that bound is the number this whole design trades for: five minutes means
	// a wrong belief about somebody's access cannot outlive five minutes
	// without a person being able to see how old it is. Before this there was
	// no bound at all — the store was fed by webhooks that drop Syndra's own
	// changes, so it could be wrong indefinitely, and was for twenty hours.
	//
	// Cheap at this scale: one paged listing of an organisation with a few
	// hundred grants. It becomes expensive somewhere in the tens of thousands,
	// which is written down in the design as the point where the answer
	// changes.
	var observeSched *periodic.Runner
	if zitadel.MgmtClient != nil {
		observeSched = periodic.New("OBSERVE", observeInterval(), 5*time.Minute, observe.Sweep)
		go observeSched.Start(ctx)
	}

	// Start server in background
	go func() {
		fmt.Println("Control Plane Backend Listening on :8080")
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server error: %v", err)
		}
	}()

	<-ctx.Done()

	fmt.Println("\nShutting down gracefully...")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("Server shutdown error: %v", err)
	}

	// Wait for the scheduler goroutine to exit, bounded by the same
	// shutdown deadline as the HTTP server. A stuck sweep cannot block
	// shutdown forever, but we must not tear down db.PG / db.Redis while
	// a sweep is mid-mutation — the worst case is a best-effort log from
	// a doomed mid-sweep call, not a silent lost intent or audit.
	if sched != nil {
		select {
		case <-sched.Done():
		case <-shutdownCtx.Done():
			log.Println("[SCHEDULER] Shutdown deadline exceeded waiting for scheduler; closing anyway")
		}
	}

	if driftSched != nil {
		select {
		case <-driftSched.Done():
		case <-shutdownCtx.Done():
			log.Println("[DRIFT] Shutdown deadline exceeded waiting for scheduler; closing anyway")
		}
	}

	if revokeSched != nil {
		select {
		case <-revokeSched.Done():
		case <-shutdownCtx.Done():
			log.Println("[REVOKE] Shutdown deadline exceeded waiting for drain; closing anyway")
		}
	}

	if reconcileSched != nil {
		select {
		case <-reconcileSched.Done():
		case <-shutdownCtx.Done():
			log.Println("[ADDON-RECONCILE] Shutdown deadline exceeded waiting for the pass; closing anyway")
		}
	}

	if addonSched != nil {
		select {
		case <-addonSched.Done():
		case <-shutdownCtx.Done():
			log.Println("[ADDON] Shutdown deadline exceeded waiting for refresh; closing anyway")
		}
	}

	db.PG.Close()
	if err := db.Redis.Close(); err != nil {
		log.Printf("Redis close error: %v", err)
	}

	fmt.Println("Server stopped.")
}

func schedulerEnabled() bool {
	v := os.Getenv("EXPIRY_SCHEDULER_ENABLED")
	if v == "" {
		return true
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		log.Printf("[SCHEDULER] Invalid EXPIRY_SCHEDULER_ENABLED=%q, defaulting to enabled", v)
		return true
	}
	return b
}

func schedulerInterval() time.Duration {
	v := os.Getenv("EXPIRY_SCHEDULER_INTERVAL")
	if v == "" {
		return 5 * time.Minute
	}
	d, err := time.ParseDuration(v)
	if err != nil || d <= 0 {
		log.Printf("[SCHEDULER] Invalid EXPIRY_SCHEDULER_INTERVAL=%q, defaulting to 5m", v)
		return 5 * time.Minute
	}
	return d
}

func schedulerBatchSize() int {
	v := os.Getenv("EXPIRY_SCHEDULER_BATCH_SIZE")
	if v == "" {
		return 500
	}
	n, err := strconv.Atoi(v)
	if err != nil || n < 1 {
		log.Printf("[SCHEDULER] Invalid EXPIRY_SCHEDULER_BATCH_SIZE=%q, defaulting to 500", v)
		return 500
	}
	return n
}

func driftSchedulerEnabled() bool {
	v := os.Getenv("DRIFT_SCHEDULER_ENABLED")
	if v == "" {
		return true
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		log.Printf("[DRIFT] Invalid DRIFT_SCHEDULER_ENABLED=%q, defaulting to enabled", v)
		return true
	}
	return b
}

// Enabled by default, because a revocation nobody drains is retained access and
// the safe default for that is to drain it. Disabling is for a deployment that
// deliberately wants every dispatch operator-observed.
func revocationDrainEnabled() bool {
	v := os.Getenv("REVOCATION_DRAIN_ENABLED")
	if v == "" {
		return true
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		log.Printf("[REVOKE] Invalid REVOCATION_DRAIN_ENABLED=%q, defaulting to enabled", v)
		return true
	}
	return b
}

func revocationDrainInterval() time.Duration {
	v := os.Getenv("REVOCATION_DRAIN_INTERVAL")
	if v == "" {
		return 5 * time.Minute
	}
	d, err := time.ParseDuration(v)
	if err != nil || d <= 0 {
		log.Printf("[REVOKE] Invalid REVOCATION_DRAIN_INTERVAL=%q, defaulting to 5m", v)
		return 5 * time.Minute
	}
	return d
}

func addonRefreshInterval() time.Duration {
	v := os.Getenv("ADDON_MANIFEST_REFRESH_INTERVAL")
	if v == "" {
		return 15 * time.Minute
	}
	d, err := time.ParseDuration(v)
	if err != nil || d <= 0 {
		log.Printf("[ADDON] Invalid ADDON_MANIFEST_REFRESH_INTERVAL=%q, defaulting to 15m", v)
		return 15 * time.Minute
	}
	return d
}

// driftInterval is the add-on reconciliation cadence (ADDON-RECONCILE) only.
// The Zitadel drift sweep no longer pays for a read of its own — it uses
// observeInterval() instead, right above driftSched's wiring in main(). Each
// add-on target still does its own live read on this schedule, which is the
// cost this env var controls.
func driftInterval() time.Duration {
	v := os.Getenv("DRIFT_RECONCILIATION_INTERVAL_HOURS")
	if v == "" {
		return 6 * time.Hour
	}
	n, err := strconv.Atoi(v)
	if err != nil || n < 1 {
		log.Printf("[DRIFT] Invalid DRIFT_RECONCILIATION_INTERVAL_HOURS=%q, defaulting to 6", v)
		return 6 * time.Hour
	}
	return time.Duration(n) * time.Hour
}
