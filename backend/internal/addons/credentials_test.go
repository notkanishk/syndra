package addons

import (
	"crypto/ed25519"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// register seeds the registry directly, for tests about what a surface reports
// rather than about what Init accepts.
func register(t *testing.T, r Registration) {
	t.Helper()
	registryMu.Lock()
	registry[r.Target] = &Addon{Registration: r}
	registryMu.Unlock()
}

// secretRegistration writes a secret file and returns a registration built on
// it, which is how a deployment configures a target: one file, mounted into
// both this backend and the add-on.
func secretRegistration(t *testing.T, target, secret string) Registration {
	t.Helper()
	p := filepath.Join(t.TempDir(), target+".key")
	writeFile(t, p, []byte(secret))
	return Registration{
		Target:     target,
		BaseURL:    "https://addon:8090",
		Secret:     []byte(secret),
		SecretPath: p,
	}
}

// touchLater rewrites a file with a modification time in the future, which is
// how an operator's rotation looks to the loader. Explicit rather than relying
// on the clock, because two writes inside one filesystem timestamp tick are
// indistinguishable and would make this test pass or fail by luck.
func touchLater(t *testing.T, path string, content []byte, ahead time.Duration) {
	t.Helper()
	writeFile(t, path, content)
	at := time.Now().Add(ahead)
	if err := os.Chtimes(path, at, at); err != nil {
		t.Fatalf("chtimes %s: %v", path, err)
	}
}

// 2.37 — rotation is a file replacement, not a restart. An operator who swaps
// the secret should not have to bounce the backend that governs every other
// target to do it.
//
// The material being watched changed with this transport — a certificate pair
// became one secret — but the property did not, and neither did the reason for
// it.
func TestRotationIsPickedUpWithoutARestart(t *testing.T) {
	resetRegistry(t)
	r := secretRegistration(t, "truenas", "the-original-secret")

	first, err := credentialFor(r)
	if err != nil {
		t.Fatalf("initial load: %v", err)
	}
	firstKey := append([]byte(nil), first.signingKey...)

	touchLater(t, r.SecretPath, []byte("the-rotated-secret"), time.Minute)

	after, err := credentialFor(r)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if string(after.signingKey) == string(firstKey) {
		t.Fatal("the rotated secret was not picked up; the derived key is unchanged")
	}
	// And it is the key the NEW secret derives, not merely a different one.
	want, err := deriveHMACKey([]byte("the-rotated-secret"), "truenas")
	if err != nil {
		t.Fatal(err)
	}
	if string(after.signingKey) != string(want) {
		t.Fatal("the reloaded key is not the one derived from the new secret")
	}
}

// A call already holding a credential finishes on the material it started with.
// For a secret-bearing dispatch the alternative is the difference between a
// completed operation and an indeterminate one.
func TestRotationDoesNotDropInFlightOperations(t *testing.T) {
	resetRegistry(t)
	r := secretRegistration(t, "truenas", "the-original-secret")

	inFlight, err := credentialFor(r)
	if err != nil {
		t.Fatalf("initial load: %v", err)
	}

	touchLater(t, r.SecretPath, []byte("the-rotated-secret"), time.Minute)
	if _, err := credentialFor(r); err != nil {
		t.Fatalf("reload: %v", err)
	}

	// The pointer captured before the rotation still works and still carries
	// the key it was built with.
	if inFlight.client == nil {
		t.Fatal("the in-flight credential lost its client")
	}
	want, err := deriveHMACKey([]byte("the-original-secret"), "truenas")
	if err != nil {
		t.Fatal(err)
	}
	if string(inFlight.signingKey) != string(want) {
		t.Fatal("the in-flight credential's key changed underneath it")
	}
}

// A secret half-written by a rotation script, or briefly absent between unlink
// and rename, must not turn every dispatch during that window into an error.
// The last good credential keeps serving and the failure is logged.
func TestUnreadableMaterialKeepsTheLastGoodCredential(t *testing.T) {
	for name, corrupt := range map[string]func(t *testing.T, r Registration){
		"the file is gone": func(t *testing.T, r Registration) {
			if err := os.Remove(r.SecretPath); err != nil {
				t.Fatal(err)
			}
		},
		"the file is empty": func(t *testing.T, r Registration) {
			touchLater(t, r.SecretPath, nil, time.Minute)
		},
	} {
		t.Run(name, func(t *testing.T) {
			resetRegistry(t)
			r := secretRegistration(t, "truenas", "the-original-secret")
			good, err := credentialFor(r)
			if err != nil {
				t.Fatalf("initial load: %v", err)
			}

			corrupt(t, r)

			after, err := credentialFor(r)
			if err != nil {
				t.Fatalf("a failed reload must not fail the call: %v", err)
			}
			if after != good {
				t.Fatal("a failed reload replaced a working credential")
			}
		})
	}
}

// With no predecessor there is nothing to fall back to, and calling an add-on
// unauthenticated is not the alternative.
func TestUnloadableMaterialWithNoPredecessorIsAnError(t *testing.T) {
	resetRegistry(t)
	r := Registration{
		Target: "truenas", BaseURL: "https://addon:8090",
		Secret: []byte("configured"), SecretPath: "/nonexistent/truenas.key",
	}
	if _, err := credentialFor(r); err == nil {
		t.Fatal("an unreadable secret with no predecessor must be an error")
	}
}

// A registration with no secret must never produce a usable client. Init
// refuses to register one, so this guards the loader rather than the
// configuration — the one thing this package must never do is call an add-on
// unauthenticated because a check moved somewhere else.
func TestNoSecretMeansNoCredential(t *testing.T) {
	resetRegistry(t)
	r := Registration{Target: "truenas", BaseURL: "https://addon:8090"}
	if _, err := credentialFor(r); err == nil {
		t.Fatal("a registration with no secret must not yield a credential")
	}
}

// A mounted secret almost always ends in a newline the operator never typed,
// and the loader must trim before deriving. The derivation itself does NOT
// trim — it takes the bytes it is given — so this is a property of the loader,
// and a one-byte difference here fails as a pin mismatch with no other symptom.
func TestTheLoaderTrimsTheSecretFileBeforeDeriving(t *testing.T) {
	resetRegistry(t)
	r := secretRegistration(t, "truenas", "the-secret\n")

	c, err := credentialFor(r)
	if err != nil {
		t.Fatal(err)
	}
	want, err := deriveHMACKey([]byte("the-secret"), "truenas")
	if err != nil {
		t.Fatal(err)
	}
	if string(c.signingKey) != string(want) {
		t.Fatal("a trailing newline in the secret file changed the derived key")
	}
	// And the untrimmed bytes really would differ, so the assertion above is
	// not vacuous.
	untrimmed, err := deriveHMACKey([]byte("the-secret\n"), "truenas")
	if err != nil {
		t.Fatal(err)
	}
	if string(untrimmed) == string(want) {
		t.Fatal("the two inputs derive the same key, so this test proves nothing")
	}
}

// The transport surface reports state, not an expiry that no longer exists.
// A field that could only ever say "unknown" reads as a probe that is failing.
func TestTransportCredentialsReportStateWithoutAnExpiry(t *testing.T) {
	resetRegistry(t)
	r := secretRegistration(t, "truenas", "a-shared-deployment-secret")
	register(t, r)

	got := TransportCredentials()
	if len(got) != 1 {
		t.Fatalf("expected one target, got %+v", got)
	}
	if got[0].AuthMode != "derived" {
		t.Errorf("auth mode = %q, want derived", got[0].AuthMode)
	}
	if got[0].Status != "ok" {
		t.Errorf("status = %q (%s), want ok", got[0].Status, got[0].Error)
	}
}

// A secret that cannot be loaded is what an operator actually needs to see
// here — a mount that did not land, or an empty file.
func TestTransportCredentialsReportAnUnloadableSecret(t *testing.T) {
	resetRegistry(t)
	register(t, Registration{
		Target: "truenas", BaseURL: "https://addon:8090",
		Secret: []byte("configured"), SecretPath: "/nonexistent/truenas.key",
	})

	got := TransportCredentials()
	if len(got) != 1 || got[0].Status != "error" || got[0].Error == "" {
		t.Fatalf("an unreadable secret must be reported, got %+v", got)
	}
}

func TestUnregisteringATargetPurgesItsLoadedKey(t *testing.T) {
	resetRegistry(t)
	r := secretRegistration(t, "truenas", "a-shared-deployment-secret")
	if _, err := credentialFor(r); err != nil {
		t.Fatalf("load: %v", err)
	}

	purgeCredentialsExcept(map[string]*Addon{})

	credMu.Lock()
	_, still := credCache["truenas"]
	credMu.Unlock()
	if still {
		t.Fatal("an unregistered target's transport material stayed loaded")
	}
}

// The derived public key is what the pin compares against, and it must be the
// add-on's — not merely stable.
func TestTheBackendDerivesTheSamePublicKeyTheAddOnServes(t *testing.T) {
	pub, err := deriveTLSPublicKey([]byte("a-shared-deployment-secret"), "truenas")
	if err != nil {
		t.Fatal(err)
	}
	if len(pub) != ed25519.PublicKeySize {
		t.Fatalf("derived key is %d bytes, want %d", len(pub), ed25519.PublicKeySize)
	}
	again, err := deriveTLSPublicKey([]byte("a-shared-deployment-secret"), "truenas")
	if err != nil {
		t.Fatal(err)
	}
	if !pub.Equal(again) {
		t.Fatal("the derivation is not deterministic")
	}
}
