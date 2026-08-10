package services

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"unicode"
)

// Argon2id parameters — tuned for a low-concurrency makerspace workload.
const (
	argon2Time    = 1
	argon2Memory  = 64 * 1024 // 64 MB
	argon2Threads = 4
	argon2KeyLen  = 32
	argon2SaltLen = 16
)

// ErrComplexity is the sentinel returned by ValidatePasswordComplexity when a
// password fails one or more strength rules. Handlers MUST classify the
// failure via errors.Is(err, ErrComplexity) — the wrapped detail message
// composes the failing requirements and is not stable.
var ErrComplexity = errors.New("password complexity")

// ValidatePasswordComplexity enforces shadow password requirements:
// 12+ characters, at least one uppercase, lowercase, digit, and symbol.
// Returns all failing requirements in a single error.
func ValidatePasswordComplexity(password string) error {
	var failures []string

	if len(password) < 12 {
		failures = append(failures, "must be at least 12 characters")
	}

	var hasUpper, hasLower, hasDigit, hasSymbol bool
	for _, r := range password {
		switch {
		case unicode.IsUpper(r):
			hasUpper = true
		case unicode.IsLower(r):
			hasLower = true
		case unicode.IsDigit(r):
			hasDigit = true
		case !unicode.IsLetter(r) && !unicode.IsDigit(r) && !unicode.IsSpace(r):
			hasSymbol = true
		}
	}
	if !hasUpper {
		failures = append(failures, "must contain at least one uppercase letter")
	}
	if !hasLower {
		failures = append(failures, "must contain at least one lowercase letter")
	}
	if !hasDigit {
		failures = append(failures, "must contain at least one digit")
	}
	if !hasSymbol {
		failures = append(failures, "must contain at least one symbol")
	}

	if len(failures) > 0 {
		return fmt.Errorf("%w: %s", ErrComplexity, strings.Join(failures, "; "))
	}
	return nil
}

// RecordCredentialSet notes that a member set a credential on a target, and
// audits it.
//
// It hashes nothing and stores nothing. The value went to the target through
// the operation that received it and is kept nowhere — no API here accepts a
// hash, so a stored one could only ever leak (change `addon-platform` group 11).
func RecordCredentialSet(ctx context.Context, userID, actorID, ipAddress string) error {
	status, err := svcHasShadowCredential(ctx, userID)
	if err != nil {
		return fmt.Errorf("check existing credential: %w", err)
	}
	action := "set"
	if status.HasCredential {
		action = "rotated"
	}

	credID, err := svcRecordCredentialSet(ctx, userID)
	if err != nil {
		return fmt.Errorf("record credential: %w", err)
	}
	log.Printf("[VAULT] Credential %s: user=%s record=%s", action, userID, credID)
	_ = svcInsertShadowCredentialAudit(ctx, userID, action, actorID, ipAddress)
	return nil
}

// ClearShadowPassword removes a user's shadow credential and records an audit entry.
func ClearShadowPassword(ctx context.Context, userID, actorID, ipAddress string) error {
	if err := svcDeleteShadowCredential(ctx, userID); err != nil {
		return err
	}

	log.Printf("[VAULT] Shadow credential cleared: user=%s", userID)

	_ = svcInsertShadowCredentialAudit(ctx, userID, "cleared", actorID, ipAddress)

	return nil
}
