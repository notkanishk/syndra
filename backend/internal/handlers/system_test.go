package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"mkauth/internal/directory"
	"mkauth/internal/models"
)

// stubSource is a minimal directory.Source implementation that returns a fixed Tag.
type stubSource struct{ tag string }

func (s stubSource) Users(context.Context) ([]models.UserProfile, error) { return nil, nil }
func (s stubSource) FindUser(context.Context, string) (models.UserProfile, bool, error) {
	return models.UserProfile{}, false, nil
}
func (s stubSource) Projects(context.Context) ([]models.ProjectCatalog, error) { return nil, nil }
func (s stubSource) FindProject(context.Context, string) (models.ProjectCatalog, bool, error) {
	return models.ProjectCatalog{}, false, nil
}
func (s stubSource) Applications(context.Context) ([]models.ApplicationCatalog, error) {
	return nil, nil
}
func (s stubSource) FindApplication(context.Context, string) (models.ApplicationCatalog, bool, error) {
	return models.ApplicationCatalog{}, false, nil
}
func (s stubSource) RoleKeysForProject(context.Context, string) ([]string, error) { return nil, nil }
func (s stubSource) ProjectName(context.Context, string) (string, error)          { return "", nil }
func (s stubSource) Tag() string                                                  { return s.tag }
func (s stubSource) InvalidateAll()                                               {}
func (s stubSource) InvalidateProject(string)                                     {}
func (s stubSource) InvalidateUsers()                                             {}

func withSystemDeps(t *testing.T, source directory.Source, configured, seeded bool) {
	t.Helper()
	withSystemDepsResidue(t, source, configured, seeded, 0, nil)
}

func withSystemDepsResidue(
	t *testing.T,
	source directory.Source,
	configured, seeded bool,
	residue int,
	residueErr error,
) {
	t.Helper()
	origSource := directorySource
	origSeed := seedActive
	origConfigured := systemZitadelConfigured
	origResidue := countDemoResidue

	directorySource = func() directory.Source { return source }
	seedActive = func() bool { return seeded }
	systemZitadelConfigured = func() bool { return configured }
	countDemoResidue = func(context.Context) (int, error) { return residue, residueErr }

	t.Cleanup(func() {
		directorySource = origSource
		seedActive = origSeed
		systemZitadelConfigured = origConfigured
		countDemoResidue = origResidue
	})
}

func TestHandleSystemMode_LiveZitadel(t *testing.T) {
	withSystemDeps(t, stubSource{tag: "zitadel"}, true, false)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/system/mode", nil)
	handleSystemMode(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}

	var got SystemModeResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	want := SystemModeResponse{
		Directory:         "zitadel",
		SeedActive:        false,
		ResetCommand:      resetCommand,
		ZitadelConfigured: true,
		Degraded:          false,
	}
	if got != want {
		t.Fatalf("response mismatch:\n got=%+v\nwant=%+v", got, want)
	}
}

func TestHandleSystemMode_DemoModeNotConfigured(t *testing.T) {
	withSystemDeps(t, stubSource{tag: "demo"}, false, true)

	rr := httptest.NewRecorder()
	handleSystemMode(rr, httptest.NewRequest(http.MethodGet, "/api/v1/system/mode", nil))

	var got SystemModeResponse
	_ = json.Unmarshal(rr.Body.Bytes(), &got)

	want := SystemModeResponse{
		Directory:         "demo",
		SeedActive:        true,
		ResetCommand:      resetCommand,
		ZitadelConfigured: false,
		Degraded:          false,
	}
	if got != want {
		t.Fatalf("response mismatch:\n got=%+v\nwant=%+v", got, want)
	}
}

// The case this whole field exists for: seeding is OFF, so SeedActive is
// false and every prior signal said the deployment was clean — while the rows
// a previous process wrote are still being served alongside real ones.
func TestHandleSystemMode_ResidueSurvivesSeedingBeingTurnedOff(t *testing.T) {
	withSystemDepsResidue(t, stubSource{tag: "zitadel"}, true, false, 31, nil)

	rr := httptest.NewRecorder()
	handleSystemMode(rr, httptest.NewRequest(http.MethodGet, "/api/v1/system/mode", nil))

	var got SystemModeResponse
	_ = json.Unmarshal(rr.Body.Bytes(), &got)

	if got.SeedActive {
		t.Fatalf("precondition: seeding must be off for this case, got %+v", got)
	}
	if got.SeedResidue != 31 {
		t.Fatalf("expected residue 31 to be reported, got %d", got.SeedResidue)
	}
	if got.Degraded {
		t.Fatalf("residue is not degradation — the directory is live, got %+v", got)
	}
	if got.ResetCommand == "" {
		t.Fatal("residue reported with no command to clear it")
	}
}

// A count that failed is not a count of zero. Reporting zero keeps the banner
// quiet rather than announcing residue that may not exist — but it must never
// be reported as a positive, and the request must still succeed.
func TestHandleSystemMode_ResidueCountFailureReportsZero(t *testing.T) {
	withSystemDepsResidue(t, stubSource{tag: "zitadel"}, true, false, 12, errors.New("connection refused"))

	rr := httptest.NewRecorder()
	handleSystemMode(rr, httptest.NewRequest(http.MethodGet, "/api/v1/system/mode", nil))

	if rr.Code != http.StatusOK {
		t.Fatalf("a failed residue count must not fail the mode probe, got %d", rr.Code)
	}

	var got SystemModeResponse
	_ = json.Unmarshal(rr.Body.Bytes(), &got)
	if got.SeedResidue != 0 {
		t.Fatalf("expected 0 on count error, got %d", got.SeedResidue)
	}
	if got.Directory != "zitadel" {
		t.Fatalf("the rest of the response must still be answered, got %+v", got)
	}
}

func TestHandleSystemMode_DegradedFallback(t *testing.T) {
	// Zitadel is configured in the env (operator intent: live mode) but the
	// directory is serving demo data — typically a missing/unreadable key.
	withSystemDeps(t, stubSource{tag: "demo"}, true, false)

	rr := httptest.NewRecorder()
	handleSystemMode(rr, httptest.NewRequest(http.MethodGet, "/api/v1/system/mode", nil))

	var got SystemModeResponse
	_ = json.Unmarshal(rr.Body.Bytes(), &got)

	if !got.Degraded {
		t.Fatalf("expected degraded=true when configured but directory=demo, got %+v", got)
	}
	if got.Directory != "demo" || !got.ZitadelConfigured {
		t.Fatalf("expected demo+configured combo, got %+v", got)
	}
}
