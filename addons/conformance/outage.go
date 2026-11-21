package conformance

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"slices"
	"strconv"
	"time"
)

// The outage checks (guide, "The outage checks"). The kit takes the app and
// the add-on down only through the operator's hooks; without a pair, its
// checks are "not checked", a skip, never a pass, and -require-outage makes
// a missing app pair a failure. The add-on-down half Syndra sees (a closed
// port, a wrong key, a hang) is the backend's own table test: the kit is a
// client of a running add-on and cannot be that.

// outage runs once: at the end of a reads-only run, or after the calls step
// of a -mutate run, so outage.write's apply and outage.restart's call are
// about the kit's own account.
func (k *kit) outage() {
	k.appDown()
	k.addonRestart()
}

// notChecked marks checks whose hooks were not given.
func (k *kit) notChecked(hooks string, require bool, ids ...string) {
	for _, id := range ids {
		if require {
			k.at(id).fail("not checked: %s were not given, and -require-outage requires them", hooks)
		} else {
			k.at(id).skip("not checked: %s were not given", hooks)
		}
	}
}

func (k *kit) appDown() {
	health, kept, none, write, unreach := k.at("outage.health"), k.at("outage.kept"), k.at("outage.none"), k.at("outage.write"), k.at("freshness.unreachable")
	ids := []string{"outage.health", "outage.kept", "outage.none"}
	if k.cfg.Mutate {
		ids = append(ids, "outage.write")
	}
	fresh := k.m.has("freshness") && len(unreach.skips) == 0 // freshness() skipped it when it cannot wait the cooldown
	if fresh {
		ids = append(ids, "freshness.unreachable")
	}
	if k.cfg.StopApp == nil || k.cfg.StartApp == nil {
		k.notChecked("-stop-app and -start-app", k.cfg.RequireOutage, ids...)
		return
	}
	if k.m.has("freshness") && !fresh {
		for _, id := range ids {
			k.at(id).skip("%s", unreach.skips[0])
		}
		return
	}

	// Before the stop: a read the add-on may keep, as a copy taken at copyAt.
	before := k.readNow()
	copyAt := ""
	if before.status == http.StatusOK {
		copyAt = str(before.doc(), "taken_at")
	}
	tried, stopped := false, false
	stop := func() bool {
		tried = true
		err := k.cfg.StopApp(k.ctx)
		stopped = health.expect(err == nil, "-stop-app failed: %v", err)
		return stopped
	}
	var applied *exchange
	var callID string
	var logBefore int64
	var logOK bool
	if k.cfg.Mutate && k.created.id != "" {
		// The plan naming the kit's account is read just before the stop, so
		// the apply is still the one toOwned allows, sent once the app is down.
		logBefore, logOK = k.logRecords(write)
		callID = k.callID()
		desired := map[string]any{k.m.account: true}
		_, applied = k.toOwned(write, desired, func(plan map[string]any) *request {
			if !stop() {
				return nil
			}
			return &request{method: http.MethodPost, target: "/apply", schema: "apply_request.json",
				body: k.applyBody(callID, str(plan, "fingerprint"), desired)}
		})
	}
	if !tried {
		stop()
	}
	if !stopped {
		return
	}

	if applied != nil {
		if k.refused(write, applied, http.StatusServiceUnavailable, "TARGET_UNREACHABLE", "an apply with the app stopped") != nil {
			s, err := strconv.Atoi(applied.header.Get("Retry-After"))
			write.expect(err == nil && s > 0, "the apply's TARGET_UNREACHABLE carries Retry-After %q, want delay-seconds above 0", applied.header.Get("Retry-After"))
		}
		k.unwritten(write, logBefore, logOK, "an apply with the app stopped")
		if k.m.has("calls") {
			k.callUnknown(write, callID)
		}
	}
	h0 := k.ok(health, k.send(request{method: http.MethodGet, target: "/health"}), "health.json")
	caps := k.send(request{method: http.MethodGet, target: "/capabilities"})
	health.expect(caps.err == nil && caps.status == http.StatusOK, "GET /capabilities with the app stopped answered %d (%v); it still answers", caps.status, caps.err)

	during := k.readNow()
	k.outageRead(kept, none, during, copyAt, true)
	if fresh {
		if copyAt == "" {
			unreach.skip("the read just before the stop answered nothing to keep")
		} else if doc := k.ok(unreach, during, "subjects.json"); doc != nil {
			stale, _ := doc["stale"].(map[string]any)
			unreach.expect(doc["current"] == false && str(stale, "reason") == "unreachable" && str(doc, "taken_at") == copyAt,
				"a fresh read with the app stopped answered current %v, stale %v, taken_at %s: want the value read at %s, stale unreachable",
				doc["current"], doc["stale"], str(doc, "taken_at"), copyAt)
		}
	}
	k.outageRead(kept, none, k.subjectsRead(false), copyAt, false)
	if h1 := k.ok(health, k.send(request{method: http.MethodGet, target: "/health"}), "health.json"); h1 != nil {
		health.expect(h1["reachable"] == false, "/health after a read failed with the app stopped answered reachable %v; reachable says whether a real call to the app just succeeded", h1["reachable"])
		if at := str(h0, "last_read_at"); at != "" {
			health.expect(str(h1, "last_read_at") == at, "last_read_at moved from %s to %q while the app was stopped", at, str(h1, "last_read_at"))
		}
	}
	for c, why := range map[*check]string{kept: "every read with the app stopped answered no copy (see outage.none)",
		none: "every read with the app stopped answered its copy (see outage.kept)"} {
		if !c.ran {
			c.skip("%s", why)
		}
	}

	// Putting the app back is never cut short.
	err := k.cfg.StartApp(context.WithoutCancel(k.ctx))
	if !health.expect(err == nil, "-start-app failed: %v", err) {
		return
	}
	if doc := k.ok(health, k.readNow(), "subjects.json"); doc != nil {
		health.expect(doc["current"] == true, "a fresh read with the app back answered current %v, want true", doc["current"])
	}
	if h := k.ok(health, k.send(request{method: http.MethodGet, target: "/health"}), "health.json"); h != nil {
		health.expect(h["reachable"] == true, "/health after a fresh read with the app back answered reachable %v, want true", h["reachable"])
	}
}

// outageRead judges one /subjects answered with the app stopped: its copy, as
// it was (outage.kept), or with none, the problem alone (outage.none).
func (k *kit) outageRead(kept, none *check, ex *exchange, copyAt string, fresh bool) {
	if ex.err == nil && ex.status == http.StatusOK {
		doc := k.ok(kept, ex, "subjects.json")
		if doc == nil {
			return
		}
		stale, _ := doc["stale"].(map[string]any)
		reason := str(stale, "reason")
		kept.expect(doc["current"] == false && (reason == "unreachable" || !fresh && reason == "cached"),
			"%s with the app stopped answered current %v, stale %v: want the copy, current false, stale unreachable", ex, doc["current"], doc["stale"])
		kept.expect(copyAt == "" || str(doc, "taken_at") == copyAt,
			"%s with the app stopped answered taken_at %s; the copy was taken at %s", ex, str(doc, "taken_at"), copyAt)
		return
	}
	if !none.expect(ex.err == nil && ex.status == http.StatusServiceUnavailable && (ex.code() == "TARGET_UNREACHABLE" || ex.code() == "NO_CURRENT_READ"),
		"%s with the app stopped answered %d %s (%v); with no copy, want 503 TARGET_UNREACHABLE or NO_CURRENT_READ", ex, ex.status, ex.code(), ex.err) {
		return
	}
	var extra []string
	for key := range ex.doc() {
		if !slices.Contains(problemMembers, key) {
			extra = append(extra, key)
		}
	}
	slices.Sort(extra)
	none.expect(len(extra) == 0, "%s with no copy carries %v beside the problem; the problem and nothing else", ex, extra)
}

// problemMembers are RFC 9457's members and the contract's code and retry:
// all a reply with nothing to serve carries.
var problemMembers = []string{"type", "title", "status", "detail", "instance", "code", "retry"}

// readNow is a read of the app now: fresh, past the cooldown, when the add-on
// declares freshness; otherwise every /subjects is one.
func (k *kit) readNow() *exchange {
	if !k.m.has("freshness") {
		return k.subjectsRead(false)
	}
	if d, why := k.cooldown(); why == "" {
		k.wait(d)
	}
	return k.subjectsRead(true)
}

// addonRestart is outage.restart: the add-on stopped and started, its disk
// kept. /subjects never dates an old read anew, and a call made before the
// stop still answers.
func (k *kit) addonRestart() {
	c := k.at("outage.restart")
	if k.cfg.StopAddon == nil || k.cfg.StartAddon == nil {
		k.notChecked("-stop-addon and -start-addon", false, "outage.restart")
		return
	}
	before := k.ok(c, k.subjectsRead(false), "subjects.json")
	if err := k.cfg.StopAddon(k.ctx); !c.expect(err == nil, "-stop-addon failed: %v", err) {
		return
	}
	if err := k.cfg.StartAddon(context.WithoutCancel(k.ctx)); !c.expect(err == nil, "-start-addon failed: %v", err) {
		return
	}
	if after := k.ok(c, k.subjectsRead(false), "subjects.json"); after != nil && after["current"] == false {
		stale, _ := after["stale"].(map[string]any)
		then, errThen := time.Parse(time.RFC3339Nano, str(before, "taken_at"))
		now, errNow := time.Parse(time.RFC3339Nano, str(after, "taken_at"))
		c.expect(str(stale, "reason") == "cached" && errNow == nil && (errThen != nil || !now.After(then)),
			"/subjects after the restart answered stale %v, taken_at %s; read %s before the stop: want the copy as cached, never dated anew",
			after["stale"], str(after, "taken_at"), str(before, "taken_at"))
	}
	if k.passwordCall == "" || !k.m.has("calls") {
		c.skip("no write's call_id is known (needs -mutate and feature calls), so a call from before the stop is not asked")
		return
	}
	k.callAnswers(c, k.passwordCall, k.passwordReply)
}

// ---- noFalseZero ------------------------------------------------------------

// nothingToServe are the codes of a read that has no value to answer: a reply
// carrying one carries no figure, list or date.
var nothingToServe = []string{"TARGET_UNREACHABLE", "NO_CURRENT_READ", "TARGET_UNREADABLE", "FRESH_READ_COOLDOWN", "STORE_UNREADABLE"}

// noFalseZero walks one reply: with nothing to serve it carries no 0, empty
// list, "" or zero date; no reply carries a zero taken_at, or current false
// without stale. A value not read is absent, never zero.
func (k *kit) noFalseZero(c *check, ex *exchange) {
	var doc any
	dec := json.NewDecoder(bytes.NewReader(ex.body))
	dec.UseNumber()
	if dec.Decode(&doc) != nil {
		return
	}
	c.mark()
	empty := ex.status >= 300 && slices.Contains(nothingToServe, ex.code())
	var walk func(path string, v any)
	walk = func(path string, v any) {
		switch v := v.(type) {
		case map[string]any:
			if cur, ok := v["current"].(bool); ok && !cur {
				_, has := v["stale"]
				c.expect(has, "%s: %s says current false without stale", ex, path)
			}
			for key, e := range v {
				if s, ok := e.(string); ok && key == "taken_at" {
					c.expect(!zeroDate(s) && s != "", "%s: %s.taken_at is %q, a date nobody read", ex, path, s)
				}
				walk(path+"."+key, e)
			}
		case []any:
			c.expect(!empty || len(v) > 0, "%s with nothing to serve carries an empty list at %s", ex, path)
			for i, e := range v {
				walk(path+"["+strconv.Itoa(i)+"]", e)
			}
		case json.Number:
			f, err := v.Float64()
			c.expect(!empty || err != nil || f != 0, "%s with nothing to serve carries %s at %s", ex, v, path)
		case string:
			c.expect(!empty || v != "" && !zeroDate(v), "%s with nothing to serve carries %q at %s", ex, v, path)
		}
	}
	walk("$", doc)
}

// zeroDate is a date-time that is Go's zero time or the Unix epoch: a
// default, never a reading.
func zeroDate(s string) bool {
	t, err := time.Parse(time.RFC3339Nano, s)
	return err == nil && (t.IsZero() || t.Unix() == 0)
}
