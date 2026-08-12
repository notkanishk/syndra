package addons

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"sort"
	"strings"
	"sync"
	"time"
)

// credential is the transport material for one add-on, already parsed into the
// client that will carry it. Built from the registration's file paths and
// rebuilt when any of those files changes on disk.
type credential struct {
	// client is immutable once built. Rotation replaces the credential in the
	// cache; a call already holding this pointer finishes on the material it
	// started with rather than losing its connection mid-operation, which for a
	// secret-bearing dispatch is the difference between a completed operation
	// and an indeterminate one.
	client     *http.Client
	mode       string
	signingKey []byte

	// sources is path -> modification time, the rotation signal.
	sources map[string]time.Time
}

var (
	credMu    sync.Mutex
	credCache = map[string]*credential{}
)

// materialPaths lists the files this registration is built from, in a stable
// order.
func (r Registration) materialPaths() []string {
	var out []string
	for _, p := range []string{r.SecretPath} {
		if p != "" {
			out = append(out, p)
		}
	}
	sort.Strings(out)
	return out
}

// credentialFor returns the transport credential for a registration, rebuilding
// it when the material on disk has changed.
//
// Rotation is therefore a file replacement, not a restart: the next call after
// an operator swaps the certificate picks it up. Calls already in flight keep
// the client they started with.
//
// A reload that fails does NOT fail the call. A certificate half-written by a
// rotation script, or briefly absent between unlink and rename, would otherwise
// turn every dispatch during that window into an error — so the last good
// credential keeps serving and the failure is logged. This is the same rule the
// manifest refresh follows: a bad new answer never destroys a good old one.
func credentialFor(r Registration) (*credential, error) {
	credMu.Lock()
	defer credMu.Unlock()

	cached := credCache[r.Target]
	stamps, err := stampMaterial(r.materialPaths())
	if err != nil {
		if cached != nil {
			log.Printf("[ADDON] %s: transport material unreadable (%v); continuing on the material loaded at startup", r.Target, err)
			return cached, nil
		}
		return nil, err
	}
	if cached != nil && sameStamps(cached.sources, stamps) {
		return cached, nil
	}

	c, err := loadCredential(r, stamps)
	if err != nil {
		if cached != nil {
			log.Printf("[ADDON] %s: transport material changed but failed to load (%v); continuing on the previous material", r.Target, err)
			return cached, nil
		}
		return nil, err
	}
	if cached != nil {
		log.Printf("[ADDON] %s: transport material reloaded (auth=%s)", r.Target, c.mode)
	}
	credCache[r.Target] = c
	return c, nil
}

// purgeCredentialsExcept drops cached material for targets the deployment no
// longer configures. Loaded private keys are held in this process's memory; a
// target that was unregistered should not leave its key resident until restart.
func purgeCredentialsExcept(keep map[string]*Addon) {
	credMu.Lock()
	defer credMu.Unlock()
	for target := range credCache {
		if _, ok := keep[target]; !ok {
			delete(credCache, target)
		}
	}
}

func stampMaterial(paths []string) (map[string]time.Time, error) {
	out := make(map[string]time.Time, len(paths))
	for _, p := range paths {
		fi, err := os.Stat(p)
		if err != nil {
			return nil, fmt.Errorf("transport material %s: %w", p, err)
		}
		out[p] = fi.ModTime()
	}
	return out, nil
}

func sameStamps(a, b map[string]time.Time) bool {
	if len(a) != len(b) {
		return false
	}
	for p, t := range a {
		if !b[p].Equal(t) {
			return false
		}
	}
	return true
}

func loadCredential(r Registration, stamps map[string]time.Time) (*credential, error) {
	c := &credential{mode: r.AuthMode(), sources: stamps}
	if len(r.Secret) == 0 {
		// Init refuses to register a target with no secret, so this is
		// unreachable through the normal path. It is still an error rather than
		// a plain client: the one thing this package must never do is call an
		// add-on unauthenticated because a check moved somewhere else.
		return nil, fmt.Errorf("addon %s: no transport secret configured", r.Target)
	}

	// The secret may have been replaced on disk since registration — that is
	// what the rotation watch above detects — so it is re-read here rather than
	// taken from the Registration captured at startup.
	secret := r.Secret
	if r.SecretPath != "" {
		raw, err := os.ReadFile(r.SecretPath)
		if err != nil {
			return nil, fmt.Errorf("addon %s: read secret: %w", r.Target, err)
		}
		trimmed := strings.TrimSpace(string(raw))
		if trimmed == "" {
			return nil, fmt.Errorf("addon %s: secret file %s is empty", r.Target, r.SecretPath)
		}
		secret = []byte(trimmed)
	}

	key, err := deriveHMACKey(secret, r.Target)
	if err != nil {
		return nil, fmt.Errorf("addon %s: %w", r.Target, err)
	}
	c.signingKey = key

	pub, err := deriveTLSPublicKey(secret, r.Target)
	if err != nil {
		return nil, fmt.Errorf("addon %s: %w", r.Target, err)
	}
	// No expiry to record. The add-on's certificate is minted in memory at
	// every boot around a key that never changes unless the secret does, so
	// there is nothing to renew and nothing to warn about. A health field
	// reporting an expiry here would be reporting one that does not exist.
	c.client = newAddonClient(pinnedTLSConfig(pub, r.Target))
	return c, nil
}

// newAddonClient builds the HTTP client for one add-on. Every add-on client is
// built here so that no future mode can be added without the redirect refusal.
//
// Redirects are NOT followed, and that is a security property rather than
// tidiness. Go's default follows up to ten of them and re-sends the body on 307
// and 308, so an add-on answering a mutating call with a redirect would have the
// backend replay the whole secret-bearing POST to a host of its choosing. Go
// strips Authorization and Cookie across hosts; it does not strip a custom
// header, so the request signature travels with the replayed body and
// authenticates it to the new destination. In signed mode that destination may
// verify against the system roots, meaning any publicly issued certificate is
// enough to receive a member's password. Nor does Go block an https-to-http
// downgrade on the way.
//
// The final 2xx would then be classified as success, hiding both the redirect
// and the fact that the registered target may never have acted at all.
//
// ErrUseLastResponse stops at the 3xx and surfaces it, so nothing is ever sent
// to a second host. The base URL is the only authority the backend will talk to,
// which is what registering an authority was supposed to mean.
func newAddonClient(tlsCfg *tls.Config) *http.Client {
	return &http.Client{
		Transport: &http.Transport{TLSClientConfig: tlsCfg},
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}


// TransportCredential is the operator-facing state of one add-on's transport
// material.
//
// No expiry is reported, and that is the honest shape rather than an omission.
// The add-on's certificate is minted in memory at every boot around a key
// derived from the deployment secret, so nothing expires and nothing needs
// renewing. The fields that used to carry a date are gone rather than reporting
// "unknown" forever: a status field that can only ever say it does not know
// reads as a probe that is failing, which is worse than the absence of one.
//
// What can still fail is loading the secret — a mount that did not land, an
// empty file — and that is what an operator needs to see here.
type TransportCredential struct {
	Target   string `json:"target"`
	AuthMode string `json:"auth_mode"`
	// Status: ok | error.
	Status string `json:"status"`
	Error  string `json:"error,omitempty"`
}

// TransportCredentials reports every registered add-on's transport state.
func TransportCredentials() []TransportCredential {
	regs := Registered()
	out := make([]TransportCredential, 0, len(regs))
	for _, r := range regs {
		tc := TransportCredential{Target: r.Target, AuthMode: r.AuthMode(), Status: "ok"}
		if _, err := credentialFor(r); err != nil {
			tc.Status = "error"
			tc.Error = err.Error()
		}
		out = append(out, tc)
	}
	return out
}

// dialFailed reports whether an error means the request never reached the
// add-on. Used to separate "nothing happened" from "something may have".
//
// Three families, all of them strictly before the first byte of the request is
// written:
//
//   - the dial itself,
//   - the TLS handshake, which is the most predictable failure this transport
//     has. Certificate expiry is a certainty, not a hypothesis — the credential
//     surface exists to warn about it — and a handshake that fails delivered
//     nothing. Classified as indeterminate it became the one outcome that is
//     never retried and never counted, so an expired certificate turned the
//     whole queue into rows nobody could resolve.
//   - the alert the peer sends when it refuses OUR certificate, which is the
//     other half of a rotation gone wrong.
//
// Every one of them is typed, deliberately. A general timeout is NOT here: a
// deadline that expired mid-response means the request was delivered, and the
// pessimistic reading is the only safe one for those.
func dialFailed(err error) bool {
	var op *net.OpError
	if errors.As(err, &op) && op.Op == "dial" {
		return true
	}
	var verify *tls.CertificateVerificationError
	var alert tls.AlertError
	var record tls.RecordHeaderError
	// The pin. It fails inside the handshake, so nothing was written — the same
	// family as the certificate errors below, and it must classify the same way
	// or a misconfigured deployment manufactures an indeterminate operation per
	// call, each one claiming a mutation may have been applied.
	var pin pinMismatchError
	var authority x509.UnknownAuthorityError
	var hostname x509.HostnameError
	var certInvalid x509.CertificateInvalidError
	switch {
	case errors.As(err, &pin),
		errors.As(err, &verify), errors.As(err, &alert), errors.As(err, &record),
		errors.As(err, &authority), errors.As(err, &hostname), errors.As(err, &certInvalid):
		return true
	}
	// net/http's own handshake deadline, which has no typed form and is the one
	// timeout that certainly delivered nothing.
	return strings.Contains(err.Error(), "TLS handshake timeout")
}
