package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"syndra/internal/models"
	"syndra/internal/services"

	"github.com/jackc/pgx/v5"
)

func stubDiscardDeps(t *testing.T, discard func(context.Context, string, string) error) *int {
	t.Helper()
	origDiscard, origDraft := svcDiscardBundleDraft, svcBundleDraft
	t.Cleanup(func() { svcDiscardBundleDraft, svcBundleDraft = origDiscard, origDraft })
	svcDiscardBundleDraft = discard
	reads := 0
	svcBundleDraft = func(_ context.Context, id string) (services.DraftDiff, error) {
		reads++
		return services.DraftDiff{BundleID: id, LatestVersion: 2, NextVersion: 3,
			Added: []models.BundleRole{}, Removed: []models.BundleRole{}}, nil
	}
	return &reads
}

func discardRequest(id string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/bundles/"+id+"/draft", nil)
	req.SetPathValue("id", id)
	rr := httptest.NewRecorder()
	handleDiscardBundleDraft(rr, req)
	return rr
}

// Discarding resets, then answers with the draft read AFTER the reset — the
// same shape GET /draft returns, so the panel can render it directly.
func TestHandleDiscardBundleDraft_ResetsThenReturnsFreshDraft(t *testing.T) {
	var gotActor, gotBundle string
	reads := stubDiscardDeps(t, func(_ context.Context, actor, bundleID string) error {
		gotActor, gotBundle = actor, bundleID
		return nil
	})

	rr := discardRequest("b1")
	if rr.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", rr.Code, rr.Body.String())
	}
	if gotActor != "system" || gotBundle != "b1" {
		t.Fatalf("discard called with actor=%q bundle=%q", gotActor, gotBundle)
	}
	if *reads != 1 {
		t.Fatalf("the draft is read once, after the reset; got %d reads", *reads)
	}
	var d services.DraftDiff
	if err := json.NewDecoder(rr.Body).Decode(&d); err != nil || d.BundleID != "b1" || d.NextVersion != 3 {
		t.Fatalf("draft body: %+v %v", d, err)
	}
}

func TestHandleDiscardBundleDraft_Failures(t *testing.T) {
	cases := []struct {
		name string
		id   string
		err  error
		want int
	}{
		{"missing id", "", nil, http.StatusBadRequest},
		{"unknown bundle", "nope", pgx.ErrNoRows, http.StatusNotFound},
		{"db fault", "b1", errors.New("boom"), http.StatusInternalServerError},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			reads := stubDiscardDeps(t, func(context.Context, string, string) error { return c.err })
			if rr := discardRequest(c.id); rr.Code != c.want {
				t.Fatalf("want %d, got %d: %s", c.want, rr.Code, rr.Body.String())
			}
			if *reads != 0 {
				t.Fatal("a failed discard must not answer with a draft")
			}
		})
	}
}
