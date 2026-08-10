package services

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"syndra/internal/models"
)

// ---------------------------------------------------------------------------
// Test helpers
// ---------------------------------------------------------------------------

func resetVaultDeps(t *testing.T) {
	t.Helper()
	origRecord := svcRecordCredentialSet
	origDelete := svcDeleteShadowCredential
	origHas := svcHasShadowCredential
	origAudit := svcInsertShadowCredentialAudit
	t.Cleanup(func() {
		svcRecordCredentialSet = origRecord
		svcDeleteShadowCredential = origDelete
		svcHasShadowCredential = origHas
		svcInsertShadowCredentialAudit = origAudit
	})
}

func noopVaultDeps() {
	svcRecordCredentialSet = func(_ context.Context, _ string) (string, error) {
		return "cred-1", nil
	}
	svcDeleteShadowCredential = func(_ context.Context, _ string) error {
		return nil
	}
	svcHasShadowCredential = func(_ context.Context, _ string) (models.ShadowCredentialStatus, error) {
		return models.ShadowCredentialStatus{HasCredential: false}, nil
	}
	svcInsertShadowCredentialAudit = func(_ context.Context, _, _, _, _ string) error {
		return nil
	}
}

// ---------------------------------------------------------------------------
// ValidatePasswordComplexity tests
// ---------------------------------------------------------------------------

func TestValidatePasswordComplexity_Valid(t *testing.T) {
	if err := ValidatePasswordComplexity("Str0ng!Pass99"); err != nil {
		t.Fatalf("expected valid, got error: %v", err)
	}
}

func TestValidatePasswordComplexity_TooShort(t *testing.T) {
	err := ValidatePasswordComplexity("Aa1!")
	if err == nil {
		t.Fatal("expected error for short password")
	}
	if !strings.Contains(err.Error(), "12 characters") {
		t.Errorf("expected length error, got: %v", err)
	}
}

func TestValidatePasswordComplexity_NoUppercase(t *testing.T) {
	err := ValidatePasswordComplexity("alllowercase1!")
	if err == nil {
		t.Fatal("expected error for no uppercase")
	}
	if !strings.Contains(err.Error(), "uppercase") {
		t.Errorf("expected uppercase error, got: %v", err)
	}
}

func TestValidatePasswordComplexity_NoLowercase(t *testing.T) {
	err := ValidatePasswordComplexity("ALLUPPERCASE1!")
	if err == nil {
		t.Fatal("expected error for no lowercase")
	}
	if !strings.Contains(err.Error(), "lowercase") {
		t.Errorf("expected lowercase error, got: %v", err)
	}
}

func TestValidatePasswordComplexity_NoDigit(t *testing.T) {
	err := ValidatePasswordComplexity("NoDigitsHere!!")
	if err == nil {
		t.Fatal("expected error for no digit")
	}
	if !strings.Contains(err.Error(), "digit") {
		t.Errorf("expected digit error, got: %v", err)
	}
}

func TestValidatePasswordComplexity_NoSymbol(t *testing.T) {
	err := ValidatePasswordComplexity("NoSymbols12345")
	if err == nil {
		t.Fatal("expected error for no symbol")
	}
	if !strings.Contains(err.Error(), "symbol") {
		t.Errorf("expected symbol error, got: %v", err)
	}
}

func TestValidatePasswordComplexity_WrapsSentinel(t *testing.T) {
	err := ValidatePasswordComplexity("short")
	if err == nil {
		t.Fatalf("expected complexity error for %q, got nil", "short")
	}
	if !errors.Is(err, ErrComplexity) {
		t.Fatalf("expected errors.Is(err, ErrComplexity)=true; got err=%v", err)
	}
}

func TestValidatePasswordComplexity_MultipleFailures(t *testing.T) {
	err := ValidatePasswordComplexity("short")
	if err == nil {
		t.Fatal("expected error for multiple failures")
	}
	msg := err.Error()
	if !strings.Contains(msg, "12 characters") {
		t.Error("expected length error in message")
	}
	if !strings.Contains(msg, "uppercase") {
		t.Error("expected uppercase error in message")
	}
	if !strings.Contains(msg, "digit") {
		t.Error("expected digit error in message")
	}
	if !strings.Contains(msg, "symbol") {
		t.Error("expected symbol error in message")
	}
}

// ---------------------------------------------------------------------------
// ClearShadowPassword tests
// ---------------------------------------------------------------------------

func TestClearShadowPassword_Success(t *testing.T) {
	resetVaultDeps(t)
	noopVaultDeps()

	var capturedAction string
	svcInsertShadowCredentialAudit = func(_ context.Context, _, action, _, _ string) error {
		capturedAction = action
		return nil
	}

	err := ClearShadowPassword(context.Background(), "u1", "admin1", "127.0.0.1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if capturedAction != "cleared" {
		t.Errorf("expected action=cleared, got %s", capturedAction)
	}
}

func TestClearShadowPassword_NotFound(t *testing.T) {
	resetVaultDeps(t)
	noopVaultDeps()

	svcDeleteShadowCredential = func(_ context.Context, _ string) error {
		return fmt.Errorf("shadow credential not found")
	}

	err := ClearShadowPassword(context.Background(), "u1", "admin1", "127.0.0.1")
	if err == nil {
		t.Fatal("expected error for non-existent credential")
	}
}
