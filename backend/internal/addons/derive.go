package addons

import (
	"crypto/ed25519"
	"crypto/hkdf"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
	"strings"
)

// The backend's half of the derived transport (design
// `addon-transport-derived-keys`).
//
// One configured secret per target. From it come both keys: the Ed25519 key the
// add-on serves and this pins, and the HMAC key that signs each request.
//
// THIS FILE MUST AGREE WITH `addons/truenas/derive.go` BYTE FOR BYTE, and the
// only thing making it do so is `addons/contract/transport_derivation.json`,
// asserted from both suites. The constants are written out on both sides rather
// than shared, for the same reason `SignatureHeader` and `ContractVersion` are:
// the two are separately deployed binaries, and a shared module would hide
// exactly the version skew the contract version exists to surface.
//
// A disagreement here does not look like a disagreement. A wrong salt, a
// swapped info string, a different hash or a hex-decoded secret each produce a
// self-consistent implementation whose only symptom is that the pin fails —
// indistinguishable, from either side, from a wrong secret.
const (
	tlsInfo  = "syndra/addon-tls/v1"
	hmacInfo = "syndra/addon-hmac/v1"
)

// deriveTLSPublicKey returns the key the add-on for this target must present.
//
// Only the public half is wanted here — the backend never serves this key, it
// recognises it. The private half is derived and discarded rather than the
// public key being derived some other way, because there is one derivation and
// a second path to the same answer is a second definition of it.
func deriveTLSPublicKey(secret []byte, target string) (ed25519.PublicKey, error) {
	seed, err := deriveTLSSeed(secret, target)
	if err != nil {
		return nil, err
	}
	return ed25519.NewKeyFromSeed(seed).Public().(ed25519.PublicKey), nil
}

// deriveTLSSeed is the one place the TLS seed is computed. The backend needs
// only the public half, but the seed is separated out so that the test harness
// standing in for an add-on derives it HERE rather than reimplementing the HKDF
// parameters — a harness with its own copy would agree with itself while the
// backend drifted, which is the two-fakes failure this contract already has a
// vector to prevent.
func deriveTLSSeed(secret []byte, target string) ([]byte, error) {
	seed, err := hkdf.Key(sha256.New, secret, []byte(target), tlsInfo, ed25519.SeedSize)
	if err != nil {
		return nil, fmt.Errorf("derive TLS key: %w", err)
	}
	return seed, nil
}

// deriveHMACKey turns the same secret into the request-signing key.
func deriveHMACKey(secret []byte, target string) ([]byte, error) {
	key, err := hkdf.Key(sha256.New, secret, []byte(target), hmacInfo, 32)
	if err != nil {
		return nil, fmt.Errorf("derive HMAC key: %w", err)
	}
	return key, nil
}

// pinnedTLSConfig verifies the add-on by its derived key and nothing else.
//
// `InsecureSkipVerify` here is NOT a downgrade, and the next reader must be
// able to see that without taking it on trust. It switches off two checks — a
// chain up to some certificate authority, and a name match — and what replaces
// them is an exact public-key comparison. A name check asks whether some
// authority vouched for a string; this asks whether the peer holds the
// deployment secret. Between two components of one deployment the second
// question is strictly the stronger one, and it is the reason an on-path
// attacker cannot reach a body carrying declared secret_params: the handshake
// fails before any body is written.
//
// Go still calls VerifyPeerCertificate when InsecureSkipVerify is set, which is
// what makes this arrangement possible rather than merely intended.
func pinnedTLSConfig(want ed25519.PublicKey, target string) *tls.Config {
	return &tls.Config{
		MinVersion:         tls.VersionTLS13,
		InsecureSkipVerify: true,
		VerifyPeerCertificate: func(raw [][]byte, _ [][]*x509.Certificate) error {
			if len(raw) == 0 {
				return fmt.Errorf("addon %s presented no certificate", target)
			}
			leaf, err := x509.ParseCertificate(raw[0])
			if err != nil {
				return fmt.Errorf("addon %s: %w", target, err)
			}
			got, ok := leaf.PublicKey.(ed25519.PublicKey)
			if !ok {
				return fmt.Errorf("addon %s presented a %T key; the derived transport is ed25519",
					target, leaf.PublicKey)
			}
			if !got.Equal(want) {
				// A TYPED error, because `dialFailed` has to recognise it.
				//
				// This failure happens inside the handshake, so nothing was
				// written and nothing can have happened at the target — the
				// only truthful outcome is "unreached". Returned as an
				// anonymous error it was classified INDETERMINATE instead,
				// which claims a mutation may have been applied and puts an
				// operator on an investigation with nothing at the end of it.
				// Every misconfigured deployment would have manufactured one
				// per call.
				// Three causes, and the message names all of them, because they
				// are indistinguishable from here and an operator who guesses
				// wrong rotates a secret that was never the problem.
				return pinMismatchError{target: target, got: got, want: want}
			}
			return nil
		},
	}
}

// resolveSecret reads a secret from the file `<NAME>_FILE` points at, or from
// `<NAME>` itself.
//
// Deliberately the same semantics as the add-on's `secretValue`, under the same
// suffix convention, because the two ends resolving one value differently is
// this contract's oldest defect: the signing key was once HMAC'd as a file's
// contents by one end and as the literal path string by the other, and the only
// symptom was "no matching signature". The backend used to accept a path only,
// which is why `.env.example` had to warn that the value "is A PATH, matching
// the backend's, which is also a path".
//
// Trimmed, because a mounted secret almost always ends in a newline and a
// one-byte difference produces a pin failure with no other symptom.
func resolveSecret(name string, getenv func(string) string) (string, error) {
	if path := strings.TrimSpace(getenv(name + "_FILE")); path != "" {
		raw, err := os.ReadFile(path)
		if err != nil {
			// Naming the path matters: "no secret configured" and "the mount did
			// not land" are the same symptom and different fixes.
			return "", fmt.Errorf("read %s_FILE (%s): %w", name, path, err)
		}
		value := strings.TrimSpace(string(raw))
		if value == "" {
			// An empty file is a mount that did not land. Falling back to the
			// environment would start the backend against whichever value the
			// other variable happened to hold.
			return "", fmt.Errorf("%s_FILE (%s) is empty", name, path)
		}
		return value, nil
	}
	return strings.TrimSpace(getenv(name)), nil
}

// pinMismatchError is the peer failing the pin.
//
// Three causes, and the message names all of them, because they are
// indistinguishable from here and an operator who guesses wrong rotates a
// secret that was never the problem.
type pinMismatchError struct {
	target    string
	got, want ed25519.PublicKey
}

func (e pinMismatchError) Error() string {
	return fmt.Sprintf("addon %s is not the one derived from this deployment's secret "+
		"(presented %x, expected %x) — the secret differs between the two ends, "+
		"the add-on's ADDON_TARGET does not match this target name, or something "+
		"else is answering on that address", e.target, e.got, e.want)
}
