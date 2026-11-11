package addons

import (
	"encoding/json"
	"testing"
	"time"
)

// The sweep waits a target's cooldown at most (drift.ReconcileAddon): the
// declared limits.fresh_read_cooldown, else constants.cooldown_default.
func TestFreshCooldownIsTheDeclaredOneElseTheDefault(t *testing.T) {
	var reg struct {
		Constants map[string]struct {
			Value any `json:"value"`
		} `json:"constants"`
	}
	if err := json.Unmarshal(mustRead(t, "registry", "constants.json"), &reg); err != nil {
		t.Fatal(err)
	}
	if got := isoCooldown(reg.Constants["cooldown_default"].Value.(string)); got != cooldownDefault {
		t.Errorf("constants.cooldown_default reads %v, the backend's default is %v", got, cooldownDefault)
	}
	for iso, want := range map[string]time.Duration{
		"PT10S": 10 * time.Second, "PT1M30S": 90 * time.Second, "PT0.5S": 500 * time.Millisecond,
		// A day is a day however it is written: the sweep caps both alike.
		"P1D": 24 * time.Hour, "PT24H": 24 * time.Hour, "P1DT1M": 24*time.Hour + time.Minute,
		"": cooldownDefault, "PT0S": cooldownDefault, "10s": cooldownDefault, "pt30s": cooldownDefault,
		// Not ISO 8601: a unit without a number, a sign, parts out of order,
		// a designator with nothing after it, a fraction not on the seconds.
		"PT30MS": cooldownDefault, "PT+30S": cooldownDefault, "PT-30S": cooldownDefault, "PT30S5M": cooldownDefault,
		"P": cooldownDefault, "PT": cooldownDefault, "ePT10S": cooldownDefault, "P1DT": cooldownDefault, "PT1.5M": cooldownDefault,
		// No fixed length; and past what a time.Duration holds.
		"P1W": cooldownDefault, "P1M": cooldownDefault, "P1Y": cooldownDefault, "P999999999D": cooldownDefault,
	} {
		if got := isoCooldown(iso); got != want {
			t.Errorf("%q reads %v, want %v", iso, got, want)
		}
	}

	register(t, Registration{Target: "cooled"})
	t.Cleanup(func() { registryMu.Lock(); delete(registry, "cooled"); registryMu.Unlock() })
	registryMu.Lock()
	registry["cooled"].manifest = &Manifest{Limits: Limits{FreshReadCooldown: "PT10S"}}
	registryMu.Unlock()
	if got := FreshCooldown("cooled"); got != 10*time.Second {
		t.Errorf("a declared PT10S reads %v", got)
	}
	if got := FreshCooldown("nobody"); got != cooldownDefault {
		t.Errorf("an unknown target reads %v, want the default", got)
	}
}
