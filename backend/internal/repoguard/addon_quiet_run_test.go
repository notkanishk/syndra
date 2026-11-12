package repoguard

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// scripts/addon-quiet-run.sh quiets the TrueNAS add-on on staging for one
// conformance kit run: the collector off, the backend's health probe stopped.
// The owner's rule (13 Nov 2026): only ever temporary; afterwards it is back
// to the production setting. These run the real script against a fake
// `docker` that keeps the compose state in files.

const fakeDocker = `#!/bin/bash
S="$FAKE_STATE"
echo "$*" >> "$S/calls"
if [[ "$1" == compose ]]; then
  shift
  case "$1 ${*: -1}" in
    "ps truenas-addon") echo addon-id ;;
    "ps backend") [[ -e "$S/backend" ]] && echo backend-id ;;
    "port 8080") echo "0.0.0.0:18080" ;;
    "up truenas-addon")
      [[ "$*" == "up -d --no-deps --wait truenas-addon" ]] || { echo "fake docker: unexpected up $*" >&2; exit 9; }
      if [[ -e "$S/sticky" ]]; then exit 0; fi
      if [[ -e "$S/fail-quiet-up" && "${TRUENAS_STATS_COLLECT_EVERY:-}" == 0 ]]; then rm "$S/fail-quiet-up"; exit 1; fi
      echo "${TRUENAS_STATS_COLLECT_EVERY:-$FAKE_DOTENV}" > "$S/every"
      echo "${TRUENAS_LIFECYCLE_STATE:-read_only}" > "$S/life" ;;
    "stop backend") rm -f "$S/backend" ;;
    "start backend") [[ -e "$S/backend-broken" ]] || touch "$S/backend" ;;
    *) echo "fake docker: unexpected compose $*" >&2; exit 9 ;;
  esac
  exit 0
fi
if [[ "$1" == inspect ]]; then
  case "$3" in
    *Config.Env*) echo "TZ=Asia/Kolkata"; echo "STATS_COLLECT_EVERY=$(cat "$S/every")"; echo "LIFECYCLE_STATE=$(cat "$S/life")" ;;
    *NetworkSettings*) echo "172.20.0.5 " ;;
  esac
  exit 0
fi
echo "fake docker: unexpected $*" >&2; exit 9
`

// curl answers /healthz only while the fake backend runs; logger keeps what
// the script would send to syslog.
const fakeCurl = `#!/bin/bash
[[ -e "$FAKE_STATE/backend" ]]
`

const fakeLogger = `#!/bin/bash
echo "${*: -1}" >> "$FAKE_STATE/syslog"
`

type quietRun struct {
	state string
	out   string
	code  int
}

func runQuiet(t *testing.T, staging bool, setup func(state string), args ...string) quietRun {
	return runQuietTo(t, staging, setup, nil, args...)
}

// runQuietTo runs the script with its output going to out (nil: captured).
func runQuietTo(t *testing.T, staging bool, setup func(state string), out *os.File, args ...string) quietRun {
	t.Helper()
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("bash not available")
	}
	dir := t.TempDir()
	bin, state, marker := filepath.Join(dir, "bin"), filepath.Join(dir, "state"), filepath.Join(dir, "marker")
	for _, d := range []string{bin, state} {
		if err := os.Mkdir(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if staging {
		if err := os.Mkdir(marker, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for name, body := range map[string]string{"docker": fakeDocker, "curl": fakeCurl, "logger": fakeLogger} {
		if err := os.WriteFile(filepath.Join(bin, name), []byte(body), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	envFile := filepath.Join(dir, "env")
	if err := os.WriteFile(envFile, []byte("ZITADEL_DOMAIN=\nZITADEL_FAKE_WORLD=/var/lib/syndra-staging/world/world.json\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	write := func(name, s string) {
		if err := os.WriteFile(filepath.Join(state, name), []byte(s), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("every", "PT1H\n")
	write("life", "read_only\n")
	write("backend", "")
	if setup != nil {
		setup(state)
	}
	cmd := exec.Command("bash", append([]string{filepath.Join(repoRoot(t), "scripts", "addon-quiet-run.sh")}, args...)...)
	cmd.Env = append(os.Environ(),
		"PATH="+bin+string(os.PathListSeparator)+os.Getenv("PATH"),
		"FAKE_STATE="+state,
		"FAKE_DOTENV=PT1H",
		"STAGING_MARKER="+marker,
		"SYNDRA_ENV_FILE="+envFile,
		"COMPOSE_FILE=",
	)
	var captured []byte
	var err error
	if out != nil {
		cmd.Stdout, cmd.Stderr = out, out
		err = cmd.Run()
	} else {
		captured, err = cmd.CombinedOutput()
	}
	code := 0
	if ee, ok := err.(*exec.ExitError); ok {
		code = ee.ExitCode()
	} else if err != nil {
		t.Fatal(err)
	}
	return quietRun{state: state, out: string(captured), code: code}
}

func (r quietRun) file(t *testing.T, name string) (string, bool) {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(r.state, name))
	if os.IsNotExist(err) {
		return "", false
	}
	if err != nil {
		t.Fatal(err)
	}
	return strings.TrimSpace(string(b)), true
}

// The command runs quiet, and afterwards the add-on and backend are as before.
func TestAddonQuietRunQuietsThenRestores(t *testing.T) {
	r := runQuiet(t, true, nil, "--", "bash", "-c",
		`echo "during every=$(cat "$FAKE_STATE/every") backend=$([[ -e "$FAKE_STATE/backend" ]] && echo up || echo down) ip=$ADDON_IP"`)
	if r.code != 0 {
		t.Fatalf("exit %d\n%s", r.code, r.out)
	}
	if !strings.Contains(r.out, "during every=0 backend=down ip=172.20.0.5") {
		t.Errorf("the command did not run quiet:\n%s", r.out)
	}
	if every, _ := r.file(t, "every"); every != "PT1H" {
		t.Errorf("STATS_COLLECT_EVERY after = %q, want the production PT1H", every)
	}
	if _, up := r.file(t, "backend"); !up {
		t.Error("the backend was not started again")
	}
}

// A failing command (the kit finding a defect) still restores, and its exit
// status is the script's.
func TestAddonQuietRunRestoresWhenTheCommandFails(t *testing.T) {
	r := runQuiet(t, true, nil, "--", "bash", "-c", "exit 3")
	if r.code != 3 {
		t.Fatalf("exit %d, want the command's 3\n%s", r.code, r.out)
	}
	if every, _ := r.file(t, "every"); every != "PT1H" {
		t.Errorf("STATS_COLLECT_EVERY after a failed run = %q, want PT1H", every)
	}
	if _, up := r.file(t, "backend"); !up {
		t.Error("the backend was not started again after a failed run")
	}
}

// A restore that does not take is reported, never claimed.
func TestAddonQuietRunReportsARestoreThatDidNotTake(t *testing.T) {
	r := runQuiet(t, true, nil, "--", "bash", "-c", `touch "$FAKE_STATE/sticky"`)
	if r.code != 1 || !strings.Contains(r.out, "FAIL  STATS_COLLECT_EVERY") {
		t.Fatalf("exit %d, want 1 with a FAIL line\n%s", r.code, r.out)
	}
}

// A backend that was already stopped is left stopped.
func TestAddonQuietRunLeavesAStoppedBackendStopped(t *testing.T) {
	r := runQuiet(t, true, func(state string) { os.Remove(filepath.Join(state, "backend")) }, "--", "true")
	if r.code != 0 {
		t.Fatalf("exit %d\n%s", r.code, r.out)
	}
	if _, up := r.file(t, "backend"); up {
		t.Error("the script started a backend that was stopped before it ran")
	}
}

// Anywhere but staging it refuses before touching anything.
func TestAddonQuietRunRefusesOffStaging(t *testing.T) {
	r := runQuiet(t, false, nil, "--", "true")
	if r.code == 0 || !strings.Contains(r.out, "not the staging host") {
		t.Fatalf("exit %d, want a refusal\n%s", r.code, r.out)
	}
	if calls, _ := r.file(t, "calls"); calls != "" {
		t.Errorf("docker was called off staging: %q", calls)
	}
}

// The quiet recreate failing leaves TRUENAS_STATS_COLLECT_EVERY=0 in the
// script's environment; the restore must still bring back .env's value, which
// only removing the override (env -u) does.
func TestAddonQuietRunRestoresWhenQuietingItselfFails(t *testing.T) {
	r := runQuiet(t, true, func(state string) {
		os.WriteFile(filepath.Join(state, "fail-quiet-up"), nil, 0o644)
	}, "--", "true")
	if r.code == 0 {
		t.Fatalf("exit 0 after a failed quiet\n%s", r.out)
	}
	if every, _ := r.file(t, "every"); every != "PT1H" {
		t.Errorf("STATS_COLLECT_EVERY after a failed quiet = %q, want PT1H", every)
	}
	if _, up := r.file(t, "backend"); !up {
		t.Error("the backend was not started again after a failed quiet")
	}
}

// An ssh session that drops mid-run: SIGHUP, and stdout a pipe nobody reads.
// The restore must still run in full and say so in syslog.
func TestAddonQuietRunRestoresAfterAHangupWithNoOneReading(t *testing.T) {
	rd, wr, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	rd.Close()
	defer wr.Close()
	r := runQuietTo(t, true, nil, wr, "--", "bash", "-c", "kill -HUP $PPID; sleep 0.2")
	if r.code != 129 {
		t.Errorf("exit %d, want 129 (SIGHUP)", r.code)
	}
	if every, _ := r.file(t, "every"); every != "PT1H" {
		t.Errorf("STATS_COLLECT_EVERY after a hangup = %q, want PT1H", every)
	}
	if _, up := r.file(t, "backend"); !up {
		t.Error("the backend was not started again after a hangup")
	}
	if log, _ := r.file(t, "syslog"); !strings.Contains(log, "restored: STATS_COLLECT_EVERY=PT1H") {
		t.Errorf("syslog = %q, want the restore recorded", log)
	}
}

// A backend that starts but does not answer is a failed restore, not "ok".
func TestAddonQuietRunReportsABackendThatDoesNotAnswer(t *testing.T) {
	r := runQuiet(t, true, func(state string) {
		os.WriteFile(filepath.Join(state, "backend-broken"), nil, 0o644)
	}, "--", "true")
	if r.code != 1 || !strings.Contains(r.out, "FAIL  the backend does not answer") {
		t.Fatalf("exit %d, want 1 with a FAIL line\n%s", r.code, r.out)
	}
	if log, _ := r.file(t, "syslog"); !strings.Contains(log, "RESTORE FAILED") {
		t.Errorf("syslog = %q, want the failure recorded", log)
	}
}

// A production-shaped .env (a real Zitadel) refuses even with the marker.
func TestAddonQuietRunRefusesAProductionEnv(t *testing.T) {
	r := runQuiet(t, true, func(state string) {
		env := filepath.Join(filepath.Dir(state), "env")
		os.WriteFile(env, []byte("ZITADEL_DOMAIN=auth.example.org\nZITADEL_FAKE_WORLD=\n"), 0o644)
	}, "--", "true")
	if r.code == 0 || !strings.Contains(r.out, "not the staging host") {
		t.Fatalf("exit %d, want a refusal\n%s", r.code, r.out)
	}
	if calls, _ := r.file(t, "calls"); calls != "" {
		t.Errorf("docker was called with a production .env: %q", calls)
	}
}

// Staging's add-on is read_only: a run stays read_only unless it asks for
// writes, and one that asks is active only for its command.
func TestAddonQuietRunIsActiveOnlyWhenAskedAndOnlyForTheCommand(t *testing.T) {
	during := `echo "during life=$(cat "$FAKE_STATE/life")"`
	r := runQuiet(t, true, nil, "--", "bash", "-c", during)
	if r.code != 0 || !strings.Contains(r.out, "during life=read_only") {
		t.Fatalf("a run without --writes was not read_only (exit %d)\n%s", r.code, r.out)
	}
	r = runQuiet(t, true, nil, "--writes", "--", "bash", "-c", during+"; exit 4")
	if r.code != 4 || !strings.Contains(r.out, "during life=active") {
		t.Fatalf("a --writes run was not active (exit %d)\n%s", r.code, r.out)
	}
	if life, _ := r.file(t, "life"); life != "read_only" {
		t.Errorf("LIFECYCLE_STATE after a --writes run = %q, want read_only back", life)
	}
}

// A --writes run whose quiet recreate fails still leaves read_only behind:
// the restore removes the lifecycle override too.
func TestAddonQuietRunPutsReadOnlyBackWhenAWritesQuietFails(t *testing.T) {
	r := runQuiet(t, true, func(state string) {
		os.WriteFile(filepath.Join(state, "fail-quiet-up"), nil, 0o644)
	}, "--writes", "--", "true")
	if r.code == 0 {
		t.Fatalf("exit 0 after a failed quiet\n%s", r.out)
	}
	if life, _ := r.file(t, "life"); life != "read_only" {
		t.Errorf("LIFECYCLE_STATE after a failed --writes quiet = %q, want read_only", life)
	}
}

// A restore that cannot put the add-on back after --writes never starts the
// backend beside an active add-on.
func TestAddonQuietRunKeepsTheBackendStoppedBesideAnActiveAddon(t *testing.T) {
	r := runQuiet(t, true, nil, "--writes", "--", "bash", "-c", `touch "$FAKE_STATE/sticky"`)
	if r.code != 1 || !strings.Contains(r.out, "backend stays stopped") {
		t.Fatalf("exit %d, want 1 with the backend held\n%s", r.code, r.out)
	}
	if _, up := r.file(t, "backend"); up {
		t.Error("the backend was started beside an add-on left active")
	}
}

// Staging's add-on must already be read_only; a run refuses otherwise.
func TestAddonQuietRunRefusesAnAddonThatIsNotReadOnly(t *testing.T) {
	r := runQuiet(t, true, func(state string) {
		os.WriteFile(filepath.Join(state, "life"), []byte("active\n"), 0o644)
	}, "--", "true")
	if r.code == 0 || !strings.Contains(r.out, "must run read_only") {
		t.Fatalf("exit %d, want a refusal\n%s", r.code, r.out)
	}
	if _, up := r.file(t, "backend"); !up {
		t.Error("the refusal touched the backend")
	}
}
