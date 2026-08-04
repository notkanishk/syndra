package main

import (
	"context"
	"log"
	"os/signal"
	"syscall"

	"syndra-sync/internal/backend"
	"syndra-sync/internal/config"
	ldapclient "syndra-sync/internal/ldap"
	"syndra-sync/internal/worker"
)

func main() {
	log.Println("[SYNC] Syndra Sync Service starting...")

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("[SYNC] Configuration error: %v", err)
	}

	// Init LLDAP connection.
	ldapPool, err := ldapclient.Connect(ldapclient.Config{
		URL:                cfg.LDAPURL,
		BindDN:             cfg.LDAPBindDN,
		BindPassword:       cfg.LDAPBindPassword,
		BaseDN:             cfg.LDAPBaseDN,
		InsecureSkipVerify: cfg.LDAPInsecureSkipVerify,
	})
	if err != nil {
		log.Fatalf("[SYNC] LLDAP connection failed: %v", err)
	}
	defer ldapPool.Close()

	// Init backend HTTP client.
	bc := backend.NewClient(cfg.BackendURL, cfg.APIKey)

	log.Printf("[SYNC] Connected. Polling every %s with %d workers", cfg.PollInterval, cfg.WorkerCount)

	// Signal-aware context for graceful shutdown.
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Run blocks until ctx is cancelled, then drains workers.
	worker.Run(ctx, worker.Config{
		PollInterval:  cfg.PollInterval,
		WorkerCount:   cfg.WorkerCount,
		IntentLimit:   cfg.IntentLimit,
		RetryAttempts: cfg.RetryAttempts,
		RetryBackoff:  cfg.RetryBackoff,
	}, bc, ldapPool)

	log.Println("[SYNC] Sync service stopped.")
}
