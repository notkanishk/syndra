package conformance

import (
	// Zone data in the binary: stats_zone names an IANA zone (RFC 9557,
	// tzdb), and the kit judges midnights in it wherever it runs.
	_ "time/tzdata"

	"encoding/json"
	"fmt"
	"maps"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"
)

// The statistics checks (feature stats). A point is one bucket of the
// OpenTelemetry metrics data model, kept by the contract as a whole calendar
// day in the manifest's stats_zone: start a midnight there, end the next
// (RFC 3339 instants with the zone's offset). The kit reads every metric from
// now minus limits.stats_buffer, all its pages, twice; reads it from a later
// since and from one older than the buffer; and, with -mutate, once more after
// its writes, when its own account may have usage.

var statsChecks = []string{"stats.descriptors", "stats.points", "stats.totals", "stats.since", "stats.idempotent",
	"stats.gaps", "stats.unbound", "stats.buffer", "stats.page-size", "stats.no-people", "stats.no-people.ids"}

// statsMaxPages bounds one read: a cursor that never ends is a failure, not a
// kit that never ends.
const statsMaxPages = 1000

// statsRead is one metric read from since to its last page.
type statsRead struct {
	first             map[string]any   // the first page: taken_at, watermark, unbound, gaps
	points            []map[string]any // every page's, in order
	keys              []string         // each point's keyset key
	watermark, takeAt time.Time
	pages             []string           // each page's cursor, "" for the first
	pagePoints        [][]map[string]any // each page's points
}

type statsSetup struct {
	zone   *time.Location
	since  time.Time
	locals map[string]bool // every target account id and username /subjects lists
	own    map[string]bool // the username and account_id of the account the kit created, once it has
	bound  map[string]bool // point subjects that are also local names, by whether the add-on's /plan says they are bound
	// unproven says, for a subject whose /plan was refused or failed, what
	// it answered: a passing hiccup is not read as a naming defect.
	unproven map[string]string
}

func (k *kit) stats() {
	if !k.m.has("stats") {
		for _, id := range statsChecks {
			k.at(id).skip("feature stats not declared")
		}
		return
	}
	set, why := k.statsSetup(k.at("stats.descriptors"))
	if why == "" {
		why = k.statsCollected()
	}
	if why != "" {
		for _, id := range statsChecks[1:] {
			k.at(id).skip("%s", why)
		}
		return
	}
	since, pageSize := k.at("stats.since"), k.at("stats.page-size")
	for _, m := range k.m.metrics {
		name := str(m, "name")
		r1, ok1 := k.readStats(name, set.since, set.zone)
		r2, ok2 := k.readStats(name, set.since, set.zone)
		if !ok1 || !ok2 {
			continue
		}
		k.judgeStats(m, r1, set)
		k.judgeStats(m, r2, set)
		k.sameFinal(k.at("stats.idempotent"), name, r1, r2)
		since.expect(!r2.watermark.Before(r1.watermark), "%s: the watermark moved back from %v to %v", name, r1.watermark, r2.watermark)

		// A cursor read again gives the same next page.
		if len(r1.pages) > 1 {
			again := k.page(name, set.since, r1.pages[1])
			pageSize.expect(again != nil && same(objects(again["points"]), r1.pagePoints[1]),
				"%s: the page after cursor %s read again is not the page it gave", name, r1.pages[1])
		}

		// A later since, halfway to now: nothing that ends at or before it.
		later := set.since.Add(time.Since(set.since) / 2)
		if r, ok := k.readStats(name, later, set.zone); ok {
			k.judgeSince(since, name, r, later)
		}

		// A since older than the buffer: a gap for what it no longer holds.
		k.buffer(k.at("stats.buffer"), name, set, len(r1.points) > 0)
	}
	ex := k.send(request{method: http.MethodGet, target: "/stats?" + url.Values{"metric": {"conformance.no_such_metric"},
		"since": {set.since.UTC().Format(time.RFC3339)}}.Encode()})
	k.refused(since, ex, 404, "UNKNOWN_METRIC", "an undeclared metric")
}

// statsAfterWrites reads every metric once more after the -mutate writes,
// when the kit's own account may have usage of its own, and judges it again.
func (k *kit) statsAfterWrites() {
	if !k.m.has("stats") {
		return
	}
	set, why := k.statsSetup(&check{})
	if why == "" {
		why = k.statsCollected()
	}
	if why != "" {
		return // said in the reads step
	}
	for _, m := range k.m.metrics {
		if r, ok := k.readStats(str(m, "name"), set.since, set.zone); ok {
			k.judgeStats(m, r, set)
		}
	}
}

// statsSetup checks the descriptors and says the zone, the since of a first
// collection, and the target's local names, or why nothing more is judged.
func (k *kit) statsSetup(c *check) (statsSetup, string) {
	var set statsSetup
	zone, err := time.LoadLocation(k.m.statsZone)
	if !c.expect(err == nil && k.m.statsZone != "" && k.m.statsZone != "Local", "stats_zone %q is not an IANA zone: %v", k.m.statsZone, err) {
		return set, "stats_zone is unusable (see stats.descriptors)"
	}
	declared := k.m.statsBuffer
	if declared == "" {
		declared = "P90D"
	}
	days, err := strconv.Atoi(strings.TrimSuffix(strings.TrimPrefix(declared, "P"), "D"))
	if !strings.HasPrefix(declared, "P") || !strings.HasSuffix(declared, "D") || err != nil {
		c.skip("limits.stats_buffer %s is not P<n>D, so the kit cannot compute a first collection's since", declared)
		return set, "limits.stats_buffer is not P<n>D, so the kit cannot compute a since"
	}
	c.expect(days >= 1 && days <= 400, "limits.stats_buffer %s is outside P1D..P400D", declared)

	limits := map[string]string{}
	for _, f := range k.m.fields {
		if f.Role == "limit" {
			limits[f.Name] = f.Unit
		}
	}
	names := map[string]bool{}
	for _, m := range k.m.metrics {
		name := str(m, "name")
		c.expect(!names[name], "metric %s is declared twice", name)
		names[name] = true
		dims := map[string]bool{}
		for _, d := range objects(m["dimensions"]) {
			c.expect(!dims[str(d, "name")], "metric %s declares dimension %s twice", name, str(d, "name"))
			dims[str(d, "name")] = true
		}
		if against := str(m, "against"); against != "" {
			unit, isLimit := limits[against]
			c.expect(isLimit && unit == str(m, "unit"), "metric %s is against %q, which is not a limit field in %s", name, against, str(m, "unit"))
		}
	}
	set.zone, set.since = zone, time.Now().In(zone).AddDate(0, 0, -days)
	set.locals = map[string]bool{}
	for _, s := range objects(k.send(request{method: http.MethodGet, target: "/subjects"}).doc()["subjects"]) {
		set.locals[str(s, "account_id")], set.locals[str(s, "username")] = true, true
	}
	delete(set.locals, "")
	set.bound, set.unproven = map[string]bool{}, map[string]string{}
	// Only the kit's own account is surely a person: its name and id are the
	// kit's to make unique. Another listed account may be a service account a
	// resource is named after (a scanner share, a scanner user), so a match on
	// it is advisory (stats.no-people.ids).
	set.own = map[string]bool{k.created.username: true, k.created.id: true}
	delete(set.own, "")
	return set, ""
}

// learnBound asks the add-on, in one withdraw-only /plan, which point
// subjects that are also a listed account's id or name are bound. A plan
// changes nothing, and its outcome carries account_id only when the subject
// has an account (apply_response.json), which for a subject is its binding.
// Withdraw-only never creates, so the empty email (which the guide allows)
// derives no name. A bound subject is a Syndra subject, whatever an account is
// named: an add-on may name it after the subject (the reference does, so every
// earlier kit run's account, disabled and still bound, collides; #51 D1). A
// subject the plan does not prove bound stays a local name; when the plan was
// refused or failed, the failure says so. Only collisions are asked, so a run
// usually sends no plan here.
func (k *kit) learnBound(set statsSetup, points []map[string]any) {
	var ask []string
	for _, p := range points {
		s := str(p, "subject")
		if _, known := set.bound[s]; s != "" && set.locals[s] && !known && !slices.Contains(ask, s) {
			ask = append(ask, s)
		}
	}
	for len(ask) > 0 {
		n := len(ask)
		if k.m.perPlan > 0 {
			n = min(n, int(k.m.perPlan))
		}
		subjects := []any{}
		for _, s := range ask[:n] {
			set.bound[s] = false
			subjects = append(subjects, map[string]any{"subject": s, "email": "", "desired": map[string]any{k.m.account: false}})
		}
		ex := k.send(request{method: http.MethodPost, target: "/plan", body: map[string]any{"contract_version": Major, "subjects": subjects}, schema: "plan_request.json"})
		why, outcomes := "", objects(ex.doc()["outcomes"])
		switch {
		case ex.err != nil:
			why = fmt.Sprintf(", and /plan failed (%v), so its binding is unproven", ex.err)
		case ex.status != http.StatusOK:
			why = fmt.Sprintf(", and /plan answered %d, so its binding is unproven", ex.status)
		}
		if why != "" {
			outcomes = nil // a refusal proves no binding, whatever its body says
			for _, s := range ask[:n] {
				set.unproven[s] = why
			}
		}
		for _, o := range outcomes {
			if s := str(o, "subject"); slices.Contains(ask[:n], s) && o["account_id"] != nil {
				set.bound[s] = true
			}
		}
		ask = ask[n:]
	}
}

// statsCollected is why nothing is judged when the add-on has read nothing
// from its app yet (503 NO_CURRENT_READ), or "".
func (k *kit) statsCollected() string {
	if len(k.m.metrics) == 0 {
		return ""
	}
	ex := k.send(request{method: http.MethodGet, target: "/stats?" + url.Values{"metric": {str(k.m.metrics[0], "name")},
		"since": {time.Now().UTC().Format(time.RFC3339)}}.Encode()})
	if ex.status == http.StatusServiceUnavailable && ex.code() == "NO_CURRENT_READ" {
		return "the add-on has collected nothing from its app yet (503 NO_CURRENT_READ); run the kit again once it has"
	}
	return ""
}

func (k *kit) statsTarget(metric string, since time.Time, cursor string) string {
	q := url.Values{"metric": {metric}, "since": {since.UTC().Format(time.RFC3339)}}
	if cursor != "" {
		q.Set("cursor", cursor)
	}
	return "/stats?" + q.Encode()
}

// page reads one page and checks it against stats.json and the reply ceiling.
func (k *kit) page(metric string, since time.Time, cursor string) map[string]any {
	ex := k.send(request{method: http.MethodGet, target: k.statsTarget(metric, since, cursor)})
	if ex.err != nil && strings.Contains(ex.err.Error(), "max_reply_bytes") {
		k.at("stats.page-size").fail("%s: %v", ex, ex.err)
	}
	doc := k.ok(k.at("stats.points"), ex, "stats.json")
	if doc != nil {
		k.at("stats.points").expect(str(doc, "metric") == metric, "%s answered for metric %q", ex, str(doc, "metric"))
	}
	return doc
}

// readStats reads one metric from since through its last page, each page's
// keys strictly after the last.
func (k *kit) readStats(metric string, since time.Time, zone *time.Location) (statsRead, bool) {
	var r statsRead
	c := k.at("stats.page-size")
	cursor := ""
	for range statsMaxPages {
		doc := k.page(metric, since, cursor)
		if doc == nil {
			return r, false
		}
		r.pages, r.pagePoints = append(r.pages, cursor), append(r.pagePoints, objects(doc["points"]))
		if r.first == nil {
			r.first = doc
			r.watermark, _ = time.Parse(time.RFC3339, str(doc, "watermark"))
			r.takeAt, _ = time.Parse(time.RFC3339, str(doc, "taken_at"))
		}
		for _, p := range objects(doc["points"]) {
			key := pointKey(p)
			if n := len(r.keys); n > 0 && key <= r.keys[n-1] {
				c.fail("%s: point %v does not come after the one before it (keyset order start, subject, dimensions): a page repeats or reorders", metric, p)
			}
			r.points, r.keys = append(r.points, p), append(r.keys, key)
		}
		next, _ := doc["next_cursor"].(string)
		if next == "" {
			c.mark()
			return r, true
		}
		cursor = next
	}
	c.fail("%s: still a next_cursor after %d pages", metric, statsMaxPages)
	return r, false
}

// pointKey orders points as the cursor does: start as an instant, then
// subject, then dimensions as canonical JSON (an absent one as {}).
func pointKey(p map[string]any) string {
	start, _ := time.Parse(time.RFC3339, str(p, "start"))
	return fmt.Sprintf("%020d\x00%s\x00%s", start.Unix(), str(p, "subject"), canonical(p["dimensions"]))
}

// canonical is dimensions as canonical JSON (encoding/json sorts map keys),
// an absent or empty one as {}.
func canonical(dims any) string {
	if d, ok := dims.(map[string]any); ok && len(d) > 0 {
		b, _ := json.Marshal(d)
		return string(b)
	}
	return "{}"
}

// midnight is the first instant of the day t falls in: 00:00, or, on a day
// whose 00:00 a daylight-saving change skips (America/Santiago, 6 Sep 2026),
// the instant the clocks resume. Go resolves a skipped 00:00 to the previous
// day's 23:00, so that one case takes the end of the zone period it fell in.
func midnight(t time.Time, zone *time.Location) time.Time {
	l := t.In(zone)
	d := time.Date(l.Year(), l.Month(), l.Day(), 0, 0, 0, 0, zone)
	if d.Day() != l.Day() {
		_, d = d.ZoneBounds()
	}
	return d
}

// nextMidnight is the next day's first instant: a calendar day, 23 or 25
// hours across a change. Noon is inside the next day whatever the zone does
// at midnight.
func nextMidnight(t time.Time, zone *time.Location) time.Time {
	l := t.In(zone)
	return midnight(time.Date(l.Year(), l.Month(), l.Day()+1, 12, 0, 0, 0, zone), zone)
}

func value(v any) float64 { n, _ := v.(json.Number); f, _ := n.Float64(); return f }

// judgeStats holds one whole read to the rules every reply keeps.
func (k *kit) judgeStats(m map[string]any, r statsRead, set statsSetup) {
	name, perSubject, counter := str(m, "name"), m["per_subject"] == true, str(m, "instrument") == "counter"
	declared := map[string]bool{}
	for _, d := range objects(m["dimensions"]) {
		declared[str(d, "name")] = true
	}
	pts, totals, gaps := k.at("stats.points"), k.at("stats.totals"), k.at("stats.gaps")
	unbound, people, ids := k.at("stats.unbound"), k.at("stats.no-people"), k.at("stats.no-people.ids")
	day := func(c *check, what string, o map[string]any) (time.Time, time.Time, bool) {
		start, err1 := time.Parse(time.RFC3339, str(o, "start"))
		end, err2 := time.Parse(time.RFC3339, str(o, "end"))
		return start, end, c.expect(err1 == nil && err2 == nil && start.Equal(midnight(start, set.zone)),
			"%s: %s starts at %s, not a midnight in %s", name, what, str(o, "start"), set.zone)
	}
	dimsOf := func(c *check, what string, o map[string]any) map[string]any {
		d, _ := o["dimensions"].(map[string]any)
		for key, v := range d {
			c.expect(declared[key], "%s: %s names dimension %q, which the metric does not declare", name, what, key)
			s, _ := v.(string)
			if people.expect(!set.own[s], "%s: dimension %s is %q, the username or account_id of the kit's own account", name, key, s) {
				ids.expect(!set.locals[s], "%s: dimension %s is %q, the username or account_id of an account on the target", name, key, s)
			}
		}
		return d
	}

	sums, maxes, totalOf := map[string]float64{}, map[string]float64{}, map[string]float64{}
	k.learnBound(set, r.points)
	for _, p := range r.points {
		what := "point " + str(p, "start")
		start, end, ok := day(pts, what, p)
		if ok {
			pts.expect(end.Equal(nextMidnight(start, set.zone)), "%s: %s ends at %s, not the next midnight in %s", name, what, str(p, "end"), set.zone)
		}
		dims := dimsOf(pts, what, p)
		v := value(p["value"])
		totals.expect(v >= 0, "%s: %s has value %v; every value is at least 0", name, what, p["value"])
		bucket := start.UTC().Format(time.RFC3339) + " " + canonical(dims)
		if subject := str(p, "subject"); subject != "" {
			pts.expect(perSubject, "%s: %s carries a subject on a metric that is not per_subject", name, what)
			unbound.expect(!set.locals[subject] || set.bound[subject], "%s: %s is reported for %q, the id or name of an account on the target, not a Syndra subject%s", name, what, subject, set.unproven[subject])
			sums[bucket] += v
			maxes[bucket] = max(maxes[bucket], v)
		} else {
			totalOf[bucket] = v
		}
	}
	for bucket, sum := range sums {
		total, has := totalOf[bucket]
		if !totals.expect(has, "%s: day and dimensions %s have per-person points and no total", name, bucket) {
			continue
		}
		if counter {
			totals.expect(sum <= total, "%s: %s: the people's points sum to %v, above the total %v", name, bucket, sum, total)
		} else {
			totals.expect(maxes[bucket] <= total, "%s: %s: a person's point %v is above the total %v", name, bucket, maxes[bucket], total)
		}
	}
	n, _ := num(objectOf(r.first["unbound"])["accounts"])
	for bucket, total := range totalOf {
		if counter && perSubject && sums[bucket] < total {
			unbound.expect(n >= 1, "%s: %s: the total %v is above its people's %v and unbound.accounts is %d: who else used it?", name, bucket, total, sums[bucket], n)
		}
	}

	for _, g := range objects(r.first["gaps"]) {
		what := "gap " + str(g, "start")
		start, end, ok := day(gaps, what, g)
		if ok {
			gaps.expect(end.After(start) && end.Equal(midnight(end, set.zone)), "%s: %s ends at %s, not a later midnight", name, what, str(g, "end"))
		}
		gaps.expect(strings.TrimSpace(str(g, "reason")) != "", "%s: %s has no reason", name, what)
		gdims := dimsOf(gaps, what, g)
		for _, p := range r.points {
			at, _ := time.Parse(time.RFC3339, str(p, "start"))
			pdims, _ := p["dimensions"].(map[string]any)
			covered := !at.Before(start) && at.Before(end)
			for key, v := range gdims {
				covered = covered && pdims[key] == v
			}
			gaps.expect(!covered, "%s: point %s %v (value %v) lies inside %s %v: a window not observed is a gap, never a value",
				name, str(p, "start"), pdims, p["value"], what, gdims)
		}
	}
	k.judgeSince(k.at("stats.since"), name, r, set.since)
}

// judgeSince: nothing ends at or before since; the watermark is a midnight,
// and no later than the start of the day taken_at is in.
func (k *kit) judgeSince(c *check, name string, r statsRead, since time.Time) {
	zone := k.statsZone()
	for _, o := range append(slices.Clone(r.points), objects(r.first["gaps"])...) {
		end, _ := time.Parse(time.RFC3339, str(o, "end"))
		c.expect(end.After(since), "%s: %v ends at or before since %s", name, o, since.Format(time.RFC3339))
	}
	c.expect(r.watermark.Equal(midnight(r.watermark, zone)) && !r.watermark.After(midnight(r.takeAt, zone)),
		"%s: watermark %s is not a midnight in %s at or before the start of taken_at's day (%s)", name, str(r.first, "watermark"), zone, str(r.first, "taken_at"))
}

func (k *kit) statsZone() *time.Location {
	zone, err := time.LoadLocation(k.m.statsZone)
	if err != nil {
		return time.UTC
	}
	return zone
}

// sameFinal: what ends at or before the watermark is final, so two reads
// with one since answer it alike.
func (k *kit) sameFinal(c *check, name string, r1, r2 statsRead) {
	final := func(r statsRead, wm time.Time) (map[string]string, []string) {
		points := map[string]string{}
		for i, p := range r.points {
			if end, _ := time.Parse(time.RFC3339, str(p, "end")); !end.After(wm) {
				points[r.keys[i]] = string(p["value"].(json.Number))
			}
		}
		var gaps []string
		for _, g := range objects(r.first["gaps"]) {
			if end, _ := time.Parse(time.RFC3339, str(g, "end")); !end.After(wm) {
				b, _ := json.Marshal(g)
				gaps = append(gaps, string(b))
			}
		}
		slices.Sort(gaps)
		return points, gaps
	}
	wm := r1.watermark // what the first read called final
	p1, g1 := final(r1, wm)
	p2, g2 := final(r2, wm)
	for key := range maps.Keys(maps.Clone(p1)) {
		c.expect(p2[key] == p1[key], "%s: a final point (%s) was %s and is now %q", name, strings.ReplaceAll(key, "\x00", " "), p1[key], p2[key])
	}
	for key, v := range p2 {
		c.expect(p1[key] != "", "%s: a final point (%s) appeared with %s, absent before", name, strings.ReplaceAll(key, "\x00", " "), v)
	}
	c.expect(slices.Equal(g1, g2), "%s: the final gaps changed between two reads: %v, then %v", name, g1, g2)
}

// buffer asks from ten days before the buffer: the add-on answers a gap
// covering that day with a reason, or shows it holds it (a point before the
// buffer's start), and still answers the points it holds.
func (k *kit) buffer(c *check, name string, set statsSetup, holds bool) {
	old := set.since.AddDate(0, 0, -10)
	doc := k.page(name, old, "")
	if doc == nil {
		return
	}
	day := midnight(old, set.zone)
	covered := false
	for _, g := range objects(doc["gaps"]) {
		start, _ := time.Parse(time.RFC3339, str(g, "start"))
		end, _ := time.Parse(time.RFC3339, str(g, "end"))
		covered = covered || !start.After(day) && end.After(day) && strings.TrimSpace(str(g, "reason")) != ""
	}
	for _, p := range objects(doc["points"]) {
		start, _ := time.Parse(time.RFC3339, str(p, "start"))
		covered = covered || start.Before(midnight(set.since, set.zone))
	}
	c.expect(covered, "%s: asked from %s, ten days before the buffer, and no gap with a reason covers that day", name, day.Format(time.DateOnly))
	c.expect(!holds || len(objects(doc["points"])) > 0, "%s: asked from before the buffer, it answered no points, which it holds", name)
}

func objectOf(v any) map[string]any { m, _ := v.(map[string]any); return m }
