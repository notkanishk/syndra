package repoguard

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// The deploy order add-on contract 2 needs (openspec/changes/addon-contract,
// task 1.10), held in both workflows that deploy: the images are built while
// the old containers still serve, then the backend stops before the add-on,
// the add-on's store is copied only once the add-on has stopped, and only then
// is anything started; a failure anywhere starts everything again; nothing
// stops the GitHub runner. A push to main runs the workflow file at the pushed
// commit, so this order has to be in the file, not in a one-off.
func TestBothDeploysBuildThenStopTheBackendThenTheAddOnBeforeStarting(t *testing.T) {
	for _, name := range []string{"deploy.yml", "_staging.yml"} {
		s := workflow(t, name)
		order := []string{
			"docker compose build\n",
			"docker compose stop backend\n",
			"docker compose stop truenas-addon\n",
			"docker compose cp truenas-addon:/var/lib/syndra-truenas/state.db",
			"docker compose up -d --build --remove-orphans",
		}
		at := -1
		for i, step := range order {
			j := strings.Index(s[at+1:], step)
			if j < 0 {
				t.Errorf("%s: %q is missing, or not after %q", name, strings.TrimSpace(step), strings.TrimSpace(order[max(0, i-1)]))
				break
			}
			at += 1 + j
		}
		// Production restarts only after a successful build: before it,
		// nothing was stopped. Staging restarts on any failure: its database
		// reset stops services before the build.
		gate := `failure\(\) && steps\.build\.outcome == 'success'`
		if name == "_staging.yml" {
			gate = `failure\(\)`
		} else if !strings.Contains(stepNamed(s, "Build the images"), "\n        id: build\n") {
			t.Errorf("%s: the build step must have id build, which the failure restart reads", name)
		}
		restart := regexp.MustCompile(`(?s)- name: [^\n]*\n\s+if: ` + gate + `\n\s+run: \|\n[^-]*docker compose up -d\n`)
		if !restart.MatchString(s) {
			t.Errorf("%s: no `if: %s` step starts everything again (docker compose up -d)", name, gate)
		}
		if strings.Contains(s, "| grep -q") {
			t.Errorf("%s: grep -q under pipefail can fail its writer with SIGPIPE; send the match to /dev/null", name)
		}
		if stop := stepNamed(s, "Stop the backend"); stop == "" || strings.Contains(stop, "\n        if:") {
			t.Errorf("%s: the stop step must exist and run for every deploy, a dispatched rollback included", name)
		}
		for _, forbidden := range []string{"svc.sh stop", "actions-runner", "systemctl stop", "cp truenas-addon:/var/lib/syndra-truenas/mutations"} {
			if strings.Contains(s, forbidden) {
				t.Errorf("%s does %q: the runner runs the job, and a restored mutation log reads as a truncated one", name, forbidden)
			}
		}
	}
}

// The rollback path: deploy.yml is dispatched with a required ref (the full
// SHA production ran before), so the CURRENT file, with its stop order,
// deploys the old code. What is deployed is decided once, by a `resolve` job:
// a hex SHA, on main, whose ci-ok passed, read from the environment (never
// spliced into a script); both deploys check out exactly that SHA. Never a
// pull_request trigger on a self-hosted runner.
func TestARollbackIsADispatchOfTheCurrentWorkflow(t *testing.T) {
	s := workflow(t, "deploy.yml")
	if !regexp.MustCompile(`(?s)workflow_dispatch:\n\s+inputs:\n\s+ref:\n[^\n]*\n\s+required: true`).MatchString(s) {
		t.Error("deploy.yml must take a required `ref` input on workflow_dispatch: the rollback path")
	}
	resolve := job(s, "resolve")
	for _, want := range []string{
		"runs-on: ubuntu-latest", "checks: read", "contents: read",
		"REQUESTED: ${{ inputs.ref || github.sha }}",
		`*[!0-9a-f]* | "")`,
		`[ "${#REQUESTED}" -ge 7 ]`,
		`sha="$(git rev-parse --verify --quiet "${REQUESTED}^{commit}")"`,
		`git merge-base --is-ancestor "$sha" origin/main`,
		`head="$(git rev-parse origin/main)"`,
		"for i in $(seq 1 90); do", "sleep 30",
		"check-runs?check_name=ci-ok", `"completed success") echo "sha=$sha" >> "$GITHUB_OUTPUT"`,
		`completed*) echo "::error::ci-ok on $sha concluded: $run"; exit 1 ;;`,
		`if length == 0 then "none"`,
		`none) [ "$sha" = "$head" ] || {`,
		"sha: ${{ steps.sha.outputs.sha }}",
	} {
		if !strings.Contains(resolve, want) {
			t.Errorf("deploy.yml's resolve job lacks %q", want)
		}
	}
	if n := strings.Count(s, "checks: read"); n != 1 || !strings.Contains(resolve, "checks: read") {
		t.Errorf("checks: read appears %d times; the resolve job alone may read checks", n)
	}
	if strings.Contains(strings.ReplaceAll(s, "REQUESTED: ${{ inputs.ref || github.sha }}", ""), "inputs.ref") {
		t.Error("deploy.yml reads inputs.ref outside resolve's environment")
	}
	staging, production := job(s, "staging"), job(s, "production")
	if !strings.Contains(staging, "needs: resolve") || !strings.Contains(staging, "ref: ${{ needs.resolve.outputs.sha }}") {
		t.Error("the staging deploy must take resolve's SHA and nothing else")
	}
	for _, want := range []string{"needs: [resolve, staging]", "DEPLOY_SHA: ${{ needs.resolve.outputs.sha }}",
		`git checkout --force --detach "$DEPLOY_SHA"`, `[ "$(git rev-parse HEAD)" = "$DEPLOY_SHA" ]`} {
		if !strings.Contains(production, want) {
			t.Errorf("the production deploy lacks %q", want)
		}
	}
	// _staging.yml checks a full SHA out exactly, never re-resolving it as a
	// branch name first.
	st := workflow(t, "_staging.yml")
	if !strings.Contains(st, `grep -Ex '[0-9a-f]{40}'`) || !strings.Contains(st, `[ "$(git rev-parse HEAD)" = "$ref" ]`) {
		t.Error("_staging.yml must check a full SHA out exactly")
	}
	if !triggers(s)["workflow_dispatch"] || !triggers(s)["push"] {
		t.Errorf("deploy.yml triggers %v, want push and workflow_dispatch", triggers(s))
	}
	for _, name := range []string{"deploy.yml", "_staging.yml", "deploy-staging.yml"} {
		if triggers(workflow(t, name))["pull_request"] || triggers(workflow(t, name))["pull_request_target"] {
			t.Errorf("%s has a pull_request trigger on a self-hosted runner", name)
		}
	}
}

// A rollback deploys an older backend onto a newer, clean, additive schema,
// so staging's migration check is "at least what this code knows, and clean".
func TestStagingAcceptsANewerCleanSchema(t *testing.T) {
	st := workflow(t, "_staging.yml")
	if !strings.Contains(st, `[ "${got#*|}" = f ] && [ "${got%%|*}" -ge "$want" ]`) {
		t.Error(`_staging.yml's migration check must be "at least the newest migration this code knows, and not dirty"`)
	}
}

// job is the text of one top-level job of a workflow.
func job(s, name string) string {
	i := strings.Index(s, "\n  "+name+":\n")
	if i < 0 {
		return ""
	}
	rest := s[i+1:]
	if j := regexp.MustCompile(`\n  [a-z_-]+:\n`).FindStringIndex(rest[1:]); j != nil {
		return rest[:j[0]+1]
	}
	return rest
}

func workflow(t *testing.T, name string) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(repoRoot(t), ".github", "workflows", name))
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

// stepNamed is the text of the step whose name begins with prefix.
func stepNamed(s, prefix string) string {
	i := strings.Index(s, "- name: "+prefix)
	if i < 0 {
		return ""
	}
	rest := s[i+1:]
	if j := strings.Index(rest, "\n      - name: "); j >= 0 {
		return rest[:j]
	}
	return rest
}

// triggers are the keys under `on:`, comments aside.
func triggers(s string) map[string]bool {
	out := map[string]bool{}
	in := false
	for _, line := range strings.Split(s, "\n") {
		switch {
		case line == "on:":
			in = true
		case in && line != "" && !strings.HasPrefix(line, " ") && !strings.HasPrefix(line, "#"):
			in = false
		case in && strings.HasPrefix(line, "  ") && !strings.HasPrefix(line, "   ") && !strings.HasPrefix(strings.TrimSpace(line), "#"):
			out[strings.TrimSuffix(strings.Fields(line)[0], ":")] = true
		}
	}
	return out
}
