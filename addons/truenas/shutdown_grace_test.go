package main

import (
	"os"
	"regexp"
	"strings"
	"testing"
	"time"
)

// The add-on gives an in-flight mutation `shutdownTimeout` to settle after a
// termination signal. Docker gives the container `stop_grace_period` and then
// SIGKILLs it. If the second is not longer than the first, the drain is cut
// short and the mutation is abandoned half-applied — which is the exact state
// the drain exists to prevent.
//
// It was cut short for the whole life of this add-on: nothing set
// `stop_grace_period`, so Docker's 10s default bound a 20s drain. Nothing
// reported it, and nothing could have — a truncated drain and a clean stop are
// indistinguishable from outside, since the process is gone either way and the
// mutation that was settling leaves the same silence as one that never began.
//
// A source guard because the two values live in different files, in different
// languages, owned by different concerns. That is the arrangement in which two
// internally-consistent definitions of one thing drift apart unobserved, and a
// comment on each saying "keep these in step" is what failed here already.
func TestTheDeploymentAllowsTheShutdownDrainToFinish(t *testing.T) {
	grace, err := composeStopGracePeriod("truenas-addon")
	if err != nil {
		t.Fatalf("reading stop_grace_period from docker-compose.yml: %v\n"+
			"Without it Docker applies its 10s default and SIGKILLs the add-on "+
			"mid-drain, abandoning an in-flight mutation half-applied.", err)
	}

	if grace <= shutdownTimeout {
		t.Fatalf("the deployment kills the add-on before its own drain can finish:\n"+
			"  shutdownTimeout   = %s  (main.go, this package)\n"+
			"  stop_grace_period = %s  (docker-compose.yml, truenas-addon)\n"+
			"The Compose value must EXCEED the constant, so the process always "+
			"reaches its own deadline first. Raise stop_grace_period, or lower "+
			"shutdownTimeout — but a mutation settling against the NAS is what "+
			"the drain is waiting for, so lowering it is rarely the right half.",
			shutdownTimeout, grace)
	}
}

// Every add-on inherits this shutdown path, so every add-on inherits the
// truncation if its service ships without the setting — silently, since the
// symptom is an absence. The first add-on's suite is the only place that can
// notice today, so it checks the shape of the deployment rather than only its
// own row.
//
// What this can assert about a future add-on is presence, not sufficiency: that
// add-on's own drain budget is a constant in its own module, which this package
// cannot read. Its module carries the comparison; this carries the floor.
func TestEveryAddOnServiceSetsAStopGracePeriod(t *testing.T) {
	for _, s := range composeAddOnServices(t) {
		if _, err := composeStopGracePeriod(s); err != nil {
			t.Errorf("%s: %v\n"+
				"Every add-on runs the same drain-then-shutdown path, so an add-on "+
				"service without this setting is killed mid-settle by Docker's 10s "+
				"default — and the symptom is an absence nobody sees.", s, err)
		}
	}
}

// composeStopGracePeriod reads the value from one service only, reusing the
// block scoping from config_env_test.go: another service's setting must not
// satisfy a guard about this one.
func composeStopGracePeriod(service string) (time.Duration, error) {
	body, err := composeServiceBody(service)
	if err != nil {
		return 0, err
	}

	m := regexp.MustCompile(`(?m)^\s*stop_grace_period:\s*(\S+)\s*$`).FindStringSubmatch(body)
	if m == nil {
		return 0, errNoGracePeriod
	}
	return time.ParseDuration(m[1])
}

// composeServiceBody returns one service's block. Scoping matters more than it
// looks: a guard about this service that another service's line can satisfy is
// a guard that passes for the wrong reason, which is the failure mode every
// source guard in this package exists to avoid.
func composeServiceBody(service string) (string, error) {
	raw, err := os.ReadFile("../../docker-compose.yml")
	if err != nil {
		return "", err
	}
	body := string(raw)

	header := "\n  " + service + ":\n"
	start := strings.Index(body, header)
	if start < 0 {
		return "", errNoAddOnService
	}
	body = body[start+len(header):]
	if end := regexp.MustCompile(`(?m)^(  [a-z][a-z0-9_-]*:|[a-z][a-z0-9_-]*:)$`).FindStringIndex(body); end != nil {
		body = body[:end[0]]
	}
	return body, nil
}

// composeAddOnServices lists every `*-addon` service in the deployment.
func composeAddOnServices(t *testing.T) []string {
	t.Helper()
	raw, err := os.ReadFile("../../docker-compose.yml")
	if err != nil {
		t.Fatal(err)
	}
	found := regexp.MustCompile(`(?m)^  ([a-z0-9][a-z0-9_-]*-addon):$`).FindAllStringSubmatch(string(raw), -1)
	if len(found) == 0 {
		t.Fatal("no *-addon services found; the naming these guards key on has changed")
	}
	out := make([]string, 0, len(found))
	for _, m := range found {
		out = append(out, m[1])
	}
	return out
}

var (
	errNoAddOnService = constError("no truenas-addon service in docker-compose.yml")
	errNoGracePeriod  = constError("the truenas-addon service sets no stop_grace_period")
)

type constError string

func (e constError) Error() string { return string(e) }
