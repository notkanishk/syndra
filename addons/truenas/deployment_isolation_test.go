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

		// Its OWN volume, and only its own. Every target's secret lives in a
		// volume of its own; an add-on holding more than one could derive the
		// other target's keys, and the per-target derivation would be
		// decoration.
		mine := target + "_addon_secret"
		want := regexp.MustCompile(`- ` + regexp.QuoteMeta(mine) + `:/run/secrets/addon:ro`)
		if !want.MatchString(body) {
			t.Errorf("%s does not mount its own transport secret volume read-only.\n"+
				"Expected `- %s:/run/secrets/addon:ro`, matching ADDON_SECRET_FILE.",
				service, mine)
		}
		for _, mount := range regexp.MustCompile(`([a-z0-9_]+_addon_secret):`).FindAllStringSubmatch(body, -1) {
			if mount[1] != mine {
				t.Errorf("%s mounts %s, which is another target's secret.\n"+
					"One compromised add-on would then hold the keys of both.",
					service, mount[1])
			}
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

// Provisioning is the deployment's job, and both readers wait for it.
//
// This was a documented human step — a `sudo` script run before the first `up`.
// A step performed before a container starts
// is a step that gets skipped, and the skip does not fail loudly: Docker
// creates a DIRECTORY at the missing mount path, the add-on exits on a secret
// it cannot read, and the backend registers a target whose every call fails at
// the handshake.
//
// So the minting is a service, and the two readers depend on it completing.
// Guarded because all three lines are ordinary-looking YAML whose absence
// produces exactly the failure above.
func TestEachAddOnsSecretIsMintedByTheDeployment(t *testing.T) {
	for _, service := range composeAddOnServices(t) {
		target := strings.TrimSuffix(service, "-addon")
		minter := target + "-addon-secret"

		body, err := composeServiceBody(minter)
		if err != nil {
			t.Fatalf("no %s service: %v\nNothing mints %s's transport secret, so "+
				"the first start depends on a human having run a script.", minter, err, target)
		}
		// Root inside the container, because the file is owned root:65532 —
		// owner read for the backend, group read for the add-on's uid.
		if !regexp.MustCompile(`user:\s*"0:0"`).MatchString(body) {
			t.Errorf("%s does not run as root, so it cannot set the ownership both "+
				"readers need", minter)
		}
		if !regexp.MustCompile(`\bln\b`).MatchString(body) {
			t.Errorf("%s does not publish with `ln`.\nrename/mv REPLACES its "+
				"destination, so two runs would both publish and the later would "+
				"destroy a secret the earlier may already have put into service.", minter)
		}
		if regexp.MustCompile(`\bprofiles:`).MatchString(body) {
			// A dependency that disappears with a profile is a start-up order
			// that changes with configuration.
			t.Errorf("%s is profile-gated, so the backend cannot depend on it "+
				"unconditionally", minter)
		}

		for _, reader := range []string{"backend", service} {
			rb, err := composeServiceBody(reader)
			if err != nil {
				t.Fatalf("%s: %v", reader, err)
			}
			dep := regexp.MustCompile(regexp.QuoteMeta(minter) + `:\s*\n\s*condition:\s*service_completed_successfully`)
			if !dep.MatchString(rb) {
				t.Errorf("%s does not wait for %s to finish.\nRegistration reads "+
					"the secret ONCE at start-up, so starting first means the target "+
					"does not register and the deployment needs a second, manual "+
					"restart to notice.", reader, minter)
			}
		}
	}
}
