package addons

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"
)

// Registration is what a deployment says about an add-on: where it lives and
// how to authenticate to it. It is configuration, read once at startup.
//
// This is deliberately separate from whether the add-on is reachable or its
// manifest is understood. Registration is a deployment fact — an operator on a
// deployment running a TrueNAS add-on sees the TrueNAS entry whether or not it
// currently answers — and callability is a runtime one. Collapsing them would
// make navigation move in response to data, which is the failure the IA rule
// exists to prevent.
type Registration struct {
	Target string
	// BaseURL is reachable only on the internal Compose network. Add-ons publish
	// no host port: one memory disclosure in the process holding the TrueNAS API
	// key must not also expose the Zitadel service account.
	BaseURL string
	// Transport material for the mutually-authenticated client. Read here so a
	// deployment misconfiguration is visible at startup rather than at the
	// first mutating call.
	//
	// Two modes, and a registration MUST configure one of them. mTLS with a
	// private CA is the default; signed requests carrying a timestamp and a
	// body hash are the fallback where mTLS is impractical. A bare shared
	// secret is neither, and is not offered: it authenticates the caller but
	// binds nothing to the request, so an intercepted call replays verbatim.
	ClientCertPath string
	ClientKeyPath  string
	CAPath         string
	SigningKeyPath string
}

// mTLS reports whether this registration carries a complete client-certificate
// pair. A half-configured pair is not a mode, it is a misconfiguration.
func (r Registration) mTLS() bool { return r.ClientCertPath != "" && r.ClientKeyPath != "" }

// signed reports whether this registration carries signing-key material.
func (r Registration) signed() bool { return r.SigningKeyPath != "" }

// AuthMode names how the backend authenticates to this add-on, for operator
// surfaces and startup logging. mTLS wins when both are configured.
func (r Registration) AuthMode() string {
	switch {
	case r.mTLS():
		return "mtls"
	case r.signed():
		return "signed"
	default:
		return "none"
	}
}

// Addon is a registration plus whatever the backend has since learned about it.
// The manifest is nil until a successful Refresh; that is the honest state for
// an add-on that has not answered yet, and it is distinct from not registered.
type Addon struct {
	Registration Registration

	manifest  *Manifest
	ops       []EffectiveOperation
	fetchedAt time.Time
	lastErr   error
}

var (
	// ErrNotRegistered means no deployment configuration names this target. It
	// is not the same as an add-on that is registered and unreachable.
	ErrNotRegistered = errors.New("addon: target is not registered")
	// ErrNoManifest means registered, but the backend has never successfully
	// read and accepted its manifest. Nothing is callable in that state.
	ErrNoManifest = errors.New("addon: no accepted manifest for this target")
	// ErrUnknownOperation means the manifest does not declare it, or backend
	// policy does not, or both. The distinction is deliberately not exposed:
	// either way it is not callable, and saying which would tell an unauthorised
	// caller something about the deployment.
	ErrUnknownOperation = errors.New("addon: operation is not offered by this target")
)

// ErrUnsupportedContract is the registration refusal for version skew. It names
// both versions, because "the add-on is newer" and "the backend is newer" are
// different operator actions.
type ErrUnsupportedContract struct {
	Target              string
	Declared, Supported int
}

func (e *ErrUnsupportedContract) Error() string {
	return fmt.Sprintf("addon %s: declares contract version %d, backend supports %d",
		e.Target, e.Declared, e.Supported)
}

var (
	registryMu sync.RWMutex
	registry   = map[string]*Addon{}
)

// Init reads the deployment configuration, records every registered add-on in
// process memory, and reconciles the database's `targets` registry to match.
//
// It contacts no add-on. An add-on that is switched off must not delay backend
// startup, and an add-on that is unreachable must still be registered, because
// operator navigation derives from the deployment rather than from what happens
// to be answering.
//
// It does reach the database, and must. Every table carrying a target resolves
// it by foreign key against `targets`, so an add-on registered only in memory
// is one whose first snapshot, plan, outbox, or drift row the database refuses.
// Configuration and the registry row are two halves of the same registration.
//
// Env shape, matching the flat style the rest of the backend uses:
//
//	ADDON_TARGETS=truenas
//	ADDON_TRUENAS_BASE_URL=http://syndra-addon-truenas:8090
//	ADDON_TRUENAS_CLIENT_CERT=/run/secrets/truenas-client.crt
//	ADDON_TRUENAS_CLIENT_KEY=/run/secrets/truenas-client.key
//	ADDON_TRUENAS_CA_CERT=/run/secrets/addon-ca.crt
//	ADDON_TRUENAS_SIGNING_KEY=/run/secrets/truenas-signing.key   # instead of the pair
func Init(ctx context.Context) error {
	fresh := map[string]*Addon{}

	for _, target := range splitTargets(getenv("ADDON_TARGETS")) {
		prefix := "ADDON_" + strings.ToUpper(target) + "_"
		base := strings.TrimSpace(getenv(prefix + "BASE_URL"))
		if base == "" {
			log.Printf("[ADDON] %s named in ADDON_TARGETS but %sBASE_URL is empty; not registered", target, prefix)
			continue
		}
		r := Registration{
			Target:         target,
			BaseURL:        strings.TrimSuffix(base, "/"),
			ClientCertPath: strings.TrimSpace(getenv(prefix + "CLIENT_CERT")),
			ClientKeyPath:  strings.TrimSpace(getenv(prefix + "CLIENT_KEY")),
			CAPath:         strings.TrimSpace(getenv(prefix + "CA_CERT")),
			SigningKeyPath: strings.TrimSpace(getenv(prefix + "SIGNING_KEY")),
		}
		// Fail closed on transport authentication. An add-on holds the target
		// credential and reaches physical infrastructure; calling one over an
		// unauthenticated channel is worse than not having it. A deployment
		// that configures neither mode has not deployed an add-on, so it does
		// not register — and therefore gets no navigation entry promising an
		// operator something that must never be called.
		if r.AuthMode() == "none" {
			log.Printf("[ADDON] %s configures neither %sCLIENT_CERT/%sCLIENT_KEY nor %sSIGNING_KEY; not registered",
				target, prefix, prefix, prefix)
			continue
		}
		fresh[target] = &Addon{Registration: r}
		log.Printf("[ADDON] Registered target=%s base=%s auth=%s", target, r.BaseURL, r.AuthMode())
	}

	registryMu.Lock()
	registry = fresh
	registryMu.Unlock()

	if len(fresh) == 0 {
		log.Println("[ADDON] No add-ons registered")
	}
	return syncTargetRegistry(ctx, fresh)
}

// syncTargetRegistry makes the database agree with the configuration: every
// configured target active, every add-on target the deployment has dropped
// disabled.
//
// Disabling on absence is what makes "remove it from the configuration" mean
// "unregister it", which is the rollback the design specifies. It is never a
// delete: propagation and drift history still points at these rows, the foreign
// key would correctly refuse, and that history is the record of what the target
// was asked to do while it was live.
func syncTargetRegistry(ctx context.Context, reg map[string]*Addon) error {
	configured := make([]string, 0, len(reg))
	for target := range reg {
		if err := dbUpsertTarget(ctx, target); err != nil {
			return fmt.Errorf("addon registry: %w", err)
		}
		configured = append(configured, target)
	}
	sort.Strings(configured)

	disabled, err := dbDisableUnconfiguredTargets(ctx, configured)
	if err != nil {
		return fmt.Errorf("addon registry: %w", err)
	}
	for _, t := range disabled {
		log.Printf("[ADDON] target=%s is no longer configured; registry state set to disabled (history retained)", t)
	}
	return nil
}

// splitTargets accepts a comma-separated list, tolerating spaces and empties so
// a trailing comma in a compose file is not a silent misregistration.
func splitTargets(v string) []string {
	var out []string
	for _, part := range strings.Split(v, ",") {
		if t := strings.ToLower(strings.TrimSpace(part)); t != "" {
			out = append(out, t)
		}
	}
	return out
}

// Registered returns every registered add-on, sorted. This is the deployment
// fact operator navigation derives from — never "what this operator can see".
func Registered() []Registration {
	registryMu.RLock()
	defer registryMu.RUnlock()
	out := make([]Registration, 0, len(registry))
	for _, a := range registry {
		out = append(out, a.Registration)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Target < out[j].Target })
	return out
}

// Refresh reads the add-on's manifest, checks the contract version, resolves
// the effective operation set against backend policy, and caches the result.
//
// A refusal here leaves any previously accepted manifest in place rather than
// clearing it. An add-on that restarts into a bad state should not silently
// widen the blast radius by taking its own previously-verified capability set
// down with it; the error is recorded and surfaced, and the last good answer
// keeps serving until it is replaced by a good one.
func Refresh(ctx context.Context, target string) error {
	registryMu.RLock()
	a := registry[target]
	registryMu.RUnlock()
	if a == nil {
		return fmt.Errorf("%w: %s", ErrNotRegistered, target)
	}

	m, err := fetchAddonManifest(ctx, a.Registration)
	if err != nil {
		registryMu.Lock()
		a.lastErr = err
		registryMu.Unlock()
		return fmt.Errorf("addon %s: fetch manifest: %w", target, err)
	}

	// The version check comes before anything reads a field, so skew fails as
	// a refusal naming both versions rather than as a field that happens to be
	// absent three layers down.
	if m.ContractVersion != ContractVersion {
		err := &ErrUnsupportedContract{Target: target, Declared: m.ContractVersion, Supported: ContractVersion}
		registryMu.Lock()
		a.lastErr = err
		registryMu.Unlock()
		log.Printf("[ADDON] %v — not callable until the versions agree", err)
		return err
	}

	ops, unknown := resolveOperations(m)
	if len(unknown) > 0 {
		// Logged once per refresh, not per call. An operation the backend has
		// no policy for is a deployment that shipped an add-on ahead of its
		// backend, and it fails closed either way; the log is what makes that
		// diagnosable instead of mysterious.
		log.Printf("[ADDON] %s declares %d operation(s) with no backend policy, ignored: %s",
			target, len(unknown), strings.Join(unknown, ", "))
	}

	registryMu.Lock()
	a.manifest = &m
	a.ops = ops
	a.fetchedAt = timeNow().UTC()
	a.lastErr = nil
	registryMu.Unlock()
	return nil
}

// RefreshAll refreshes every registered add-on and returns nil regardless of
// individual failures.
//
// It never reports an error upward, and that is the fail-open posture rather
// than laziness: one unreachable NAS must not mark the refresh pass failed and
// pull an operator's attention away from the targets that ARE answering. Each
// failure is logged and recorded on its own add-on, where the health surface
// reads it. Shaped for periodic.Runner, which runs it once at startup and then
// on each tick — without a refresh, a registered add-on stays permanently
// uncallable, so this loop is what turns registration into capability.
func RefreshAll(ctx context.Context) error {
	for _, r := range Registered() {
		if err := Refresh(ctx, r.Target); err != nil {
			log.Printf("[ADDON] refresh %s: %v", r.Target, err)
		}
	}
	return nil
}

// FetchedAt is when the currently held manifest was accepted. Zero when none
// has been. The health surface labels a stale answer with its age rather than
// presenting it as current.
func (a *Addon) FetchedAt() time.Time {
	registryMu.RLock()
	defer registryMu.RUnlock()
	return a.fetchedAt
}

// Get returns the registered add-on for a target.
func Get(target string) (*Addon, error) {
	registryMu.RLock()
	defer registryMu.RUnlock()
	a := registry[target]
	if a == nil {
		return nil, fmt.Errorf("%w: %s", ErrNotRegistered, target)
	}
	return a, nil
}

// Manifest returns the accepted manifest, or ErrNoManifest.
func (a *Addon) Manifest() (Manifest, error) {
	registryMu.RLock()
	defer registryMu.RUnlock()
	if a.manifest == nil {
		return Manifest{}, fmt.Errorf("%w: %s", ErrNoManifest, a.Registration.Target)
	}
	return *a.manifest, nil
}

// Operations returns the effective operation set for rendering, including the
// unavailable ones. A surface shows those disabled with their reason; omitting
// them would leave an operator wondering whether the feature exists.
func (a *Addon) Operations() []EffectiveOperation {
	registryMu.RLock()
	defer registryMu.RUnlock()
	out := make([]EffectiveOperation, len(a.ops))
	copy(out, a.ops)
	return out
}

// LastError reports why the most recent refresh failed, or nil. Held so an
// operator surface can distinguish "never answered" from "answered wrongly".
func (a *Addon) LastError() error {
	registryMu.RLock()
	defer registryMu.RUnlock()
	return a.lastErr
}

// ResolveOperation is the callability gate: every invocation path goes through
// it,
// and it refuses anything the effective set does not offer or the add-on cannot
// currently perform. An operation the add-on implements but does not declare is
// not callable, and neither is one the manifest declares that policy does not.
func ResolveOperation(target, id string) (EffectiveOperation, error) {
	a, err := Get(target)
	if err != nil {
		return EffectiveOperation{}, err
	}
	registryMu.RLock()
	defer registryMu.RUnlock()
	if a.manifest == nil {
		return EffectiveOperation{}, fmt.Errorf("%w: %s", ErrNoManifest, target)
	}
	for _, op := range a.ops {
		if op.ID != id {
			continue
		}
		if !op.Available {
			return EffectiveOperation{}, &ErrOperationUnavailable{Target: target, ID: id, Reason: op.UnavailableReason}
		}
		return op, nil
	}
	return EffectiveOperation{}, fmt.Errorf("%w: %s/%s", ErrUnknownOperation, target, id)
}

// maxManifestBytes bounds the manifest read. The add-on is the least trusted
// component in the system and an unbounded read from it is a denial of service
// against the backend that governs every other target.
const maxManifestBytes = 1 << 20

// httpFetchManifest reads GET {base}/capabilities.
//
// Decoding is strict. An unknown field from the least trusted component means
// the two sides disagree about the contract, and the contract version exists
// precisely so that disagreement is a refusal rather than a guess — accepting
// unknown fields would let a newer add-on introduce a field the backend ignores
// while both believe they agree.
func httpFetchManifest(ctx context.Context, r Registration) (Manifest, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, r.BaseURL+"/capabilities", nil)
	if err != nil {
		return Manifest{}, err
	}
	resp, err := manifestHTTPClient.Do(req)
	if err != nil {
		return Manifest{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return Manifest{}, fmt.Errorf("capabilities returned %d", resp.StatusCode)
	}

	dec := json.NewDecoder(io.LimitReader(resp.Body, maxManifestBytes))
	dec.DisallowUnknownFields()
	var m Manifest
	if err := dec.Decode(&m); err != nil {
		return Manifest{}, fmt.Errorf("decode manifest: %w", err)
	}
	if err := dec.Decode(new(any)); err != io.EOF {
		return Manifest{}, errors.New("decode manifest: multiple JSON values in body")
	}
	return m, nil
}
