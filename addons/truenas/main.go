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

	nasURL     string
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
		nasAPIKey:  os.Getenv("TRUENAS_API_KEY"),
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
	if key := strings.TrimSpace(os.Getenv("SIGNING_KEY")); key != "" {
		// Trimmed: a mounted secret almost always carries a trailing newline,
		// and a one-byte difference surfaces as "no matching signature" — a
		// failure whose message points nowhere near its cause.
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
		return config{}, errors.New("configure TLS_CLIENT_CA_FILE (mutual TLS) or SIGNING_KEY (signed requests): one is required")
	case c.caFile != "" && len(c.signingKey) > 0:
		// Both is not "belt and braces", it is an ambiguity: a caller would
		// choose, and the caller is the thing being authenticated.
		return config{}, errors.New("configure exactly one of TLS_CLIENT_CA_FILE and SIGNING_KEY, not both")
	}
	for i := range c.supported {
		c.supported[i] = strings.TrimSpace(c.supported[i])
	}
	return c, nil
}

func main() {
	log.SetFlags(log.LstdFlags | log.LUTC)

	cfg, err := loadConfig()
	if err != nil {
		log.Fatalf("[STARTUP] %v", err)
	}

	store, err := OpenStore(cfg.statePath)
	if err != nil {
		log.Fatalf("[STARTUP] %v", err)
	}
	defer store.Close()

	mlog, err := OpenMutationLog(cfg.logDir, cfg.logMaxSize, cfg.logKeep)
	if err != nil {
		// Fatal on purpose. The mutation log is the record that survives losing
		// Syndra's audit tables; starting without one would mean mutations with
		// no independent trace, which is the guarantee the design states.
		log.Fatalf("[STARTUP] mutation log: %v", err)
	}
	defer mlog.Close()

	nas := newNAS(dialTrueNAS(cfg.nasURL, cfg.nasAPIKey, cfg.nasVerify), cfg.supported)
	// Probed once at startup so `/capabilities` can answer with a version, and
	// non-fatally: an add-on that refused to start because the NAS was off
	// would be unreachable in exactly the situation an operator is trying to
	// diagnose.
	if v, err := nas.SystemVersion(); err != nil {
		log.Printf("[STARTUP] could not read the target version yet: %v", err)
	} else {
		log.Printf("[STARTUP] target version %s", v)
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
	}

	tlsConf, err := serverTLS(cfg)
	if err != nil {
		log.Fatalf("[STARTUP] %v", err)
	}
	httpSrv := &http.Server{
		Addr:              cfg.listen,
		Handler:           srv.routes(),
		TLSConfig:         tlsConf,
		ReadHeaderTimeout: 10 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

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
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if err := httpSrv.Shutdown(shutdownCtx); err != nil {
		log.Printf("[SHUTDOWN] %v", err)
	}
	nas.drop()
	log.Println("[SHUTDOWN] stopped")
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
	return conf, nil
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
