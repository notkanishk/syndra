package main

import (
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

// The shipped binary's shutdown path, run for real.
//
// `shutdown_grace_test.go` asserts the NUMBERS — that Compose's
// `stop_grace_period` exceeds this package's `shutdownTimeout`, so Docker
// cannot cut the drain short. That is the defect that was there, and the guard
// belongs. But numbers are all it asserts: a drain that never ran, or one that
// hung past its own deadline, would satisfy it exactly.
//
// This runs the real binary and sends it the real signal. What it can prove
// without a NAS is that the process enters its drain, completes it, and exits
// on its own rather than being killed — well inside the budget Compose grants.
//
// What it cannot prove is the part needing hardware: that a mutation ACTUALLY
// IN FLIGHT against a target settles rather than being abandoned half-applied.
// That is `addon-shutdown-grace-period` 3.3 and it stays operator-gated. This
// closes the half that does not need a NAS, and is written so nobody mistakes
// it for the other half.
func TestTheBinaryDrainsAndExitsOnSIGTERM(t *testing.T) {
	if testing.Short() {
		t.Skip("builds and runs the add-on binary")
	}

	bin := filepath.Join(t.TempDir(), "truenas-addon")
	build := exec.Command("go", "build", "-o", bin, ".")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("building: %v\n%s", err, out)
	}

	// A port the OS just told us is free — the binary logs the address it was
	// given, so :0 would leave nothing to wait on.
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
		"ADDON_TARGET=truenas",
		"ADDON_SECRET=a-secret-for-the-shutdown-test",
		// Deliberately unreachable: nothing here touches the NAS, and a value
		// that could accidentally resolve would make a slow shutdown ambiguous
		// between the drain and a hanging target call.
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
	defer func() {
		_ = cmd.Process.Kill()
	}()

	waitForListener(t, addr, cmd, &out)

	if err := cmd.Process.Signal(syscall.SIGTERM); err != nil {
		t.Fatal(err)
	}

	// The budget the deployment grants, not an arbitrary one: Compose's
	// stop_grace_period is what stands between the drain and a SIGKILL, and the
	// process must finish inside it without help.
	grace, err := composeStopGracePeriod("truenas-addon")
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()

	select {
	case err := <-done:
		// A clean exit. `log.Fatalf` would be a non-zero status, and so would a
		// panic during the drain — either is the drain failing, not finishing.
		if err != nil {
			t.Fatalf("the add-on exited badly on SIGTERM: %v\n%s", err, out.String())
		}
	case <-time.After(grace):
		t.Fatalf("the add-on did not finish its drain within the %s Compose grants it; "+
			"Docker would have SIGKILLed it here, which is the defect this whole "+
			"change exists to remove.\n%s", grace, out.String())
	}

	// And it drained rather than merely dying quickly. Without this, a process
	// that crashed on the signal would pass the timing assertion above.
	if !strings.Contains(out.String(), "[SHUTDOWN] draining") {
		t.Errorf("no drain was logged, so the process did not take the shutdown path:\n%s", out.String())
	}
}

func waitForListener(t *testing.T, addr string, cmd *exec.Cmd, out *strings.Builder) {
	t.Helper()
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", addr, time.Second)
		if err == nil {
			_ = conn.Close()
			return
		}
		if cmd.ProcessState != nil && cmd.ProcessState.Exited() {
			t.Fatalf("the add-on exited before serving:\n%s", out.String())
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("the add-on never accepted a connection on %s:\n%s", addr, out.String())
}
