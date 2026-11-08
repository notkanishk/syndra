package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"syndra/internal/zitadel"
)

// liveStub is any client that is not the stand-in.
type liveStub struct{ zitadel.ZitadelClient }

func (liveStub) ListProjects(context.Context, zitadel.SearchParams) (*zitadel.SearchResult[zitadel.ZitadelProject], error) {
	return &zitadel.SearchResult[zitadel.ZitadelProject]{Total: 1}, nil
}

// The stand-in answers health as itself, never as a live Zitadel.
func TestZitadelHealthNamesTheStandIn(t *testing.T) {
	path := filepath.Join(t.TempDir(), "world.json")
	world := `{"v":1,"users":[],"projects":[{"id":"p1","name":"Laser","state":"","roles":[],"apps":[]}],"grants":[]}`
	if err := os.WriteFile(path, []byte(world), 0o600); err != nil {
		t.Fatal(err)
	}
	wc, err := zitadel.NewWorldClient(path)
	if err != nil {
		t.Fatal(err)
	}
	orig := zitadel.MgmtClient
	t.Cleanup(func() { zitadel.MgmtClient = orig })

	for _, tc := range []struct {
		client zitadel.ZitadelClient
		want   string
	}{{wc, "stand-in"}, {liveStub{}, "live"}} {
		zitadel.MgmtClient = tc.client
		rec := httptest.NewRecorder()
		handleZitadelHealth(rec, httptest.NewRequest(http.MethodGet, "/api/v1/zitadel/health", nil))
		var got zitadelHealthResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
			t.Fatal(err)
		}
		if got.Mode != tc.want {
			t.Errorf("%T: mode %q, want %q", tc.client, got.Mode, tc.want)
		}
	}
}
