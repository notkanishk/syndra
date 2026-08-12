// Command truenas is the Syndra add-on for TrueNAS SCALE.
//
// It is a target adapter behind Syndra's policy engine, not an autonomous
// controller: it holds no mapping policy, no desired state of its own, and no
// reconcile loop. Syndra decides who and what; this decides how — username
// derivation, group and ACL semantics, the WebSocket session and its rate
// limit, version probing.
//
// It runs on the internal Compose network with no published host port, because
// it holds the NAS API key and putting that in the same process as the Zitadel
// service account would mean one memory disclosure exposes identity, storage
// and physical access together.
package main

import (
	"context"
	"crypto/ed25519"
	"crypto/tls"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// shutdownTimeout is how long an in-flight mutation has to settle after a
// termination signal, before the listener is torn down regardless.
//
// A named constant rather than a literal because the deployment has to agree
// with it and nothing else makes the two agree. Compose's `stop_grace_period`
// on this service must EXCEED it, so that the process always reaches its own
// deadline first — Docker's default is 10s and then SIGKILL, which for the
// whole life of this add-on has been cutting this drain in half. A truncated
// drain is invisible from outside: the process is gone either way, and the
// mutation that was settling leaves the same silence as one that never began.
//
// `TestTheDeploymentAllowsTheShutdownDrainToFinish` reads this value and the
// Compose one and fails if the relationship inverts. Changing this number
// without changing the deployment is the mistake it exists to catch.
const shutdownTimeout = 20 * time.Second

type config struct {
	listen string

	// target is this add-on's name in the deployment, and it MUST match the
	// entry the backend carries in ADDON_TARGETS. It is the HKDF salt, so a
	// disagreement about it produces two different keys from one secret — and
	// the symptom is a pin failure that looks exactly like a wrong secret,
	// which is why the handshake error names this as a cause.
	target string

	// secret is the whole transport configuration: one value per target, from
	// which both keys are derived (derive.go). Nothing else is configured, and
	// there is no second mode to choose between.
	//
	// This add-on still always serves HTTPS. There is no plaintext mode and no
	// localhost exemption, but the reason is narrower than it used to be: the
	// body carries declared secret_params — a member's plaintext credential and,
	// on the purge path, an elevated target credential — and a request signature
	// establishes neither the confidentiality of that body nor the authenticity
	// of the response.
	secret []byte

	// signingKey is derived from secret, not configured. Kept as a field
	// because the authenticator reads it and the derivation happens once.
	signingKey []byte

	nasURL string
	// shareHost is the name a MEMBER types into a file manager, which is
	// frequently not the middleware endpoint above: the API may be reached on an
	// internal address while SMB is published on another. Optional — unset, the
	// manifest carries no connection block and the member's page omits the
	// instructions rather than printing a host that does not answer.
	shareHost  string
	nasAPIKey  string
	nasVerify  bool
	keyExpiry  time.Time
	supported  []string
	statePath  string
	logDir     string
	logMaxSize int64
	logKeep    int
	lifecycle  string
}

func loadConfig() (config, error) {
	c := config{
		listen:     envOr("LISTEN_ADDR", ":8443"),
		target:     strings.TrimSpace(os.Getenv("ADDON_TARGET")),
		nasURL:     os.Getenv("TRUENAS_URL"),
		shareHost:  strings.TrimSpace(os.Getenv("TRUENAS_SHARE_HOST")),
		statePath:  envOr("STATE_PATH", "/var/lib/syndra-truenas/state.db"),
		logDir:     envOr("MUTATION_LOG_DIR", "/var/lib/syndra-truenas"),
		lifecycle:  envOr("LIFECYCLE_STATE", LifecycleActive),
		supported:  strings.Split(envOr("TRUENAS_SUPPORTED_MAJORS", "25.04,25.10,26.04"), ","),
		logMaxSize: int64(envInt("MUTATION_LOG_MAX_BYTES", defaultLogMaxSize)),
		logKeep:    envInt("MUTATION_LOG_KEEP", defaultLogKeep),
		// Verifying the NAS certificate is the default and turning it off is a
		// deliberate act. TrueNAS ships a self-signed certificate, so a lab
		// deployment will need this — which is exactly why it must be an
		// explicit `false` rather than something that quietly defaults off.
		nasVerify: envBool("TRUENAS_VERIFY_TLS", true),
	}
	apiKey, err := secretValue("TRUENAS_API_KEY")
	if err != nil {
		return config{}, err
	}
	c.nasAPIKey = apiKey

	secret, err := secretValue("ADDON_SECRET")
	if err != nil {
		return config{}, err
	}
	c.secret = []byte(secret)
	if raw := os.Getenv("TRUENAS_API_KEY_EXPIRES_AT"); raw != "" {
		t, err := time.Parse(time.RFC3339, raw)
		if err != nil {
			return config{}, fmt.Errorf("TRUENAS_API_KEY_EXPIRES_AT must be RFC3339: %w", err)
		}
		c.keyExpiry = t
	}

	switch {
	case c.nasURL == "":
		return config{}, errors.New("TRUENAS_URL is required")
	case c.nasAPIKey == "":
		return config{}, errors.New("TRUENAS_API_KEY is required")
	case c.target == "":
		// The HKDF salt. Without it the derivation is not merely unconfigured,
		// it is a different derivation — so this must be an error rather than a
		// default, which would silently produce keys the backend cannot match.
		return config{}, errors.New("ADDON_TARGET is required: it is the derivation salt and must match the backend's ADDON_TARGETS entry")
	case len(c.secret) == 0:
		// Fail closed, for the reason the two-mode check used to carry: a
		// component holding the target credential must not answer an
		// unauthenticated caller. There is one mode now, so there is no
		// ambiguity left to refuse — only its absence.
		return config{}, errors.New("ADDON_SECRET (or ADDON_SECRET_FILE) is required: this add-on does not answer unauthenticated callers")
	}

	// Derived once, here, so a failure surfaces at startup rather than on the
	// first request.
	if c.signingKey, err = deriveHMACKey(c.secret, c.target); err != nil {
		return config{}, err
	}
	for i := range c.supported {
		c.supported[i] = strings.TrimSpace(c.supported[i])
	}
	return c, nil
}

// main does nothing but exit codes. Everything else is in `run`, because
// `log.Fatalf` calls os.Exit and os.Exit does not run deferred functions — so
// every startup failure after the store was opened closed neither the bbolt
// file nor the mutation log.
func main() {
	log.SetFlags(log.LstdFlags | log.LUTC)
	if err := run(); err != nil {
		log.Fatalf("[STARTUP] %v", err)
	}
}

func run() error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}

	store, err := OpenStore(cfg.statePath)
	if err != nil {
		return err
	}
	defer store.Close()

	mlog, err := OpenMutationLog(cfg.logDir, cfg.logMaxSize, cfg.logKeep)
	if err != nil {
		// Fatal on purpose. The mutation log is the record that survives losing
		// Syndra's audit tables; starting without one would mean mutations with
		// no independent trace, which is the guarantee the design states.
		return fmt.Errorf("mutation log: %w", err)
	}
	defer mlog.Close()

	nas := newNAS(dialTrueNAS(cfg.nasURL, cfg.nasAPIKey, cfg.nasVerify), cfg.supported)
	// Probed at startup so `/capabilities` can answer with a version straight
	// away, and non-fatally: an add-on that refused to start because the NAS
	// was off would be unreachable in exactly the situation an operator is
	// trying to diagnose. Nothing depends on this succeeding — every call
	// re-probes a fresh connection — which is what stops an add-on that started
	// first from refusing mutations for the rest of its life.
	if err := nas.Probe(); err != nil {
		log.Printf("[STARTUP] could not read the target version yet: %v", err)
	} else {
		log.Printf("[STARTUP] target version %s", nas.Version())
		if ok, why := nas.MajorSupported(); !ok {
			log.Printf("[STARTUP] WARNING: %s — mutations will be refused, reads continue", why)
		}
	}

	life := newLifecycle(cfg.lifecycle)
	if state, note := life.State(); state != LifecycleActive {
		log.Printf("[STARTUP] lifecycle=%s (%s)", state, note)
	}

	srv := &server{
		auth:      &authenticator{signingKey: cfg.signingKey, now: time.Now},
		nas:       nas,
		store:     store,
		log:       mlog,
		life:      life,
		keyExpiry: cfg.keyExpiry,
		product:   "truenas_scale",
		// Nil unless the deployment named a share host. The manifest omits the
		// block, and the member's page omits the instructions with it.
		connection: shareConnection(cfg.shareHost),
		// A fresh session per purge, under the injected key, closed immediately
		// after. Never the shared one: an elevated credential must not outlive
		// the single call it was injected for.
		elevated: func(apiKey string) (rpc, error) {
			return dialTrueNAS(cfg.nasURL, apiKey, cfg.nasVerify)()
		},
	}

	tlsConf, err := serverTLS(cfg)
	if err != nil {
		return err
	}
	httpSrv := &http.Server{
		Addr:      cfg.listen,
		Handler:   srv.routes(),
		TLSConfig: tlsConf,
		// All four, not just the header one. The body size is bounded before it
		// is read, but nothing bounded the TIME it took to arrive: a slow body
		// pins a handler goroutine and, on a mutating route, one of the
		// lifecycle's in-flight slots — which is how a service that drains
		// before shutdown never drains.
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		// Longer than a call to the NAS, which is what a handler waits on.
		WriteTimeout: 90 * time.Second,
		IdleTimeout:  2 * time.Minute,
		// The container's liveness probe is `nc -z 127.0.0.1 8443` — a TCP
		// connect that closes without ever sending a ClientHello, deliberately,
		// because every route here is authenticated and a real probe would need
		// a credential this container has no business holding.
		//
		// net/http logs each one as `http: TLS handshake error from
		// 127.0.0.1:NNNNN: EOF`. Every 30 seconds, forever, in the log an
		// operator reads during a bring-up — where "TLS handshake error" is
		// precisely the phrase that sends them to look at the transport they
		// just configured. It is not information; it is the health check
		// working.
		ErrorLog: log.New(probeNoiseFilter{}, "", 0),
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// The retention the store documents, actually applied. Nothing called the
	// sweep outside a test, so the idempotency bucket grew for the life of the
	// deployment — a 30-day window that never ended is a bucket, not a window.
	go sweepIdempotency(ctx, store)

	go func() {
		// The public key, logged at startup. An operator diagnosing a pin
		// failure needs to compare what this add-on serves against what the
		// backend expects, and without this the only way to see it is to open
		// a TLS connection to a service that is refusing them.
		log.Printf("[STARTUP] listening on %s target=%s key=%x",
			cfg.listen, cfg.target, servedPublicKey(httpSrv.TLSConfig))
		// Empty paths: the certificate is already in TLSConfig, derived and
		// self-signed in memory. There is no file to serve it from.
		if err := httpSrv.ListenAndServeTLS("", ""); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("[SERVE] %v", err)
		}
	}()

	<-ctx.Done()
	log.Println("[SHUTDOWN] draining")
	// Draining before the deadline, so an in-flight mutation settles rather
	// than being abandoned half-applied with no record of how far it got.
	_ = life.Set(LifecycleDraining, "shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	if err := httpSrv.Shutdown(shutdownCtx); err != nil {
		log.Printf("[SHUTDOWN] %v", err)
	}
	nas.drop()
	log.Println("[SHUTDOWN] stopped")
	return nil
}

// sweepIdempotency applies the store's stated retention.
//
// Daily, and once at startup: the entries it removes are weeks old by
// definition, so the cadence only has to be shorter than "never".
func sweepIdempotency(ctx context.Context, store *Store) {
	for {
		if removed, err := store.SweepIdempotency(); err != nil {
			log.Printf("[STORE] idempotency sweep failed: %v", err)
		} else if removed > 0 {
			log.Printf("[STORE] idempotency sweep removed %d expired result(s)", removed)
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(24 * time.Hour):
		}
	}
}

// serverTLS builds the listener's configuration around the derived key.
//
// TLS 1.3 minimum: both ends are ours and ship together, so there is no legacy
// peer to accommodate and every reason not to leave a downgrade available.
//
// No ClientCAs and no ClientAuth: the caller is authenticated by the signature
// over its request, not by a certificate. The TLS here is carrying
// confidentiality for a body that contains declared secret_params, and the
// backend's half of the mutual authentication is its pin on the key derived
// below — which it can only reproduce by holding the same secret.
func serverTLS(cfg config) (*tls.Config, error) {
	priv, err := deriveTLSKey(cfg.secret, cfg.target)
	if err != nil {
		return nil, err
	}
	cert, err := selfSignedCert(priv, cfg.target)
	if err != nil {
		return nil, err
	}
	return &tls.Config{
		MinVersion:   tls.VersionTLS13,
		Certificates: []tls.Certificate{cert},
	}, nil
}

// probeNoiseFilter drops the liveness probe's handshake errors and passes
// everything else through untouched.
//
// Narrow on purpose, and narrow in the two ways that matter: only EOF (a peer
// that sent nothing), and only from loopback. A real backend connection arrives
// over the Compose network and never from 127.0.0.1, so no failure that an
// operator needs is silenced here — including a genuine handshake failure from
// the backend, which is the one this add-on most needs to report.
type probeNoiseFilter struct{}

func (probeNoiseFilter) Write(p []byte) (int, error) {
	if isProbeNoise(string(p)) {
		return len(p), nil
	}
	return os.Stderr.Write(p)
}

// isProbeNoise is the whole of the decision, in one place because the test has
// to assert THIS and not a copy of it. A predicate restated in a test agrees
// with itself while the code drifts, which is the defect this branch keeps
// paying for.
func isProbeNoise(line string) bool {
	return strings.Contains(line, "TLS handshake error from 127.0.0.1:") &&
		strings.HasSuffix(strings.TrimSpace(line), "EOF")
}

// servedPublicKey is for the startup log only: the key an operator compares
// against the backend's expectation when a pin fails.
func servedPublicKey(conf *tls.Config) []byte {
	if conf == nil || len(conf.Certificates) == 0 {
		return nil
	}
	pub, ok := conf.Certificates[0].PrivateKey.(ed25519.PrivateKey)
	if !ok {
		return nil
	}
	return pub.Public().(ed25519.PublicKey)
}

// secretValue reads a secret from the file `<NAME>_FILE` points at, or from
// `<NAME>` itself.
//
// The file is preferred, and the backend's half of every one of these is
// already a path. Two reasons, and the second is the one that bit: an
// environment value is readable from `docker inspect` and /proc/1/environ in
// the container the Dockerfile calls the least trusted in the deployment, while
// the TLS material for that same container arrives as a mounted file — and the
// two ends of the signing key disagreed about its FORM, so one HMACed the
// file's contents and the other the literal path string. The only symptom was
// "no matching signature".
//
// Trimmed, because a mounted secret almost always carries a trailing newline
// and a one-byte difference surfaces as that same failure.
func secretValue(name string) (string, error) {
	if path := strings.TrimSpace(os.Getenv(name + "_FILE")); path != "" {
		raw, err := os.ReadFile(path)
		if err != nil {
			// Naming the path, matching the backend's resolveSecret: "no secret
			// configured" and "the mount did not land" are the same symptom and
			// different fixes.
			return "", fmt.Errorf("read %s_FILE (%s): %w", name, path, err)
		}
		value := strings.TrimSpace(string(raw))
		if value == "" {
			// An empty file is a mount that did not land. Silently falling back
			// to the environment would start the add-on in whichever mode the
			// other variable happened to configure.
			return "", fmt.Errorf("%s_FILE (%s) is empty", name, path)
		}
		return value, nil
	}
	return strings.TrimSpace(os.Getenv(name)), nil
}

func envOr(name, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(name)); v != "" {
		return v
	}
	return fallback
}

func envInt(name string, fallback int) int {
	v := strings.TrimSpace(os.Getenv(name))
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil || n <= 0 {
		log.Printf("[STARTUP] invalid %s=%q, using %d", name, v, fallback)
		return fallback
	}
	return n
}

func envBool(name string, fallback bool) bool {
	v := strings.TrimSpace(os.Getenv(name))
	if v == "" {
		return fallback
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		log.Printf("[STARTUP] invalid %s=%q, using %t", name, v, fallback)
		return fallback
	}
	return b
}

// shareConnection is the member-facing address, or nothing.
//
// Never derived from TRUENAS_URL. The middleware endpoint and the SMB host are
// frequently different names for the same machine and occasionally different
// machines, and a guess here reaches a member as a path that does not work —
// which teaches them to distrust the whole page, starting with the parts that
// were right.
func shareConnection(host string) *Connection {
	if host == "" {
		return nil
	}
	return &Connection{Protocol: "smb", Host: host}
}
