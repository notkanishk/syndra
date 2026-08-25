package addons

import (
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// The backend's half of the derivation contract (see
// ../../../addons/contract/README.md).
//
// The add-on's suite asserts THIS SAME FILE. That is the whole mechanism:
// neither end can catch a disagreement alone, because a wrong salt, a swapped
// info string, a different hash or a hex-decoded secret each produce an
// implementation that is entirely self-consistent and whose only symptom in
// production is a pin failure — indistinguishable, from either side, from a
// wrong secret.
//
// This is the same shape as `contract_test.go`'s envelope fixtures, one layer
// down: those pin what the two ends say, this pins how they authenticate before
// either is read.

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

func loadDerivationVector(t *testing.T) derivationVector {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "..", "..", "addons", "contract", "transport_derivation.json"))
	if err != nil {
		t.Fatalf("read derivation vector: %v", err)
	}
	var v derivationVector
	if err := json.Unmarshal(raw, &v); err != nil {
		t.Fatalf("parse derivation vector: %v", err)
	}
	if len(v.Derivations) < 2 {
		t.Fatal("the vector must carry at least two targets, or it cannot catch a dropped salt")
	}
	return v
}

func TestBackendDerivationMatchesTheContractVector(t *testing.T) {
	v := loadDerivationVector(t)

	// The encoding decision, asserted before anything derived from it: the
	// secret is the configured value's UTF-8 bytes and is NOT hex-decoded, even
	// though `openssl rand -hex 32` makes it look like hex.
	if got := hex.EncodeToString([]byte(v.SecretUTF8)); got != v.SecretBytesHex {
		t.Fatalf("the vector's own two statements of the secret disagree:\n  utf8 as hex = %s\n  secret_bytes_hex = %s",
			got, v.SecretBytesHex)
	}
	secret := []byte(v.SecretUTF8)

	for _, d := range v.Derivations {
		seed, err := deriveTLSSeed(secret, d.Target)
		if err != nil {
			t.Fatalf("%s: %v", d.Target, err)
		}
		if got := hex.EncodeToString(seed); got != d.TLSSeedHex {
			t.Errorf("%s: TLS seed\n  got  %s\n  want %s", d.Target, got, d.TLSSeedHex)
		}
		pub, err := deriveTLSPublicKey(secret, d.Target)
		if err != nil {
			t.Fatalf("%s: %v", d.Target, err)
		}
		if got := hex.EncodeToString(pub); got != d.TLSPublicHex {
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

// The signature construction, against a key derived from the vector's secret.
// The backend's ComputeSignature and the add-on's computeMAC are separate
// implementations of one format; the vector is what holds them together.
func TestBackendSignatureMatchesTheContractVector(t *testing.T) {
	v := loadDerivationVector(t)

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
	// ComputeSignature returns the full header; the vector pins the MAC inside
	// it, so the header is parsed rather than string-matched — the header format
	// is asserted elsewhere and pinning it twice would make one of the two the
	// place a change is missed.
	header := ComputeSignature(time.Unix(unix, 0), v.Signature.Method, v.Signature.Path, []byte(v.Signature.BodyUTF8), derived)
	_, macHex, ok := strings.Cut(header, ",v1=")
	if !ok {
		t.Fatalf("signature header has no v1 component: %q", header)
	}
	if got := macHex; got != v.Signature.MACHex {
		t.Fatalf("ComputeSignature disagrees with the vector\n  got  %s\n  want %s", macHex, v.Signature.MACHex)
	}
}

// The two derived keys are different bytes, and neither is the secret. Domain
// separation is not decoration: a flaw in one use must not hand over the other.
func TestBackendDerivedKeysAreSeparated(t *testing.T) {
	secret := []byte("a-secret-that-is-only-for-this-test")
	seed, err := deriveTLSSeed(secret, "truenas")
	if err != nil {
		t.Fatal(err)
	}
	mac, err := deriveHMACKey(secret, "truenas")
	if err != nil {
		t.Fatal(err)
	}
	if string(seed) == string(mac) {
		t.Fatal("the TLS seed and the HMAC key are the same bytes; the info strings are not separating them")
	}
	if string(seed) == string(secret) || string(mac) == string(secret) {
		t.Fatal("a derived key equals the configured secret; the secret is being used directly")
	}
}

// resolveSecret must behave exactly as the add-on's `secretValue` does, because
// a value the two ends resolve differently is this contract's oldest defect.
func TestResolveSecretMatchesTheAddOnsSemantics(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "s.key")
	writeFile(t, path, []byte("  the-secret\n"))

	env := func(m map[string]string) func(string) string {
		return func(k string) string { return m[k] }
	}

	t.Run("the file wins over the inline value", func(t *testing.T) {
		got, err := resolveSecret("ADDON_X_SECRET", env(map[string]string{
			"ADDON_X_SECRET":      "the-inline-one",
			"ADDON_X_SECRET_FILE": path,
		}))
		if err != nil {
			t.Fatal(err)
		}
		if got != "the-secret" {
			t.Fatalf("got %q; the file must win and must be trimmed", got)
		}
	})

	t.Run("a trailing newline is trimmed to the same value", func(t *testing.T) {
		got, err := resolveSecret("ADDON_X_SECRET", env(map[string]string{"ADDON_X_SECRET": "the-secret\n"}))
		if err != nil || got != "the-secret" {
			t.Fatalf("got %q, %v", got, err)
		}
	})

	t.Run("an empty file is an error, not a fallback", func(t *testing.T) {
		empty := filepath.Join(dir, "empty.key")
		writeFile(t, empty, []byte("   \n"))
		// The fallback is the dangerous behaviour: a mount that did not land
		// would start the backend against whichever value the other variable
		// happened to hold.
		if _, err := resolveSecret("ADDON_X_SECRET", env(map[string]string{
			"ADDON_X_SECRET":      "the-inline-one",
			"ADDON_X_SECRET_FILE": empty,
		})); err == nil {
			t.Fatal("an empty secret file fell back to the inline value")
		}
	})

	t.Run("a missing file names the path", func(t *testing.T) {
		_, err := resolveSecret("ADDON_X_SECRET", env(map[string]string{"ADDON_X_SECRET_FILE": "/nonexistent/s.key"}))
		if err == nil {
			t.Fatal("a missing secret file must be an error")
		}
		// "No secret configured" and "the mount is missing" are the same symptom
		// and different fixes.
		if !strings.Contains(err.Error(), "/nonexistent/s.key") {
			t.Fatalf("the error must name the path, got %q", err)
		}
	})

	t.Run("nothing configured is empty, not an error", func(t *testing.T) {
		got, err := resolveSecret("ADDON_X_SECRET", env(map[string]string{}))
		if err != nil || got != "" {
			t.Fatalf("got %q, %v; Init decides what an absent secret means", got, err)
		}
	})
}

// The pin must reject a peer that is otherwise entirely well-formed. Name-based
// verification passing where key pinning must fail is the regression that would
// silently restore the weaker check.
func TestThePinRejectsAWellFormedButWrongKey(t *testing.T) {
	want, err := deriveTLSPublicKey([]byte("the-real-secret"), "truenas")
	if err != nil {
		t.Fatal(err)
	}
	conf := pinnedTLSConfig(want, "truenas")

	other := derivedServerCert(t, "a-different-secret", "truenas")
	err = conf.VerifyPeerCertificate(other.Certificate, nil)
	if err == nil {
		t.Fatal("the pin accepted a certificate built around another key")
	}
	var mismatch pinMismatchError
	if !errors.As(err, &mismatch) {
		t.Fatalf("the pin must fail with a typed error dialFailed recognises, got %T", err)
	}

	// And it accepts the right one, so the assertion above is not vacuous.
	real := derivedServerCert(t, "the-real-secret", "truenas")
	if err := conf.VerifyPeerCertificate(real.Certificate, nil); err != nil {
		t.Fatalf("the pin rejected the add-on it was built for: %v", err)
	}
}

// A pin failure happens inside the handshake, so nothing was written. Its
// outcome is "unreached" and never "indeterminate": the second claims a
// mutation may have been applied, and a misconfigured deployment would
// manufacture one per call, each sending an operator on an investigation with
// nothing at the end of it.
func TestAPinFailureClassifiesAsUnreached(t *testing.T) {
	err := pinMismatchError{target: "truenas", got: ed25519.PublicKey{1}, want: ed25519.PublicKey{2}}
	if !dialFailed(err) {
		t.Fatal("a pin mismatch must classify as never-delivered")
	}
	if !dialFailed(fmt.Errorf("Post \"https://addon:8443/apply\": %w", err)) {
		t.Fatal("and it must survive the wrapping net/http applies")
	}
}
