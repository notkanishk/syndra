package addons

import (
	"context"
	"errors"
	"testing"
	"time"
)

// withEnv swaps the getenv seam for a map, restoring it afterwards.
func withEnv(t *testing.T, env map[string]string) {
	t.Helper()
	saved := getenv
	getenv = func(k string) string { return env[k] }
	t.Cleanup(func() { getenv = saved })
}

// withManifest swaps the fetch seam for a fixed answer.
func withManifest(t *testing.T, m Manifest, err error) *int {
	t.Helper()
	calls := 0
	saved := fetchAddonManifest
	fetchAddonManifest = func(context.Context, Registration) (Manifest, error) {
		calls++
		return m, err
	}
	t.Cleanup(func() { fetchAddonManifest = saved })
	return &calls
}

func withClock(t *testing.T, at time.Time) {
	t.Helper()
	saved := timeNow
	timeNow = func() time.Time { return at }
	t.Cleanup(func() { timeNow = saved })
}

// resetRegistry clears package state between tests, since the registry is a
// process-wide singleton like the rest of the backend's clients.
func resetRegistry(t *testing.T) {
	t.Helper()
	registryMu.Lock()
	registry = map[string]*Addon{}
	registryMu.Unlock()
	t.Cleanup(func() {
		registryMu.Lock()
		registry = map[string]*Addon{}
		registryMu.Unlock()
	})
}

func goodManifest() Manifest {
	return Manifest{
		ContractVersion: ContractVersion,
		Product:         "TrueNAS SCALE",
		ProductVersion:  "25.10.0",
		EntitlementSchema: []EntitlementField{
			{Name: "group", Type: "string[]"},
			{Name: "enabled", Type: "bool", Lifecycle: true},
			{Name: "smb_enabled", Type: "bool", Lifecycle: true},
		},
		Operations: []Operation{
			{ID: "health.get", Scope: ScopeAdmin, Available: true},
			{ID: "password.set", Scope: ScopeMember, SecretParams: []string{"password"}, Available: true},
		},
	}
}

func registerTrueNAS(t *testing.T) {
	t.Helper()
	resetRegistry(t)
	withEnv(t, map[string]string{
		"ADDON_TARGETS":             "truenas",
		"ADDON_TRUENAS_BASE_URL":    "http://addon-truenas:8090/",
		"ADDON_TRUENAS_CLIENT_CERT": "/run/secrets/c.crt",
	})
	Init()
}

// 2.2 — registration is deployment configuration, read without touching the
// network. An add-on that is switched off must not delay startup, and one that
// is unreachable must still be registered: operator navigation derives from
// this list, and a nav row that appears when a container comes up is structure
// moving in response to data.
func TestInitRegistersFromConfigWithoutFetching(t *testing.T) {
	calls := withManifest(t, goodManifest(), nil)
	registerTrueNAS(t)

	if *calls != 0 {
		t.Errorf("Init must not fetch anything; made %d manifest call(s)", *calls)
	}
	reg := Registered()
	if len(reg) != 1 || reg[0].Target != "truenas" {
		t.Fatalf("expected truenas registered, got %+v", reg)
	}
	if reg[0].BaseURL != "http://addon-truenas:8090" {
		t.Errorf("trailing slash must be trimmed so paths do not double up, got %q", reg[0].BaseURL)
	}
	if reg[0].ClientCertPath != "/run/secrets/c.crt" {
		t.Errorf("transport material must be read at startup, got %q", reg[0].ClientCertPath)
	}
}

// A target named without a base URL is a misconfiguration, not a registration.
// Registering it would put a nav entry in front of an operator with nothing
// behind it.
func TestInitSkipsTargetWithNoBaseURL(t *testing.T) {
	resetRegistry(t)
	withEnv(t, map[string]string{"ADDON_TARGETS": "truenas, ,unifi"})
	Init()
	if got := Registered(); len(got) != 0 {
		t.Errorf("no target has a base URL, so none may register; got %+v", got)
	}
}

// 2.4 — an unregistered add-on is not callable, and the error says so rather
// than reporting the operation as unknown. The two are different operator
// actions: deploy it, versus fix its manifest.
func TestUnregisteredAddonIsNotCallable(t *testing.T) {
	resetRegistry(t)
	if _, err := ResolveOperation("unifi", "health.get"); !errors.Is(err, ErrNotRegistered) {
		t.Errorf("expected ErrNotRegistered, got %v", err)
	}
}

// Registered but never successfully read: nothing is callable, and that is a
// third distinct state, not an absence and not an unknown operation.
func TestRegisteredWithoutManifestIsNotCallable(t *testing.T) {
	registerTrueNAS(t)
	if _, err := ResolveOperation("truenas", "health.get"); !errors.Is(err, ErrNoManifest) {
		t.Errorf("expected ErrNoManifest before any refresh, got %v", err)
	}
	a, err := Get("truenas")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if _, err := a.Manifest(); !errors.Is(err, ErrNoManifest) {
		t.Errorf("Manifest must report ErrNoManifest, got %v", err)
	}
}

// 2.4 — an operation the add-on implements but does not declare is not
// callable. The manifest is the ceiling on what the backend will even attempt.
func TestOperationAbsentFromManifestIsRejected(t *testing.T) {
	registerTrueNAS(t)
	m := goodManifest()
	m.Operations = []Operation{{ID: "health.get", Scope: ScopeAdmin, Available: true}}
	withManifest(t, m, nil)
	withClock(t, time.Unix(1770000000, 0))

	if err := Refresh(context.Background(), "truenas"); err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	if _, err := ResolveOperation("truenas", "health.get"); err != nil {
		t.Fatalf("declared operation must resolve: %v", err)
	}
	// password.set has a backend policy entry. It is still not callable here,
	// because policy is a ceiling and not a grant.
	if _, err := ResolveOperation("truenas", "password.set"); !errors.Is(err, ErrUnknownOperation) {
		t.Errorf("an operation absent from the manifest must be rejected even though policy permits it; got %v", err)
	}
}

// 2.3 — version skew fails at registration, naming both versions, rather than
// surfacing later as a field that happens to be absent.
func TestUnsupportedContractVersionIsRefused(t *testing.T) {
	registerTrueNAS(t)
	m := goodManifest()
	m.ContractVersion = ContractVersion + 1
	withManifest(t, m, nil)

	err := Refresh(context.Background(), "truenas")
	var ver *ErrUnsupportedContract
	if !errors.As(err, &ver) {
		t.Fatalf("expected ErrUnsupportedContract, got %v", err)
	}
	if ver.Declared != ContractVersion+1 || ver.Supported != ContractVersion {
		t.Errorf("the refusal must name both versions, got declared=%d supported=%d", ver.Declared, ver.Supported)
	}
	if _, err := ResolveOperation("truenas", "health.get"); !errors.Is(err, ErrNoManifest) {
		t.Errorf("a refused manifest must leave the add-on not callable, got %v", err)
	}
}

// A refusal must not take a previously accepted manifest down with it. An
// add-on restarting into a bad state should not also revoke the capability set
// the backend already verified.
func TestRefusalKeepsTheLastGoodManifest(t *testing.T) {
	registerTrueNAS(t)
	withManifest(t, goodManifest(), nil)
	withClock(t, time.Unix(1770000000, 0))
	if err := Refresh(context.Background(), "truenas"); err != nil {
		t.Fatalf("first refresh: %v", err)
	}

	bad := goodManifest()
	bad.ContractVersion = 99
	withManifest(t, bad, nil)
	if err := Refresh(context.Background(), "truenas"); err == nil {
		t.Fatal("expected the second refresh to be refused")
	}
	if _, err := ResolveOperation("truenas", "health.get"); err != nil {
		t.Errorf("the last accepted manifest must keep serving after a refusal, got %v", err)
	}
	a, _ := Get("truenas")
	if a.LastError() == nil {
		t.Error("the refusal must still be recorded, or an operator cannot see why it is stale")
	}
}

// A fetch failure is recorded, not swallowed: "never answered" and "answered
// wrongly" are different things to put in front of an operator.
func TestFetchFailureIsRecorded(t *testing.T) {
	registerTrueNAS(t)
	withManifest(t, Manifest{}, errors.New("connection refused"))

	if err := Refresh(context.Background(), "truenas"); err == nil {
		t.Fatal("expected the refresh to fail")
	}
	a, _ := Get("truenas")
	if a.LastError() == nil {
		t.Error("a fetch failure must be recorded on the add-on")
	}
	if _, err := ResolveOperation("truenas", "health.get"); !errors.Is(err, ErrNoManifest) {
		t.Errorf("an add-on that never answered is not callable, got %v", err)
	}
}

// 2.6 / §18 — an operation the target cannot currently perform is refused with
// its reason, not attempted and not silently absent. Surfaces still render it,
// disabled and explained, which is why Operations() keeps it.
func TestUnavailableOperationIsRefusedWithItsReason(t *testing.T) {
	registerTrueNAS(t)
	m := goodManifest()
	m.Operations = []Operation{
		{ID: "health.get", Scope: ScopeAdmin, Available: true},
		{ID: "activity.get", Scope: ScopeAdmin, Available: false, UnavailableReason: "SMB auditing is disabled on every share"},
	}
	withManifest(t, m, nil)
	withClock(t, time.Unix(1770000000, 0))
	if err := Refresh(context.Background(), "truenas"); err != nil {
		t.Fatalf("Refresh: %v", err)
	}

	_, err := ResolveOperation("truenas", "activity.get")
	var un *ErrOperationUnavailable
	if !errors.As(err, &un) {
		t.Fatalf("expected ErrOperationUnavailable, got %v", err)
	}
	if un.Reason != "SMB auditing is disabled on every share" {
		t.Errorf("the add-on's reason must survive to the surface, got %q", un.Reason)
	}

	a, _ := Get("truenas")
	var found bool
	for _, op := range a.Operations() {
		if op.ID == "activity.get" {
			found = true
		}
	}
	if !found {
		t.Error("an unavailable operation must still be rendered, disabled and explained — omitting it leaves an operator wondering whether the feature exists")
	}
}

// An add-on reporting unavailability with no reason gets one, because "disabled
// and explained" is the contract and a blank explanation fails it.
func TestUnavailableWithoutReasonStillExplains(t *testing.T) {
	ops, _ := resolveOperations(Manifest{
		ContractVersion: ContractVersion,
		Operations:      []Operation{{ID: "health.get", Scope: ScopeAdmin, Available: false}},
	})
	if len(ops) != 1 || ops[0].UnavailableReason == "" {
		t.Errorf("an unavailable operation must always carry a reason, got %+v", ops)
	}
}
