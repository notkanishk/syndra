package addons

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"math/big"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// writeFile writes a test file, failing the test rather than the caller.
func writeFile(t *testing.T, path string, content []byte) {
	t.Helper()
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

// A stand-in for the add-on's transport half.
//
// It exists to be wrong in the ways a real add-on can be wrong: serving a key
// derived from a different secret, or from a different target name, or one this
// deployment never had anything to do with. All three are indistinguishable to
// the backend by design — it pins one key, and everything else is "not that
// key" — so the tests assert the refusal rather than which kind of wrong it
// was.
//
// This replaced a private-CA harness: a CA, a server certificate, a client
// keypair, and a second CA kept for the negative case. All of it was
// scaffolding for an authentication that no longer happens.
type derivedAddon struct {
	secret string
	srv    *httptest.Server
}

// serveDerived starts a TLS server presenting the key derived from secret and
// target, exactly as the add-on does at boot.
func serveDerived(t *testing.T, secret, target string, h http.Handler) *derivedAddon {
	t.Helper()
	if h == nil {
		h = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })
	}
	srv := httptest.NewUnstartedServer(h)
	srv.TLS = &tls.Config{
		Certificates: []tls.Certificate{derivedServerCert(t, secret, target)},
		MinVersion:   tls.VersionTLS13,
	}
	srv.StartTLS()
	t.Cleanup(srv.Close)
	return &derivedAddon{secret: secret, srv: srv}
}

// derivedServerCert mints what the add-on serves: a self-signed certificate
// around the derived key, with nothing about it load-bearing except the key.
//
// The seed comes from the package's own deriveTLSSeed rather than a copy of the
// HKDF call, so this harness cannot drift into agreeing with itself.
func derivedServerCert(t *testing.T, secret, target string) tls.Certificate {
	t.Helper()
	seed, err := deriveTLSSeed([]byte(secret), target)
	if err != nil {
		t.Fatal(err)
	}
	priv := ed25519.NewKeyFromSeed(seed)
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: target + "-addon"},
		DNSNames:     []string{target + "-addon", "localhost"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, priv.Public(), priv)
	if err != nil {
		t.Fatal(err)
	}
	return tls.Certificate{Certificate: [][]byte{der}, PrivateKey: priv}
}

// registration wires this stand-in as a configured target.
func (d *derivedAddon) registration(t *testing.T, target string) Registration {
	t.Helper()
	p := filepath.Join(t.TempDir(), target+".key")
	writeFile(t, p, []byte(d.secret))
	return Registration{
		Target:     target,
		BaseURL:    d.srv.URL,
		Secret:     []byte(d.secret),
		SecretPath: p,
	}
}
