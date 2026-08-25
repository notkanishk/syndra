package main

import (
	"crypto/ed25519"
	"crypto/hkdf"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"fmt"
	"math/big"
	"time"
)

// The transport identity, derived rather than distributed (design
// `addon-transport-derived-keys` §2).
//
// One configured secret per target. From it come both keys: the Ed25519 key
// this add-on serves and the backend pins, and the HMAC key that authenticates
// each request. Neither is transmitted, neither is stored, and neither expires.
//
// Everything in this file has to agree with the backend byte for byte, and
// nothing but `../contract/transport_derivation.json` makes it. A disagreement
// about the hash, the salt, an info string or the input encoding presents
// exactly as a wrong secret — each side internally consistent and certain. The
// vector is asserted from here and from the backend; changing either constant
// below is a wire-contract change.
const (
	// Domain separation. The raw secret is never used as either key: a flaw in
	// one use must not hand over the other.
	tlsInfo  = "syndra/addon-tls/v1"
	hmacInfo = "syndra/addon-hmac/v1"
)

// deriveTLSKey turns the configured secret into this add-on's TLS identity.
//
// Ed25519 and not ECDSA, because `NewKeyFromSeed` is RFC 8032: a 32-byte seed
// defines the keypair by specification. Deriving an ECDSA key deterministically
// means handing `ecdsa.GenerateKey` a seeded reader and depending on how the
// standard library happens to consume randomness — reproducible today and not a
// property Go promises. A silent change there would break the pin on a routine
// toolchain bump, at both ends, with no code change to point at.
func deriveTLSKey(secret []byte, target string) (ed25519.PrivateKey, error) {
	seed, err := hkdf.Key(sha256.New, secret, []byte(target), tlsInfo, ed25519.SeedSize)
	if err != nil {
		return nil, fmt.Errorf("derive TLS key: %w", err)
	}
	return ed25519.NewKeyFromSeed(seed), nil
}

// deriveHMACKey turns the same secret into the request-signing key.
func deriveHMACKey(secret []byte, target string) ([]byte, error) {
	key, err := hkdf.Key(sha256.New, secret, []byte(target), hmacInfo, 32)
	if err != nil {
		return nil, fmt.Errorf("derive HMAC key: %w", err)
	}
	return key, nil
}

// selfSignedCert wraps the derived key in a certificate to serve.
//
// Nothing here is pinned except the public key, so the serial and the validity
// window are deliberately NOT deterministic. The backend verifies the key and
// nothing else — under its pin the chain, the name and the expiry are all
// unchecked — so making the DER reproducible would be solving a problem this
// design does not have, and would invite the belief that the certificate itself
// is the credential. It is not; the key is.
//
// In memory, never on disk. There is nothing here worth persisting: a restart
// derives the same key again from the same secret.
func selfSignedCert(priv ed25519.PrivateKey, target string) (tls.Certificate, error) {
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("serial: %w", err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: serial,
		Subject:      pkix.Name{CommonName: target + "-addon"},
		DNSNames:     []string{target + "-addon", "localhost"},
		// Backdated an hour against clock skew between two containers, and long
		// enough that a deployment left running does not trip an operator who
		// later turns verification on for an unrelated reason.
		NotBefore:   time.Now().Add(-time.Hour),
		NotAfter:    time.Now().AddDate(10, 0, 0),
		KeyUsage:    x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, priv.Public(), priv)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("self-sign: %w", err)
	}
	return tls.Certificate{Certificate: [][]byte{der}, PrivateKey: priv}, nil
}
