package main

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"
)

// 3.3, the half no numbers can assert: a mutation ACTUALLY IN FLIGHT when the
// stop arrives settles, and its terminal status is written.
//
// Everything else in this change asserts the budget — `stop_grace_period`
// against `shutdownTimeout`, the binary entering and completing a drain on a
// real SIGTERM. All of it is satisfied by a drain that has nothing to drain.
// The defect this change exists to remove is a write abandoned half-applied
// with no record of how far it got, and the only thing that can demonstrate its
// absence is a write that is genuinely underway when the shutdown begins.
//
// What produces the window here is a target that has not answered yet — the
// same thing that produces it in production, where it is a NAS under load. The
// rest of the path is real: the HTTP server that main.go builds, the routes it
// serves, the authenticator, the lifecycle, and `Shutdown` called exactly as
// the shutdown sequence calls it.
//
// What this still does NOT cover is a real NAS being slow, which is why 3.3
// stays open against hardware. It covers everything on this side of that call.

// blockingRPC holds `user.update` open until a test releases it, and announces
// when a call has arrived so the test never has to guess.
type blockingRPC struct {
	mutatingRPC
	entered chan struct{}
	release chan struct{}
}

func (b *blockingRPC) Call(method string, timeout int64, params any) (json.RawMessage, error) {
	if method == "user.update" {
		// Announced before blocking, so the shutdown starts with the handler
		// demonstrably inside the target call rather than merely dispatched.
		select {
		case b.entered <- struct{}{}:
		default:
		}
		<-b.release
	}
	return b.mutatingRPC.Call(method, timeout, params)
}

func TestAMutationInFlightSettlesRatherThanBeingAbandoned(t *testing.T) {
	users := `[{"username":"ada","id":11,"uid":3001,"locked":true,"smb":false,"groups":[42]}]`
	b := &blockingRPC{
		mutatingRPC: mutatingRPC{fakeRPC: fakeRPC{users: users, groups: fixtureGroups}},
		entered:     make(chan struct{}, 1),
		release:     make(chan struct{}),
	}
	s := testServer(t, &b.fakeRPC)
	s.nas = newNAS(func() (rpc, error) { return b, nil }, []string{"25.04"})
	s.nas.version, s.nas.probed = "25.04.2", true
	s.auth = signedAuth(time.Now())
	if err := s.store.PutBinding(Binding{SubjectID: "sub-1", Username: "ada", UID: 3001}); err != nil {
		t.Fatal(err)
	}

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	srv := &http.Server{Handler: s.routes()}
	go func() { _ = srv.Serve(ln) }()
	base := "http://" + ln.Addr().String()

	// Both bodies are built BEFORE anything is in flight. `withFingerprint`
	// reads the subject, the add-on serialises target calls, and a read issued
	// while the blocking write is held would queue behind it — the test would
	// deadlock waiting to construct the request it needs in order to proceed.
	body := withContractVersion(t, withFingerprint(t, s, `{"call_id":"c1","subject":"sub-1",
		"email":"ada@x.edu","desired":{"group":["lab_makers"],"enabled":true}}`))
	second := withContractVersion(t, withFingerprint(t, s,
		`{"call_id":"c2","subject":"sub-1","email":"ada@x.edu","desired":{"enabled":false}}`))

	applied := make(chan int, 1)
	go func() {
		resp, err := http.DefaultClient.Do(liveSigned(t, base, http.MethodPost, "/apply", body))
		if err != nil {
			applied <- 0
			return
		}
		defer resp.Body.Close()
		_, _ = io.Copy(io.Discard, resp.Body)
		applied <- resp.StatusCode
	}()

	select {
	case <-b.entered:
	case <-time.After(5 * time.Second):
		t.Fatal("the apply never reached the target call")
	}
	if s.life.InFlight() != 1 {
		t.Fatalf("want one mutation in flight, got %d", s.life.InFlight())
	}

	// The shutdown sequence, in main.go's order: draining first, so nothing new
	// is accepted, and only then the wait.
	if err := s.life.Set(LifecycleDraining, "shutting down"); err != nil {
		t.Fatal(err)
	}
	if s.life.Drained() {
		t.Fatal("a drain must not report itself finished with a mutation still in flight")
	}
	// A second mutation arriving mid-drain is refused rather than queued behind
	// the first — the half of `draining` that keeps the window from reopening.
	refused, err := http.DefaultClient.Do(liveSigned(t, base, http.MethodPost, "/apply", second))
	if err != nil {
		t.Fatal(err)
	}
	refused.Body.Close()
	if refused.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("a draining add-on must refuse new mutations, got %d", refused.StatusCode)
	}

	stopped := make(chan error, 1)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		stopped <- srv.Shutdown(ctx)
	}()

	// Shutdown must still be waiting: the handler is inside the target call.
	select {
	case err := <-stopped:
		t.Fatalf("shutdown returned while a mutation was in flight: %v", err)
	case <-time.After(150 * time.Millisecond):
	}

	close(b.release)

	select {
	case code := <-applied:
		if code != http.StatusOK {
			t.Fatalf("the in-flight mutation must complete, got status %d", code)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("the in-flight mutation never completed")
	}
	select {
	case err := <-stopped:
		if err != nil {
			t.Fatalf("shutdown: %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("shutdown never returned after the mutation settled")
	}

	if !s.life.Drained() || s.life.InFlight() != 0 {
		t.Fatalf("the drain must be finished: drained=%t in_flight=%d", s.life.Drained(), s.life.InFlight())
	}

	// The terminal status, which is the whole point. A write that completed and
	// left no record is the same incident as one that was abandoned: nobody can
	// say afterwards how far it got.
	written, err := os.ReadFile(mutationLogPath(t, s))
	if err != nil {
		t.Fatal(err)
	}
	var settled bool
	for _, line := range strings.Split(string(written), "\n") {
		if line == "" {
			continue
		}
		var r Record
		if err := json.Unmarshal([]byte(line), &r); err != nil {
			t.Fatalf("mutation log line does not decode: %v", err)
		}
		if r.Operation == "apply" && r.Subject == "sub-1" && r.Outcome == "succeeded" {
			settled = true
		}
	}
	if !settled {
		t.Fatalf("the settled mutation must have written its terminal record:\n%s", written)
	}
}

// liveSigned signs a request aimed at a real listener, using the same MAC input
// the add-on verifies. Built here rather than reusing the httptest helper
// because that one produces a server-side request, which no client can send.
func liveSigned(t *testing.T, base, method, path, body string) *http.Request {
	t.Helper()
	ts := time.Now()
	mac := computeMAC(ts.Unix(), method, path, []byte(body), []byte(testKey))
	r, err := http.NewRequest(method, base+path, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	r.Header.Set(SignatureHeader, fmt.Sprintf("t=%s,v1=%s",
		strconv.FormatInt(ts.Unix(), 10), hex.EncodeToString(mac)))
	return r
}
