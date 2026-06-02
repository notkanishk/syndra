package worker

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"mkauth-sync/internal/backend"
	ldapclient "mkauth-sync/internal/ldap"
)

// BackendClient defines the backend API surface used by the worker.
type BackendClient interface {
	ClaimIntents(ctx context.Context, limit int) ([]backend.ProvisioningIntent, error)
	CompleteIntent(ctx context.Context, intentID string) error
	FailIntent(ctx context.Context, intentID string, errMsg string) error
	GetShadowCredentialHash(ctx context.Context, uid string) (hash, algorithm string, err error)
	GetUserProfile(ctx context.Context, uid string) (backend.UserProfile, error)
}

// LDAPPool defines the LLDAP operations used by the worker.
type LDAPPool interface {
	EnsureUser(ctx context.Context, uid, displayName, email string) error
	EnsureGroup(ctx context.Context, group string) error
	AddUserToGroup(ctx context.Context, targetUID, lldapGroup string) error
	RemoveUserFromGroup(ctx context.Context, targetUID, lldapGroup string) error
	SetUserPassword(ctx context.Context, targetUID, passwordHash string) error
}

// Config holds worker pool configuration.
type Config struct {
	PollInterval  time.Duration
	WorkerCount   int
	IntentLimit   int
	RetryAttempts int
	RetryBackoff  time.Duration
}

// Run starts the polling loop and worker pool.
// Blocks until ctx is cancelled, then drains in-flight work.
func Run(ctx context.Context, cfg Config, bc BackendClient, lp LDAPPool) {
	intentCh := make(chan backend.ProvisioningIntent, cfg.IntentLimit)
	locker := NewUIDLocker()
	var wg sync.WaitGroup

	// Start workers.
	for i := 0; i < cfg.WorkerCount; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for intent := range intentCh {
				processIntent(ctx, intent, bc, lp, locker, cfg)
			}
		}(i)
	}

	// Polling loop.
	ticker := time.NewTicker(cfg.PollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Println("[SYNC] Shutdown signal received, draining workers...")
			close(intentCh)
			wg.Wait()
			log.Println("[SYNC] All workers drained")
			return
		case <-ticker.C:
			poll(ctx, bc, intentCh, cfg.IntentLimit)
		}
	}
}

func poll(ctx context.Context, bc BackendClient, ch chan<- backend.ProvisioningIntent, limit int) {
	intents, err := bc.ClaimIntents(ctx, limit)
	if err != nil {
		log.Printf("[SYNC] Poll error: %v", err)
		return
	}
	if len(intents) == 0 {
		return
	}
	log.Printf("[SYNC] Claimed %d intents", len(intents))
	for _, intent := range intents {
		ch <- intent
	}
}

func processIntent(
	ctx context.Context,
	intent backend.ProvisioningIntent,
	bc BackendClient,
	lp LDAPPool,
	locker *UIDLocker,
	cfg Config,
) {
	log.Printf("[SYNC] Processing intent %s: %s user=%s group=%s",
		intent.ID, intent.Action, intent.TargetUID, intent.LLDAPGroup)

	locker.Lock(intent.TargetUID)
	defer locker.Unlock(intent.TargetUID)

	var err error
	switch intent.Action {
	case "add":
		err = processAdd(ctx, intent, bc, lp, cfg)
	case "remove":
		err = retryTransient(ctx, cfg, func() error {
			return lp.RemoveUserFromGroup(ctx, intent.TargetUID, intent.LLDAPGroup)
		})
	default:
		err = fmt.Errorf("unknown action: %s", intent.Action)
	}

	if err != nil {
		log.Printf("[SYNC] Intent %s FAILED: %v", intent.ID, err)
		_ = bc.FailIntent(ctx, intent.ID, err.Error())
		return
	}

	// Best-effort shadow password sync.
	syncShadowPassword(ctx, intent.TargetUID, bc, lp, cfg)

	if err := bc.CompleteIntent(ctx, intent.ID); err != nil {
		log.Printf("[SYNC] Failed to mark intent %s complete: %v", intent.ID, err)
		return
	}

	log.Printf("[SYNC] Intent %s COMPLETED", intent.ID)
}

func processAdd(
	ctx context.Context,
	intent backend.ProvisioningIntent,
	bc BackendClient,
	lp LDAPPool,
	cfg Config,
) error {
	// Fetch user profile for LLDAP user attributes.
	profile, err := bc.GetUserProfile(ctx, intent.TargetUID)
	if err != nil {
		log.Printf("[SYNC] Warning: could not fetch profile for %s: %v (proceeding with uid only)", intent.TargetUID, err)
		profile = backend.UserProfile{UserID: intent.TargetUID, DisplayName: intent.TargetUID}
	}

	// Ensure user exists in LLDAP with readable attributes.
	if err := retryTransient(ctx, cfg, func() error {
		return lp.EnsureUser(ctx, intent.TargetUID, profile.DisplayName, profile.Email)
	}); err != nil {
		return fmt.Errorf("ensure user: %w", err)
	}

	// Ensure group exists.
	if err := retryTransient(ctx, cfg, func() error {
		return lp.EnsureGroup(ctx, intent.LLDAPGroup)
	}); err != nil {
		return fmt.Errorf("ensure group: %w", err)
	}

	// Add user to group.
	if err := retryTransient(ctx, cfg, func() error {
		return lp.AddUserToGroup(ctx, intent.TargetUID, intent.LLDAPGroup)
	}); err != nil {
		return fmt.Errorf("add to group: %w", err)
	}

	return nil
}

func syncShadowPassword(
	ctx context.Context,
	uid string,
	bc BackendClient,
	lp LDAPPool,
	cfg Config,
) {
	hash, _, err := bc.GetShadowCredentialHash(ctx, uid)
	if err != nil {
		log.Printf("[SYNC] Shadow credential check failed for %s: %v", uid, err)
		return
	}
	if hash == "" {
		return // No shadow credential set.
	}

	// The shadow-credential hash is a Go string (immutable; the GC retains
	// the original allocation until collection). A defer-loop that zeros a
	// []byte(hash) copy is theatre: it cannot reach the source string. The
	// real mitigation is keeping the lifetime short — pass directly to
	// SetUserPassword and let the function return GC the value.
	err = retryTransient(ctx, cfg, func() error {
		return lp.SetUserPassword(ctx, uid, hash)
	})
	if err != nil {
		log.Printf("[SYNC] Shadow password sync failed for %s: %v", uid, err)
		return
	}
	log.Printf("[SYNC] Shadow password synced for %s", uid)
}

// retryTransient retries fn on transient LDAP errors with exponential backoff.
// Permanent errors are returned immediately.
func retryTransient(ctx context.Context, cfg Config, fn func() error) error {
	var lastErr error
	for attempt := 0; attempt <= cfg.RetryAttempts; attempt++ {
		if attempt > 0 {
			backoff := cfg.RetryBackoff * time.Duration(1<<(attempt-1))
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(backoff):
			}
		}
		lastErr = fn()
		if lastErr == nil {
			return nil
		}
		if !ldapclient.IsConnectionError(lastErr) {
			return lastErr // Permanent failure — don't retry.
		}
		log.Printf("[SYNC] Transient error (attempt %d/%d): %v", attempt+1, cfg.RetryAttempts+1, lastErr)
	}
	return fmt.Errorf("exhausted %d retries: %w", cfg.RetryAttempts+1, lastErr)
}
