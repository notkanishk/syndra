package addons

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
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

// mTLS reports whether this registration carries COMPLETE mutual-TLS material:
// a client certificate, its key, and the private CA to verify the add-on with.
//
// The CA is not optional trimming. Omit it and the client verifies the add-on
// against the system root store, which means an internal service on a private
// CA would be authenticated by the public web PKI instead — the add-on's own
// certificate would fail to verify, and any certificate a public CA issued
// would pass. That is not weaker mutual TLS, it is a different and wrong trust
// anchor, so an incomplete triple is not this mode.
func (r Registration) mTLS() bool {
	return r.ClientCertPath != "" && r.ClientKeyPath != "" && r.CAPath != ""
}

// partialMTLS reports material that was meant to be mutual TLS and is not
// complete. Worth naming separately: it is the difference between "this
// deployment chose signed requests" and "this deployment thinks it configured
// mTLS", and only one of those should pass without a word.
//
// A private CA on its own is NOT incomplete mutual TLS. In signed mode it is
// the anchor the add-on's server certificate is verified against — a deliberate
// and recommended configuration, not a half-finished one — so warning about it
// would train an operator to ignore the warning that matters.
func (r Registration) partialMTLS() bool {
	return (r.ClientCertPath != "" || r.ClientKeyPath != "") && !r.mTLS()
}

// signed reports whether this registration carries signing-key material.
func (r Registration) signed() bool { return r.SigningKeyPath != "" }

// AuthMode names how the backend authenticates to this add-on, for operator
// surfaces and startup logging. Complete mTLS wins; incomplete mTLS is not a
// mode and never silently becomes one.
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

	// br is this target's circuit breaker. Per add-on, never shared: one NAS
	// being down is not a reason to stop calling a different one. Guarded by its
	// own mutex rather than registryMu so a call in flight never holds the lock
	// every read of the registry needs.
	br breaker
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
//	ADDON_TRUENAS_BASE_URL=https://syndra-addon-truenas:8090
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
		if err := requireHTTPS(base); err != nil {
			log.Printf("[ADDON] %s: %sBASE_URL %v; not registered", target, prefix, err)
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
			log.Printf("[ADDON] %s configures neither complete mTLS (%sCLIENT_CERT + %sCLIENT_KEY + %sCA_CERT) nor %sSIGNING_KEY; not registered",
				target, prefix, prefix, prefix, prefix)
			continue
		}
		// Incomplete mTLS material alongside a signing key is a working
		// deployment, so it registers — but silently is the one way it must not
		// happen. An operator who believes mutual TLS is on and is actually on
		// signed requests has a wrong mental model of their own trust
		// boundary, and nothing else in the system will ever tell them.
		if r.partialMTLS() {
			log.Printf("[ADDON] WARNING: %s has incomplete mTLS material (cert=%t key=%t ca=%t); proceeding with auth=%s",
				target, r.ClientCertPath != "", r.ClientKeyPath != "", r.CAPath != "", r.AuthMode())
		}
		fresh[target] = &Addon{Registration: r}
		log.Printf("[ADDON] Registered target=%s base=%s auth=%s", target, r.BaseURL, r.AuthMode())
	}

	registryMu.Lock()
	registry = fresh
	registryMu.Unlock()
	purgeCredentialsExcept(fresh)

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

// requireHTTPS refuses any base URL that is not HTTPS.
//
// This is not defence in depth over the transport authentication — it is what
// makes that authentication happen at all. A client's TLS configuration is
// consulted only when a TLS handshake occurs, so an `http://` base URL means
// the client certificate is never presented, the private CA is never used, and
// the registration reports auth=mtls while nothing whatsoever authenticates the
// connection. It is the same wrong mental model an incomplete certificate
// triple creates, reachable through a URL scheme instead.
//
// Signed mode needs it just as much for the other two properties. The signature
// authenticates the request; it does nothing for the response, and nothing for
// confidentiality. Over plain HTTP a secret-bearing body is readable by
// anything on the Compose network, and an on-path peer can forge a 2xx that the
// backend records as a completed mutation — or forge a capability set the
// backend then makes authorization decisions against.
//
// There is deliberately no development escape hatch. An exemption for localhost
// is an exemption that ships.
func requireHTTPS(base string) error {
	u, err := url.Parse(base)
	if err != nil {
		return fmt.Errorf("is not a URL (%v)", err)
	}
	if u.Host == "" {
		return errors.New("names no host")
	}
	if u.Scheme != "https" {
		return fmt.Errorf("uses scheme %q; add-on transport requires https, because a client certificate is never presented and a private CA is never consulted on a connection that performs no handshake", u.Scheme)
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
// Concurrent, each under its own RefreshTimeout. Add-ons share nothing but the
// registry lock, and refreshing them in sequence would make startup cost scale
// with how many targets a deployment has rather than with how slow the slowest
// one is.
//
// It never reports an error upward, and that is the fail-open posture rather
// than laziness: one unreachable NAS must not mark the refresh pass failed and
// pull an operator's attention away from the targets that ARE answering. Each
// failure is logged and recorded on its own add-on, where the health surface
// reads it. Shaped for periodic.Runner, which runs it once at startup and then
// on each tick — without a refresh, a registered add-on stays permanently
// uncallable, so this is what turns registration into capability.
func RefreshAll(ctx context.Context) error {
	var wg sync.WaitGroup
	for _, r := range Registered() {
		wg.Add(1)
		go func(target string) {
			defer wg.Done()
			c, cancel := context.WithTimeout(ctx, refreshTimeout)
			defer cancel()
			if err := Refresh(c, target); err != nil {
				log.Printf("[ADDON] refresh %s: %v", target, err)
			}
		}(r.Target)
	}
	wg.Wait()
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

// httpFetchManifest reads GET {base}/capabilities over the add-on's own
// authenticated transport — the same mutual TLS or request signing every
// mutating call uses.
//
// Not a formality on a read. The manifest is what the backend intersects its
// policy against to decide what is callable, so an unauthenticated read of it
// lets anyone on the path edit the capability set the backend then reasons
// from: mark an operation unavailable and the target quietly stops working,
// mark one available and an operator is offered something that must not be
// offered. The channel that carries the decision has to be as trustworthy as
// the one that carries the mutation.
//
// Decoding is strict. An unknown field from the least trusted component means
// the two sides disagree about the contract, and the contract version exists
// precisely so that disagreement is a refusal rather than a guess — accepting
// unknown fields would let a newer add-on introduce a field the backend ignores
// while both believe they agree.
func httpFetchManifest(ctx context.Context, r Registration) (Manifest, error) {
	cred, err := credentialFor(r)
	if err != nil {
		return Manifest{}, err
	}
	// Bounded by the caller's context, which RefreshAll already gives its own
	// per-target deadline; callTimeout is the ceiling, not the budget.
	resp := doAuthenticated(ctx, cred, http.MethodGet, r.BaseURL+"/capabilities", nil, callTimeout)
	if resp.Outcome != OutcomeSucceeded {
		if resp.Status != 0 {
			return Manifest{}, fmt.Errorf("capabilities returned %d", resp.Status)
		}
		return Manifest{}, resp.Err
	}

	// Already bounded: doAuthenticated caps every response at maxResponseBytes
	// and reports an oversized one as a non-success, so a manifest too large to
	// read whole has been refused above. A second limit here would look
	// load-bearing while guarding nothing.
	dec := json.NewDecoder(bytes.NewReader(resp.Body))
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
