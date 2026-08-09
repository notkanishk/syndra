package addons

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// testPKI is a throwaway private CA with a server certificate and a client
// certificate, written to files so the credential loader is exercised the way a
// deployment exercises it: from disk, by path.
//
// Real certificates rather than fakes because the property under test is that
// the transport refuses a peer it cannot verify, and only a real handshake can
// demonstrate that. A mock TLS layer would prove that the mock refuses.
type testPKI struct {
	dir            string
	caPEM          []byte
	caPath         string
	clientCertPath string
	clientKeyPath  string
	serverCert     tls.Certificate
	// A client certificate from a second, unrelated CA: syntactically valid,
	// correctly shaped, and issued by nobody this deployment trusts.
	otherCertPath string
	otherKeyPath  string
}

func newTestPKI(t *testing.T, notAfter time.Time) *testPKI {
	t.Helper()
	dir := t.TempDir()
	p := &testPKI{dir: dir}

	caCert, caKey, caPEM := makeCA(t, "syndra-addon-ca", notAfter)
	p.caPEM = caPEM
	p.caPath = filepath.Join(dir, "ca.crt")
	writeFile(t, p.caPath, caPEM)

	clientPEM, clientKeyPEM := issue(t, caCert, caKey, "syndra-backend", notAfter, x509.ExtKeyUsageClientAuth, nil)
	p.clientCertPath = filepath.Join(dir, "client.crt")
	p.clientKeyPath = filepath.Join(dir, "client.key")
	writeFile(t, p.clientCertPath, clientPEM)
	writeFile(t, p.clientKeyPath, clientKeyPEM)

	serverPEM, serverKeyPEM := issue(t, caCert, caKey, "addon-truenas", notAfter,
		x509.ExtKeyUsageServerAuth, []net.IP{net.ParseIP("127.0.0.1")})
	sc, err := tls.X509KeyPair(serverPEM, serverKeyPEM)
	if err != nil {
		t.Fatalf("server keypair: %v", err)
	}
	p.serverCert = sc

	otherCert, otherKey, _ := makeCA(t, "someone-elses-ca", notAfter)
	oPEM, oKeyPEM := issue(t, otherCert, otherKey, "impostor", notAfter, x509.ExtKeyUsageClientAuth, nil)
	p.otherCertPath = filepath.Join(dir, "impostor.crt")
	p.otherKeyPath = filepath.Join(dir, "impostor.key")
	writeFile(t, p.otherCertPath, oPEM)
	writeFile(t, p.otherKeyPath, oKeyPEM)

	return p
}

// caPool is the verification pool a server uses to check our client.
func (p *testPKI) caPool(t *testing.T) *x509.CertPool {
	t.Helper()
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(p.caPEM) {
		t.Fatal("test CA PEM did not parse")
	}
	return pool
}

// registration returns a Registration pointing at base with this PKI's mTLS
// material.
func (p *testPKI) registration(target, base string) Registration {
	return Registration{
		Target:         target,
		BaseURL:        base,
		ClientCertPath: p.clientCertPath,
		ClientKeyPath:  p.clientKeyPath,
		CAPath:         p.caPath,
	}
}

func makeCA(t *testing.T, cn string, notAfter time.Time) (*x509.Certificate, *ecdsa.PrivateKey, []byte) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("ca key: %v", err)
	}
	tmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: cn},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              notAfter,
		IsCA:                  true,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("ca cert: %v", err)
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatalf("parse ca: %v", err)
	}
	return cert, key, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
}

func issue(t *testing.T, ca *x509.Certificate, caKey *ecdsa.PrivateKey, cn string, notAfter time.Time, usage x509.ExtKeyUsage, ips []net.IP) ([]byte, []byte) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("leaf key: %v", err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(time.Now().UnixNano()),
		Subject:      pkix.Name{CommonName: cn},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     notAfter,
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{usage},
		IPAddresses:  ips,
		DNSNames:     []string{cn},
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, ca, &key.PublicKey, caKey)
	if err != nil {
		t.Fatalf("issue %s: %v", cn, err)
	}
	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		t.Fatalf("marshal key: %v", err)
	}
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}),
		pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})
}

func writeFile(t *testing.T, path string, b []byte) {
	t.Helper()
	if err := os.WriteFile(path, b, 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
