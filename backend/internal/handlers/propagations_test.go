package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"syndra/internal/models"
	"syndra/internal/services/propagation"
)

func resetPropagationDeps(t *testing.T) {
	t.Helper()
	origDrain := svcDrainPropagations
	origGet := dbGetPendingPropagations
	origGaveUp, origWaits := dbListGaveUp, dbOutboxRowsWaitForRole
	t.Cleanup(func() {
		svcDrainPropagations = origDrain
		dbGetPendingPropagations = origGet
		dbListGaveUp, dbOutboxRowsWaitForRole = origGaveUp, origWaits
	})
	dbListGaveUp = func(context.Context, string) ([]models.PendingPropagation, error) { return nil, nil }
	dbOutboxRowsWaitForRole = func(context.Context, []string) (map[string]bool, error) { return map[string]bool{}, nil }
}

// Re-audit R15 and R11: a row whose role was marked to wait after it was sent
// no longer promises a next try (the background drain skips it); the changes
// Syndra stopped sending are listed beside the queue for the asking person;
// and with the background drain off, a retried row says it waits for Send.
func TestHandleListPendingPropagations_RetriesSayWhatReallyHappens(t *testing.T) {
	resetPropagationDeps(t)
	next := time.Now().Add(time.Minute)
	dbGetPendingPropagations = func(context.Context) ([]models.PendingPropagation, error) {
		return []models.PendingPropagation{
			{ID: "held", OpType: "add", Status: "pending", Attempts: 1, NotBefore: &next, WaitReason: "halted:retrying"},
			{ID: "due", OpType: "add", Status: "pending", Attempts: 1, NotBefore: &next, WaitReason: "halted:retrying"},
		}, nil
	}
	dbOutboxRowsWaitForRole = func(_ context.Context, ids []string) (map[string]bool, error) {
		return map[string]bool{"held": true}, nil
	}
	var asked string
	dbListGaveUp = func(_ context.Context, actor string) ([]models.PendingPropagation, error) {
		asked = actor
		return []models.PendingPropagation{{ID: "spent", OpType: "revoke", Status: "failed", Attempts: 6}}, nil
	}
	read := func() map[string]any {
		w := httptest.NewRecorder()
		handleListPendingPropagations(w, httptest.NewRequest(http.MethodGet, "/api/v1/propagations", nil))
		var body struct {
			Pending []models.PendingPropagation `json:"pending"`
			GaveUp  []models.PendingPropagation `json:"gave_up"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
			t.Fatalf("decode: %v", err)
		}
		out := map[string]any{"gave_up": len(body.GaveUp)}
		for _, row := range body.Pending {
			out[row.ID] = row
		}
		return out
	}
	got := read()
	held, due := got["held"].(models.PendingPropagation), got["due"].(models.PendingPropagation)
	if held.Retry != nil || held.WaitReason != "role_waits" {
		t.Errorf("a row whose role now waits must say so, not a next try: %+v", held)
	}
	if due.Retry == nil || due.Retry.State != "retrying" {
		t.Errorf("a due retry keeps its next try: %+v", due.Retry)
	}
	if got["gave_up"] != 1 || asked == "" {
		t.Errorf("the give-ups are listed for the asking person: %v (asked %q)", got["gave_up"], asked)
	}

	propagation.SetAutoRetry(false)
	t.Cleanup(func() { propagation.SetAutoRetry(true) })
	if due := read()["due"].(models.PendingPropagation); due.Retry == nil || due.Retry.State != "waiting" || due.Retry.NextTry != nil {
		t.Errorf("with the background drain off nothing sends it again: want waiting, got %+v", due.Retry)
	}
}

func TestHandleDrainPropagations_ReturnsResult(t *testing.T) {
	resetPropagationDeps(t)
	svcDrainPropagations = func(context.Context) (propagation.DrainResult, error) {
		return propagation.DrainResult{Applied: 2}, nil
	}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/propagations/drain", nil)
	w := httptest.NewRecorder()
	handleDrainPropagations(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), `"applied":2`) {
		t.Fatalf("body: %s", w.Body)
	}
}

// A give-up stays on Home and the rail until an operator marks it seen; the
// marker records who. A failed write is a 500, never a silent "seen".
func TestHandleMarkGaveUpSeen(t *testing.T) {
	orig := dbMarkGaveUpSeen
	t.Cleanup(func() { dbMarkGaveUpSeen = orig })
	var by string
	dbMarkGaveUpSeen = func(_ context.Context, actor string) error { by = actor; return nil }
	req := httptest.NewRequest(http.MethodPost, "/api/v1/propagations/gave-up/seen", nil)
	w := httptest.NewRecorder()
	handleMarkGaveUpSeen(w, req)
	if w.Code != http.StatusOK || by == "" {
		t.Fatalf("want 200 with an actor recorded, got %d actor %q", w.Code, by)
	}

	dbMarkGaveUpSeen = func(context.Context, string) error { return context.DeadlineExceeded }
	w = httptest.NewRecorder()
	handleMarkGaveUpSeen(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("want 500 when the marker can't be written, got %d", w.Code)
	}
}

func TestHandleDrainPropagations_502OnError(t *testing.T) {
	resetPropagationDeps(t)
	svcDrainPropagations = func(context.Context) (propagation.DrainResult, error) {
		return propagation.DrainResult{}, context.DeadlineExceeded
	}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/propagations/drain", nil)
	w := httptest.NewRecorder()
	handleDrainPropagations(w, req)
	if w.Code != http.StatusBadGateway {
		t.Fatalf("want 502 on drain error, got %d", w.Code)
	}
}

func TestHandleListPendingPropagations_ReturnsRows(t *testing.T) {
	resetPropagationDeps(t)
	dbGetPendingPropagations = func(context.Context) ([]models.PendingPropagation, error) {
		return []models.PendingPropagation{{ID: "p1", OpType: "add", Status: "pending"}}, nil
	}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/propagations", nil)
	w := httptest.NewRecorder()
	handleListPendingPropagations(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), `"p1"`) || !strings.Contains(w.Body.String(), `"pending"`) {
		t.Fatalf("body: %s", w.Body)
	}
}

func TestHandleListPendingPropagations_EmptyEncodesAsArray(t *testing.T) {
	resetPropagationDeps(t)
	dbGetPendingPropagations = func(context.Context) ([]models.PendingPropagation, error) {
		return nil, nil
	}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/propagations", nil)
	w := httptest.NewRecorder()
	handleListPendingPropagations(w, req)
	if !strings.Contains(w.Body.String(), `"pending":[]`) {
		t.Fatalf("empty list must encode as [], got: %s", w.Body)
	}
}
