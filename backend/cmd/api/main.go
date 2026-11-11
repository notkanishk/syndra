package main

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"
	// The key replacement's quiet window is local time (TZ); the alpine image
	// carries no zone files.
	_ "time/tzdata"

	"syndra/internal/addons"
	"syndra/internal/cache"
	"syndra/internal/claimserve"
	"syndra/internal/db"
	"syndra/internal/directory"
	"syndra/internal/handlers"
	"syndra/internal/observe"
	"syndra/internal/ratelimit"
	"syndra/internal/seed"
	"syndra/internal/services"
	"syndra/internal/services/drift"
	"syndra/internal/services/expiry"
	"syndra/internal/services/periodic"
	"syndra/internal/services/propagation"
	"syndra/internal/services/stats"
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

// liveAuthConfigProblems names what makes a live deployment (ZITADEL_DOMAIN set)
// unfit to start. An empty audience makes every token check answer 500 rather
// than fail at boot, and the dev API key is a published literal; neither should
// reach a running service.
func liveAuthConfigProblems() []string {
	if os.Getenv("ZITADEL_DOMAIN") == "" {
		return nil // demo mode: the shared key is the whole auth story, by design
	}
	var problems []string
	if os.Getenv("ZITADEL_AUDIENCE") == "" {
		problems = append(problems, "ZITADEL_AUDIENCE is empty")
	}
	if os.Getenv("SYNDRA_API_KEY") == "dev_auth_token_secret" {
		problems = append(problems, "SYNDRA_API_KEY is the development default")
	}
	if os.Getenv("ZITADEL_FAKE_WORLD") != "" {
		problems = append(problems, "ZITADEL_FAKE_WORLD is set (the staging stand-in never runs beside a real Zitadel)")
	}
	return problems
}

// claimsConfigProblems names the claims configuration that must refuse a
// start (claims-delivery-integrity §4f): a config error is loud at boot, never
// a silent miss on every token. A dependency that is down is not one of these:
// Syndra starts degraded and /healthz/claims says which.
func claimsConfigProblems(getenv func(string) string) []string {
	var problems []string
	live := getenv("ZITADEL_DOMAIN") != ""
	if key := getenv("ZITADEL_ACTION_SIGNING_KEY"); live && key != "" {
		if p := claimserve.KeyProblem(key); p != "" {
			problems = append(problems, "ZITADEL_ACTION_SIGNING_KEY "+p)
		}
	}
	ext := getenv("SYNDRA_EXTERNAL_URL")
	u, err := url.Parse(ext)
	switch {
	case live && ext == "":
		problems = append(problems, "SYNDRA_EXTERNAL_URL is empty (Zitadel calls it, and the L1 probe checks it)")
	case ext != "" && (err != nil || u.Host == "" || (u.Scheme != "https" && u.Scheme != "http")):
		problems = append(problems, "SYNDRA_EXTERNAL_URL is not an http(s) URL")
	case live && u.Scheme != "https":
		problems = append(problems, "SYNDRA_EXTERNAL_URL is not https")
	}
	// L2 needs a project; the key defaults to Syndra's own service account and
	// the client id to the token's own. A key or client id with no project is
	// half set.
	project := getenv("SYNDRA_CANARY_PROJECT_ID")
	var extra []string
	for _, k := range []string{"SYNDRA_CANARY_KEY_PATH", "SYNDRA_CANARY_CLIENT_ID"} {
		if getenv(k) != "" {
			extra = append(extra, k)
		}
	}
	switch {
	case project == "" && len(extra) > 0:
		problems = append(problems, "the L2 canary is half set ("+strings.Join(extra, ", ")+" set, SYNDRA_CANARY_PROJECT_ID not)")
	case project != "" && !live:
		problems = append(problems, "the L2 canary needs ZITADEL_DOMAIN")
	case project != "" && getenv("SYNDRA_CANARY_KEY_PATH") == "" && getenv("ZITADEL_MACHINE_KEY_PATH") == "":
		problems = append(problems, "the L2 canary has no key (SYNDRA_CANARY_KEY_PATH or ZITADEL_MACHINE_KEY_PATH)")
	}
	return problems
}

func requireClaimsConfig() {
	if p := claimsConfigProblems(os.Getenv); len(p) > 0 {
		log.Fatalf("[STARTUP] Refusing to start: %s.", strings.Join(p, "; "))
	}
}

func requireLiveAuthConfig() {
	if p := liveAuthConfigProblems(); len(p) > 0 {
		log.Fatalf("[STARTUP] Production refusing to start: ZITADEL_DOMAIN is set but %s.", strings.Join(p, " and "))
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

// startupWaits is the one start-up schedule (about two minutes in all) that
// both awaitFirstManifests and awaitFirstObservation wait on before giving up
// onto their regular interval.
var startupWaits = []time.Duration{2 * time.Second, 3 * time.Second, 5 * time.Second, 10 * time.Second, 15 * time.Second, 30 * time.Second, 45 * time.Second}

func awaitFirstManifests(ctx context.Context) {
	// Each start-up attempt that finds the add-on not listening yet is the
	// ordinary case, not an error: quiet while waiting, one line at the end.
	addons.QuietStartup(true)
	defer addons.QuietStartup(false)
	for _, wait := range startupWaits {
		if len(addons.PendingManifests()) == 0 {
			return
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(wait):
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
	for _, wait := range startupWaits {
		if checkObservation(ctx) == nil {
			return
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(wait):
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
	requireLiveAuthConfig()
	requireClaimsConfig()
	fmt.Println("Syndra Backend Starting...")

	// Root context cancelled on SIGINT/SIGTERM. Created before anything that
	// runs in the background so all of it shares one shutdown signal.
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// The claims data plane first (claims-delivery-integrity S2, ADR-004): it
	// needs only Redis, so it loads before migrations, the Zitadel client and
	// the add-on refresh. Since S7 the claims container (cmd/claims) owns :8090
	// and this process serves the claims route only on :8080, the fallback
	// until Caddy routes /api/action/* to the container; the detectors read
	// this process's snapshot either way.
	db.ConnectRedis()
	handlers.StartClaimsPlane(ctx, db.Redis)
	// Failed signatures per source, and a member's own writes per person
	// (rate-limits); a verified call is never counted, Redis down allows all.
	handlers.SetRateLimiter(ratelimit.FromEnv(db.Redis, os.Getenv))

	// The admin side needs both datastores. Waiting for them is not fatal:
	// the data plane above keeps answering while they come up (S15).
	if err := db.AwaitRedis(ctx); err != nil {
		return
	}
	for {
		err := db.ConnectPostgres()
		if err == nil {
			break
		}
		log.Printf("[STARTUP] waiting for Postgres (claims keep serving): %v", err)
		select {
		case <-ctx.Done():
			return
		case <-time.After(2 * time.Second):
		}
	}

	// Initialize Zitadel service account auth before seeding so the seed path
	// can see whether we're in live mode and skip itself accordingly.
	if err := zitadel.InitClient(); err != nil {
		// A stand-in that won't load, or one beside a real Zitadel, refuses
		// the start: falling back to the demo would hide it.
		if os.Getenv("ZITADEL_FAKE_WORLD") != "" {
			log.Fatalf("[STARTUP] Refusing to start: %v", err)
		}
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
	// Usually before the add-on container listens: quiet, and the start-up
	// wait (awaitFirstManifests) says once what is still missing.
	addons.QuietStartup(true)
	_ = addons.RefreshAll(context.Background())

	if err := seed.EnsureDemoData(context.Background()); err != nil {
		log.Fatalf("Demo seed failed: %v", err)
	}

	warnIfWelcomeBundleMissing(context.Background())

	mux := handlers.NewRouter()

	server := &http.Server{
		Addr:    ":8080",
		Handler: mux,
		// Slow-header and idle-connection limits only. No WriteTimeout: drain and
		// reconcile handlers legitimately run for minutes and would be cut off
		// mid-mutation.
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	// The write side of the claims snapshot: every project Syndra or Zitadel
	// knows, published now and every minute. Saves publish at once; this pass
	// catches a publish that failed, a failure mode edited in SQL and a
	// project new in Zitadel. Only what changed moves claims:gen.
	claimPublishSched := periodic.New("CLAIM-PUBLISH", time.Minute, time.Minute, handlers.PublishAllClaimConfig)
	go claimPublishSched.Start(ctx)

	// Claims warm-up: the token path serves only compiled claims, which change
	// events write and a day's TTL removes. One pass now, then twice per TTL, so
	// a restart or a quiet day never leaves someone's token degraded.
	warmSched := periodic.New("CACHE", 12*time.Hour, 12*time.Hour, func(ctx context.Context) error {
		return cache.Warm(ctx)
	})
	go warmSched.Start(ctx)

	// Claims seed (A-14): every app that existed under Actions v1 gets v1's
	// exact shape as version 1, so turning v1 off changes nothing. Apps live in
	// Zitadel, so this can't be SQL; it is idempotent, so hourly catches an app
	// a partial directory read missed. Reading an app's claims also seeds it.
	seedSched := periodic.New("CLAIM-SEED", time.Hour, time.Hour, handlers.SeedClaimShapes)
	go seedSched.Start(ctx)

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
	// it must not wait on somebody opening the right page. It also sends again
	// any change already sent once that Zitadel didn't take, at its next try
	// (ClaimDueRetries; never a row that waits for you). Shares the operator
	// drain's advisory lock, so the two never dispatch concurrently; a pass that
	// cannot take it simply returns and the next tick tries again.
	var revokeSched *periodic.Runner
	// Said wherever a retry is: off, "Syndra sends it again at …" would be false.
	propagation.SetAutoRetry(revocationDrainEnabled())
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
		// After the add-ons have answered, like the reconcile below. Started
		// at once, its first pass ran in the second before the add-on
		// container listened and reported a target it could not reach, on
		// every deploy. Bounded: an add-on that never answers delays the first
		// pass by awaitFirstManifests' ceiling (under two minutes), no more.
		if len(addons.Registered()) > 0 {
			go func(r *periodic.Runner) {
				awaitFirstManifests(ctx)
				r.Start(ctx)
			}(revokeSched)
		} else {
			go revokeSched.Start(ctx)
		}
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

	// Statistics (addon-contract 6.6, 6.7): each add-on's declared metrics
	// collected hourly, each from its own watermark and in its own
	// transaction, and the nightly retention. Runners beside the reconcile,
	// never steps inside it: a slow /stats must not delay convergence, and a
	// failed collection is not a failed reconcile. Collection starts once the
	// manifests are in (a metric is collected only once accepted);
	// STATS_COLLECTION_ENABLED=false stops both, for a kit run against a
	// target this backend also reads.
	var statsSched, statsPruneSched *periodic.Runner
	if len(addons.Registered()) > 0 && statsCollectionEnabled() {
		statsSched = periodic.New("ADDON-STATS", time.Hour, time.Hour, stats.CollectAll)
		statsPruneSched = periodic.New("STATS-RETENTION", 24*time.Hour, 24*time.Hour, stats.Prune)
		go func(r *periodic.Runner) {
			awaitFirstManifests(ctx)
			r.Start(ctx)
		}(statsSched)
		go statsPruneSched.Start(ctx)
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
		// Quiet before the first refresh, which runs at once and usually meets
		// an add-on still starting; awaitFirstManifests lifts it.
		addons.QuietStartup(true)
		go addonSched.Start(ctx)
		go awaitFirstManifests(ctx)
	}

	// Target health: one read per add-on every 30s, so "offline" is confirmed
	// (three misses in a row, addons.HealthOf) rather than said on the first
	// slow answer, and the count keeps moving when nobody has a page open.
	// Read-only, and never awaited at shutdown: an abandoned read changes nothing.
	if len(addons.Registered()) > 0 {
		go periodic.New("HEALTH", 30*time.Second, 30*time.Second, addons.ProbeAll).Start(ctx)
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

	// Insights "As of a date": one snapshot of the access graph per local day,
	// kept from now on because the audit log cannot rebuild a past graph. Runs at
	// once and every six hours; it only builds when today has no row, and a failed
	// build is logged and retried on the next tick. Started after the first
	// observation so the day's holder counts are Zitadel's, not the fallback.
	insightsSched := periodic.New("FINDINGS", 6*time.Hour, 6*time.Hour, services.TakeDailyInsightsSnapshot)
	go func(r *periodic.Runner) {
		awaitFirstObservation(ctx)
		r.Start(ctx)
	}(insightsSched)

	// Insights usage: drop "last seen" rows nobody has refreshed in 400 days. Once
	// a day, at once on boot; a failure is logged and retried on the next tick.
	// The same nightly pass rolls claim-delivery telemetry older than 48h into
	// hourly rows and drops hourly rows past 90 days, and recounts the daily
	// sign-in totals 000079 asks for (a no-op once done).
	usagePruneSched := periodic.New("FINDINGS", 24*time.Hour, 24*time.Hour, func(ctx context.Context) error {
		uerr := errors.Join(services.PruneUsageLastSeen(ctx), handlers.RecountSignInDays(ctx))
		n, cerr := db.RollClaimDelivery(ctx, time.Now())
		if n > 0 {
			log.Printf("[DATA PLANE] telemetry: rolled %d five-minute rows into hours", n)
		}
		return errors.Join(uerr, cerr)
	})
	go usagePruneSched.Start(ctx)

	// Webhook log: the operations page lists the newest 200, so rows past the
	// retention window are only table weight. Same cadence as the usage prune.
	webhookPruneSched := periodic.New("WEBHOOK", 24*time.Hour, 24*time.Hour, func(ctx context.Context) error {
		n, err := db.PruneWebhookEvents(ctx, db.WebhookEventRetentionDays)
		if n > 0 {
			log.Printf("[WEBHOOK] pruned %d events older than %d days", n, db.WebhookEventRetentionDays)
		}
		return err
	})
	go webhookPruneSched.Start(ctx)

	// Claim delivery telemetry: counters fold into Postgres once a minute; a
	// failed flush keeps them for the next. Repair-on-miss recompiles people
	// whose facts a token found missing, at most 20 a second.
	// The claims container's counters arrive through Redis and are folded on
	// the same tick; its repair, keyring and debug signals every second.
	claimTelSched := periodic.New("CLAIM-TEL", time.Minute, time.Minute, func(ctx context.Context) error {
		return errors.Join(handlers.FlushClaimDelivery(ctx), handlers.FoldClaimRelay(ctx))
	})
	go claimTelSched.Start(ctx)
	go handlers.RunClaimRepair(ctx)
	go handlers.RunClaimSignals(ctx)

	// The detectors (claims-delivery-integrity S3-S5). The correlator checks
	// sign-ins against calls every minute. With Zitadel connected, the
	// configuration reconciler reads its Actions setup at start and every ten
	// minutes, and the keyring follows the claims target's signing key. With
	// SYNDRA_DEADMAN_URL set, Syndra pings it every five minutes while claim
	// delivery is ok; a missed ping is the owner's alert.
	claimWatchSched := periodic.New("CLAIM-WATCH", time.Minute, time.Minute, handlers.RunClaimCorrelation)
	go claimWatchSched.Start(ctx)
	var claimConfigSched *periodic.Runner
	if zitadel.MgmtClient != nil {
		claimConfigSched = periodic.New("CLAIM-CONFIG", 10*time.Minute, 10*time.Minute, handlers.RunClaimConfigCheck)
		go claimConfigSched.Start(ctx)
		go handlers.RunKeyring(ctx)
		// Syndra replaces the Actions signing keys itself a few days before
		// the limit, in the quiet window (action-key-autorotation, ADR-005).
		// SYNDRA_ACTION_KEY_AUTOROTATE=off is the kill switch; the look still
		// runs, to confirm an unverified replacement once the probes pass.
		go periodic.New("ACTION-KEY", handlers.ActionKeyAutoEvery, handlers.ActionKeyAutoEvery, handlers.RunActionKeyAutoRotate).Start(ctx)
	}
	if handlers.DeadmanConfigured() {
		go periodic.New("DEADMAN", handlers.DeadmanEvery, handlers.DeadmanEvery, handlers.PingDeadman).Start(ctx)
	} else {
		log.Println("[DATA PLANE] dead-man: SYNDRA_DEADMAN_URL is unset: no external alert is configured (shown on /healthz/claims)")
	}

	// Listen, then serve in the background. The listener is open before the
	// probes start, so the first L1 can reach this door through Caddy.
	ln, err := net.Listen("tcp", server.Addr)
	if err != nil {
		log.Fatalf("Server error: %v", err)
	}
	go func() {
		fmt.Println("Control Plane Backend Listening on :8080")
		if err := server.Serve(ln); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server error: %v", err)
		}
	}()

	// The self-check after listen (§4f): L1 at once and every minute (the
	// reconciler above already ran at start); L2 every 15 minutes once the
	// owner has made the canary in Zitadel. The deploy job waits for L1.
	var probeScheds []*periodic.Runner
	if handlers.ProbeL1Enabled() {
		probeScheds = append(probeScheds, periodic.New("PROBE-L1", time.Minute, time.Minute, handlers.RunProbeL1))
	}
	if handlers.ProbeL2Enabled() {
		probeScheds = append(probeScheds, periodic.New("PROBE-L2", 15*time.Minute, 15*time.Minute, handlers.RunProbeL2))
	}
	for _, r := range probeScheds {
		// 5s in: the start-up publish and the warm (which writes the canary
		// first) have landed and the snapshot has polled, so a boot does not
		// raise a finding that the next minute settles. Still within §4f's 10s.
		go func(r *periodic.Runner) {
			select {
			case <-ctx.Done():
				return
			case <-time.After(5 * time.Second):
			}
			r.Start(ctx)
		}(r)
	}

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

	select {
	case <-warmSched.Done():
	case <-shutdownCtx.Done():
		log.Println("[CACHE] Shutdown deadline exceeded waiting for the warm-up; closing anyway")
	}
	select {
	case <-seedSched.Done():
	case <-shutdownCtx.Done():
		log.Println("[CLAIM-SEED] Shutdown deadline exceeded waiting for the seed; closing anyway")
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

	for _, r := range []*periodic.Runner{statsSched, statsPruneSched} {
		if r == nil {
			continue
		}
		select {
		case <-r.Done():
		case <-shutdownCtx.Done():
			log.Println("[STATS] Shutdown deadline exceeded waiting for a collection; closing anyway")
		}
	}

	if addonSched != nil {
		select {
		case <-addonSched.Done():
		case <-shutdownCtx.Done():
			log.Println("[ADDON] Shutdown deadline exceeded waiting for refresh; closing anyway")
		}
	}

	select {
	case <-insightsSched.Done():
	case <-shutdownCtx.Done():
		log.Println("[FINDINGS] Shutdown deadline exceeded waiting for the snapshot; closing anyway")
	}

	select {
	case <-usagePruneSched.Done():
	case <-shutdownCtx.Done():
		log.Println("[FINDINGS] Shutdown deadline exceeded waiting for the usage prune; closing anyway")
	}

	select {
	case <-webhookPruneSched.Done():
	case <-shutdownCtx.Done():
		log.Println("[WEBHOOK] Shutdown deadline exceeded waiting for the webhook prune; closing anyway")
	}

	select {
	case <-claimTelSched.Done():
	case <-shutdownCtx.Done():
	}
	// The last minute's counters: the server has stopped, so nothing adds to them.
	if err := handlers.FlushClaimDelivery(shutdownCtx); err != nil {
		log.Printf("[DATA PLANE] telemetry: final flush failed; the last minute of counts is lost: %v", err)
	}

	select {
	case <-claimPublishSched.Done():
	case <-shutdownCtx.Done():
	}
	select {
	case <-claimWatchSched.Done():
	case <-shutdownCtx.Done():
	}
	if claimConfigSched != nil {
		select {
		case <-claimConfigSched.Done():
		case <-shutdownCtx.Done():
		}
	}

	for _, r := range probeScheds {
		select {
		case <-r.Done():
		case <-shutdownCtx.Done():
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

// statsCollectionEnabled is STATS_COLLECTION_ENABLED, on unless it says
// otherwise.
func statsCollectionEnabled() bool {
	b, err := strconv.ParseBool(cmp.Or(os.Getenv("STATS_COLLECTION_ENABLED"), "true"))
	if err != nil {
		log.Printf("[STATS] Invalid STATS_COLLECTION_ENABLED=%q, defaulting to enabled", os.Getenv("STATS_COLLECTION_ENABLED"))
		return true
	}
	return b
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

// A minute, because the screens name a change's next try ("Syndra sends it
// again at 14:05") and the shortest backoff is a minute: a slower tick would
// make that time a promise the runner keeps late. Each tick is one cheap probe
// when there is nothing to send.
func revocationDrainInterval() time.Duration {
	v := os.Getenv("REVOCATION_DRAIN_INTERVAL")
	if v == "" {
		return time.Minute
	}
	d, err := time.ParseDuration(v)
	if err != nil || d <= 0 {
		log.Printf("[REVOKE] Invalid REVOCATION_DRAIN_INTERVAL=%q, defaulting to 1m", v)
		return time.Minute
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
