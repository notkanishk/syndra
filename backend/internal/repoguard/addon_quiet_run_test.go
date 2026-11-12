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
    "up truenas-addon")
      if [[ -e "$S/sticky" ]]; then exit 0; fi
      echo "${TRUENAS_STATS_COLLECT_EVERY:-$FAKE_DOTENV}" > "$S/every" ;;
    "stop backend") rm -f "$S/backend" ;;
    "start backend") touch "$S/backend" ;;
    *) echo "fake docker: unexpected compose $*" >&2; exit 9 ;;
  esac
  exit 0
fi
if [[ "$1" == inspect ]]; then
  case "$3" in
    *Config.Env*) echo "TZ=Asia/Kolkata"; echo "STATS_COLLECT_EVERY=$(cat "$S/every")" ;;
    *NetworkSettings*) echo "172.20.0.5 " ;;
  esac
  exit 0
fi
echo "fake docker: unexpected $*" >&2; exit 9
`

type quietRun struct {
	state string
	out   string
	code  int
}

func runQuiet(t *testing.T, staging bool, setup func(state string), args ...string) quietRun {
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
	if err := os.WriteFile(filepath.Join(bin, "docker"), []byte(fakeDocker), 0o755); err != nil {
		t.Fatal(err)
	}
	write := func(name, s string) {
		if err := os.WriteFile(filepath.Join(state, name), []byte(s), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("every", "PT1H\n")
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
	)
	out, err := cmd.CombinedOutput()
	code := 0
	if ee, ok := err.(*exec.ExitError); ok {
		code = ee.ExitCode()
	} else if err != nil {
		t.Fatal(err)
	}
	return quietRun{state: state, out: string(out), code: code}
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
