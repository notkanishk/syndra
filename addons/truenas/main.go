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
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
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

	// TLS: this add-on always serves HTTPS. There is no plaintext mode and no
	// localhost exemption — a client's transport settings are consulted only
	// where a handshake happens, so a plaintext listener means the backend's
	// certificate is never presented and its private CA never consulted, while
	// both ends report themselves mutually authenticated.
	certFile string
	keyFile  string
	caFile   string

	// signingKey is set only in signed mode, and the two modes are exclusive:
	// accepting either would mean the weaker one is always available.
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
		certFile:   os.Getenv("TLS_CERT_FILE"),
		keyFile:    os.Getenv("TLS_KEY_FILE"),
		caFile:     os.Getenv("TLS_CLIENT_CA_FILE"),
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

	key, err := secretValue("SIGNING_KEY")
	if err != nil {
		return config{}, err
	}
	if key != "" {
		c.signingKey = []byte(key)
	}
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
	case c.certFile == "" || c.keyFile == "":
		// No plaintext fallback. An add-on that came up serving HTTP because
		// its certificate was missing would authenticate nothing while
		// reporting itself available.
		return config{}, errors.New("TLS_CERT_FILE and TLS_KEY_FILE are required: this add-on does not serve plaintext")
	case c.caFile == "" && len(c.signingKey) == 0:
		// Exactly one authentication mode must be configured. Neither means
		// anything on the internal network can order a credential reset.
		return config{}, errors.New("configure TLS_CLIENT_CA_FILE (mutual TLS) or SIGNING_KEY_FILE (signed requests): one is required")
	case c.caFile != "" && len(c.signingKey) > 0:
		// Both is not "belt and braces", it is an ambiguity: a caller would
		// choose, and the caller is the thing being authenticated.
		return config{}, errors.New("configure exactly one of TLS_CLIENT_CA_FILE and SIGNING_KEY_FILE, not both")
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
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// The retention the store documents, actually applied. Nothing called the
	// sweep outside a test, so the idempotency bucket grew for the life of the
	// deployment — a 30-day window that never ended is a bucket, not a window.
	go sweepIdempotency(ctx, store)

	go func() {
		mode := "mtls"
		if len(cfg.signingKey) > 0 {
			mode = "signed"
		}
		log.Printf("[STARTUP] listening on %s auth=%s", cfg.listen, mode)
		if err := httpSrv.ListenAndServeTLS(cfg.certFile, cfg.keyFile); err != nil && !errors.Is(err, http.ErrServerClosed) {
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

// serverTLS builds the listener's configuration.
//
// TLS 1.3 minimum: both ends are ours and ship together, so there is no legacy
// peer to accommodate and every reason not to leave a downgrade available.
func serverTLS(cfg config) (*tls.Config, error) {
	conf := &tls.Config{MinVersion: tls.VersionTLS13}
	if cfg.caFile == "" {
		return conf, nil
	}
	pem, err := os.ReadFile(cfg.caFile)
	if err != nil {
		return nil, fmt.Errorf("read client CA: %w", err)
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(pem) {
		// An unparseable CA file must not degrade to "no client verification".
		// That is the failure where both ends believe mutual TLS is on and
		// neither of them is checking anything.
		return nil, fmt.Errorf("client CA file %s contains no usable certificate", filepath.Base(cfg.caFile))
	}
	conf.ClientCAs = pool
	// RequireAndVerify, never VerifyIfGiven: the second lets an anonymous
	// caller through, which is the whole authentication in this mode.
	conf.ClientAuth = tls.RequireAndVerifyClientCert
	// And the certificate has to have been issued FOR this. Verification
	// against the pool alone admits anything the private CA ever signed —
	// including the add-on's own server certificate, which the same CA issues
	// and which is mounted in this container. A client certificate is one with
	// client authentication in its extended key usage; a server certificate
	// presented as a client credential is a misuse the chain cannot see.
	conf.VerifyPeerCertificate = requireClientAuthEKU
	return conf, nil
}

func requireClientAuthEKU(_ [][]byte, chains [][]*x509.Certificate) error {
	for _, chain := range chains {
		if len(chain) == 0 {
			continue
		}
		leaf := chain[0]
		if len(leaf.ExtKeyUsage) == 0 {
			// Unconstrained. Old certificates in a lab deployment have no EKU
			// at all, and refusing those would break a working install on an
			// upgrade — so an absent EKU is accepted and a WRONG one is not.
			return nil
		}
		for _, usage := range leaf.ExtKeyUsage {
			if usage == x509.ExtKeyUsageClientAuth || usage == x509.ExtKeyUsageAny {
				return nil
			}
		}
	}
	return errors.New("the client certificate is not marked for client authentication")
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
			return "", fmt.Errorf("read %s_FILE: %w", name, err)
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
