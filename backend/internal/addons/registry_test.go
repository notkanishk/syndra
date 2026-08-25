package addons

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"syndra/internal/db"
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

// withTargetRegistry swaps the database registry calls for an in-memory record
// of what Init asked the database to do.
type fakeTargetRegistry struct {
	upserted []string
	disabled []string
	// keep is what the fake pretends is already active and add-on-owned, so
	// "disable what is no longer configured" has something to disable.
	active    []string
	upsertErr error
}

func withTargetRegistry(t *testing.T, f *fakeTargetRegistry) {
	t.Helper()
	savedUp, savedDis := dbUpsertTarget, dbDisableUnconfiguredTargets
	dbUpsertTarget = func(_ context.Context, target string) error {
		if f.upsertErr != nil {
			return f.upsertErr
		}
		f.upserted = append(f.upserted, target)
		return nil
	}
	dbDisableUnconfiguredTargets = func(_ context.Context, configured []string) ([]db.DisabledTarget, error) {
		keep := map[string]bool{}
		for _, c := range configured {
			keep[c] = true
		}
		for _, a := range f.active {
			if !keep[a] {
				f.disabled = append(f.disabled, a)
			}
		}
		out := make([]db.DisabledTarget, 0, len(f.disabled))
		for _, d := range f.disabled {
			out = append(out, db.DisabledTarget{Target: d})
		}
		return out, nil
	}
	t.Cleanup(func() { dbUpsertTarget, dbDisableUnconfiguredTargets = savedUp, savedDis })
}

// resetRegistry clears package state between tests, since the registry is a
// process-wide singleton like the rest of the backend's clients. The credential
// cache goes with it: it is keyed by target, and a target name reused across
// tests with different material would otherwise be served the first test's
// certificate.
func resetRegistry(t *testing.T) {
	t.Helper()
	clear := func() {
		registryMu.Lock()
		registry = map[string]*Addon{}
		registryMu.Unlock()
		credMu.Lock()
		credCache = map[string]*credential{}
		credMu.Unlock()
		// The on-demand manifest read's own state goes with it. It is keyed by
		// target and remembers when it last tried, so a target name reused
		// across tests would otherwise inherit the previous test's cooldown and
		// silently skip the read the next test is asserting.
		manifestRetryMu.Lock()
		manifestRetries = map[string]*manifestRetry{}
		manifestRetryMu.Unlock()
	}
	clear()
	t.Cleanup(clear)
}

// installAddon puts a target into the registry with an already-accepted
// manifest, which is the state every dispatch test needs and no dispatch test
// is about reaching.
func installAddon(t *testing.T, r Registration, m Manifest) *Addon {
	t.Helper()
	resetRegistry(t)
	ops, _ := resolveOperations(m)
	a := &Addon{Registration: r}
	a.manifest = &m
	a.ops = ops
	registryMu.Lock()
	registry = map[string]*Addon{r.Target: a}
	registryMu.Unlock()

	// Every dispatch now claims a durable row at the moment it sends, so a test
	// that calls without a database needs one. This default accepts any id once
	// and echoes the call back, which is what the real claim does when the
	// record matches; tests about the claim itself override it.
	savedClaim := dbClaimAddonOperation
	var mu sync.Mutex
	claimed := map[string]bool{}
	dbClaimAddonOperation = func(_ context.Context, id, target, operation, subject string) (db.AddonOperation, error) {
		mu.Lock()
		defer mu.Unlock()
		if claimed[id] {
			return db.AddonOperation{}, db.ErrAddonOperationNotOpen
		}
		claimed[id] = true
		return db.AddonOperation{ID: id, Target: target, Operation: operation, SubjectID: subject,
			Status: db.AddonOpDispatching}, nil
	}
	t.Cleanup(func() { dbClaimAddonOperation = savedClaim })

	return a
}

// testClock is a movable clock, for the breaker's cooldown. withClock freezes
// time, which cannot express "and then thirty seconds passed".
type testClock struct {
	mu  sync.Mutex
	now time.Time
}

func withTestClock(t *testing.T, start time.Time) *testClock {
	t.Helper()
	c := &testClock{now: start}
	saved := timeNow
	timeNow = func() time.Time {
		c.mu.Lock()
		defer c.mu.Unlock()
		return c.now
	}
	t.Cleanup(func() { timeNow = saved })
	return c
}

func (c *testClock) advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
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

func registerTrueNAS(t *testing.T) *fakeTargetRegistry {
	t.Helper()
	resetRegistry(t)
	withEnv(t, map[string]string{
		"ADDON_TARGETS":          "truenas",
		"ADDON_TRUENAS_BASE_URL": "https://addon-truenas:8090/",
		"ADDON_TRUENAS_SECRET":   "a-test-transport-secret-for-truenas",
	})
	f := &fakeTargetRegistry{}
	withTargetRegistry(t, f)
	if err := Init(context.Background()); err != nil {
		t.Fatalf("Init: %v", err)
	}
	return f
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
	if reg[0].BaseURL != "https://addon-truenas:8090" {
		t.Errorf("trailing slash must be trimmed so paths do not double up, got %q", reg[0].BaseURL)
	}
	if string(reg[0].Secret) != "a-test-transport-secret-for-truenas" {
		t.Errorf("the transport secret must be resolved at startup, got %q", reg[0].Secret)
	}
}

// A target named without a base URL is a misconfiguration, not a registration.
// Registering it would put a nav entry in front of an operator with nothing
// behind it.
func TestInitSkipsTargetWithNoBaseURL(t *testing.T) {
	resetRegistry(t)
	withEnv(t, map[string]string{"ADDON_TARGETS": "truenas, ,unifi"})
	withTargetRegistry(t, &fakeTargetRegistry{})
	if err := Init(context.Background()); err != nil {
		t.Fatalf("Init: %v", err)
	}
	if got := Registered(); len(got) != 0 {
		t.Errorf("no target has a base URL, so none may register; got %+v", got)
	}
}

// A target name that cannot form an environment variable never registers, and
// says why.
//
// The hyphen is the real case. `${ADDON_MY-NAS_BASE_URL}` is Compose's
// default-value operator, not a variable reference, so the value silently
// becomes something else and the only symptom is "BASE_URL is empty" — pointing
// an operator at a line they set correctly.
func TestInitRefusesATargetNameThatCannotFormAVariable(t *testing.T) {
	resetRegistry(t)
	withEnv(t, map[string]string{
		"ADDON_TARGETS":          "my-nas,truenas",
		"ADDON_TRUENAS_BASE_URL": "https://addon-truenas:8090",
		"ADDON_TRUENAS_SECRET":   "a-test-transport-secret-for-truenas",
	})
	withTargetRegistry(t, &fakeTargetRegistry{})
	if err := Init(context.Background()); err != nil {
		t.Fatalf("Init: %v", err)
	}
	got := Registered()
	// The usable name beside it still registers: one bad entry must not take
	// the deployment's working add-ons down with it.
	if len(got) != 1 || got[0].Target != "truenas" {
		t.Fatalf("expected only truenas registered, got %+v", got)
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
	// The fetch seam is stubbed to fail because resolving now ATTEMPTS one read
	// for a target that has no manifest. Left unstubbed this test would make a
	// real HTTP call to the registered address.
	calls := withManifest(t, Manifest{}, errors.New("nothing is listening"))
	if _, err := ResolveOperation("truenas", "health.get"); !errors.Is(err, ErrNoManifest) {
		t.Errorf("expected ErrNoManifest before any refresh, got %v", err)
	}
	if *calls != 1 {
		t.Errorf("the refusal must cost exactly one read, got %d", *calls)
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

// A target that exists only in process memory is a target whose first
// snapshot, plan, outbox, or drift row the database refuses: every one of those
// tables resolves `target` by foreign key against the registry. Configuration
// and the registry row are two halves of one registration.
func TestInitRegistersTheTargetInTheDatabase(t *testing.T) {
	f := registerTrueNAS(t)
	if len(f.upserted) != 1 || f.upserted[0] != "truenas" {
		t.Errorf("Init must record the configured target in the database registry, upserted=%v", f.upserted)
	}
}

// Removing a target from the configuration is how an operator unregisters one,
// and unregistering is disabling — never deleting, because propagation and
// drift history still points at the row and is the record of what the target
// was asked to do while it was live.
func TestInitDisablesTargetsTheDeploymentNoLongerConfigures(t *testing.T) {
	resetRegistry(t)
	withEnv(t, map[string]string{
		"ADDON_TARGETS":          "truenas",
		"ADDON_TRUENAS_BASE_URL": "https://addon-truenas:8090",
		"ADDON_TRUENAS_SECRET":   "a-test-secret",
	})
	f := &fakeTargetRegistry{active: []string{"truenas", "unifi"}}
	withTargetRegistry(t, f)
	if err := Init(context.Background()); err != nil {
		t.Fatalf("Init: %v", err)
	}
	if len(f.disabled) != 1 || f.disabled[0] != "unifi" {
		t.Errorf("a target no longer configured must be disabled, not left active; disabled=%v", f.disabled)
	}
}

// A database failure during registration is fatal to the caller, not swallowed.
// The alternative is a backend that believes a target is registered while every
// write naming it will be refused.
func TestInitReportsARegistryFailure(t *testing.T) {
	resetRegistry(t)
	withEnv(t, map[string]string{
		"ADDON_TARGETS":          "truenas",
		"ADDON_TRUENAS_BASE_URL": "https://addon-truenas:8090",
		"ADDON_TRUENAS_SECRET":   "a-test-secret",
	})
	withTargetRegistry(t, &fakeTargetRegistry{upsertErr: errors.New("connection refused")})
	if err := Init(context.Background()); err == nil {
		t.Fatal("a registry write failure must be reported, not logged and ignored")
	}
}

// 2.2 / 2.35 — either transport mode may be configured. A shared secret is not
// offered as a third: it authenticates the caller but binds nothing to the
// request, so an intercepted call replays verbatim.
func TestSigningKeyIsAValidTransportMode(t *testing.T) {
	resetRegistry(t)
	withEnv(t, map[string]string{
		"ADDON_TARGETS":          "truenas",
		"ADDON_TRUENAS_BASE_URL": "https://addon-truenas:8090",
		"ADDON_TRUENAS_SECRET":   "an-inline-transport-secret",
	})
	withTargetRegistry(t, &fakeTargetRegistry{})
	if err := Init(context.Background()); err != nil {
		t.Fatalf("Init: %v", err)
	}
	reg := Registered()
	if len(reg) != 1 {
		t.Fatalf("an inline secret must be enough to register, got %+v", reg)
	}
	if reg[0].AuthMode() != "derived" {
		t.Errorf("expected derived transport, got %q", reg[0].AuthMode())
	}
	if string(reg[0].Secret) != "an-inline-transport-secret" {
		t.Errorf("the secret must be resolved, got %q", reg[0].Secret)
	}
}

// Two targets sharing one secret is the misconfiguration the derivation cannot
// survive: the salt keeps their keys distinct, but anything holding the value
// derives both, so a compromise of one add-on hands over the other.
//
// BOTH are refused, never one in preference. Registering either would leave an
// operator believing two add-ons are isolated when one credential opens both —
// and the surface would agree with them.
func TestTwoTargetsSharingASecretAreBothRefused(t *testing.T) {
	resetRegistry(t)
	withEnv(t, map[string]string{
		"ADDON_TARGETS":          "truenas,unifi",
		"ADDON_TRUENAS_BASE_URL": "https://addon-truenas:8090",
		"ADDON_TRUENAS_SECRET":   "the-same-secret-in-both-places",
		"ADDON_UNIFI_BASE_URL":   "https://addon-unifi:8090",
		"ADDON_UNIFI_SECRET":     "the-same-secret-in-both-places",
	})
	withTargetRegistry(t, &fakeTargetRegistry{})
	if err := Init(context.Background()); err != nil {
		t.Fatalf("Init: %v", err)
	}
	if reg := Registered(); len(reg) != 0 {
		t.Fatalf("neither target may register on a shared secret, got %+v", reg)
	}
}

// Distinct secrets register normally. The guard above must refuse a collision,
// not two add-ons.
func TestTwoTargetsWithDistinctSecretsBothRegister(t *testing.T) {
	resetRegistry(t)
	withEnv(t, map[string]string{
		"ADDON_TARGETS":          "truenas,unifi",
		"ADDON_TRUENAS_BASE_URL": "https://addon-truenas:8090",
		"ADDON_TRUENAS_SECRET":   "the-truenas-secret",
		"ADDON_UNIFI_BASE_URL":   "https://addon-unifi:8090",
		"ADDON_UNIFI_SECRET":     "the-unifi-secret",
	})
	withTargetRegistry(t, &fakeTargetRegistry{})
	if err := Init(context.Background()); err != nil {
		t.Fatalf("Init: %v", err)
	}
	if reg := Registered(); len(reg) != 2 {
		t.Fatalf("two distinctly-configured targets must both register, got %+v", reg)
	}
}

// The duplicate check compares RESOLVED bytes, not the configured strings. One
// target inline and the other by file is the case that would otherwise slip
// through — and it is the likelier one, since it is what a half-finished
// migration to file-based secrets looks like.
func TestADuplicateIsCaughtAcrossConfigurationForms(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "unifi.key")
	// With a trailing newline, as every mounted secret has: the resolution must
	// trim before comparing, or the duplicate hides behind one byte.
	if err := os.WriteFile(path, []byte("the-same-secret-in-both-places\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	resetRegistry(t)
	withEnv(t, map[string]string{
		"ADDON_TARGETS":           "truenas,unifi",
		"ADDON_TRUENAS_BASE_URL":  "https://addon-truenas:8090",
		"ADDON_TRUENAS_SECRET":    "the-same-secret-in-both-places",
		"ADDON_UNIFI_BASE_URL":    "https://addon-unifi:8090",
		"ADDON_UNIFI_SECRET_FILE": path,
	})
	withTargetRegistry(t, &fakeTargetRegistry{})
	if err := Init(context.Background()); err != nil {
		t.Fatalf("Init: %v", err)
	}
	if reg := Registered(); len(reg) != 0 {
		t.Fatalf("a duplicate across configuration forms must refuse both, got %+v", reg)
	}
}

// An add-on holds the target credential and reaches physical infrastructure.
// Calling one over an unauthenticated channel is worse than not having it, so a
// deployment configuring neither mode has not deployed an add-on — and gets no
// navigation entry promising an operator something that must never be called.
func TestNoTransportAuthMeansNoRegistration(t *testing.T) {
	for name, env := range map[string]map[string]string{
		"neither": {
			"ADDON_TARGETS":          "truenas",
			"ADDON_TRUENAS_BASE_URL": "https://addon-truenas:8090",
		},
		"an empty inline secret": {
			"ADDON_TARGETS":          "truenas",
			"ADDON_TRUENAS_BASE_URL": "https://addon-truenas:8090",
			"ADDON_TRUENAS_SECRET":   "   ",
		},
		"a secret file that is not there": {
			"ADDON_TARGETS":             "truenas",
			"ADDON_TRUENAS_BASE_URL":    "https://addon-truenas:8090",
			"ADDON_TRUENAS_SECRET_FILE": "/nonexistent/truenas.key",
		},
	} {
		t.Run(name, func(t *testing.T) {
			resetRegistry(t)
			withEnv(t, env)
			f := &fakeTargetRegistry{}
			withTargetRegistry(t, f)
			if err := Init(context.Background()); err != nil {
				t.Fatalf("Init: %v", err)
			}
			if got := Registered(); len(got) != 0 {
				t.Errorf("expected no registration, got %+v", got)
			}
			if len(f.upserted) != 0 {
				t.Errorf("an unregistered target must not be activated in the database, upserted=%v", f.upserted)
			}
		})
	}
}

// One mode, so AuthMode has one thing to say and one absence to report. The
// two-mode machinery this replaced — complete-mTLS-wins, partial-mTLS-warns —
// is gone with the certificates, and its tests with it: they asserted a choice
// that no longer exists, and a test describing a decision the code does not
// make is the shape §32 records.
func TestAuthModeReportsDerivedOrNothing(t *testing.T) {
	if got := (Registration{Secret: []byte("x")}).AuthMode(); got != "derived" {
		t.Errorf("a configured secret must report derived, got %q", got)
	}
	if got := (Registration{}).AuthMode(); got != "none" {
		t.Errorf("no secret must report none, got %q", got)
	}
}

// The refresh budget is per target, not per pass. A shared one means the first
// unreachable add-on spends it and every target behind it is cancelled before
// it is even asked — so one switched-off NAS would suppress the contract check
// on every other target at startup, which is the check's whole purpose.
func TestRefreshAllBoundsEachTargetSeparately(t *testing.T) {
	resetRegistry(t)
	// The names are ordered on purpose. Registered() sorts, so "aslow" is
	// refreshed first: under a shared budget it drains the whole thing and
	// "zfast" is cancelled unasked. Reverse the names and a sequential
	// implementation passes this test by luck, which is worse than no test.
	withEnv(t, map[string]string{
		"ADDON_TARGETS":        "aslow,zfast",
		"ADDON_ASLOW_BASE_URL": "https://slow:8090",
		"ADDON_ASLOW_SECRET":   "the-slow-secret",
		"ADDON_ZFAST_BASE_URL": "https://fast:8090",
		"ADDON_ZFAST_SECRET":   "the-fast-secret",
	})
	withTargetRegistry(t, &fakeTargetRegistry{})
	withClock(t, time.Unix(1770000000, 0))
	if err := Init(context.Background()); err != nil {
		t.Fatalf("Init: %v", err)
	}

	// slowtarget hangs until its own context expires; fasttarget answers at
	// once. Under a shared budget the slow one drains it and the fast one is
	// cancelled unasked.
	savedTimeout := refreshTimeout
	refreshTimeout = 100 * time.Millisecond
	t.Cleanup(func() { refreshTimeout = savedTimeout })

	saved := fetchAddonManifest
	t.Cleanup(func() { fetchAddonManifest = saved })
	fetchAddonManifest = func(ctx context.Context, r Registration) (Manifest, error) {
		if r.Target == "aslow" {
			<-ctx.Done()
			return Manifest{}, ctx.Err()
		}
		// Respects its context, as any real HTTP call does: a request issued on
		// an already-expired context fails without reaching the wire. Without
		// this the fake would succeed on a dead context and a shared-budget
		// implementation would pass by accident.
		if err := ctx.Err(); err != nil {
			return Manifest{}, err
		}
		return goodManifest(), nil
	}

	done := make(chan struct{})
	go func() {
		_ = RefreshAll(context.Background())
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("RefreshAll did not return; targets are not bounded independently")
	}

	if _, err := ResolveOperation("zfast", "health.get"); err != nil {
		t.Errorf("the reachable target must still be refreshed alongside an unreachable one: %v", err)
	}
	slow, _ := Get("aslow")
	if slow.LastError() == nil {
		t.Error("the unreachable target must record its own failure")
	}
}

// 11.7 — the start-up race, and the four limits that keep its fix from becoming
// its own outage.
//
// `RefreshAll` runs once at start-up and then on a period. An add-on still
// starting during that first pass left its target refused until the next tick,
// while the add-on was up and answering. Met on the dev deployment: `docker
// compose up -d` raced the add-on's start and a release came back `no accepted
// manifest` against a target that was fine.

func TestACallReadsTheManifestTheStartupPassMissed(t *testing.T) {
	registerTrueNAS(t)
	calls := withManifest(t, goodManifest(), nil)
	withClock(t, time.Unix(1770000000, 0))

	// No Refresh first: this is the state a backend that started ahead of its
	// add-on is in.
	if _, err := ResolveOperation("truenas", "health.get"); err != nil {
		t.Fatalf("a target whose add-on is up must become callable on demand: %v", err)
	}
	if *calls != 1 {
		t.Fatalf("want exactly one capability read, got %d", *calls)
	}
}

// The single flight. A burst of refused calls must produce ONE capability read
// between them — the retry must not become the thing that keeps a starting
// add-on down.
func TestABurstOfCallsCostsOneManifestRead(t *testing.T) {
	registerTrueNAS(t)
	var mu sync.Mutex
	calls := 0
	saved := fetchAddonManifest
	fetchAddonManifest = func(context.Context, Registration) (Manifest, error) {
		mu.Lock()
		calls++
		mu.Unlock()
		// Long enough that every other goroutine is queued behind this one when
		// it returns, which is the case the re-check inside the lock exists for.
		time.Sleep(20 * time.Millisecond)
		return goodManifest(), nil
	}
	t.Cleanup(func() { fetchAddonManifest = saved })
	withClock(t, time.Unix(1770000000, 0))

	var wg sync.WaitGroup
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := ResolveOperation("truenas", "health.get"); err != nil {
				t.Errorf("resolve: %v", err)
			}
		}()
	}
	wg.Wait()

	mu.Lock()
	defer mu.Unlock()
	if calls != 1 {
		t.Fatalf("twelve concurrent calls must read the manifest once, got %d", calls)
	}
}

// And once, never a loop. A target that genuinely has no manifest must refuse
// quickly: the calls arriving after a failed attempt cost nothing until the
// cooldown expires.
func TestARefusedManifestReadIsNotRetriedPerCall(t *testing.T) {
	registerTrueNAS(t)
	calls := withManifest(t, Manifest{}, errors.New("connection refused"))
	withClock(t, time.Unix(1770000000, 0))

	for i := 0; i < 5; i++ {
		if _, err := ResolveOperation("truenas", "health.get"); !errors.Is(err, ErrNoManifest) {
			t.Fatalf("want ErrNoManifest, got %v", err)
		}
	}
	if *calls != 1 {
		t.Fatalf("five refused calls must not buy five reads, got %d", *calls)
	}
}

// The cooldown ends. An add-on that comes up a minute later is picked up by the
// next call rather than by the next tick, which is the whole point.
func TestTheReadIsTriedAgainOnceTheCooldownHasPassed(t *testing.T) {
	registerTrueNAS(t)
	at := time.Unix(1770000000, 0)
	withClock(t, at)
	saved := fetchAddonManifest
	calls := 0
	fetchAddonManifest = func(context.Context, Registration) (Manifest, error) {
		calls++
		if calls == 1 {
			return Manifest{}, errors.New("connection refused")
		}
		return goodManifest(), nil
	}
	t.Cleanup(func() { fetchAddonManifest = saved })

	if _, err := ResolveOperation("truenas", "health.get"); !errors.Is(err, ErrNoManifest) {
		t.Fatalf("want ErrNoManifest while the add-on is down, got %v", err)
	}
	timeNow = func() time.Time { return at.Add(manifestRetryCooldown + time.Second) }
	if _, err := ResolveOperation("truenas", "health.get"); err != nil {
		t.Fatalf("an add-on that came up must be picked up by a call: %v", err)
	}
	if calls != 2 {
		t.Fatalf("want one read per side of the cooldown, got %d", calls)
	}
}

// The limit that keeps this from eating a decision. A manifest that WAS read and
// whose operation is not offered is a refusal an operator must act on; re-reading
// it would turn that into a loop, and would hide the reason.
func TestAnOperationRefusalNeverTriggersAManifestRead(t *testing.T) {
	m := goodManifest()
	m.Operations = []Operation{{ID: "health.get", Scope: ScopeAdmin, Available: true}}
	installAddon(t, Registration{Target: "truenas", BaseURL: "http://x", Secret: []byte("a-test-secret")}, m)
	calls := withManifest(t, goodManifest(), nil)

	if _, err := ResolveOperation("truenas", "account.purge"); !errors.Is(err, ErrUnknownOperation) {
		t.Fatalf("want ErrUnknownOperation, got %v", err)
	}
	if *calls != 0 {
		t.Fatalf("a held manifest must never be re-read by a refusal, got %d reads", *calls)
	}
}
