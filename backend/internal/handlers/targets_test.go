package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"syndra/internal/addons"
	"syndra/internal/db"
	"syndra/internal/services/drift"
)

// §23 — the surface has always said "this stays until somebody resolves it".
//
// Nothing could, so a legitimate volume replacement pinned a target as
// compromised permanently and that sentence named an action that did not exist.
// The three properties that make the action safe are the ones tested here.
func TestResolvingALogFinding(t *testing.T) {
	var got struct {
		target, actor, note, head string
	}
	orig := dbResolveLogViolation
	t.Cleanup(func() { dbResolveLogViolation = orig })
	dbResolveLogViolation = func(_ context.Context, target, actor, note, head string) (db.LogAnchor, error) {
		got.target, got.actor, got.note, got.head = target, actor, note, head
		return db.LogAnchor{Target: target, Head: head}, nil
	}

	resolve := func(body string) *httptest.ResponseRecorder {
		rr := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodPost, "/api/v1/targets/truenas/log-anchor/resolve",
			strings.NewReader(body))
		r.SetPathValue("target", "truenas")
		handleResolveLogFinding(rr, r)
		return rr
	}

	// Unconfirmed, and the copy says what is GIVEN UP rather than what is done.
	// It is the only action in the product that discards evidence.
	rr := resolve(`{"head":"abc","note":"we replaced the volume"}`)
	if rr.Code != http.StatusUnprocessableEntity {
		t.Fatalf("want a confirmation gate, got %d", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "stops being able to tell you") {
		t.Errorf("the confirmation must name what is lost: %s", rr.Body.String())
	}

	// An explanation is required. "We replaced the volume" and "we do not know"
	// are the same anchor state and completely different facts.
	if rr := resolve(`{"head":"abc","note":"  ","confirmed":true}`); rr.Code != http.StatusBadRequest {
		t.Errorf("a resolution with no explanation must be refused, got %d", rr.Code)
	}

	if rr := resolve(`{"head":"abc","note":"replaced the volume","confirmed":true}`); rr.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", rr.Code, rr.Body.String())
	}
	// The cited head travels, which is what makes this an adoption of something
	// the operator saw rather than of whatever the target says now.
	if got.head != "abc" || got.note != "replaced the volume" || got.target != "truenas" {
		t.Errorf("the citation and the reason must reach the store: %+v", got)
	}
}

// And a finding that moved is refused, not re-baselined onto a head nobody read.
func TestResolvingAFindingThatMovedIsRefused(t *testing.T) {
	orig := dbResolveLogViolation
	t.Cleanup(func() { dbResolveLogViolation = orig })
	dbResolveLogViolation = func(context.Context, string, string, string, string) (db.LogAnchor, error) {
		return db.LogAnchor{}, db.ErrAnchorMoved
	}

	rr := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/api/v1/targets/truenas/log-anchor/resolve",
		strings.NewReader(`{"head":"stale","note":"replaced the volume","confirmed":true}`))
	r.SetPathValue("target", "truenas")
	handleResolveLogFinding(rr, r)

	if rr.Code != http.StatusConflict {
		t.Fatalf("want 409, got %d: %s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "changed again") {
		t.Errorf("the refusal must say the log moved, which is the event this mechanism is for: %s", rr.Body.String())
	}
}

// A registered target whose transport secret stopped loading says so on the
// roster, before the next call fails at the handshake.
//
// The states are deliberately not merged. "Registered" is a deployment fact
// read once at start-up; this is a live read of the same material, and the gap
// between them is exactly the window an operator needs told about — a mount
// that was unmounted, a rotation that half-finished, a `_FILE` path that
// stopped resolving. The failure it precedes names three causes it cannot tell
// apart, so arriving first is the whole of this field's value.
func TestTheRosterReportsATransportSecretThatStoppedLoading(t *testing.T) {
	origReg, origTC := addonsRegistered, addonsTransportCredentials
	t.Cleanup(func() { addonsRegistered, addonsTransportCredentials = origReg, origTC })

	addonsRegistered = func() []addons.Registration {
		return []addons.Registration{
			{Target: "truenas", BaseURL: "https://truenas-addon:8443", Secret: []byte("configured")},
			{Target: "unifi", BaseURL: "https://unifi-addon:8443", Secret: []byte("configured")},
		}
	}
	addonsTransportCredentials = func() []addons.TransportCredential {
		return []addons.TransportCredential{
			{Target: "truenas", AuthMode: "derived", Status: "error",
				Error: "read ADDON_TRUENAS_SECRET_FILE (/run/secrets/addon/truenas.key): no such file or directory"},
			{Target: "unifi", AuthMode: "derived", Status: "ok"},
		}
	}

	rec := httptest.NewRecorder()
	handleListTargets(rec, httptest.NewRequest(http.MethodGet, "/api/v1/targets", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}

	var body struct {
		Targets []struct {
			Target          string `json:"target"`
			Registered      bool   `json:"registered"`
			TransportStatus string `json:"transport_status"`
			TransportError  string `json:"transport_error"`
		} `json:"targets"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	byTarget := map[string]struct {
		Target          string `json:"target"`
		Registered      bool   `json:"registered"`
		TransportStatus string `json:"transport_status"`
		TransportError  string `json:"transport_error"`
	}{}
	for _, row := range body.Targets {
		byTarget[row.Target] = row
	}

	broken, ok := byTarget["truenas"]
	if !ok {
		t.Fatal("the broken target vanished from the roster; it is still deployed")
	}
	// Still registered. Withdrawing the row would make a mount that fell off
	// look like a target that was never deployed, and those are different
	// sentences with different fixes.
	if !broken.Registered {
		t.Error("a target with an unloadable secret is still registered")
	}
	if broken.TransportStatus != "error" || broken.TransportError == "" {
		t.Errorf("the unloadable secret is not reported: %+v", broken)
	}
	// The path, because "no secret configured" and "the mount is missing" are
	// the same symptom and different fixes.
	if !strings.Contains(broken.TransportError, "/run/secrets/addon/truenas.key") {
		t.Errorf("the error must carry the path it was given, got %q", broken.TransportError)
	}
	// And a healthy target beside it is unaffected — the per-row read must not
	// smear one target's failure across the roster.
	if byTarget["unifi"].TransportStatus != "ok" {
		t.Errorf("the healthy target reports %q", byTarget["unifi"].TransportStatus)
	}
}

// An operator can ask a target to reconcile now, and gets what the pass found.
//
// Until this existed, `ReconcileAddon` ran only from the six-hour scheduler:
// [Reconcile now] covered Zitadel and nothing else, so the answer to "is this
// target in step?" was to wait and read a log line.
func TestATargetCanBeReconciledOnDemand(t *testing.T) {
	origGet, origRec := addonsGet, svcReconcileAddon
	t.Cleanup(func() { addonsGet, svcReconcileAddon = origGet, origRec })

	addonsGet = func(string) (*addons.Addon, error) { return &addons.Addon{}, nil }
	called := ""
	svcReconcileAddon = func(_ context.Context, target string) (drift.AddonReconcileResult, error) {
		called = target
		return drift.AddonReconcileResult{
			Target: target, Bound: new(4), Queued: 1, Read: readNow(),
			Stale: []drift.StaleBinding{{SubjectID: "s", Username: "becky", AccountID: func() *string { s := "3999"; return &s }()}},
		}, nil
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/targets/truenas/reconcile", nil)
	req.SetPathValue("target", "truenas")
	handleReconcileTarget(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body.String())
	}
	if called != "truenas" {
		t.Errorf("reconciled %q", called)
	}
	var got drift.AddonReconcileResult
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	// The stale bindings are the point of reading the result: they are what a
	// sweep refuses to act on and an operator has to decide about.
	if len(got.Stale) != 1 || got.Stale[0].Username != "becky" {
		t.Errorf("the stale bindings did not survive the response: %+v", got.Stale)
	}
}

// A target the deployment does not register cannot be reconciled, and the
// refusal says which of the two it is — an empty pass reported as a healthy one
// is the failure this prevents.
func TestReconcilingAnUnregisteredTargetIsRefused(t *testing.T) {
	origGet := addonsGet
	t.Cleanup(func() { addonsGet = origGet })
	addonsGet = func(string) (*addons.Addon, error) { return nil, addons.ErrNotRegistered }

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/targets/unifi/reconcile", nil)
	req.SetPathValue("target", "unifi")
	handleReconcileTarget(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}

// GET /targets carries each target's kind and its accepted entitlement schema,
// every field with its role, label, unit, combine, unlimited and per_person,
// so the UI renders fields by role and never from a list of its own; and
// nothing at all before a manifest is accepted (addon-contract slice 2, for
// task 3.7).
func TestATargetSaysItsKindAndFieldsOnlyOnceAManifestIsAccepted(t *testing.T) {
	origGet, origManifest := addonsGet, addonsAcceptedShape
	t.Cleanup(func() { addonsGet, addonsAcceptedShape = origGet, origManifest })
	addonsGet = func(string) (*addons.Addon, error) { return &addons.Addon{}, nil }
	addonsAcceptedShape = func(ctx context.Context, target string) (addons.Shape, error) {
		if target != "truenas" {
			return addons.Shape{}, addons.ErrNoManifest
		}
		return addons.Shape{Kind: "storage", EntitlementSchema: truenasSchema}, nil
	}

	raw, _ := json.Marshal(describeTarget(context.Background(), addons.Registration{Target: "truenas"}, addons.TransportCredential{}))
	var got struct {
		Kind   string           `json:"kind"`
		Schema []map[string]any `json:"entitlement_schema"`
	}
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatal(err)
	}
	if got.Kind != "storage" || len(got.Schema) != 5 {
		t.Fatalf("kind %q, schema %v", got.Kind, got.Schema)
	}
	quota := got.Schema[2]
	for k, want := range map[string]any{"name": "home_quota", "role": "limit", "label": "Home folder limit", "unit": "bytes", "combine": "max", "unlimited": "0", "per_person": true} {
		if quota[k] != want {
			t.Errorf("home_quota %s = %v, want %v", k, quota[k], want)
		}
	}

	silent, _ := json.Marshal(describeTarget(context.Background(), addons.Registration{Target: "unifi"}, addons.TransportCredential{}))
	if strings.Contains(string(silent), `"kind"`) || strings.Contains(string(silent), "entitlement_schema") {
		t.Fatalf("no accepted manifest must say nothing, not guess: %s", silent)
	}
}
