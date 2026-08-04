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

	"syndra/internal/db"
	"syndra/internal/directory"
	"syndra/internal/handlers"
	"syndra/internal/seed"
	"syndra/internal/services/drift"
	"syndra/internal/services/expiry"
	"syndra/internal/services/periodic"
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
			return nil
		})
		go sched.Start(ctx)
	} else {
		log.Println("[SCHEDULER] Disabled via EXPIRY_SCHEDULER_ENABLED=false")
	}

	// Drift reconciliation scheduler: periodic Zitadel↔Syndra sweep (B2/C6).
	var driftSched *periodic.Runner
	if driftSchedulerEnabled() {
		driftSched = periodic.New("DRIFT", driftInterval(), 6*time.Hour, func(ctx context.Context) error {
			_, err := drift.Sweep(ctx)
			return err
		})
		go driftSched.Start(ctx)
	} else {
		log.Println("[DRIFT] Disabled via DRIFT_SCHEDULER_ENABLED=false")
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
