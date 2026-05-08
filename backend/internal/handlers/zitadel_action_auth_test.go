package handlers

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

const testSigningKey = "test-signing-secret"
const testSecretEnv = "ZITADEL_ACTION_SIGNING_KEY"

// signActionRequest returns the ZITADEL-Signature header value for the given
// body and timestamp. Mirrors the algorithm in Zitadel's Actions v2
// ComputeSignatureHeader so tests exercise the real wire shape.
func signActionRequest(t *testing.T, body []byte, ts time.Time, key string) string {
	t.Helper()
	mac := hmac.New(sha256.New, []byte(key))
	_, _ = fmt.Fprintf(mac, "%d", ts.Unix())
	mac.Write([]byte("."))
	mac.Write(body)
	return fmt.Sprintf("t=%d,v1=%s", ts.Unix(), hex.EncodeToString(mac.Sum(nil)))
}

func pokeHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}

func TestWithZitadelActionSignature_ValidSignaturePasses(t *testing.T) {
	t.Setenv(testSecretEnv, testSigningKey)

	body := []byte(`{"hello":"world"}`)
	sig := signActionRequest(t, body, time.Now(), testSigningKey)

	req := httptest.NewRequest(http.MethodPost, "/api/action/inject", strings.NewReader(string(body)))
	req.Header.Set(zitadelSignatureHeader, sig)
	rr := httptest.NewRecorder()

	withZitadelActionSignature(testSecretEnv, pokeHandler)(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("valid signature should pass, got %d (body=%s)", rr.Code, rr.Body.String())
	}
}

func TestWithZitadelActionSignature_BodyIsRewoundForDownstreamDecoder(t *testing.T) {
	t.Setenv(testSecretEnv, testSigningKey)

	body := []byte(`{"marker":"seen"}`)
	sig := signActionRequest(t, body, time.Now(), testSigningKey)
	req := httptest.NewRequest(http.MethodPost, "/api/action/inject", strings.NewReader(string(body)))
	req.Header.Set(zitadelSignatureHeader, sig)
	rr := httptest.NewRecorder()

	downstream := func(w http.ResponseWriter, r *http.Request) {
		var got map[string]string
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Fatalf("downstream decode: %v", err)
		}
		if got["marker"] != "seen" {
			t.Fatalf("body was not rewound for downstream, got %v", got)
		}
		w.WriteHeader(http.StatusOK)
	}
	withZitadelActionSignature(testSecretEnv, downstream)(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
}

func TestWithZitadelActionSignature_InvalidSignatureRejected(t *testing.T) {
	t.Setenv(testSecretEnv, testSigningKey)

	body := []byte(`{"hello":"world"}`)
	// Sign with the wrong key — valid structure, invalid HMAC.
	sig := signActionRequest(t, body, time.Now(), "wrong-key")

	req := httptest.NewRequest(http.MethodPost, "/api/action/inject", strings.NewReader(string(body)))
	req.Header.Set(zitadelSignatureHeader, sig)
	rr := httptest.NewRecorder()

	withZitadelActionSignature(testSecretEnv, pokeHandler)(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("wrong-key signature should be 401, got %d", rr.Code)
	}
	got := decodeErrorResponse(t, rr)
	if got.Error != "INVALID_SIGNATURE" {
		t.Fatalf("expected INVALID_SIGNATURE, got %s", got.Error)
	}
}

func TestWithZitadelActionSignature_MissingHeaderRejected(t *testing.T) {
	t.Setenv(testSecretEnv, testSigningKey)

	req := httptest.NewRequest(http.MethodPost, "/api/action/inject", strings.NewReader(`{}`))
	rr := httptest.NewRecorder()

	withZitadelActionSignature(testSecretEnv, pokeHandler)(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("missing header should be 401, got %d", rr.Code)
	}
}

func TestWithZitadelActionSignature_StaleTimestampRejected(t *testing.T) {
	t.Setenv(testSecretEnv, testSigningKey)

	body := []byte(`{}`)
	// One hour in the past — far outside the 300s tolerance.
	stale := time.Now().Add(-1 * time.Hour)
	sig := signActionRequest(t, body, stale, testSigningKey)

	req := httptest.NewRequest(http.MethodPost, "/api/action/inject", strings.NewReader(string(body)))
	req.Header.Set(zitadelSignatureHeader, sig)
	rr := httptest.NewRecorder()

	withZitadelActionSignature(testSecretEnv, pokeHandler)(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("stale timestamp should be 401, got %d", rr.Code)
	}
}

func TestWithZitadelActionSignature_DevModePassthrough(t *testing.T) {
	// Dev mode = both ZITADEL_DOMAIN and the signing-key env are empty.
	t.Setenv(testSecretEnv, "")
	t.Setenv("ZITADEL_DOMAIN", "")

	req := httptest.NewRequest(http.MethodPost, "/api/action/inject", strings.NewReader(`{}`))
	rr := httptest.NewRecorder()

	withZitadelActionSignature(testSecretEnv, pokeHandler)(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("dev mode (both unset) should pass through, got %d", rr.Code)
	}
}

func TestWithZitadelActionSignature_ProductionRefusesEmptySecret(t *testing.T) {
	// Production: ZITADEL_DOMAIN set, signing-key env empty. Even though the
	// startup gate should have refused this configuration, the middleware MUST
	// refuse the request rather than fall through silently.
	t.Setenv(testSecretEnv, "")
	t.Setenv("ZITADEL_DOMAIN", "zitadel.example.test")

	req := httptest.NewRequest(http.MethodPost, "/api/action/inject", strings.NewReader(`{}`))
	rr := httptest.NewRecorder()

	withZitadelActionSignature(testSecretEnv, pokeHandler)(rr, req)

	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("production with empty secret must return 503, got %d (body=%s)", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "MISCONFIGURED") {
		t.Fatalf("expected MISCONFIGURED error code in body, got %s", rr.Body.String())
	}
}

func TestWithZitadelActionSignature_RotatedKeyAccepted(t *testing.T) {
	// Zitadel allows multiple v1= signatures in one header during rotation.
	// The middleware MUST accept the request if ANY v1= matches.
	t.Setenv(testSecretEnv, testSigningKey)

	body := []byte(`{"hello":"world"}`)
	now := time.Now()

	// Valid signature under the real key.
	mac := hmac.New(sha256.New, []byte(testSigningKey))
	_, _ = fmt.Fprintf(mac, "%d", now.Unix())
	mac.Write([]byte("."))
	mac.Write(body)
	validSig := hex.EncodeToString(mac.Sum(nil))

	// A stale/retired key's signature — garbage under the current secret.
	header := fmt.Sprintf("t=%d,v1=%s,v1=%s", now.Unix(), "deadbeef0011", validSig)

	req := httptest.NewRequest(http.MethodPost, "/api/action/inject", strings.NewReader(string(body)))
	req.Header.Set(zitadelSignatureHeader, header)
	rr := httptest.NewRecorder()

	withZitadelActionSignature(testSecretEnv, pokeHandler)(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 when one of multiple v1= signatures matches, got %d", rr.Code)
	}
}
