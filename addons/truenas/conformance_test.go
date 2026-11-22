package main

import (
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
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
	// kitHomeRoot is reachFake's main › home_dir, a dataset of its own.
	kitHomeRoot = "/mnt/tank/main/home_dir"
	kitTarget   = "truenas"
	kitSecret   = "conformance-truenas-test-secret-not-a-credential"
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

// kitAddon is one add-on the kit runs against, and the file whose presence
// switches its stub NAS off: the app hooks are touch and rm, as the
// reference's are.
type kitAddon struct{ url, nasOff string }

// stubNAS is the NAS fake behind the add-on's own dialer seam (rpc in nas.go),
// switched off by a file. Off, a dial is refused and a call on a session
// already open fails on its socket, as a NAS that went away does: everything
// above the seam (the session, ErrNotSent, the breaker, the fresher) runs as
// it does against TrueNAS. Holding "hang", the file makes it a blackholed NAS
// instead: a dial or a call answers nothing until the file goes, then times
// out, as the 8g staging run's unroutable address did (22 Nov 2026). ponytail:
// the seam, not a websocket server; the fake answers Go-typed params the wire
// would turn into float64 and []any.
type stubNAS struct {
	*mutatingRPC
	off string
}

func (n stubNAS) down() bool {
	_, err := os.Stat(n.off)
	return err == nil
}

func (n stubNAS) hung() bool {
	b, err := os.ReadFile(n.off)
	return err == nil && strings.TrimSpace(string(b)) == "hang"
}

// gone is what the NAS answers now: nothing (after waiting out a hang, or
// the wait's timeout when it has one, as the client's Call does), or the
// failure of a NAS that is not there.
func (n stubNAS) gone(failure string, timeout time.Duration) error {
	if !n.down() {
		return nil
	}
	if n.hung() {
		for until := time.Now().Add(timeout); n.hung() && (timeout == 0 || time.Now().Before(until)); {
			time.Sleep(20 * time.Millisecond)
		}
		return errors.New("dial tcp: i/o timeout")
	}
	return errors.New(failure)
}

func (n stubNAS) Call(method string, timeout int64, params any) (json.RawMessage, error) {
	if err := n.gone("failed to send call: use of closed network connection", time.Duration(timeout)*time.Second); err != nil {
		return nil, err
	}
	return n.mutatingRPC.Call(method, timeout, params)
}

func (n stubNAS) dial() (rpc, error) {
	if err := n.gone("failed to connect: connection refused", 0); err != nil {
		return nil, err
	}
	return n, nil
}

// kitAddOn is the add-on as main.go builds it, over a NAS fake that holds the
// accounts a run creates and changes: one member account already there, the
// shares and folders of reachFake, and homeRoot as the place chosen for home
// folders ("" for none chosen, as on a NAS where nobody has chosen one yet).
func kitAddOn(t *testing.T, homeRoot string) kitAddon {
	t.Helper()
	fake := reachFake()
	fake.users = `[{"username":"rey_mysterio","id":11,"uid":3051,"locked":false,"smb":true,"groups":[51],"home":"/var/empty"}]`
	fake.refuse = map[string]string{"filesystem.stat": "[ENOENT] No such file or directory"}
	// What the statistics collector reads: an account no binding names
	// connecting to main, audited for everyone, and using it; nothing in
	// open, whose auditing is off.
	fake.shares = `[{"name":"main","path":"/mnt/tank/main","enabled":true,"audit":{"enable":true,"watch_list":[],"ignore_list":[]}},
	                {"name":"old","path":"/mnt/tank/old","enabled":false},
	                {"name":"open","path":"/mnt/tank/open","enabled":true}]`
	fake.audit = fmt.Sprintf(`[{"message_timestamp":%d,"event":"CONNECT","success":true,"username":"rey_mysterio","session":"s1",
		"service_data":{"service":"main"}}]`, time.Now().Unix())
	fake.answers["pool.dataset.get_quota tank/main"] = `[{"id":3051,"used_bytes":1234}]`
	fake.answers["pool.dataset.get_quota tank/open"] = `[]`
	m := &mutatingRPC{fakeRPC: *fake, folders: []string{kitHomeRoot}}
	s := testServer(t, &m.fakeRPC)
	nas := stubNAS{m, filepath.Join(t.TempDir(), "nas-off")}
	s.nas = newNAS(nas.dial, []string{"25.04"})
	// Short bounds, so a hang costs the run a second, not nasDialTimeout,
	// callTimeout and pingTimeout (TestNASBoundsFitUnderTheBackendsCallTimeout
	// holds those), and the floor after it kitFloor, not reconnectCooldown.
	s.nas.dialWithin, s.nas.callWithin, s.nas.pingWithin, s.nas.floor = time.Second, time.Second, time.Second, kitFloor
	s.nas.version, s.nas.probed = "25.04.2", true
	s.nas.selfName, s.nas.selfUID = "syndra", 3001 // auth.me has named the add-on's own account, as after its first probe
	key, err := deriveHMACKey([]byte(kitSecret), kitTarget)
	if err != nil {
		t.Fatal(err)
	}
	s.auth = &authenticator{signingKey: key, now: time.Now}
	// The kit waits the cooldown out before each freshness check.
	s.fresh.cooldown = time.Second
	// One collection before the kit runs, and no collector after it
	// (STATS_COLLECT_EVERY=0): its reads would move target_reads under the
	// freshness checks. every is the default interval, for the chain of
	// sightings alone: nothing here runs the timer.
	s.stats = statsClock{zone: kolkata, every: time.Hour}
	recordingSince(t, s, map[string]time.Time{"main": time.Now().AddDate(0, 0, -2)}) // main seen recording everyone before today
	s.collectStatsOnce(t.Context())
	if homeRoot != "" {
		if err := s.store.PutSetting(settingHomeRoot, Setting{Value: homeRoot, SetBy: "operator", SetAt: time.Now().UTC()}); err != nil {
			t.Fatal(err)
		}
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
	return kitAddon{srv.URL, nas.off}
}

// kitFloor is the add-on's reconnect floor in the kit runs (kitAddOn).
const kitFloor = time.Second

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

// runKit runs the kit as CI must: with the app hooks and -require-outage
// (guide, "The outage checks"). The NAS back, the add-on dials it again only
// once its reconnect floor has passed since the dial that failed, so the
// start hook waits that out: until then the app is not back for the add-on.
func runKit(t *testing.T, bin string, a kitAddon, args ...string) kitReport {
	t.Helper()
	secret := filepath.Join(t.TempDir(), "secret")
	if err := os.WriteFile(secret, []byte(kitSecret), 0o600); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(bin, append([]string{"-url", a.url, "-target", kitTarget, "-secret-file", secret, "-json", "-",
		"-require-outage", "-stop-app", "touch '" + a.nasOff + "'", "-hang-app", "printf hang > '" + a.nasOff + "'",
		"-start-app", fmt.Sprintf("rm -f '%s' && sleep %d", a.nasOff, int(kitFloor/time.Second)+1)}, args...)...)
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
	addon := kitAddOn(t, kitHomeRoot)

	reads := runKit(t, bin, addon)
	mutate := runKit(t, bin, addon, "-mutate", "-subject", "conformance-truenas-1")
	settings := runKit(t, bin, addon, "-mutate", "-mutate-settings", "-subject", "conformance-truenas-2")
	for _, r := range []kitReport{reads, mutate, settings} {
		if r.Pass == 0 {
			t.Errorf("a %s run passed nothing, so it proved nothing", r.Mode)
		}
	}
	// The run that writes judges what reads cannot: these must have run.
	for _, id := range []string{"requests.exact-names", "replay", "replay.refusal", "read-back", "limit-combine", "limit-empty", "operations.reads", "health", "channel-follows-account",
		"withdraw-only.existing", "lifecycle", "operations", "plan-equals-apply", "create-is-new", "bound-to-created", "operations.result",
		"freshness.cached", "freshness.cooldown", "freshness.coalesce", "calls.unknown", "calls.recorded",
		"stats.descriptors", "stats.points", "stats.totals", "stats.since", "stats.idempotent", "stats.gaps",
		"stats.unbound", "stats.buffer", "stats.page-size", "stats.no-people", "stats.no-people.ids",
		"adopt.shown", "adopt.not-shown", "release.not-shown", "outage.health", "outage.hang", "outage.write", "outage.kept", "no-false-zero"} {
		ran := false
		for _, res := range mutate.Results {
			if res.ID == id {
				ran = true
				if res.Status != "pass" {
					t.Errorf("-mutate: %s is %s, want pass: %v", id, res.Status, res.Detail)
				}
			}
		}
		if !ran {
			t.Errorf("-mutate: %s did not run", id)
		}
	}
	if !strings.HasPrefix(mutate.Left, "one disabled account") {
		t.Errorf("-mutate left %q, want one disabled account", mutate.Left)
	}
}

// The staging run of 14 Nov 2026 against TrueNAS 25.10.7: no place chosen for
// home folders, so the kit's account got none, and with no home there is no
// limit to read. Its state carries no home_quota, which is "not known", never
// "no limit" (homeLimits), so neither limit check can judge it: each is a skip
// that says why, never the FAIL that run reported for limit-combine, and never
// the pass it reported for limit-empty on reads that saw no limit.
func TestTheKitSkipsALimitThisAddOnDoesNotReport(t *testing.T) {
	if testing.Short() {
		t.Skip("builds and runs the conformance kit")
	}
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("no go toolchain to build the kit with")
	}
	mutate := runKit(t, conformanceKit(t), kitAddOn(t, ""), "-mutate", "-subject", "conformance-truenas-3")
	for _, id := range []string{"limit-combine", "limit-empty"} {
		found := false
		for _, res := range mutate.Results {
			if res.ID != id {
				continue
			}
			found = true
			if res.Status != "skip" || !strings.Contains(strings.Join(res.Detail, " "), "home_quota is not in the kit's account's state") {
				t.Errorf("%s with no home folder: %s %v, want a skip saying home_quota is not read", id, res.Status, res.Detail)
			}
		}
		if !found {
			t.Errorf("%s did not run", id)
		}
	}
}

// syndra-addon-call's exact bodies, through this add-on's real decodeStrict
// and handlers: a home_root set and clear, and a release of a conformance-
// subject, all accepted; and a refused body never reaches the add-on at all.
func TestTheAddonCallToolIsAcceptedByThisAddOn(t *testing.T) {
	if testing.Short() {
		t.Skip("builds and runs the call tool")
	}
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("no go toolchain to build the tool with")
	}
	bin := filepath.Join(t.TempDir(), "syndra-addon-call")
	build := exec.Command("go", "build", "-o", bin, "./cmd/syndra-addon-call")
	build.Dir = filepath.Join("..", "conformance")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build the tool: %v\n%s", err, out)
	}
	url := kitAddOn(t, "").url
	secret := filepath.Join(t.TempDir(), "secret")
	if err := os.WriteFile(secret, []byte(kitSecret), 0o600); err != nil {
		t.Fatal(err)
	}
	call := func(args ...string) (string, error) {
		out, err := exec.Command(bin, append([]string{"-url", url, "-target", kitTarget, "-secret-file", secret}, args...)...).CombinedOutput()
		return string(out), err
	}
	for _, c := range [][]string{
		{"PUT", "/settings/home_root", `{"value":"` + kitHomeRoot + `"}`},
		{"GET", "/settings"},
		{"PUT", "/settings/home_root", `{"value":""}`},
		{"POST", "/operations/account.release", `{"subject":"conformance-wwe-rey"}`},
	} {
		if out, err := call(c...); err != nil || !strings.HasPrefix(out, "200\n") {
			t.Errorf("%v: %v, %s", c, err, out)
		}
	}
	if out, err := call("POST", "/operations/account.release", `{"subject":"conformance-x","ſubject":"wwe-the-rock"}`); func() int {
		ee, _ := err.(*exec.ExitError)
		if ee == nil {
			return -1
		}
		return ee.ExitCode()
	}() != 2 {
		t.Errorf("a folded key must be refused before sending (exit 2): %v, %s", err, out)
	}
}
