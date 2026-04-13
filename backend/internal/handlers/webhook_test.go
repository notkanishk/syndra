package handlers

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"
)

// signWebhook computes the correct HMAC-SHA256 over (tsHeader + "\n" + body).
func signWebhook(t *testing.T, secret string, tsHeader string, body []byte) string {
	t.Helper()
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(tsHeader))
	mac.Write([]byte("\n"))
	mac.Write(body)
	return hex.EncodeToString(mac.Sum(nil))
}

func TestVerifyWebhookSignature_ValidSignature(t *testing.T) {
	t.Setenv("ZITADEL_WEBHOOK_SECRET", "test-secret")

	body := []byte(`{"user_id":"u1","source_project":"p1","role_key":"member"}`)
	ts := fmt.Sprintf("%d", time.Now().Unix())
	sig := signWebhook(t, "test-secret", ts, body)

	if err := verifyWebhookSignature(body, ts, sig); err != nil {
		t.Fatalf("expected valid signature to pass: %v", err)
	}
}

func TestVerifyWebhookSignature_InvalidSignature(t *testing.T) {
	t.Setenv("ZITADEL_WEBHOOK_SECRET", "test-secret")

	body := []byte(`{"user_id":"u1","source_project":"p1","role_key":"member"}`)
	ts := fmt.Sprintf("%d", time.Now().Unix())
	if err := verifyWebhookSignature(body, ts, "deadbeefdeadbeef"); err == nil {
		t.Fatal("expected invalid signature to fail")
	}
}

func TestVerifyWebhookSignature_FreshTimestampWithStaleBodySignature(t *testing.T) {
	// Core replay attack scenario: attacker has a captured (body, sig) pair and
	// replaces the stale timestamp with a fresh one. The signature must now fail
	// because the fresh timestamp was not part of the original signed input.
	t.Setenv("ZITADEL_WEBHOOK_SECRET", "test-secret")

	body := []byte(`{"user_id":"u1","source_project":"p1","role_key":"member"}`)
	staleTs := fmt.Sprintf("%d", time.Now().Add(-10*time.Minute).Unix())

	// Capture: signature computed over the stale timestamp
	capturedSig := signWebhook(t, "test-secret", staleTs, body)

	// Replay attempt: reuse captured body+sig with a fresh timestamp
	freshTs := fmt.Sprintf("%d", time.Now().Unix())
	if err := verifyWebhookSignature(body, freshTs, capturedSig); err == nil {
		t.Fatal("expected replay with swapped timestamp to fail signature check")
	}
}

func TestVerifyWebhookSignature_MissingSignatureHeader(t *testing.T) {
	t.Setenv("ZITADEL_WEBHOOK_SECRET", "test-secret")

	body := []byte(`{}`)
	ts := fmt.Sprintf("%d", time.Now().Unix())
	if err := verifyWebhookSignature(body, ts, ""); err == nil {
		t.Fatal("expected missing signature header to fail")
	}
}

func TestVerifyWebhookSignature_MissingTimestampHeader(t *testing.T) {
	t.Setenv("ZITADEL_WEBHOOK_SECRET", "test-secret")

	body := []byte(`{}`)
	if err := verifyWebhookSignature(body, "", "somesig"); err == nil {
		t.Fatal("expected missing timestamp header to fail signature check")
	}
}

func TestVerifyWebhookSignature_NoSecretLocalDev(t *testing.T) {
	// When ZITADEL_WEBHOOK_SECRET is not set, verification is skipped (local-dev mode)
	t.Setenv("ZITADEL_WEBHOOK_SECRET", "")

	body := []byte(`{"user_id":"u1"}`)
	if err := verifyWebhookSignature(body, "", ""); err != nil {
		t.Fatalf("expected no error in local-dev mode: %v", err)
	}
}

func TestVerifyWebhookFreshness_FreshTimestamp(t *testing.T) {
	t.Setenv("ZITADEL_WEBHOOK_SECRET", "test-secret")

	ts := strconv.FormatInt(time.Now().Unix(), 10)
	if err := verifyWebhookFreshness(ts); err != nil {
		t.Fatalf("expected fresh timestamp to pass: %v", err)
	}
}

func TestVerifyWebhookFreshness_StaleTimestamp(t *testing.T) {
	t.Setenv("ZITADEL_WEBHOOK_SECRET", "test-secret")

	// 10 minutes ago — beyond the 5-minute window
	ts := strconv.FormatInt(time.Now().Add(-10*time.Minute).Unix(), 10)
	if err := verifyWebhookFreshness(ts); err == nil {
		t.Fatal("expected stale timestamp to fail")
	}
}

func TestVerifyWebhookFreshness_MissingTimestamp(t *testing.T) {
	t.Setenv("ZITADEL_WEBHOOK_SECRET", "test-secret")

	if err := verifyWebhookFreshness(""); err == nil {
		t.Fatal("expected missing timestamp to fail")
	}
}

func TestVerifyWebhookFreshness_NoSecretLocalDev(t *testing.T) {
	t.Setenv("ZITADEL_WEBHOOK_SECRET", "")

	if err := verifyWebhookFreshness(""); err != nil {
		t.Fatalf("expected no error in local-dev mode: %v", err)
	}
}

func TestHandleZitadelWebhook_RejectsInvalidSignature(t *testing.T) {
	t.Setenv("ZITADEL_WEBHOOK_SECRET", "test-secret")

	body := []byte(`{"user_id":"u1","source_project":"p1","role_key":"member","project_ids":["p1"]}`)
	ts := fmt.Sprintf("%d", time.Now().Unix())
	req := httptest.NewRequest(http.MethodPost, "/api/webhooks/zitadel", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Zitadel-Signature", "badhash")
	req.Header.Set("X-Zitadel-Timestamp", ts)
	rr := httptest.NewRecorder()

	HandleZitadelWebhook(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rr.Code)
	}
	got := decodeErrorResponse(t, rr)
	if got.Error != "WEBHOOK_UNAUTHORIZED" {
		t.Fatalf("expected WEBHOOK_UNAUTHORIZED, got %s", got.Error)
	}
}

func TestHandleZitadelWebhook_RejectsStaleTimestamp(t *testing.T) {
	t.Setenv("ZITADEL_WEBHOOK_SECRET", "test-secret")

	body := []byte(`{"user_id":"u1","source_project":"p1","role_key":"member","project_ids":["p1"]}`)
	staleTs := fmt.Sprintf("%d", time.Now().Add(-10*time.Minute).Unix())

	// Signature computed over the stale timestamp (as a legitimate sender would)
	sig := signWebhook(t, "test-secret", staleTs, body)

	req := httptest.NewRequest(http.MethodPost, "/api/webhooks/zitadel", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Zitadel-Signature", sig)
	req.Header.Set("X-Zitadel-Timestamp", staleTs)
	rr := httptest.NewRecorder()

	HandleZitadelWebhook(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for stale webhook, got %d", rr.Code)
	}
	got := decodeErrorResponse(t, rr)
	if got.Error != "WEBHOOK_STALE" {
		t.Fatalf("expected WEBHOOK_STALE, got %s", got.Error)
	}
}
