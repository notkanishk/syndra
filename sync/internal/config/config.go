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
		RetryAttempts:          3,
		RetryBackoff:           1 * time.Second,
	}

	var err error
	cfg.PollInterval, err = time.ParseDuration(envOrDefault("SYNC_POLL_INTERVAL", "10s"))
	if err != nil {
		return cfg, fmt.Errorf("invalid SYNC_POLL_INTERVAL: %w", err)
	}

	cfg.WorkerCount, err = strconv.Atoi(envOrDefault("SYNC_WORKER_COUNT", "5"))
	if err != nil {
		return cfg, fmt.Errorf("invalid SYNC_WORKER_COUNT: %w", err)
	}

	cfg.IntentLimit, err = strconv.Atoi(envOrDefault("SYNC_INTENT_LIMIT", "50"))
	if err != nil {
		return cfg, fmt.Errorf("invalid SYNC_INTENT_LIMIT: %w", err)
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
