package addons

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"math/big"
	"time"
)

// Seams for another package's tests: the handlers' reachability table (design
// §13.7) drives every read route against a stub add-on through this package's
// real pipe, and cannot reach the registry from outside. Never called outside
// a test, as auth.SetKeysForTesting.

// InstallForTesting registers r with m accepted, as Init and Refresh leave a
// target whose add-on says whose accounts are whose, with a fresh breaker and
// no credential cached. It replaces the whole registry; restore puts it back.
func InstallForTesting(r Registration, m Manifest) (restore func()) {
	ops, _ := resolveOperations(m)
	a := &Addon{Registration: r, manifest: &m, ops: ops, fetchedAt: timeNow().UTC()}
	a.attribution.Store(attributionSaid)
	registryMu.Lock()
	saved := registry
	registry = map[string]*Addon{r.Target: a}
	registryMu.Unlock()
	forgetCredentials()
	return func() {
		registryMu.Lock()
		registry = saved
		registryMu.Unlock()
		forgetCredentials()
	}
}

// HandshakeTimeoutForTesting shortens the TLS handshake's deadline (design
// §13.5), so a blackholed add-on is answered in a test's time; restore puts
// it back. The dial's own timeout needs no such seam: a blackhole accepts.
func HandshakeTimeoutForTesting(d time.Duration) (restore func()) {
	saved := tlsHandshakeTimeout
	tlsHandshakeTimeout = d
	forgetCredentials()
	return func() { tlsHandshakeTimeout = saved; forgetCredentials() }
}

func forgetCredentials() {
	credMu.Lock()
	credCache = map[string]*credential{}
	credMu.Unlock()
}

// CertificateForTesting is what an add-on serves for secret and target at
// boot: a self-signed certificate around the derived key, which is all the
// backend pins.
func CertificateForTesting(secret []byte, target string) (tls.Certificate, error) {
	seed, err := deriveTLSSeed(secret, target)
	if err != nil {
		return tls.Certificate{}, err
	}
	priv := ed25519.NewKeyFromSeed(seed)
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: target + "-addon"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, priv.Public(), priv)
	if err != nil {
		return tls.Certificate{}, err
	}
	return tls.Certificate{Certificate: [][]byte{der}, PrivateKey: priv}, nil
}
