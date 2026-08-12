package main

import (
	"regexp"
	"strings"
	"testing"
)

// The two structural halves of add-on isolation, asserted against the
// deployment rather than described in a design document.
//
// The derivation gives each target its own keys from its own secret, so one
// add-on cannot authenticate as another. That is worth exactly as much as the
// two facts below, and neither is visible in any Go file:
//
//   - an add-on that can READ its neighbour's secret derives its neighbour's
//     keys, and the per-target derivation buys nothing;
//   - an add-on that can REACH its neighbour has an attack surface it should
//     never have been able to open a socket to.
//
// Both are one line of YAML each, both were wrong at some point on this branch,
// and both fail silently — a shared mount and a shared network are invisible
// until someone is already inside one container. These are source guards for
// the same reason `config_env_test.go` and `shutdown_grace_test.go` are: the
// value lives in a file no compiler reads.
//
// The first add-on's suite carries them because it is the only suite that
// exists. They are written about every `*-addon` service, not about this one.

func TestEachAddOnMountsOnlyItsOwnTransportSecret(t *testing.T) {
	for _, service := range composeAddOnServices(t) {
		target := strings.TrimSuffix(service, "-addon")
		body, err := composeServiceBody(service)
		if err != nil {
			t.Fatalf("%s: %v", service, err)
		}

		// The backend mounts this directory — it holds every target's secret,
		// because it is the one component that talks to all of them. An add-on
		// mounting the same directory reads every other target's secret and can
		// derive every other target's keys.
		if regexp.MustCompile(`:/run/secrets/addon:`).MatchString(body) {
			t.Errorf("%s mounts the whole transport secrets DIRECTORY.\n"+
				"That hands it every other target's secret, and the per-target "+
				"derivation is then decoration: one compromised add-on derives "+
				"the keys of all of them. Mount only %s.key.", service, target)
		}

		want := regexp.MustCompile(`/` + regexp.QuoteMeta(target) + `\.key:/run/secrets/addon/` + regexp.QuoteMeta(target) + `\.key:ro`)
		if !want.MatchString(body) {
			t.Errorf("%s does not mount its own transport secret read-only at the "+
				"expected path.\nExpected a volume ending "+
				"`/%s.key:/run/secrets/addon/%s.key:ro`, matching ADDON_SECRET_FILE "+
				"and what scripts/gen-addon-secret.sh writes.", service, target, target)
		}
	}
}

func TestNoTwoAddOnsShareANetwork(t *testing.T) {
	services := composeAddOnServices(t)
	owner := map[string]string{}

	for _, service := range services {
		body, err := composeServiceBody(service)
		if err != nil {
			t.Fatalf("%s: %v", service, err)
		}
		nets := composeNetworks(body)
		if len(nets) == 0 {
			t.Errorf("%s joins no explicit network, so Compose puts it on the "+
				"default one — alongside the datastores it must never reach.", service)
			continue
		}
		if len(nets) > 1 {
			// Not a style rule. An add-on holds one target's credential and
			// reaches one target; a second segment is a second thing it can
			// touch, and the reason it runs in a container of its own.
			t.Errorf("%s joins %d networks (%s); an add-on belongs on its own "+
				"segment and nothing else.", service, len(nets), strings.Join(nets, ", "))
		}
		for _, n := range nets {
			if other, taken := owner[n]; taken {
				t.Errorf("%s and %s share network %q. No add-on should be able to "+
					"open a socket to another: each holds a different target's "+
					"credential, and a shared segment makes the weakest one the "+
					"way in to the rest.", other, service, n)
				continue
			}
			owner[n] = service
		}
	}

	// And the backend really does join them, or the isolation above is just a
	// deployment that cannot work — which is the failure this guard would
	// otherwise make look like a security win.
	backend, err := composeServiceBody("backend")
	if err != nil {
		t.Fatalf("backend service: %v", err)
	}
	joined := map[string]bool{}
	for _, n := range composeNetworks(backend) {
		joined[n] = true
	}
	for net, service := range owner {
		if !joined[net] {
			t.Errorf("the backend does not join %q, so it cannot reach %s at all.",
				net, service)
		}
	}
}

// composeNetworks reads the inline `networks: [a, b]` form this file uses.
func composeNetworks(body string) []string {
	m := regexp.MustCompile(`(?m)^\s*networks:\s*\[([^\]]*)\]\s*$`).FindStringSubmatch(body)
	if m == nil {
		return nil
	}
	var out []string
	for _, part := range strings.Split(m[1], ",") {
		if n := strings.TrimSpace(part); n != "" {
			out = append(out, n)
		}
	}
	return out
}
