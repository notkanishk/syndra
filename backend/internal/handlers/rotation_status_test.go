package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// withFixedNow pins nowFn for a single test and restores it via t.Cleanup.
func withFixedNow(t *testing.T, now time.Time) {
	t.Helper()
	orig := nowFn
	nowFn = func() time.Time { return now }
	t.Cleanup(func() { nowFn = orig })
}

// withKeyInstalled sets ZITADEL_ACTION_SIGNING_KEY to a non-empty value so the
// handler reaches the age-classification branches. Every test that asserts
// ok/warn/stale/unknown MUST call this — otherwise status collapses to
// "disabled" and the assertion becomes meaningless.
func withKeyInstalled(t *testing.T) {
	t.Helper()
	t.Setenv(envActionKey, "test-signing-key")
}

func decodeRotationStatus(t *testing.T, rr *httptest.ResponseRecorder) ActionRotationStatus {
	t.Helper()
	var got ActionRotationStatus
	if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode ActionRotationStatus: %v\nbody: %s", err, rr.Body.String())
	}
	return got
}

func TestClassifyRotationStatus(t *testing.T) {
	cases := []struct {
		age, threshold int
		want           string
	}{
		{0, 90, "ok"},
		{1, 90, "ok"},
		{89, 90, "ok"},
		{90, 90, "warn"}, // boundary: >= threshold is warn
		{120, 90, "warn"},
		{179, 90, "warn"},
		{180, 90, "stale"}, // boundary: >= 2*threshold is stale
		{365, 90, "stale"},
		{0, 30, "ok"},
		{30, 30, "warn"},
		{60, 30, "stale"},
	}
	for _, c := range cases {
		if got := classifyRotationStatus(c.age, c.threshold); got != c.want {
			t.Errorf("classifyRotationStatus(age=%d, threshold=%d) = %q; want %q", c.age, c.threshold, got, c.want)
		}
	}
}

func TestAgeInDays_ReturnsSignedDelta(t *testing.T) {
	now := time.Date(2026, 4, 24, 12, 0, 0, 0, time.UTC)
	if got := ageInDays(now, now.Add(-48*time.Hour)); got != 2 {
		t.Errorf("past timestamp: want 2, got %d", got)
	}
	if got := ageInDays(now, now); got != 0 {
		t.Errorf("same instant: want 0, got %d", got)
	}
	// Future timestamp: handler filters these before calling ageInDays, but
	// the function itself remains pure and returns a negative delta so the
	// "future" condition is observable at the call site rather than silently
	// clamped.
	if got := ageInDays(now, now.Add(48*time.Hour)); got >= 0 {
		t.Errorf("future timestamp should return negative, got %d", got)
	}
}

func TestHandleActionRotationStatus_KeyUnset_Disabled(t *testing.T) {
	// No ZITADEL_ACTION_SIGNING_KEY => signature verification is off. Even
	// if the operator set a recent ROTATED_AT, status MUST be "disabled" —
	// reporting "ok" would mask the production misconfiguration.
	t.Setenv(envActionKey, "")
	t.Setenv(envActionKeyRotatedAt, time.Now().UTC().Format(time.RFC3339))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/zitadel/action-rotation-status", nil)
	rr := httptest.NewRecorder()
	HandleActionRotationStatus(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	got := decodeRotationStatus(t, rr)
	if got.Status != "disabled" {
		t.Errorf("expected status=disabled, got %q", got.Status)
	}
	if got.KeyInstalled {
		t.Errorf("expected key_installed=false")
	}
	if got.LastRotatedAt != nil {
		t.Errorf("disabled branch should not populate last_rotated_at, got %v", got.LastRotatedAt)
	}
	if got.AgeDays != nil {
		t.Errorf("disabled branch should not populate age_days, got %v", got.AgeDays)
	}
}

func TestHandleActionRotationStatus_UnsetRotatedAt_Unknown(t *testing.T) {
	withKeyInstalled(t)
	t.Setenv(envActionKeyRotatedAt, "")
	t.Setenv(envActionKeyThresholdDays, "")

	req := httptest.NewRequest(http.MethodGet, "/api/v1/zitadel/action-rotation-status", nil)
	rr := httptest.NewRecorder()
	HandleActionRotationStatus(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	got := decodeRotationStatus(t, rr)
	if got.Status != "unknown" {
		t.Errorf("expected status=unknown, got %q", got.Status)
	}
	if !got.KeyInstalled {
		t.Errorf("expected key_installed=true")
	}
	if got.ThresholdDays != defaultRotationThresholdDays {
		t.Errorf("expected default threshold %d, got %d", defaultRotationThresholdDays, got.ThresholdDays)
	}
	if got.RotateCommand != "make zitadel-actions-rotate-key" {
		t.Errorf("unexpected rotate_command: %q", got.RotateCommand)
	}
}

func TestHandleActionRotationStatus_MalformedRotatedAt_Unknown(t *testing.T) {
	withKeyInstalled(t)
	t.Setenv(envActionKeyRotatedAt, "not-a-timestamp")

	req := httptest.NewRequest(http.MethodGet, "/api/v1/zitadel/action-rotation-status", nil)
	rr := httptest.NewRecorder()
	HandleActionRotationStatus(rr, req)

	got := decodeRotationStatus(t, rr)
	if got.Status != "unknown" {
		t.Fatalf("malformed timestamp should yield status=unknown, got %q", got.Status)
	}
}

func TestHandleActionRotationStatus_FutureRotatedAt_Unknown(t *testing.T) {
	// Future-dated timestamps indicate a config error (clock skew, YY typo).
	// Treating them as fresh rotations would suppress warn/stale indefinitely,
	// so the handler MUST flag them as unknown — not silently clamp age to 0.
	now := time.Date(2026, 4, 24, 12, 0, 0, 0, time.UTC)
	withFixedNow(t, now)
	withKeyInstalled(t)
	t.Setenv(envActionKeyRotatedAt, now.Add(48*time.Hour).Format(time.RFC3339))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/zitadel/action-rotation-status", nil)
	rr := httptest.NewRecorder()
	HandleActionRotationStatus(rr, req)

	got := decodeRotationStatus(t, rr)
	if got.Status != "unknown" {
		t.Fatalf("future timestamp should yield status=unknown, got %q", got.Status)
	}
	if got.LastRotatedAt != nil {
		t.Errorf("unknown branch should not populate last_rotated_at, got %v", got.LastRotatedAt)
	}
	if got.AgeDays != nil {
		t.Errorf("unknown branch should not populate age_days, got %v", got.AgeDays)
	}
}

func TestHandleActionRotationStatus_Fresh_Ok(t *testing.T) {
	now := time.Date(2026, 4, 24, 12, 0, 0, 0, time.UTC)
	withFixedNow(t, now)
	withKeyInstalled(t)
	t.Setenv(envActionKeyRotatedAt, now.Add(-10*24*time.Hour).Format(time.RFC3339))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/zitadel/action-rotation-status", nil)
	rr := httptest.NewRecorder()
	HandleActionRotationStatus(rr, req)

	got := decodeRotationStatus(t, rr)
	if got.Status != "ok" {
		t.Fatalf("10d-old key should be ok, got %q", got.Status)
	}
	if got.AgeDays == nil || *got.AgeDays != 10 {
		t.Fatalf("expected age=10, got %v", got.AgeDays)
	}
	if !got.KeyInstalled {
		t.Errorf("expected key_installed=true")
	}
}

func TestHandleActionRotationStatus_PastThreshold_Warn(t *testing.T) {
	now := time.Date(2026, 4, 24, 12, 0, 0, 0, time.UTC)
	withFixedNow(t, now)
	withKeyInstalled(t)
	t.Setenv(envActionKeyRotatedAt, now.Add(-100*24*time.Hour).Format(time.RFC3339))
	// Threshold stays at default (90); 100 >= 90 < 180 -> warn.

	req := httptest.NewRequest(http.MethodGet, "/api/v1/zitadel/action-rotation-status", nil)
	rr := httptest.NewRecorder()
	HandleActionRotationStatus(rr, req)

	got := decodeRotationStatus(t, rr)
	if got.Status != "warn" {
		t.Fatalf("100d-old key at default 90d threshold should be warn, got %q", got.Status)
	}
	if got.AgeDays == nil || *got.AgeDays != 100 {
		t.Fatalf("expected age=100, got %v", got.AgeDays)
	}
}

func TestHandleActionRotationStatus_WayPastThreshold_Stale(t *testing.T) {
	now := time.Date(2026, 4, 24, 12, 0, 0, 0, time.UTC)
	withFixedNow(t, now)
	withKeyInstalled(t)
	t.Setenv(envActionKeyRotatedAt, now.Add(-365*24*time.Hour).Format(time.RFC3339))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/zitadel/action-rotation-status", nil)
	rr := httptest.NewRecorder()
	HandleActionRotationStatus(rr, req)

	got := decodeRotationStatus(t, rr)
	if got.Status != "stale" {
		t.Fatalf("365d-old key at default 90d threshold should be stale, got %q", got.Status)
	}
}

func TestHandleActionRotationStatus_CustomThreshold_Honoured(t *testing.T) {
	now := time.Date(2026, 4, 24, 12, 0, 0, 0, time.UTC)
	withFixedNow(t, now)
	withKeyInstalled(t)
	t.Setenv(envActionKeyRotatedAt, now.Add(-40*24*time.Hour).Format(time.RFC3339))
	t.Setenv(envActionKeyThresholdDays, "30")
	// 40d old vs 30d threshold: warn.

	req := httptest.NewRequest(http.MethodGet, "/api/v1/zitadel/action-rotation-status", nil)
	rr := httptest.NewRecorder()
	HandleActionRotationStatus(rr, req)

	got := decodeRotationStatus(t, rr)
	if got.ThresholdDays != 30 {
		t.Errorf("expected threshold=30, got %d", got.ThresholdDays)
	}
	if got.Status != "warn" {
		t.Fatalf("40d at 30d threshold should be warn, got %q", got.Status)
	}
}

func TestHandleActionRotationStatus_InvalidThreshold_FallsBackToDefault(t *testing.T) {
	withKeyInstalled(t)
	t.Setenv(envActionKeyThresholdDays, "not-a-number")

	req := httptest.NewRequest(http.MethodGet, "/api/v1/zitadel/action-rotation-status", nil)
	rr := httptest.NewRecorder()
	HandleActionRotationStatus(rr, req)

	got := decodeRotationStatus(t, rr)
	if got.ThresholdDays != defaultRotationThresholdDays {
		t.Errorf("invalid threshold should fall back to default, got %d", got.ThresholdDays)
	}
}

func TestHandleActionRotationStatus_ZeroOrNegativeThreshold_FallsBackToDefault(t *testing.T) {
	withKeyInstalled(t)
	t.Setenv(envActionKeyThresholdDays, "0")
	req := httptest.NewRequest(http.MethodGet, "/api/v1/zitadel/action-rotation-status", nil)
	rr := httptest.NewRecorder()
	HandleActionRotationStatus(rr, req)
	got := decodeRotationStatus(t, rr)
	if got.ThresholdDays != defaultRotationThresholdDays {
		t.Errorf("zero threshold should fall back to default, got %d", got.ThresholdDays)
	}

	t.Setenv(envActionKeyThresholdDays, "-5")
	rr2 := httptest.NewRecorder()
	HandleActionRotationStatus(rr2, req)
	got2 := decodeRotationStatus(t, rr2)
	if got2.ThresholdDays != defaultRotationThresholdDays {
		t.Errorf("negative threshold should fall back to default, got %d", got2.ThresholdDays)
	}
}

func TestHandleActionRotationStatus_MethodNotAllowed(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/v1/zitadel/action-rotation-status", nil)
	rr := httptest.NewRecorder()
	HandleActionRotationStatus(rr, req)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", rr.Code)
	}
}
