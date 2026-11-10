package reference

import (
	"crypto/ed25519"
	"crypto/hkdf"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log"
	"math/big"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// The derived-key transport, copied from addons/truenas/transport.go and
// derive.go rather than imported: the two ends of the wire never share a Go
// module, and nothing but ../contract/transport_derivation.json makes them
// agree. transport_test.go holds this copy to that file.
//
// One per-target secret gives two keys through HKDF-SHA-256 (salt = the target
// name): the Ed25519 key this add-on serves and Syndra pins, and the HMAC key
// every request is signed with. Nothing is transmitted and nothing expires.

// SignatureHeader carries "t=<unix>,v1=<hex HMAC-SHA-256>".
const SignatureHeader = "X-Syndra-Addon-Signature"

const (
	tlsInfo  = "syndra/addon-tls/v1"
	hmacInfo = "syndra/addon-hmac/v1"

	// signatureTolerance is the replay window: clock skew between two
	// containers on one host, and nothing else.
	signatureTolerance = 2 * time.Minute
	maxRequestBytes    = 1 << 20
)

func deriveKeys(secret []byte, target string) (ed25519.PrivateKey, []byte, error) {
	seed, err := hkdf.Key(sha256.New, secret, []byte(target), tlsInfo, ed25519.SeedSize)
	if err != nil {
		return nil, nil, fmt.Errorf("derive TLS key: %w", err)
	}
	mac, err := hkdf.Key(sha256.New, secret, []byte(target), hmacInfo, 32)
	if err != nil {
		return nil, nil, fmt.Errorf("derive HMAC key: %w", err)
	}
	return ed25519.NewKeyFromSeed(seed), mac, nil
}

// selfSigned wraps the derived key in a certificate. Only the key is pinned,
// so the serial and validity are deliberately not deterministic.
func selfSigned(priv ed25519.PrivateKey, target string) (tls.Certificate, error) {
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return tls.Certificate{}, err
	}
	tmpl := &x509.Certificate{
		SerialNumber: serial,
		Subject:      pkix.Name{CommonName: target + "-addon"},
		DNSNames:     []string{target + "-addon", "localhost"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().AddDate(10, 0, 0),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, priv.Public(), priv)
	if err != nil {
		return tls.Certificate{}, err
	}
	return tls.Certificate{Certificate: [][]byte{der}, PrivateKey: priv}, nil
}

// signedTarget is what the MAC covers besides the timestamp, method and body:
// the escaped path exactly as sent, plus "?" and the raw query when there is
// one (design §4, decision 24). Not sorted, not re-encoded: a reordered query
// is a different query.
func signedTarget(u *url.URL) string {
	if u.RawQuery == "" {
		return u.EscapedPath()
	}
	return u.EscapedPath() + "?" + u.RawQuery
}

func computeMAC(unix int64, method, target string, body, key []byte) []byte {
	m := hmac.New(sha256.New, key)
	fmt.Fprintf(m, "%d.%s.%s.", unix, method, target)
	m.Write(body)
	return m.Sum(nil)
}

type authenticator struct {
	key []byte
	now func() time.Time
}

// verify authenticates a request and returns the body the handler parses: the
// bytes that were verified are the bytes that are acted on, never a re-read.
func (a authenticator) verify(r *http.Request) ([]byte, error) {
	body, err := io.ReadAll(io.LimitReader(r.Body, maxRequestBytes+1))
	if err != nil || len(body) > maxRequestBytes {
		return nil, fmt.Errorf("body unreadable or over %d bytes", maxRequestBytes)
	}
	var ts, mac string
	for _, part := range strings.Split(r.Header.Get(SignatureHeader), ",") {
		k, v, _ := strings.Cut(strings.TrimSpace(part), "=")
		switch k {
		case "t":
			ts = v
		case "v1":
			mac = v
		}
	}
	seconds, err := strconv.ParseInt(ts, 10, 64)
	got, herr := hex.DecodeString(mac)
	if err != nil || herr != nil || len(got) == 0 {
		return nil, errors.New("no signature")
	}
	// The window before the MAC: an expired signature is refused whether or
	// not it verifies.
	if age := a.now().Sub(time.Unix(seconds, 0)); age > signatureTolerance || age < -signatureTolerance {
		return nil, errors.New("signature timestamp outside the window")
	}
	if subtle.ConstantTimeCompare(computeMAC(seconds, r.Method, signedTarget(r.URL), body, a.key), got) != 1 {
		return nil, errors.New("signature does not match")
	}
	return body, nil
}

// authenticated wraps a handler so it cannot be reached unsigned. The reason
// for a refusal is logged, never returned: a caller that cannot authenticate
// learns nothing about which half it got wrong.
func (a authenticator) authenticated(next func(http.ResponseWriter, *http.Request, []byte)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		body, err := a.verify(r)
		if err != nil {
			log.Printf("refused %s %s: %v", r.Method, r.URL.Path, err)
			problem(http.StatusUnauthorized, "UNAUTHENTICATED", "The request is not signed by Syndra.", nil).write(w)
			return
		}
		next(w, r, body)
	}
}
