package addons

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"maps"
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"syndra/internal/db"
	"syndra/internal/services/periodic"
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
	// The transport secret for this target, resolved to bytes. One value, from
	// which both derived keys come (derive.go).
	//
	// There used to be two modes here — mutual TLS against a private CA, and
	// signed requests — chosen between by which files were configured. Both are
	// gone with the CA. What replaced them is not a weaker third option: the
	// request is still signed over its timestamp, method, path and body, and
	// the add-on is still verified, now by an exact key rather than by a chain.
	// A bare shared secret PRESENTED as a credential remains not offered, for
	// the reason it never was: it authenticates the caller and binds nothing to
	// the request, so an intercepted call replays verbatim.
	Secret []byte
	// SecretPath is the file Secret came from, or empty when it was configured
	// inline. Kept because it is the rotation signal: the credential cache
	// watches this path's modification time.
	SecretPath string
}

// AuthMode names how the backend authenticates to this add-on, for operator
// surfaces and startup logging.
//
// One mode now, so this answers "derived" or "none" — and "none" is not a mode
// the deployment can choose, only the absence of configuration, which Init
// refuses to register.
func (r Registration) AuthMode() string {
	if len(r.Secret) > 0 {
		return "derived"
	}
	return "none"
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
//	ADDON_TRUENAS_BASE_URL=https://truenas-addon:8443
//	ADDON_TRUENAS_SECRET_FILE=/run/secrets/addon/truenas.key
//	ADDON_TRUENAS_SECRET=<the value inline, where a mount is impractical>
//
// One value per target, from which both keys are derived (derive.go). The host
// and port are the Compose SERVICE name and the add-on's real listener — an
// earlier version of this comment named neither, and the only symptom was an
// add-on that registered and never answered.
func Init(ctx context.Context) error {
	fresh := map[string]*Addon{}
	// Resolved secret -> the target that claimed it first, for the duplicate
	// check below; and the targets refused because of one.
	seen := map[string]string{}
	refused := map[string]bool{}

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
		secret, err := resolveSecret(prefix+"SECRET", getenv)
		if err != nil {
			log.Printf("[ADDON] %s: %v; not registered", target, err)
			continue
		}
		r := Registration{
			Target:     target,
			BaseURL:    strings.TrimSuffix(base, "/"),
			Secret:     []byte(secret),
			SecretPath: strings.TrimSpace(getenv(prefix + "SECRET_FILE")),
		}
		// Fail closed on transport authentication. An add-on holds the target
		// credential and reaches physical infrastructure; calling one over an
		// unauthenticated channel is worse than not having it. A deployment
		// that configures no secret has not deployed an add-on, so it does not
		// register — and therefore gets no navigation entry promising an
		// operator something that must never be called.
		if r.AuthMode() == "none" {
			log.Printf("[ADDON] %s sets neither %sSECRET nor %sSECRET_FILE; not registered", target, prefix, prefix)
			continue
		}
		// Two targets sharing a secret is the one misconfiguration the
		// derivation cannot survive: the salt keeps their keys distinct, but
		// anything holding the value derives both, so a compromise of one
		// add-on hands over the other. Generation covers first setup only — a
		// copied .env block or a rotation that reuses a value reintroduces it
		// with no symptom at all.
		//
		// Both are refused, never one in preference: registering either would
		// leave an operator believing two add-ons are isolated when one
		// credential opens both. Compared on RESOLVED bytes, because one target
		// may be configured inline and the other by file.
		if other, dup := seen[string(secret)]; dup {
			log.Printf("[ADDON] %s and %s are configured with the SAME transport secret; neither is registered. "+
				"Each target must have its own: a compromise of one add-on otherwise derives the other's keys.", other, target)
			delete(fresh, other)
			refused[other] = true
			refused[target] = true
			continue
		}
		seen[string(secret)] = target
		if refused[target] {
			continue
		}
		fresh[target] = &Addon{Registration: r}
		// The key this backend will PIN, logged beside the registration.
		//
		// The add-on logs the key it serves at its own startup, and a pin
		// failure names three causes it cannot tell apart. With both lines the
		// operator diffs two hex strings instead of investigating; without this
		// one, the only way to see what was expected was to read the source and
		// recompute it by hand. Public key, so there is nothing here to leak.
		if pub, err := deriveTLSPublicKey(r.Secret, target); err == nil {
			log.Printf("[ADDON] Registered target=%s base=%s auth=%s pinned_key=%x",
				target, r.BaseURL, r.AuthMode(), pub)
		} else {
			log.Printf("[ADDON] Registered target=%s base=%s auth=%s (could not derive the pinned key: %v)",
				target, r.BaseURL, r.AuthMode(), err)
		}
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
	for _, d := range disabled {
		log.Printf("[ADDON] target=%s is no longer configured; registry state set to disabled (history retained)", d.Target)
		// Loud, because these are approved changes that will now never reach
		// anyone. A deregistration that quietly swallowed queued work would be
		// the silent discard this plane is built to avoid.
		for _, w := range d.Abandoned {
			if w.Dispatched {
				log.Printf("[ADDON] target=%s subject=%s outbox=%s was IN FLIGHT when the target was deregistered; whether it applied is unknown", d.Target, w.SubjectID, w.OutboxID)
				continue
			}
			log.Printf("[ADDON] target=%s subject=%s outbox=%s was queued and will never be dispatched", d.Target, w.SubjectID, w.OutboxID)
		}
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
	if u.User != nil {
		// Credentials in the URL. This value is logged at registration and
		// appears in every error naming the base URL, so a password here is a
		// password in the log — and the transport authenticates with a client
		// certificate or a signature, so it would not even be used.
		return errors.New("carries credentials in the URL; the transport authenticates with a certificate or a signature, and a URL is logged")
	}
	if u.RawQuery != "" || u.Fragment != "" {
		// The base URL is concatenated with "/apply" and "/operations/<name>".
		// A query string or fragment would land before the path and produce a
		// request to something else entirely.
		return errors.New("carries a query string or fragment; the base URL is joined with a path and would not survive it")
	}
	return nil
}

// splitTargets accepts a comma-separated list, tolerating spaces and empties so
// a trailing comma in a compose file is not a silent misregistration.
func splitTargets(v string) []string {
	var out []string
	for _, part := range strings.Split(v, ",") {
		t := strings.ToLower(strings.TrimSpace(part))
		if t == "" {
			continue
		}
		// A target name becomes part of an environment variable name
		// (ADDON_<TARGET>_BASE_URL), so the charset is not cosmetic. A hyphen
		// is the case that bites: `${ADDON_MY-NAS_BASE_URL}` is Compose's
		// default-value operator, so the variable expands to something else
		// entirely and the target registers with no base URL — reported as
		// "named in ADDON_TARGETS but BASE_URL is empty", which sends an
		// operator to look at a line they already set correctly.
		if !targetName.MatchString(t) {
			log.Printf("[ADDON] %q in ADDON_TARGETS is not a usable target name "+
				"(want %s); it forms part of ADDON_<TARGET>_* variable names, "+
				"where a hyphen is Compose's default-value operator and would "+
				"expand to something else. Not registered.", t, targetName)
			continue
		}
		out = append(out, t)
	}
	return out
}

// The shape a target name has to have to survive Compose interpolation.
var targetName = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)

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

	// What the manifest means is checked before anything is accepted from it:
	// a kind this backend does not know, a role outside the vocabulary, or
	// anything but one account field is a refusal with its reason (design §3,
	// §4). So is a manifest that re-purposes a field: a changed role on an
	// existing name, or another account field, would silently change what
	// withdraw-only and revocation do (design §2.6). The last good manifest
	// and role map stay in place.
	if err := m.validate(); err != nil {
		return refuseManifest(a, target, err)
	}
	if err := acceptRoles(ctx, target, m); err != nil {
		return refuseManifest(a, target, err)
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
	if prev := a.manifest; len(m.Unknown) > 0 && (prev == nil || !slices.Equal(prev.Unknown, m.Unknown)) {
		// Logged once per change, never per call (design §2(a)): a key this
		// backend does not know is an add-on ahead of it, or a typo the kit
		// would fail. Shown on the target page; never used for a decision.
		log.Printf("[ADDON] %s's manifest also says (not used): %s", target, strings.Join(m.Unknown, "; "))
	}
	a.manifest = &m
	a.ops = ops
	a.fetchedAt = timeNow().UTC()
	a.lastErr = nil
	registryMu.Unlock()
	return nil
}

// refuseManifest records why a manifest was not accepted and keeps the last
// good one serving.
func refuseManifest(a *Addon, target string, err error) error {
	err = &ErrManifestRefused{Target: target, Reason: err.Error()}
	registryMu.Lock()
	a.lastErr = err
	registryMu.Unlock()
	log.Printf("[ADDON] %v — the last accepted manifest stays in use", err)
	return err
}

// ErrManifestRefused is a manifest whose meaning this backend refuses, with
// the reason the target page shows.
type ErrManifestRefused struct {
	Target, Reason string
}

func (e *ErrManifestRefused) Error() string {
	return fmt.Sprintf("addon %s: manifest refused: %s", e.Target, e.Reason)
}

// acceptRoles holds the manifest's role map to the one persisted for the
// target and persists the result (targets.field_roles, design §3). A field
// keeps its role for ever: a manifest naming an existing field with another
// role, or naming another field the account, is refused and the stored map
// kept. A field the manifest no longer declares keeps its entry, so it cannot
// come back meaning something else, and is recorded retired: the target no
// longer accepts it, so nothing reads or sends it until a manifest declares it
// again. The account field can never be retired: validate requires exactly
// one account and this refuses any but the stored one.
func acceptRoles(ctx context.Context, target string, m Manifest) error {
	stored, _, err := dbTargetFieldRoles(ctx, target)
	if err != nil {
		return fmt.Errorf("the stored field roles could not be read, so a change of meaning could not be ruled out: %w", err)
	}
	merged := maps.Clone(stored.Roles)
	if merged == nil {
		merged = map[string]string{}
	}
	account := Roles(stored.Roles).Account()
	declared := map[string]bool{}
	for _, f := range m.EntitlementSchema {
		if was, known := stored.Roles[f.Name]; known && was != f.Role {
			return fmt.Errorf("field %q was declared with role %s and is now declared %s; a field is never re-purposed, a new meaning is a new name", f.Name, was, f.Role)
		}
		if f.Role == RoleAccount && account != "" && f.Name != account {
			return fmt.Errorf("field %q is declared the account, and %q is; the account field never moves", f.Name, account)
		}
		merged[f.Name] = f.Role
		declared[f.Name] = true
	}
	retired := []string{}
	for name := range merged {
		if !declared[name] {
			retired = append(retired, name)
		}
	}
	slices.Sort(retired)
	if maps.Equal(merged, stored.Roles) && slices.Equal(retired, stored.Retired) {
		return nil
	}
	return dbSaveTargetFieldRoles(ctx, target, db.FieldRoleMap{Roles: merged, Retired: retired})
}

// FieldRoles is a target's role map of the fields its accepted manifest
// declares, field name → role, which every reader of a field's meaning reads
// instead of the live manifest: it holds while the add-on is down and across
// a backend restart (design §3). A retired field is not in it: kept in the
// store so its name keeps its meaning, never sent, since the target refuses it.
func FieldRoles(ctx context.Context, target string) (Roles, error) {
	stored, found, err := dbTargetFieldRoles(ctx, target)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, fmt.Errorf("%w: %s has no accepted contract-2 manifest yet", ErrNoManifest, target)
	}
	roles := maps.Clone(Roles(stored.Roles))
	for _, name := range stored.Retired {
		delete(roles, name)
	}
	return roles, nil
}

// AlsoSays is what the target's accepted manifest carries that this backend
// does not know, quoted and capped, for the target page.
func (a *Addon) AlsoSays() []string {
	registryMu.RLock()
	defer registryMu.RUnlock()
	if a.manifest == nil {
		return nil
	}
	return slices.Clone(a.manifest.Unknown)
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
// quietStartup silences RefreshAll's per-target failure lines while start-up
// waits for add-ons to listen (awaitFirstManifests), which then says once what
// is still missing.
var quietStartup atomic.Bool

// QuietStartup turns that silence on and off.
func QuietStartup(on bool) { quietStartup.Store(on) }

func RefreshAll(ctx context.Context) error {
	var wg sync.WaitGroup
	for _, r := range Registered() {
		wg.Add(1)
		go func(target string) {
			defer wg.Done()
			defer periodic.Recover("ADDON")
			c, cancel := context.WithTimeout(ctx, refreshTimeout)
			defer cancel()
			if err := Refresh(c, target); err != nil && !quietStartup.Load() {
				log.Printf("[ADDON] refresh %s: %v", target, err)
			}
		}(r.Target)
	}
	wg.Wait()
	return nil
}

// PendingManifests names the registered targets that have never served one.
//
// The backend and its add-ons start together, so the first manifest read
// frequently reaches a listener that is not up yet — a `connection refused`
// two seconds into a deploy. Nothing was wrong and nothing retried: the next
// attempt was a full refresh interval away, and for that whole window the
// deployment reported the add-on as configured-and-silent while the add-on sat
// answering. Three separate screens said three different things about it,
// because only one of them re-asked.
func PendingManifests() []string {
	registryMu.RLock()
	defer registryMu.RUnlock()
	var pending []string
	for target, a := range registry {
		if a.fetchedAt.IsZero() {
			pending = append(pending, target)
		}
	}
	sort.Strings(pending)
	return pending
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

// EntitlementSchema returns the fields a target declares it understands, for
// the callers that need the schema and nothing else.
//
// Narrower than handing out the Addon: mapping validation asks one question —
// "is this a field this target has" — and a caller holding the whole
// registration could answer a different one by accident. It reports
// ErrNotRegistered and ErrNoManifest distinctly, because a target the
// deployment does not run and one that has not answered yet send an operator to
// different places.
func EntitlementSchema(target string) ([]EntitlementField, error) {
	a, err := Get(target)
	if err != nil {
		return nil, err
	}
	m, err := a.Manifest()
	if err != nil {
		return nil, err
	}
	return m.EntitlementSchema, nil
}

// AcceptedManifest is a target's accepted manifest, for the pages that show
// its kind and fields; ErrNotRegistered or ErrNoManifest otherwise.
func AcceptedManifest(target string) (Manifest, error) {
	a, err := Get(target)
	if err != nil {
		return Manifest{}, err
	}
	return a.Manifest()
}

// ConnectionFor is how a member reaches a target, or nil.
//
// Narrower than handing out the manifest for the same reason EntitlementSchema
// is: the member's page asks one question, and a caller holding the whole
// registration could answer a different one by accident. Nil is a legitimate
// answer — a deployment that has not named a share host has not named one — so
// this reports no error for it, only for a target that is not registered.
func ConnectionFor(target string) (*Connection, error) {
	a, err := Get(target)
	if err != nil {
		return nil, err
	}
	m, err := a.Manifest()
	if err != nil {
		// No manifest yet is no connection yet, and it is not an error on a
		// page that renders fine without one.
		return nil, nil
	}
	return m.Connection, nil
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

// hasManifest answers the one question the retry below is about, and takes the
// read lock for exactly as long as that answer.
func hasManifest(target string) bool {
	registryMu.RLock()
	defer registryMu.RUnlock()
	a := registry[target]
	return a != nil && a.manifest != nil
}

// manifestRetry is one target's on-demand refresh, serialised.
//
// Its mutex is the single-flight: concurrent callers queue on it, and the one
// that gets through re-checks whether the manifest has since arrived, so a burst
// of refused calls costs ONE capability read between them rather than one each.
// `last` is when that read was attempted, which is what stops the burst after a
// failed one from buying another read per caller.
type manifestRetry struct {
	mu   sync.Mutex
	last time.Time
}

var (
	manifestRetryMu sync.Mutex
	manifestRetries = map[string]*manifestRetry{}
)

func retryStateFor(target string) *manifestRetry {
	manifestRetryMu.Lock()
	defer manifestRetryMu.Unlock()
	g := manifestRetries[target]
	if g == nil {
		g = &manifestRetry{}
		manifestRetries[target] = g
	}
	return g
}

// refreshForManifest reads a target's capabilities once when it has none, and
// reports whether it has one afterwards.
//
// `RefreshAll` runs at start-up and then on a period. An add-on that was still
// starting during that first pass leaves its target with no accepted manifest,
// and every call on it is refused until the next tick — while the add-on is up,
// healthy and answering. Met on a real deployment: `docker compose up -d` raced
// the add-on's start, and a release was refused with `no accepted manifest`
// against a target that was fine.
//
// Availability rather than correctness: nothing is written, and the refusal is
// honest about what the backend knows. What makes it worth closing is that the
// fault is invisible from the target's own side, so the machine an operator
// inspects is the one that is working.
//
// Deliberately narrow, and each limit is load-bearing:
//
//   - ONLY for a missing manifest. A manifest that was read and then refused —
//     contract skew, an operation policy does not allow — is a decision, and
//     re-reading it would turn a refusal an operator must act on into a loop.
//     That is why this is reached from the `a.manifest == nil` path alone.
//   - ONCE per call, never a loop. A target that genuinely has no manifest must
//     still refuse quickly.
//   - Single-flighted per target, so the retry does not become the thing that
//     keeps a starting add-on down.
//   - Rate-limited by its own cooldown, so calls arriving after a failed attempt
//     refuse immediately rather than each buying another read.
//
// Compose ordering narrows the window this closes; it cannot close it. Nothing
// `depends_on` can express promises the add-on has SERVED a manifest by the time
// the backend asks for one.
func refreshForManifest(target string) bool {
	g := retryStateFor(target)
	g.mu.Lock()
	defer g.mu.Unlock()

	// Re-checked inside the lock: whoever held it before this caller may have
	// been the one read that succeeded.
	if hasManifest(target) {
		return true
	}
	if !g.last.IsZero() && timeNow().Sub(g.last) < manifestRetryCooldown {
		return false
	}
	g.last = timeNow()

	// Its own bounded context, and not the caller's. A caller that has already
	// spent most of its budget must not turn this into a read that gives up
	// before the add-on can answer, leaving the next caller to pay for it again.
	ctx, cancel := context.WithTimeout(context.Background(), refreshTimeout)
	defer cancel()
	if err := Refresh(ctx, target); err != nil {
		log.Printf("[ADDON] on-demand manifest read for %s: %v", target, err)
		return false
	}
	return hasManifest(target)
}

// rereadManifest reads the manifest again after a shape refusal, single-flighted
// and on manifestRetry's cooldown, so a burst of refused calls costs one read.
// A manifest the backend refuses (a contract-1 add-on after a rollback) leaves
// the last accepted one in place and the target refusing calls as unreached:
// queued work waits, nothing becomes target_refused.
func (a *Addon) rereadManifest() {
	g := retryStateFor(a.Registration.Target)
	g.mu.Lock()
	defer g.mu.Unlock()
	if !g.last.IsZero() && timeNow().Sub(g.last) < manifestRetryCooldown {
		return
	}
	g.last = timeNow()
	ctx, cancel := context.WithTimeout(context.Background(), refreshTimeout)
	defer cancel()
	if err := Refresh(ctx, a.Registration.Target); err != nil {
		log.Printf("[ADDON] re-reading %s's manifest after a shape refusal: %v", a.Registration.Target, err)
	}
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
	// The one on-demand manifest read. Outside the lock, because the read it
	// may perform takes the write lock.
	if !hasManifest(target) && !refreshForManifest(target) {
		return EffectiveOperation{}, fmt.Errorf("%w: %s", ErrNoManifest, target)
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
// Decoding is TOLERANT (contract 2, design §2): the manifest is the one message
// meant to describe change, so a key this backend does not know is kept aside
// in Unknown, quoted and capped, and never refused — a strict decode made every
// new manifest key a breaking change. What the manifest MEANS is held at
// Refresh (validate, acceptRoles); the conformance kit holds its spelling.
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
	return decodeManifest(resp.Body)
}

// decodeManifest reads a manifest tolerantly and lists what it carries beyond
// what this backend knows.
func decodeManifest(body []byte) (Manifest, error) {
	dec := json.NewDecoder(bytes.NewReader(body))
	var raw map[string]any
	if err := dec.Decode(&raw); err != nil {
		return Manifest{}, fmt.Errorf("decode manifest: %w", err)
	}
	if err := dec.Decode(new(any)); err != io.EOF {
		return Manifest{}, errors.New("decode manifest: multiple JSON values in body")
	}
	var m Manifest
	if err := json.Unmarshal(body, &m); err != nil {
		return Manifest{}, fmt.Errorf("decode manifest: %w", err)
	}
	m.Unknown = unknownKeys("", raw, manifestKeys)
	return m, nil
}

// manifestKeys is every manifest key this backend knows, by object. A key
// listed with nil is known and not looked inside: metrics are feature stats,
// which this backend does not read yet.
var manifestKeys = keySpec{
	"contract_version": nil, "product": nil, "product_version": nil, "kind": nil, "features": nil,
	"entitlement_schema": {"name": nil, "role": nil, "label": nil, "unit": nil, "combine": nil, "unlimited": nil, "per_person": nil},
	"operations":         {"id": nil, "scope": nil, "confirm": nil, "secret_params": nil, "available": nil, "unavailable_reason": nil, "params": nil},
	"connection":         {"protocol": nil, "host": nil},
	"limits":             {"subjects_per_plan": nil, "fresh_read_cooldown": nil, "stats_buffer": nil},
	"metrics":            nil,
}

type keySpec map[string]keySpec

// alsoSaysCap bounds one shown value: the least trusted component chooses it.
const alsoSaysCap = 200

// unknownKeys walks v against spec and returns each key spec does not name, as
// `path: value`, the value as JSON (quoted and escaped) and capped. An x- key
// is vendor data and listed the same way: shown, never used.
func unknownKeys(path string, v any, spec keySpec) []string {
	var out []string
	switch x := v.(type) {
	case map[string]any:
		for _, k := range slices.Sorted(maps.Keys(x)) {
			p := k
			if path != "" {
				p = path + "." + k
			}
			inner, known := spec[k]
			if !known {
				raw, _ := json.Marshal(x[k])
				shown := string(raw)
				if len(shown) > alsoSaysCap {
					shown = shown[:alsoSaysCap] + "…"
				}
				out = append(out, strconv.Quote(p)+": "+shown)
				continue
			}
			if inner != nil {
				out = append(out, unknownKeys(p, x[k], inner)...)
			}
		}
	case []any:
		for i, e := range x {
			out = append(out, unknownKeys(fmt.Sprintf("%s[%d]", path, i), e, spec)...)
		}
	}
	return out
}
