package addons

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// The two binaries, actually talking to each other.
//
// Everything else in this package proves the transport against a harness that
// derives its key with THIS package's `deriveTLSSeed`. That is deliberate — a
// harness with its own copy of the HKDF parameters would agree with itself
// while the backend drifted — but it leaves one claim resting entirely on
// `addons/contract/transport_derivation.json`: that the separately compiled
// add-on, in its own module, derives the same bytes.
//
// The vector is a strong tie and it is not a handshake. This is the handshake:
// the real add-on binary is built and run, and the backend's real client dials
// it, pins its key, signs a request and reads the manifest back. If the two
// modules ever disagree about the salt, the info strings, the hash, or the
// encoding of the secret, this fails where nothing else would until a
// deployment did.
//
// §17 is why this exists. The backend and the add-on had never spoken to each
// other, each suite was written against its own fake, and the two fakes agreed
// — so every real /apply would have been answered 400 and neither suite could
// see it.
//
// No NAS is involved. `/capabilities` is served whether or not the target is
// reachable (addons/truenas/server.go), which is what makes this runnable
// anywhere: the leg under test is backend <-> add-on, and pulling the NAS into
// it would recreate the exact ambiguity the bring-up order exists to prevent.
func TestTheRealAddOnBinaryAndThisClientAuthenticateEachOther(t *testing.T) {
	if testing.Short() {
		t.Skip("builds and runs the add-on binary")
	}
	const secret = "a-deployment-secret-for-the-cross-binary-test"

	bin := buildAddOnBinary(t)
	addr := runAddOn(t, bin, secret, "truenas")

	reg := Registration{
		Target:  "truenas",
		BaseURL: "https://" + addr,
		Secret:  []byte(secret),
	}

	t.Run("the manifest reads over the derived transport", func(t *testing.T) {
		resetRegistry(t)
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()

		m, err := httpFetchManifest(ctx, reg)
		if err != nil {
			t.Fatalf("the real add-on refused the real client: %v\n"+
				"This is the disagreement the contract vector exists to prevent — "+
				"a wrong salt, info string, hash or secret encoding on either side "+
				"produces exactly this, with both ends internally consistent.", err)
		}
		if m.ContractVersion != ContractVersion {
			t.Errorf("contract version = %d, want %d", m.ContractVersion, ContractVersion)
		}
		// Proof the body is the add-on's own and not an empty struct that
		// happened to decode.
		if len(m.Operations) == 0 {
			t.Error("the manifest carried no operations")
		}
	})

	t.Run("a client holding a different secret never reaches it", func(t *testing.T) {
		resetRegistry(t)
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()

		wrong := reg
		wrong.Secret = []byte("a-secret-this-deployment-never-had")

		_, err := httpFetchManifest(ctx, wrong)
		if err == nil {
			t.Fatal("a client that does not hold the deployment secret read the manifest")
		}
		// And it failed at the PIN rather than anywhere later: the handshake
		// ends before a request is written, which is what keeps a body carrying
		// declared secret_params off the wire.
		var mismatch pinMismatchError
		if !errors.As(err, &mismatch) {
			t.Fatalf("expected the pin to reject it, got %T: %v", err, err)
		}
	})
}

// buildAddOnBinary compiles the add-on from source, in its own module.
func buildAddOnBinary(t *testing.T) string {
	t.Helper()
	src, err := filepath.Abs(filepath.Join("..", "..", "..", "addons", "truenas"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(src, "go.mod")); err != nil {
		t.Skipf("add-on module not present at %s", src)
	}
	bin := filepath.Join(t.TempDir(), "truenas-addon")

	cmd := exec.Command("go", "build", "-o", bin, ".")
	cmd.Dir = src
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("building the add-on: %v\n%s", err, out)
	}
	return bin
}

// runAddOn starts the binary as a deployment would and returns its address.
func runAddOn(t *testing.T, bin, secret, target string) string {
	t.Helper()

	// A port the OS just told us is free. The add-on logs the address it was
	// GIVEN, so :0 would leave nothing to dial.
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := l.Addr().String()
	if err := l.Close(); err != nil {
		t.Fatal(err)
	}

	state := t.TempDir()
	cmd := exec.Command(bin)
	cmd.Env = append(os.Environ(),
		"LISTEN_ADDR="+addr,
		"ADDON_TARGET="+target,
		"ADDON_SECRET="+secret,
		// Required config, deliberately unreachable. Nothing in this test
		// touches the NAS, and a value that could accidentally resolve would
		// make a failure here ambiguous between the two legs.
		"TRUENAS_URL=wss://127.0.0.1:1/api/current",
		"TRUENAS_API_KEY=not-a-real-key",
		"STATE_PATH="+filepath.Join(state, "state.db"),
		"MUTATION_LOG_DIR="+state,
	)
	var out strings.Builder
	cmd.Stdout, cmd.Stderr = &out, &out
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = cmd.Process.Kill()
		_, _ = cmd.Process.Wait()
		if t.Failed() {
			t.Logf("add-on output:\n%s", out.String())
		}
	})

	// Wait for the listener rather than for a log line: the log says it is
	// about to serve, the socket says it is.
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", addr, time.Second)
		if err == nil {
			_ = conn.Close()
			return addr
		}
		if cmd.ProcessState != nil && cmd.ProcessState.Exited() {
			t.Fatalf("the add-on exited before serving:\n%s", out.String())
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatal(fmt.Sprintf("the add-on never accepted a connection on %s:\n%s", addr, out.String()))
	return ""
}
