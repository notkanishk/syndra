package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"mkauth/internal/db"
	"mkauth/internal/directory"
	"mkauth/internal/handlers"
	"mkauth/internal/seed"
	"mkauth/internal/services/expiry"
	"mkauth/internal/zitadel"
)

func main() {
	fmt.Println("MkAuth Backend Starting...")

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
	var sched *expiry.Scheduler
	if schedulerEnabled() {
		sched = expiry.NewScheduler(schedulerInterval(), schedulerBatchSize())
		go sched.Start(ctx)
	} else {
		log.Println("[SCHEDULER] Disabled via EXPIRY_SCHEDULER_ENABLED=false")
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
