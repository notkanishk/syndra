package services

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"log"
	"strings"
	"unicode"

	"golang.org/x/crypto/argon2"
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

// hashPassword hashes a plaintext password with Argon2id and returns the PHC-format
// hash string and the salt parameters. The plaintext byte slice is zeroed after use.
func hashPassword(plaintext string) (hash, saltParams string, err error) {
	pw := []byte(plaintext)
	defer func() {
		for i := range pw {
			pw[i] = 0
		}
	}()

	salt := make([]byte, argon2SaltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", "", fmt.Errorf("generate salt: %w", err)
	}

	key := argon2.IDKey(pw, salt, argon2Time, argon2Memory, argon2Threads, argon2KeyLen)

	saltB64 := base64.RawStdEncoding.EncodeToString(salt)
	keyB64 := base64.RawStdEncoding.EncodeToString(key)

	saltParams = fmt.Sprintf("t=%d,m=%d,p=%d", argon2Time, argon2Memory, argon2Threads)
	hash = fmt.Sprintf("$argon2id$v=19$m=%d,t=%d,p=%d$%s$%s",
		argon2Memory, argon2Time, argon2Threads, saltB64, keyB64)

	return hash, saltParams, nil
}

// SetShadowPassword validates complexity, hashes with Argon2id, stores the credential,
// and records an audit entry. Audit action is "set" for first-time or "rotated" for updates.
func SetShadowPassword(ctx context.Context, userID, actorID, plaintext, ipAddress string) error {
	if err := ValidatePasswordComplexity(plaintext); err != nil {
		_ = svcInsertShadowCredentialAudit(ctx, userID, "failed_validation", actorID, ipAddress)
		return err
	}

	// Determine if this is a new set or a rotation.
	status, err := svcHasShadowCredential(ctx, userID)
	if err != nil {
		return fmt.Errorf("check existing credential: %w", err)
	}
	action := "set"
	if status.HasCredential {
		action = "rotated"
	}

	hash, saltParams, err := hashPassword(plaintext)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}

	credID, err := svcUpsertShadowCredential(ctx, userID, hash, "argon2id", saltParams)
	if err != nil {
		return fmt.Errorf("persist shadow credential: %w", err)
	}

	log.Printf("[VAULT] Shadow credential %s: user=%s cred=%s", action, userID, credID)

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
