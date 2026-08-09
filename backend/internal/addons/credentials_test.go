package addons

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// mustKeyFile writes a signing key and returns its path.
func mustKeyFile(t *testing.T) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "sign.key")
	writeFile(t, p, []byte("a-real-signing-key"))
	return p
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
// the certificate should not have to bounce the backend that governs every
// other target to do it.
func TestRotationIsPickedUpWithoutARestart(t *testing.T) {
	resetRegistry(t)
	pki := newTestPKI(t, time.Now().Add(365*24*time.Hour))
	r := pki.registration("truenas", "https://addon:8090")

	first, err := credentialFor(r)
	if err != nil {
		t.Fatalf("initial load: %v", err)
	}
	if again, _ := credentialFor(r); again != first {
		t.Fatal("unchanged material was reloaded — every call would re-parse a certificate")
	}

	// A fresh certificate from the same CA, as a rotation script would leave it.
	rotated := newTestPKI(t, time.Now().Add(30*24*time.Hour))
	newCert, _ := os.ReadFile(rotated.clientCertPath)
	newKey, _ := os.ReadFile(rotated.clientKeyPath)
	touchLater(t, r.ClientCertPath, newCert, time.Minute)
	touchLater(t, r.ClientKeyPath, newKey, time.Minute)

	after, err := credentialFor(r)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if after == first {
		t.Fatal("rotated material was not picked up")
	}
	if !after.clientCertNotAfter.Before(first.clientCertNotAfter) {
		t.Fatal("the reloaded credential is not the rotated certificate")
	}
}

// 2.38 — rotation does not drop in-flight operations. A call already holding a
// client finishes on the material it started with; for a secret-bearing
// dispatch that is the difference between a completed operation and an
// indeterminate one nobody can resolve.
func TestRotationDoesNotDropInFlightOperations(t *testing.T) {
	resetRegistry(t)
	pki := newTestPKI(t, time.Now().Add(365*24*time.Hour))
	r := pki.registration("truenas", "https://addon:8090")

	inFlight, err := credentialFor(r)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	heldClient := inFlight.client

	rotated := newTestPKI(t, time.Now().Add(30*24*time.Hour))
	newCert, _ := os.ReadFile(rotated.clientCertPath)
	newKey, _ := os.ReadFile(rotated.clientKeyPath)
	touchLater(t, r.ClientCertPath, newCert, time.Minute)
	touchLater(t, r.ClientKeyPath, newKey, time.Minute)

	next, err := credentialFor(r)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if next.client == heldClient {
		t.Fatal("the rotation did not produce a new client")
	}
	if inFlight.client != heldClient {
		t.Fatal("rotation mutated a credential a call was already holding")
	}
}

// 2.37 — a rotation caught mid-write must not take the transport down. A
// certificate that is briefly absent between unlink and rename, or half
// written, would otherwise fail every dispatch in that window.
func TestUnreadableMaterialKeepsTheLastGoodCredential(t *testing.T) {
	resetRegistry(t)
	pki := newTestPKI(t, time.Now().Add(365*24*time.Hour))
	r := pki.registration("truenas", "https://addon:8090")

	good, err := credentialFor(r)
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	t.Run("absent file", func(t *testing.T) {
		if err := os.Remove(r.ClientCertPath); err != nil {
			t.Fatalf("remove: %v", err)
		}
		c, err := credentialFor(r)
		if err != nil {
			t.Fatalf("a missing file during rotation failed the call: %v", err)
		}
		if c != good {
			t.Fatal("expected the previously loaded credential to keep serving")
		}
	})

	t.Run("garbage file", func(t *testing.T) {
		touchLater(t, r.ClientCertPath, []byte("-----BEGIN CERTIFICATE-----\nhalf-writ"), time.Minute)
		c, err := credentialFor(r)
		if err != nil {
			t.Fatalf("a half-written certificate failed the call: %v", err)
		}
		if c != good {
			t.Fatal("a broken reload replaced a working credential")
		}
	})
}

// 2.37 — with nothing loaded yet there is no last-good to fall back to, and the
// honest answer is an error rather than a plain unauthenticated client.
func TestUnloadableMaterialWithNoPredecessorIsAnError(t *testing.T) {
	resetRegistry(t)
	r := Registration{
		Target: "truenas", BaseURL: "https://addon:8090",
		ClientCertPath: "/nonexistent/c.crt", ClientKeyPath: "/nonexistent/c.key", CAPath: "/nonexistent/ca.crt",
	}
	if _, err := credentialFor(r); err == nil {
		t.Fatal("missing material at startup produced a usable credential")
	}
}

// 2.38 — an expiring transport credential is surfaced before it fails, so
// rotation is scheduled rather than scrambled during an incident.
func TestExpiringCertificateIsSurfacedBeforeItFails(t *testing.T) {
	now := time.Date(2026, 8, 9, 12, 0, 0, 0, time.UTC)

	cases := []struct {
		name   string
		expiry time.Time
		want   string
	}{
		{"comfortable", now.Add(200 * 24 * time.Hour), "ok"},
		{"inside the warning window", now.Add(10 * 24 * time.Hour), "warn"},
		{"already expired", now.Add(-time.Hour), "expired"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resetRegistry(t)
			pki := newTestPKI(t, tc.expiry)
			registryMu.Lock()
			registry = map[string]*Addon{"truenas": {Registration: pki.registration("truenas", "https://addon:8090")}}
			registryMu.Unlock()
			withClock(t, now)

			got := TransportCredentials()
			if len(got) != 1 {
				t.Fatalf("got %d credentials, want 1", len(got))
			}
			if got[0].Status != tc.want {
				t.Fatalf("status = %q, want %q (expires %s)", got[0].Status, tc.want, tc.expiry)
			}
			if got[0].AuthMode != "mtls" {
				t.Fatalf("auth mode = %q", got[0].AuthMode)
			}
			if got[0].DaysRemaining == nil || got[0].ExpiresAt == nil {
				t.Fatal("an expiry status with no date for an operator to act on")
			}
		})
	}
}

// 2.38 — the CA's expiry counts. A current client certificate presented against
// an expired CA fails exactly as hard as an expired one, so reporting only the
// certificate's own date would be a reassurance the connection cannot keep.
func TestCAExpiryDominatesWhenSoonerThanTheClientCertificate(t *testing.T) {
	now := time.Date(2026, 8, 9, 12, 0, 0, 0, time.UTC)
	resetRegistry(t)

	// A long-lived client certificate, and a CA about to lapse under it.
	longLived := newTestPKI(t, now.Add(300*24*time.Hour))
	shortCA := newTestPKI(t, now.Add(5*24*time.Hour))
	caPEM, _ := os.ReadFile(shortCA.caPath)
	writeFile(t, longLived.caPath, caPEM)

	registryMu.Lock()
	registry = map[string]*Addon{"truenas": {Registration: longLived.registration("truenas", "https://addon:8090")}}
	registryMu.Unlock()
	withClock(t, now)

	got := TransportCredentials()
	if got[0].Status != "warn" {
		t.Fatalf("status = %q, want warn — the CA expires in five days", got[0].Status)
	}
	if d := *got[0].DaysRemaining; d > 6 {
		t.Fatalf("days remaining = %d; the report is following the client certificate, not the chain", d)
	}
}

// 2.37 — signed mode has no expiry, and the surface says so rather than
// inventing an "ok" that was never checked.
func TestSignedModeReportsNoExpiryRatherThanAFalseOk(t *testing.T) {
	resetRegistry(t)
	registryMu.Lock()
	registry = map[string]*Addon{"truenas": {Registration: Registration{
		Target: "truenas", BaseURL: "https://addon:8090", SigningKeyPath: mustKeyFile(t),
	}}}
	registryMu.Unlock()

	got := TransportCredentials()
	if got[0].AuthMode != "signed" {
		t.Fatalf("auth mode = %q", got[0].AuthMode)
	}
	if got[0].Status != "unknown" {
		t.Fatalf("status = %q, want unknown — an HMAC key has no expiry", got[0].Status)
	}
	if got[0].ExpiresAt != nil {
		t.Fatal("an expiry date was reported for material that has none")
	}
}

// 2.37 — a secret mounted from a file almost always arrives with a trailing
// newline the operator never typed, and a key differing by one byte fails as
// "no matching signature": indistinguishable from an attack, and days of
// debugging.
func TestSigningKeyTrailingWhitespaceIsTrimmed(t *testing.T) {
	dir := t.TempDir()
	withNewline := filepath.Join(dir, "a.key")
	without := filepath.Join(dir, "b.key")
	writeFile(t, withNewline, []byte("shared-key\n"))
	writeFile(t, without, []byte("shared-key"))

	a, err := readSigningKey(withNewline)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	b, err := readSigningKey(without)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if string(a) != string(b) {
		t.Fatalf("a trailing newline changed the key: %q vs %q", a, b)
	}
}

// 2.37 — an empty key file is a misconfiguration, not a key. Accepting it would
// authenticate every call under the empty string.
func TestEmptySigningKeyIsRefused(t *testing.T) {
	p := filepath.Join(t.TempDir(), "empty.key")
	writeFile(t, p, []byte("\n\n  \n"))
	if _, err := readSigningKey(p); err == nil {
		t.Fatal("an empty signing key was accepted as transport authentication")
	}
}

// 2.2 — unregistering a target drops its loaded private key from memory rather
// than leaving it resident until the next restart.
func TestUnregisteringATargetPurgesItsLoadedKey(t *testing.T) {
	resetRegistry(t)
	pki := newTestPKI(t, time.Now().Add(365*24*time.Hour))
	r := pki.registration("truenas", "https://addon:8090")
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
