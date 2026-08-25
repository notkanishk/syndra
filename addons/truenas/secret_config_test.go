package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// How the secret is OBTAINED, which the contract vector cannot reach.
//
// `transport_derivation.json` answers "given these bytes, what comes out". Every
// case here is about which bytes each end ends up holding, and the vector is
// blind to all of them: a deployment can satisfy it perfectly and still fail,
// because one end trimmed and the other did not, or one preferred the file and
// the other the inline value. The symptom is always the same pin failure, and
// nothing in it says which of the two resolved differently.
//
// The backend asserts the identical set against `resolveSecret`
// (backend/internal/addons/derive_test.go). Two copies on purpose — the claim is
// that two separately deployed binaries agree, and a shared helper would make
// them agree by construction while the deployment still disagreed.

func TestSecretValueResolution(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "s.key")
	if err := os.WriteFile(path, []byte("  the-secret\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	// 8.1 — the file wins where both are set. Divergence here means the two ends
	// read different secrets while each reports itself correctly configured.
	t.Run("the file wins over the inline value", func(t *testing.T) {
		t.Setenv("ADDON_X_SECRET", "the-inline-one")
		t.Setenv("ADDON_X_SECRET_FILE", path)
		got, err := secretValue("ADDON_X_SECRET")
		if err != nil {
			t.Fatal(err)
		}
		if got != "the-secret" {
			t.Fatalf("got %q; the file must win and must be trimmed", got)
		}
	})

	// 8.2 — the likeliest real-world mismatch: almost every mounted secret
	// carries a trailing newline the operator never typed, and one byte of
	// difference derives an entirely different key.
	t.Run("a file with a trailing newline equals an inline value without one", func(t *testing.T) {
		t.Setenv("ADDON_X_SECRET_FILE", path)
		fromFile, err := secretValue("ADDON_X_SECRET")
		if err != nil {
			t.Fatal(err)
		}
		t.Setenv("ADDON_X_SECRET_FILE", "")
		t.Setenv("ADDON_X_SECRET", "the-secret")
		inline, err := secretValue("ADDON_X_SECRET")
		if err != nil {
			t.Fatal(err)
		}
		if fromFile != inline {
			t.Fatalf("file %q and inline %q resolve differently", fromFile, inline)
		}
		// And they derive the same keys, which is the property that actually
		// matters — the string equality above is how, not what.
		a, err := deriveHMACKey([]byte(fromFile), "truenas")
		if err != nil {
			t.Fatal(err)
		}
		b, err := deriveHMACKey([]byte(inline), "truenas")
		if err != nil {
			t.Fatal(err)
		}
		if string(a) != string(b) {
			t.Fatal("the two forms derive different HMAC keys")
		}
	})

	// 8.3 — an empty file is a mount that did not land. Falling back would start
	// the add-on under whichever value the other variable happened to hold, which
	// is the failure the fallback looks most helpful in.
	t.Run("an empty file is an error, not a fallback", func(t *testing.T) {
		empty := filepath.Join(dir, "empty.key")
		if err := os.WriteFile(empty, []byte("   \n"), 0o600); err != nil {
			t.Fatal(err)
		}
		t.Setenv("ADDON_X_SECRET", "the-inline-one")
		t.Setenv("ADDON_X_SECRET_FILE", empty)
		if _, err := secretValue("ADDON_X_SECRET"); err == nil {
			t.Fatal("an empty secret file fell back to the inline value")
		}
	})

	// 8.4 — failing to distinguish "no secret configured" from "the mount is
	// missing" is what turns a five-minute fix into an afternoon.
	t.Run("a missing file names the path", func(t *testing.T) {
		t.Setenv("ADDON_X_SECRET_FILE", "/nonexistent/s.key")
		_, err := secretValue("ADDON_X_SECRET")
		if err == nil {
			t.Fatal("a missing secret file must be an error")
		}
		if !strings.Contains(err.Error(), "/nonexistent/s.key") {
			t.Fatalf("the error must name the path, got %q", err)
		}
	})

	t.Run("nothing configured is empty, not an error", func(t *testing.T) {
		t.Setenv("ADDON_X_SECRET", "")
		t.Setenv("ADDON_X_SECRET_FILE", "")
		got, err := secretValue("ADDON_X_SECRET")
		if err != nil || got != "" {
			// Startup decides what an absent secret means — for ADDON_SECRET it
			// is fatal, and that refusal belongs with the caller, not here.
			t.Fatalf("got %q, %v", got, err)
		}
	})
}
