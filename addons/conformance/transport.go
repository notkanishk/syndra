package conformance

import (
	"crypto/ed25519"
	"crypto/hkdf"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

// The derived-key transport, client side, written out rather than imported
// (the backend's addons/derive.go and credentials.go are the model): the kit
// stands outside every module it checks, and transport_derivation.json is
// what holds all of them, this one included, to the same bytes.

const signatureHeader = "X-Syndra-Addon-Signature"

const (
	tlsInfo  = "syndra/addon-tls/v1"
	hmacInfo = "syndra/addon-hmac/v1"
)

func deriveKeys(secret []byte, target string) (ed25519.PublicKey, []byte, error) {
	seed, err := hkdf.Key(sha256.New, secret, []byte(target), tlsInfo, ed25519.SeedSize)
	if err != nil {
		return nil, nil, err
	}
	key, err := hkdf.Key(sha256.New, secret, []byte(target), hmacInfo, 32)
	if err != nil {
		return nil, nil, err
	}
	return ed25519.NewKeyFromSeed(seed).Public().(ed25519.PublicKey), key, nil
}

// pinnedClient trusts exactly one key: the one derived from the secret. Chain,
// name and expiry are not what authenticates the add-on, the key is, so chain
// verification is REPLACED by the pin, never skipped: a certificate whose key
// is not the derived one fails the handshake, and no flag turns that off.
// Redirects are never followed; the base URL is the only authority.
func pinnedClient(want ed25519.PublicKey) *http.Client {
	return &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{TLSClientConfig: &tls.Config{
			MinVersion:         tls.VersionTLS13,
			InsecureSkipVerify: true, // replaced by VerifyPeerCertificate below
			VerifyPeerCertificate: func(raw [][]byte, _ [][]*x509.Certificate) error {
				if len(raw) == 0 {
					return fmt.Errorf("the add-on presented no certificate")
				}
				leaf, err := x509.ParseCertificate(raw[0])
				if err != nil {
					return err
				}
				got, ok := leaf.PublicKey.(ed25519.PublicKey)
				if !ok || !got.Equal(want) {
					return fmt.Errorf("the add-on's TLS key is not the one derived from this secret and target "+
						"(presented %x, derived %x): a different secret, a different -target, or something else on that address",
						leaf.PublicKey, want)
				}
				return nil
			},
		}},
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
}

// signedTarget is the escaped path as sent, plus "?" and the raw query when
// there is one (design §4, decision 24).
func signedTarget(u *url.URL) string {
	if u.RawQuery == "" {
		return u.EscapedPath()
	}
	return u.EscapedPath() + "?" + u.RawQuery
}

func mac(unix int64, method, target string, body, key []byte) string {
	m := hmac.New(sha256.New, key)
	fmt.Fprintf(m, "%d.%s.%s.", unix, method, target)
	m.Write(body)
	return hex.EncodeToString(m.Sum(nil))
}
