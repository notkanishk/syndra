package addons

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"sort"
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

	// Expiry of the material, so it can be surfaced before it fails rather than
	// discovered as a handshake error during an incident. Zero in signed mode:
	// an HMAC key has no expiry, and inventing one would be a lie in a field an
	// operator is meant to trust.
	clientCertNotAfter time.Time
	caNotAfter         time.Time

	// sources is path -> modification time, the rotation signal.
	sources map[string]time.Time
}

var (
	credMu    sync.Mutex
	credCache = map[string]*credential{}
)

// credentialWarnDays is how far ahead an expiring transport certificate is
// surfaced. Thirty days is enough for a rotation to be scheduled rather than
// scrambled, and short enough that the warning still means something when it
// appears.
const credentialWarnDays = 30

// materialPaths lists the files this registration is built from, in a stable
// order.
func (r Registration) materialPaths() []string {
	var out []string
	for _, p := range []string{r.ClientCertPath, r.ClientKeyPath, r.CAPath, r.SigningKeyPath} {
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

	switch c.mode {
	case "mtls":
		pair, err := tls.LoadX509KeyPair(r.ClientCertPath, r.ClientKeyPath)
		if err != nil {
			return nil, fmt.Errorf("client keypair: %w", err)
		}
		leaf, err := x509.ParseCertificate(pair.Certificate[0])
		if err != nil {
			return nil, fmt.Errorf("client certificate: %w", err)
		}
		c.clientCertNotAfter = leaf.NotAfter

		tlsCfg, err := serverTrust(r.CAPath)
		if err != nil {
			return nil, err
		}
		tlsCfg.Certificates = []tls.Certificate{pair}
		c.caNotAfter = tlsCfg.notAfter
		c.client = newAddonClient(tlsCfg.Config)

	case "signed":
		key, err := readSigningKey(r.SigningKeyPath)
		if err != nil {
			return nil, err
		}
		c.signingKey = key
		// Signed mode is still TLS. The signature authenticates the request and
		// does nothing for the response or for confidentiality: without TLS the
		// secret-bearing body is readable on the wire and a forged 2xx is
		// recorded as a completed mutation. Where a private CA is configured it
		// is the anchor; otherwise the system roots are, which is weaker but is
		// a real check rather than none.
		tlsCfg, err := serverTrust(r.CAPath)
		if err != nil {
			return nil, err
		}
		c.caNotAfter = tlsCfg.notAfter
		c.client = newAddonClient(tlsCfg.Config)

	default:
		// Init refuses to register an add-on with no transport mode, so this is
		// unreachable through the normal path. It is still an error rather than
		// a plain client: the one thing this package must never do is call an
		// add-on unauthenticated because a check moved somewhere else.
		return nil, fmt.Errorf("addon %s: no transport authentication configured", r.Target)
	}
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

// trustConfig is a TLS client configuration plus the expiry of whatever anchors
// it, so the caller does not have to re-read and re-parse the CA to report it.
type trustConfig struct {
	*tls.Config
	notAfter time.Time
}

// serverTrust builds the client TLS configuration used to verify the ADD-ON.
// Shared by both modes, because both need it: mutual TLS adds a client
// certificate on top, it does not replace this.
//
// An empty caPath falls back to the system roots. That is the honest weaker
// option for signed mode, and it is never reachable for mutual TLS, where
// AuthMode requires the CA before it will call the mode mTLS at all.
func serverTrust(caPath string) (trustConfig, error) {
	cfg := trustConfig{Config: &tls.Config{
		// Both ends of this connection are ours and ship together. There is no
		// legacy peer to accommodate, so there is no reason to negotiate
		// anything older.
		MinVersion: tls.VersionTLS13,
	}}
	if caPath == "" {
		return cfg, nil
	}
	caPEM, err := os.ReadFile(caPath)
	if err != nil {
		return cfg, fmt.Errorf("private CA: %w", err)
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(caPEM) {
		return cfg, errors.New("private CA: no certificate found in PEM")
	}
	cfg.RootCAs = pool
	cfg.notAfter = earliestNotAfter(caPEM)
	return cfg, nil
}

// readSigningKey reads the HMAC key, rejecting an empty one.
//
// Trailing whitespace is trimmed. A secret mounted from a file almost always
// arrives with a trailing newline the operator never typed, and a key that
// differs from the add-on's by one byte fails as "no matching signature" —
// indistinguishable from an attack, and days of debugging.
func readSigningKey(path string) ([]byte, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("signing key: %w", err)
	}
	key := trimTrailingSpace(raw)
	if len(key) == 0 {
		return nil, errors.New("signing key: file is empty")
	}
	return key, nil
}

func trimTrailingSpace(b []byte) []byte {
	end := len(b)
	for end > 0 {
		switch b[end-1] {
		case '\n', '\r', '\t', ' ':
			end--
		default:
			return b[:end]
		}
	}
	return b[:0]
}

// earliestNotAfter returns the soonest expiry among the certificates in a PEM
// bundle. A chain is only valid until its first link expires.
func earliestNotAfter(pemBytes []byte) time.Time {
	var earliest time.Time
	rest := pemBytes
	for {
		var block *pem.Block
		block, rest = pem.Decode(rest)
		if block == nil {
			return earliest
		}
		if block.Type != "CERTIFICATE" {
			continue
		}
		cert, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			continue
		}
		if earliest.IsZero() || cert.NotAfter.Before(earliest) {
			earliest = cert.NotAfter
		}
	}
}

// TransportCredential is the operator-facing state of one add-on's transport
// material.
type TransportCredential struct {
	Target   string `json:"target"`
	AuthMode string `json:"auth_mode"`
	// ExpiresAt is the soonest expiry among the material — client certificate
	// and private CA both. A current client certificate presented against an
	// expired CA fails exactly as hard as an expired one, so reporting only the
	// certificate's own date would be a reassurance the connection cannot keep.
	ExpiresAt     *time.Time `json:"expires_at,omitempty"`
	DaysRemaining *int       `json:"days_remaining,omitempty"`
	// Status: ok | warn | expired | unknown. unknown covers signed mode, where
	// there is no expiry to report, and material that could not be loaded.
	Status string `json:"status"`
	Error  string `json:"error,omitempty"`
}

// TransportCredentials reports every registered add-on's transport material and
// how long it has left, so an expiring certificate is an item on a surface
// rather than an outage.
func TransportCredentials() []TransportCredential {
	now := timeNow().UTC()
	regs := Registered()
	out := make([]TransportCredential, 0, len(regs))
	for _, r := range regs {
		tc := TransportCredential{Target: r.Target, AuthMode: r.AuthMode(), Status: "unknown"}
		c, err := credentialFor(r)
		if err != nil {
			tc.Error = err.Error()
			out = append(out, tc)
			continue
		}
		expiry := soonest(c.clientCertNotAfter, c.caNotAfter)
		if expiry.IsZero() {
			// Signed mode. No expiry exists; saying "ok" would imply one was
			// checked.
			out = append(out, tc)
			continue
		}
		days := int(expiry.Sub(now).Hours() / 24)
		tc.ExpiresAt = &expiry
		tc.DaysRemaining = &days
		switch {
		case !expiry.After(now):
			tc.Status = "expired"
		case days < credentialWarnDays:
			tc.Status = "warn"
		default:
			tc.Status = "ok"
		}
		out = append(out, tc)
	}
	return out
}

func soonest(a, b time.Time) time.Time {
	switch {
	case a.IsZero():
		return b
	case b.IsZero():
		return a
	case b.Before(a):
		return b
	default:
		return a
	}
}

// dialFailed reports whether an error means the request never reached the
// add-on. Used to separate "nothing happened" from "something may have".
func dialFailed(err error) bool {
	var op *net.OpError
	return errors.As(err, &op) && op.Op == "dial"
}
