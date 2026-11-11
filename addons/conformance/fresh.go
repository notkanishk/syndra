package conformance

import (
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

// The freshness checks (feature freshness, design §7). `fresh=true` is the
// contract's request `Cache-Control: no-cache` (RFC 9111 §5.2.1.4); a cooldown
// answer carries retry_after, or is 429 FRESH_READ_COOLDOWN with Retry-After
// (RFC 6585 §4, RFC 9110 §10.2.3); concurrent fresh reads collapse into one.
//
// What a read cost the app is measured black-box by health.target_reads, read
// before and after, less what one /health read itself costs (calibrated from
// two back to back: the TrueNAS add-on's health reads the NAS). The kit
// assumes the add-on is its alone while it runs, and waits the declared
// cooldown before each fresh step.

var freshChecks = []string{"freshness.cached", "freshness.cooldown", "freshness.coalesce", "freshness.unreachable"}

func (k *kit) freshness() {
	skip := func(why string) {
		for _, id := range freshChecks {
			k.at(id).skip("%s", why)
		}
	}
	if !k.m.has("freshness") {
		skip("feature freshness not declared")
		return
	}
	cooldown, why := k.cooldown()
	if why != "" {
		skip(why)
		return
	}
	r0, ok0 := k.targetReads()
	r1, ok1 := k.targetReads()
	if !ok0 || !ok1 {
		skip("health carries no readable target_reads, so what a read costs the app cannot be measured (see health)")
		return
	}
	base := r1 - r0
	measure := func(do func()) (int64, bool) {
		before, ok := k.targetReads()
		do()
		after, ok2 := k.targetReads()
		return after - before - base, ok && ok2
	}
	k.at("freshness.unreachable").skip("a black-box kit cannot take the app offline; the add-on's own test holds it " +
		"(TestAnUnreachableAppAnswersTheLastValueNotAnError in addons/reference)")

	// cached: a plain read after a fresh one may come from the mirror, and
	// then says so and costs the app nothing.
	if c := k.at("freshness.cached"); k.wait(cooldown) {
		k.ok(c, k.subjectsRead(true), "subjects.json")
		var doc map[string]any
		n, ok := measure(func() { doc = k.ok(c, k.subjectsRead(false), "subjects.json") })
		stale, _ := doc["stale"].(map[string]any)
		switch {
		case doc == nil:
		case !ok:
			c.fail("health could not be read around the read")
		case doc["current"] == true:
			c.expect(n >= 1, "a read without fresh said current: true and read the app %d times: a mirror answer marked current", n)
		case str(stale, "reason") == "cached":
			c.expect(n == 0, "a read without fresh said stale cached and read the app %d times", n)
		}
	}

	// cooldown: after the wait a fresh read reads the app now; a second at
	// once does not; /plan straight after is a live read all the same. What
	// one fresh read costs the app (one read is any number of the app's own
	// calls: TrueNAS reads accounts, groups and quotas) is the unit the
	// coalesce check counts in.
	oneRead := int64(-1)
	if c := k.at("freshness.cooldown"); k.wait(cooldown) {
		var first, second *exchange
		d1, ok := measure(func() { first = k.subjectsRead(true) })
		n, ok2 := measure(func() { second = k.subjectsRead(true) })
		doc := k.ok(c, first, "subjects.json")
		c.expect(doc == nil || doc["current"] == true, "a fresh read %v after the last began answered current %v: past the cooldown, fresh reads the app now", cooldown, doc["current"])
		c.expect(ok && d1 >= 1, "a fresh read past the cooldown read the app %d times; it reads it now", d1)
		k.cooledDown(c, second)
		c.expect(ok2 && n == 0, "a fresh read inside the cooldown read the app %d times; want 0", n)
		if ok && d1 >= 1 {
			oneRead = d1
		}
		plan := k.readFor("/plan", k.planBody(map[string]any{k.m.account: false}), "plan_request.json")
		c.expect(plan != nil && plan["current"] == true, "/plan straight after a fresh read answered current %v: /plan is never cooled down", plan["current"])
	}

	// coalesce: two fresh reads sent together are one read of the app. The
	// one that joined a read already running is current or cooldown (a read
	// that began before it arrived is not current, guide chapter 4).
	if c := k.at("freshness.coalesce"); k.wait(cooldown) {
		var pair [2]*exchange
		n, ok := measure(func() {
			var wg sync.WaitGroup
			start := make(chan struct{})
			for i := range pair {
				wg.Add(1)
				go func() { defer wg.Done(); <-start; pair[i] = k.subjectsRead(true) }()
			}
			close(start)
			wg.Wait()
		})
		current := 0
		for _, ex := range pair {
			doc := k.ok(c, ex, "subjects.json")
			stale, _ := doc["stale"].(map[string]any)
			switch {
			case doc == nil:
			case doc["current"] == true:
				current++
			default:
				c.expect(str(stale, "reason") == "cooldown", "%s sent with another answered stale %q; a collapsed fresh read is current or cooldown", ex, str(stale, "reason"))
			}
		}
		c.expect(current >= 1, "neither of two concurrent fresh reads was current")
		c.expect(ok && oneRead >= 1 && n == oneRead,
			"two concurrent fresh reads read the app %d times; one fresh read alone reads it %d (request collapsing)", n, oneRead)
	}
}

// cooledDown judges a fresh read sent inside the cooldown: the last value with
// stale cooldown and retry_after, or 429 FRESH_READ_COOLDOWN with Retry-After
// in delay-seconds.
func (k *kit) cooledDown(c *check, ex *exchange) {
	if ex.status == http.StatusTooManyRequests {
		if k.refused(c, ex, 429, "FRESH_READ_COOLDOWN", "a fresh read inside the cooldown") != nil {
			s, err := strconv.Atoi(ex.header.Get("Retry-After"))
			c.expect(err == nil && s > 0, "FRESH_READ_COOLDOWN with Retry-After %q, want delay-seconds above 0", ex.header.Get("Retry-After"))
		}
		return
	}
	doc := k.ok(c, ex, "subjects.json")
	if doc == nil {
		return
	}
	stale, _ := doc["stale"].(map[string]any)
	retry, hasRetry := num(stale["retry_after"])
	c.expect(doc["current"] == false && str(stale, "reason") == "cooldown" && hasRetry && retry > 0,
		"a fresh read inside the cooldown answered current %v, stale %v: want the last value, stale cooldown with retry_after above 0",
		doc["current"], doc["stale"])
}

// currentSubjects is the read Syndra's drift sweep makes (ReconcileAddon in
// backend/internal/services/drift/addon.go): fresh when the add-on declares
// freshness, and, answered inside the cooldown (429 or the last value as
// cooldown), sent once more after the retry_after it named (RFC 9110
// §10.2.3), never sooner. The sweep waits at most the declared cooldown, the
// kit at most FreshWait; a second cooldown answer is the sweep's skipped pass,
// recorded as a stale read.
func (k *kit) currentSubjects() *exchange {
	if !k.m.has("freshness") {
		return k.subjectsRead(false)
	}
	ex := k.subjectsRead(true)
	wait := -1
	if ex.status == http.StatusTooManyRequests && ex.code() == "FRESH_READ_COOLDOWN" {
		wait, _ = strconv.Atoi(ex.header.Get("Retry-After"))
	} else if stale, _ := ex.doc()["stale"].(map[string]any); str(stale, "reason") == "cooldown" {
		n, _ := num(stale["retry_after"])
		wait = int(n)
	}
	if d := time.Duration(wait) * time.Second; wait >= 0 && d <= k.cfg.FreshWait && k.wait(d) {
		ex = k.subjectsRead(true)
	}
	return ex
}

func (k *kit) subjectsRead(fresh bool) *exchange {
	if fresh {
		return k.send(request{method: http.MethodGet, target: "/subjects?fresh=true"})
	}
	return k.send(request{method: http.MethodGet, target: "/subjects"})
}

// targetReads is health.target_reads now.
func (k *kit) targetReads() (int64, bool) {
	ex := k.send(request{method: http.MethodGet, target: "/health"})
	if ex.err != nil || ex.status != http.StatusOK {
		return 0, false
	}
	return num(ex.doc()["target_reads"])
}

// cooldown is limits.fresh_read_cooldown (constants.cooldown_default when
// undeclared) as a wait, or why the kit will not wait it.
func (k *kit) cooldown() (time.Duration, string) {
	declared := k.m.cooldown
	if declared == "" {
		declared = k.c.cooldownDefault
	}
	d, ok := isoDuration(declared)
	switch {
	case !ok:
		return 0, "limits.fresh_read_cooldown " + strconv.Quote(declared) + " is not PT<n>S or PT<n>M, so the kit cannot wait it out"
	case d > k.cfg.FreshWait:
		return 0, "limits.fresh_read_cooldown " + declared + " is longer than the kit waits (" + k.cfg.FreshWait.String() + ")"
	}
	// A little over the cooldown, so an add-on whose clock reads coarser
	// than the kit's is never caught a moment early.
	return d + 100*time.Millisecond, ""
}

// isoDuration reads the two ISO 8601 forms a cooldown is written in.
// ponytail: PT<n>S and PT<n>M only; anything else is a skip, never a guess.
func isoDuration(s string) (time.Duration, bool) {
	num, unit := strings.TrimPrefix(s, "PT"), time.Second
	switch {
	case !strings.HasPrefix(s, "PT"):
		return 0, false
	case strings.HasSuffix(num, "M"):
		unit = time.Minute
	case !strings.HasSuffix(num, "S"):
		return 0, false
	}
	n, err := strconv.Atoi(num[:len(num)-1])
	if err != nil || n < 0 {
		return 0, false
	}
	return time.Duration(n) * unit, true
}

// wait sleeps d unless the run is interrupted first.
func (k *kit) wait(d time.Duration) bool {
	select {
	case <-time.After(d):
		return true
	case <-k.ctx.Done():
		return false
	}
}
