package main

import (
	"crypto/tls"
	"encoding/json"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// The conformance kit (addons/conformance, design §7) against THIS add-on:
// the real routes, the real authenticator and the real derived TLS key, in
// process, over a fake NAS that holds state as a NAS does. The kit is driven
// as the operator drives it, through its binary, so this module needs no
// dependency on the kit's (the image builds from this directory alone, and the
// kit stays a black box that cannot see this add-on's types).
//
// It runs reads-only, then -mutate, then -mutate -mutate-settings with another
// subject, and every run must report no MUST failure. Task 1 of
// openspec/changes/addon-contract: the kit passes against the contract-2
// TrueNAS add-on before the deploy (1.10).

const (
	kitTarget = "truenas"
	kitSecret = "conformance-truenas-test-secret-not-a-credential"
)

// conformanceKit builds the kit binary into the test's own temporary
// directory, which the test removes: nothing is left behind.
func conformanceKit(t *testing.T) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "syndra-addon-conform")
	cmd := exec.Command("go", "build", "-o", bin, "./cmd/syndra-addon-conform")
	cmd.Dir = filepath.Join("..", "conformance")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build the conformance kit: %v\n%s", err, out)
	}
	return bin
}

// kitAddOn is the add-on as main.go builds it, over a NAS fake that holds the
// accounts a run creates and changes: one member account already there, the
// shares and folders of reachFake, and a place chosen for home folders.
func kitAddOn(t *testing.T) string {
	t.Helper()
	fake := reachFake()
	fake.users = `[{"username":"rey_mysterio","id":11,"uid":3051,"locked":false,"smb":true,"groups":[51],"home":"/var/empty"}]`
	fake.refuse = map[string]string{"filesystem.stat": "[ENOENT] No such file or directory"}
	m := &mutatingRPC{fakeRPC: *fake}
	s := testServer(t, &m.fakeRPC)
	s.nas = newNAS(func() (rpc, error) { return m, nil }, []string{"25.04"})
	s.nas.version, s.nas.probed = "25.04.2", true
	key, err := deriveHMACKey([]byte(kitSecret), kitTarget)
	if err != nil {
		t.Fatal(err)
	}
	s.auth = &authenticator{signingKey: key, now: time.Now}
	// The kit waits the cooldown out before each freshness check.
	s.fresh.cooldown = time.Second
	if err := s.store.PutSetting(settingHomeRoot, Setting{Value: "/mnt/tank/main/home_dir", SetBy: "operator", SetAt: time.Now().UTC()}); err != nil {
		t.Fatal(err)
	}

	priv, err := deriveTLSKey([]byte(kitSecret), kitTarget)
	if err != nil {
		t.Fatal(err)
	}
	cert, err := selfSignedCert(priv, kitTarget)
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewUnstartedServer(s.routes())
	srv.TLS = &tls.Config{MinVersion: tls.VersionTLS13, Certificates: []tls.Certificate{cert}}
	// The kit is one client making one request at a time; the fake is not
	// safe for more, and nothing here needs more.
	srv.StartTLS()
	t.Cleanup(srv.Close)
	return srv.URL
}

type kitReport struct {
	Mode         string `json:"mode"`
	MustFailures int    `json:"must_failures"`
	Pass         int    `json:"pass"`
	Left         string `json:"left"`
	Results      []struct {
		ID, Status, Level string
		Detail            []string
	} `json:"results"`
}

func runKit(t *testing.T, bin, url string, args ...string) kitReport {
	t.Helper()
	secret := filepath.Join(t.TempDir(), "secret")
	if err := os.WriteFile(secret, []byte(kitSecret), 0o600); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(bin, append([]string{"-url", url, "-target", kitTarget, "-secret-file", secret, "-json", "-"}, args...)...)
	out, err := cmd.Output()
	var r kitReport
	if jerr := json.Unmarshal(out, &r); jerr != nil {
		t.Fatalf("kit %v: %v, output %s", args, err, out)
	}
	for _, res := range r.Results {
		if res.Status == "fail" {
			t.Errorf("kit %v: %s %s: %s", args, res.Level, res.ID, strings.Join(res.Detail, "; "))
		}
	}
	if r.MustFailures != 0 || err != nil {
		t.Errorf("kit %v: %d MUST failures (exit %v); left: %s", args, r.MustFailures, err, r.Left)
	}
	return r
}

func TestTheConformanceKitPassesThisAddOn(t *testing.T) {
	if testing.Short() {
		t.Skip("builds and runs the conformance kit")
	}
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("no go toolchain to build the kit with")
	}
	bin := conformanceKit(t)
	url := kitAddOn(t)

	reads := runKit(t, bin, url)
	mutate := runKit(t, bin, url, "-mutate", "-subject", "conformance-truenas-1")
	settings := runKit(t, bin, url, "-mutate", "-mutate-settings", "-subject", "conformance-truenas-2")
	for _, r := range []kitReport{reads, mutate, settings} {
		if r.Pass == 0 {
			t.Errorf("a %s run passed nothing, so it proved nothing", r.Mode)
		}
	}
	// The run that writes judges what reads cannot: these must have run.
	for _, id := range []string{"replay", "replay.refusal", "read-back", "limit-combine", "limit-empty", "operations.reads", "health", "channel-follows-account",
		"withdraw-only.existing", "lifecycle", "operations", "plan-equals-apply", "create-is-new", "bound-to-created", "results",
		"freshness.cached", "freshness.cooldown", "freshness.coalesce"} {
		for _, res := range mutate.Results {
			if res.ID == id && res.Status != "pass" {
				t.Errorf("-mutate: %s is %s, want pass: %v", id, res.Status, res.Detail)
			}
		}
	}
	if !strings.HasPrefix(mutate.Left, "one disabled account") {
		t.Errorf("-mutate left %q, want one disabled account", mutate.Left)
	}
}
