package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

// Config holds all sync service configuration, populated from environment variables.
type Config struct {
	// Backend API
	BackendURL string
	APIKey     string

	// LLDAP
	LDAPURL                string
	LDAPBindDN             string
	LDAPBindPassword       string
	LDAPBaseDN             string
	LDAPInsecureSkipVerify bool

	// Worker
	PollInterval  time.Duration
	WorkerCount   int
	IntentLimit   int
	RetryAttempts int
	RetryBackoff  time.Duration
}

// Load reads configuration from environment variables, applies defaults,
// and validates required fields.
func Load() (Config, error) {
	cfg := Config{
		BackendURL:             envOrDefault("BACKEND_URL", "http://backend:8080"),
		APIKey:                 os.Getenv("MKAUTH_API_KEY"),
		LDAPURL:                envOrDefault("LLDAP_URL", "ldaps://lldap:636"),
		LDAPBindDN:             os.Getenv("LLDAP_BIND_DN"),
		LDAPBindPassword:       os.Getenv("LLDAP_BIND_PASSWORD"),
		LDAPBaseDN:             envOrDefault("LLDAP_BASE_DN", "dc=example,dc=com"),
		LDAPInsecureSkipVerify: envOrDefault("LLDAP_INSECURE_SKIP_VERIFY", "false") == "true",
	}

	var err error
	if cfg.PollInterval, err = getEnvDuration("SYNC_POLL_INTERVAL", "10s"); err != nil {
		return cfg, err
	}
	if cfg.WorkerCount, err = getEnvInt("SYNC_WORKER_COUNT", "5"); err != nil {
		return cfg, err
	}
	if cfg.IntentLimit, err = getEnvInt("SYNC_INTENT_LIMIT", "50"); err != nil {
		return cfg, err
	}

	if cfg.RetryAttempts, err = getEnvInt("SYNC_RETRY_ATTEMPTS", "3"); err != nil {
		return cfg, err
	}
	if cfg.RetryAttempts < 1 {
		// The spec requires a positive integer. retryTransient runs
		// RetryAttempts+1 total attempts (it iterates `attempt <= RetryAttempts`
		// from 0): a negative value skips the loop so fn is never called and the
		// caller gets a spurious "exhausted 0 retries"; 0 means no retries at
		// all. Both are disallowed by the positive-integer contract.
		return cfg, fmt.Errorf("invalid SYNC_RETRY_ATTEMPTS: must be a positive integer, got %d", cfg.RetryAttempts)
	}

	if cfg.RetryBackoff, err = getEnvDuration("SYNC_RETRY_BACKOFF", "1s"); err != nil {
		return cfg, err
	}
	if cfg.RetryBackoff <= 0 {
		// A non-positive backoff makes exponential backoff a no-op — retries
		// would fire immediately, hammering a struggling LLDAP.
		return cfg, fmt.Errorf("invalid SYNC_RETRY_BACKOFF: must be a positive duration, got %s", cfg.RetryBackoff)
	}

	// Validate required fields.
	if cfg.APIKey == "" {
		return cfg, fmt.Errorf("MKAUTH_API_KEY is required")
	}
	if cfg.LDAPBindDN == "" {
		return cfg, fmt.Errorf("LLDAP_BIND_DN is required")
	}
	if cfg.LDAPBindPassword == "" {
		return cfg, fmt.Errorf("LLDAP_BIND_PASSWORD is required")
	}

	return cfg, nil
}

func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// getEnvInt reads key as an integer, falling back to fallback when unset/empty.
// A parse error is wrapped with the key name so the caller can return it as-is.
func getEnvInt(key, fallback string) (int, error) {
	v, err := strconv.Atoi(envOrDefault(key, fallback))
	if err != nil {
		return 0, fmt.Errorf("invalid %s: %w", key, err)
	}
	return v, nil
}

// getEnvDuration reads key as a Go duration, falling back to fallback when
// unset/empty. A parse error is wrapped with the key name.
func getEnvDuration(key, fallback string) (time.Duration, error) {
	v, err := time.ParseDuration(envOrDefault(key, fallback))
	if err != nil {
		return 0, fmt.Errorf("invalid %s: %w", key, err)
	}
	return v, nil
}
