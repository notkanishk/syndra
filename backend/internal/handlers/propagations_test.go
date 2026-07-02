package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"mkauth/internal/models"
	"mkauth/internal/services/propagation"
)

func resetPropagationDeps(t *testing.T) {
	t.Helper()
	origDrain := svcDrainPropagations
	origGet := dbGetPendingPropagations
	t.Cleanup(func() {
		svcDrainPropagations = origDrain
		dbGetPendingPropagations = origGet
	})
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
