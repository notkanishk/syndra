package config

import (
	"testing"
	"time"
)

func TestLoad_RetryAttemptsAndBackoffFromEnv(t *testing.T) {
	// Required vars (LoadConfig errors without them).
	t.Setenv("SYNDRA_API_KEY", "test-key")
	t.Setenv("LLDAP_BIND_DN", "uid=admin,dc=example,dc=com")
	t.Setenv("LLDAP_BIND_PASSWORD", "test-pw")

	t.Setenv("SYNC_RETRY_ATTEMPTS", "5")
	t.Setenv("SYNC_RETRY_BACKOFF", "250ms")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if cfg.RetryAttempts != 5 {
		t.Errorf("RetryAttempts: got %d, want 5", cfg.RetryAttempts)
	}
	if cfg.RetryBackoff != 250*time.Millisecond {
		t.Errorf("RetryBackoff: got %v, want 250ms", cfg.RetryBackoff)
	}
}

func TestLoad_RetryDefaultsWhenEnvAbsent(t *testing.T) {
	t.Setenv("SYNDRA_API_KEY", "test-key")
	t.Setenv("LLDAP_BIND_DN", "uid=admin,dc=example,dc=com")
	t.Setenv("LLDAP_BIND_PASSWORD", "test-pw")
	// Explicitly clear in case the host env has them.
	t.Setenv("SYNC_RETRY_ATTEMPTS", "")
	t.Setenv("SYNC_RETRY_BACKOFF", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if cfg.RetryAttempts != 3 {
		t.Errorf("RetryAttempts default: got %d, want 3", cfg.RetryAttempts)
	}
	if cfg.RetryBackoff != 1*time.Second {
		t.Errorf("RetryBackoff default: got %v, want 1s", cfg.RetryBackoff)
	}
}

func TestLoad_InvalidRetryAttemptsReturnsError(t *testing.T) {
	t.Setenv("SYNDRA_API_KEY", "test-key")
	t.Setenv("LLDAP_BIND_DN", "uid=admin,dc=example,dc=com")
	t.Setenv("LLDAP_BIND_PASSWORD", "test-pw")
	t.Setenv("SYNC_RETRY_ATTEMPTS", "not-a-number")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for invalid SYNC_RETRY_ATTEMPTS, got nil")
	}
}

// The spec requires SYNC_RETRY_ATTEMPTS to be a positive integer.
// retryTransient iterates `attempt <= RetryAttempts` from 0: a negative value
// makes the loop body never execute, so fn is never called and the caller gets
// a spurious "exhausted 0 retries" error; 0 means no retries at all. Both are
// rejected by the positive-integer contract.
func TestLoad_NonPositiveRetryAttemptsReturnsError(t *testing.T) {
	for _, v := range []string{"0", "-1"} {
		t.Run(v, func(t *testing.T) {
			t.Setenv("SYNDRA_API_KEY", "test-key")
			t.Setenv("LLDAP_BIND_DN", "uid=admin,dc=example,dc=com")
			t.Setenv("LLDAP_BIND_PASSWORD", "test-pw")
			t.Setenv("SYNC_RETRY_ATTEMPTS", v)

			if _, err := Load(); err == nil {
				t.Fatalf("expected error for non-positive SYNC_RETRY_ATTEMPTS=%q, got nil", v)
			}
		})
	}
}

// A zero or negative backoff makes exponential backoff a no-op (retries fire
// immediately), defeating the purpose of the setting.
func TestLoad_NonPositiveRetryBackoffReturnsError(t *testing.T) {
	for _, v := range []string{"0s", "-5s"} {
		t.Run(v, func(t *testing.T) {
			t.Setenv("SYNDRA_API_KEY", "test-key")
			t.Setenv("LLDAP_BIND_DN", "uid=admin,dc=example,dc=com")
			t.Setenv("LLDAP_BIND_PASSWORD", "test-pw")
			t.Setenv("SYNC_RETRY_BACKOFF", v)

			if _, err := Load(); err == nil {
				t.Fatalf("expected error for non-positive SYNC_RETRY_BACKOFF=%q, got nil", v)
			}
		})
	}
}
