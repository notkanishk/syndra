package main

import (
	"crypto/ed25519"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// The add-on's half of the derivation contract (see ../contract/README.md).
//
// The backend's suite asserts the same file. Neither end can catch a
// disagreement alone: a wrong salt, a swapped info string, a different hash or
// a hex-decoded secret all produce a self-consistent implementation whose only
// symptom in production is a pin failure that looks exactly like a wrong
// secret. The vector is the third party that makes them agree.

type derivationVector struct {
	SecretUTF8     string `json:"secret_utf8"`
	SecretBytesHex string `json:"secret_bytes_hex"`
	Derivations    []struct {
		Target       string `json:"target"`
		TLSSeedHex   string `json:"tls_seed_hex"`
		TLSPublicHex string `json:"tls_public_key_hex"`
		HMACKeyHex   string `json:"hmac_key_hex"`
	} `json:"derivations"`
	Signature struct {
		Target     string `json:"target"`
		Unix       string `json:"unix"`
		Method     string `json:"method"`
		Path       string `json:"path"`
		BodyUTF8   string `json:"body_utf8"`
		HMACKeyHex string `json:"hmac_key_hex"`
		MACHex     string `json:"mac_hex"`
	} `json:"signature"`
}

func loadVector(t *testing.T) derivationVector {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "contract", "transport_derivation.json"))
	if err != nil {
		t.Fatalf("read derivation vector: %v", err)
	}
	var v derivationVector
	if err := json.Unmarshal(raw, &v); err != nil {
		t.Fatalf("parse derivation vector: %v", err)
	}
	if len(v.Derivations) < 2 {
		// Two targets on purpose: an implementation that ignored the salt would
		// satisfy a single-target vector for as long as only one add-on exists,
		// and fail the day a second is deployed.
		t.Fatal("the vector must carry at least two targets, or it cannot catch a dropped salt")
	}
	return v
}

func TestDerivationMatchesTheContractVector(t *testing.T) {
	v := loadVector(t)

	// The encoding decision, asserted before anything derived from it. The
	// secret is the value's UTF-8 bytes and is NOT hex-decoded, even though
	// `openssl rand -hex 32` makes it look like hex. The vector states it twice
	// so this cannot be misread from the artifact.
	if got := hex.EncodeToString([]byte(v.SecretUTF8)); got != v.SecretBytesHex {
		t.Fatalf("the vector's own two statements of the secret disagree:\n  utf8 as hex = %s\n  secret_bytes_hex = %s", got, v.SecretBytesHex)
	}
	secret := []byte(v.SecretUTF8)

	for _, d := range v.Derivations {
		priv, err := deriveTLSKey(secret, d.Target)
		if err != nil {
			t.Fatalf("%s: %v", d.Target, err)
		}
		if got := hex.EncodeToString(priv.Seed()); got != d.TLSSeedHex {
			t.Errorf("%s: TLS seed\n  got  %s\n  want %s", d.Target, got, d.TLSSeedHex)
		}
		if got := hex.EncodeToString(priv.Public().(ed25519.PublicKey)); got != d.TLSPublicHex {
			t.Errorf("%s: TLS public key\n  got  %s\n  want %s", d.Target, got, d.TLSPublicHex)
		}
		mac, err := deriveHMACKey(secret, d.Target)
		if err != nil {
			t.Fatalf("%s: %v", d.Target, err)
		}
		if got := hex.EncodeToString(mac); got != d.HMACKeyHex {
			t.Errorf("%s: HMAC key\n  got  %s\n  want %s", d.Target, got, d.HMACKeyHex)
		}
	}
}

// The two derived keys must never be the same bytes, and neither may be the
// secret itself. Domain separation is not decoration: a flaw in one use must
// not hand over the other.
func TestTheTwoDerivedKeysAreSeparated(t *testing.T) {
	secret := []byte("a-secret-that-is-only-for-this-test")
	priv, err := deriveTLSKey(secret, "truenas")
	if err != nil {
		t.Fatal(err)
	}
	mac, err := deriveHMACKey(secret, "truenas")
	if err != nil {
		t.Fatal(err)
	}
	if hex.EncodeToString(priv.Seed()) == hex.EncodeToString(mac) {
		t.Fatal("the TLS seed and the HMAC key are the same bytes; the info strings are not separating them")
	}
	if string(priv.Seed()) == string(secret) || string(mac) == string(secret) {
		t.Fatal("a derived key equals the configured secret; the secret is being used directly")
	}
}

// The salt is the target name, so one secret misconfigured across two add-ons
// still yields different keys. This is NOT a security control — anything
// holding the secret knows the algorithm and the target names — and the spec
// says so. It is here because dropping the salt is otherwise invisible while
// only one add-on exists.
func TestTheTargetSaltSeparatesAddOns(t *testing.T) {
	secret := []byte("one-secret-mistakenly-shared-by-two")
	a, err := deriveTLSKey(secret, "truenas")
	if err != nil {
		t.Fatal(err)
	}
	b, err := deriveTLSKey(secret, "unifi")
	if err != nil {
		t.Fatal(err)
	}
	if a.Public().(ed25519.PublicKey).Equal(b.Public().(ed25519.PublicKey)) {
		t.Fatal("two targets derived the same key; the salt is not reaching the derivation")
	}
}

// The MAC construction, pinned against a key derived from the vector's secret.
//
// This is the half that already shipped, and it is in the vector so that a
// change to `computeMAC` fails here rather than in production as "no matching
// signature". Its expected value was produced against this function, not
// against a reimplementation of it.
func TestTheSignatureVectorMatchesComputeMAC(t *testing.T) {
	v := loadVector(t)

	key, err := hex.DecodeString(v.Signature.HMACKeyHex)
	if err != nil {
		t.Fatal(err)
	}
	// The signing key in the vector must be the one the derivation produces for
	// that target, or the signature entry is pinned to a key nothing uses.
	derived, err := deriveHMACKey([]byte(v.SecretUTF8), v.Signature.Target)
	if err != nil {
		t.Fatal(err)
	}
	if hex.EncodeToString(derived) != v.Signature.HMACKeyHex {
		t.Fatalf("the signature entry's key is not the one derived for target %q", v.Signature.Target)
	}

	var unix int64
	if _, err := fmt.Sscan(v.Signature.Unix, &unix); err != nil {
		t.Fatalf("vector unix timestamp: %v", err)
	}
	got := computeMAC(unix, v.Signature.Method, v.Signature.Path, []byte(v.Signature.BodyUTF8), key)
	if hex.EncodeToString(got) != v.Signature.MACHex {
		t.Fatalf("computeMAC disagrees with the vector\n  got  %s\n  want %s",
			hex.EncodeToString(got), v.Signature.MACHex)
	}
}

// The served certificate must carry the derived key and nothing else must be
// load-bearing about it. The backend pins the public key, so a change to the
// certificate's subject, serial or validity is free — but a certificate built
// around a different key is the failure this whole design turns on.
func TestTheServedCertificateCarriesTheDerivedKey(t *testing.T) {
	secret := []byte("the-deployment-secret-for-this-test")
	cfg := config{secret: secret, target: "truenas"}

	conf, err := serverTLS(cfg)
	if err != nil {
		t.Fatal(err)
	}
	want, err := deriveTLSKey(secret, "truenas")
	if err != nil {
		t.Fatal(err)
	}
	got, ok := conf.Certificates[0].PrivateKey.(ed25519.PrivateKey)
	if !ok {
		t.Fatalf("the served key is %T, not ed25519", conf.Certificates[0].PrivateKey)
	}
	if !got.Public().(ed25519.PublicKey).Equal(want.Public().(ed25519.PublicKey)) {
		t.Fatal("the served certificate is not built around the derived key")
	}
	if conf.MinVersion != tls.VersionTLS13 {
		t.Errorf("MinVersion = %x, want TLS 1.3", conf.MinVersion)
	}
	// No client CAs and no client auth: the caller is authenticated by its
	// signature, not by a certificate. Leaving RequireAndVerifyClientCert here
	// would refuse the backend outright, since it no longer holds one.
	if conf.ClientAuth != tls.NoClientCert || conf.ClientCAs != nil {
		t.Error("the listener still expects a client certificate; nothing issues one any more")
	}
}
