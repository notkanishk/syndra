package conformance

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strconv"
	"testing"
	"time"

	"syndra/addons/reference"
)

// The stats checks skip, with the reason, without the feature, and pass the
// reference serving it, reads only and with -mutate (when the kit's own
// account has usage of its own).
func TestTheStatsChecksJudgeOnlyWhatIsDeclared(t *testing.T) {
	url, _ := serve(t, testSecret, nil)
	r := run(t, config(url, false))
	for _, id := range statsChecks {
		if res := result(r, id); res.Status != "skip" || !slices.Equal(res.Detail, []string{"feature stats not declared"}) {
			t.Errorf("%s says %s %v without the feature, want skip: feature stats not declared", id, res.Status, res.Detail)
		}
	}
	for _, mutate := range []bool{false, true} {
		url, _ := serveRewriting(t, testSecret, nil, nil, []string{"stats"})
		r := run(t, config(url, mutate))
		for _, f := range failures(r) {
			t.Errorf("mutate %v: %s failed against the reference serving stats: %v", mutate, f.ID, f.Detail)
		}
		for _, id := range statsChecks {
			if got := r.Status(id); got != "pass" {
				t.Errorf("mutate %v: %s says %s, want pass", mutate, id, got)
			}
		}
	}
}

// statsRewrite edits every /stats (or /capabilities) reply of the reference
// serving stats.
func statsRewrite(path string, edit func(doc map[string]any)) func(*http.Request, []byte) []byte {
	return func(r *http.Request, reply []byte) []byte {
		var doc map[string]any
		if r.URL.Path != path || json.Unmarshal(reply, &doc) != nil || doc["code"] != nil {
			return reply
		}
		edit(doc)
		out, _ := json.Marshal(doc)
		return out
	}
}

func eachPoint(edit func(p map[string]any)) func(map[string]any) {
	return func(doc map[string]any) {
		for _, p := range doc["points"].([]any) {
			edit(p.(map[string]any))
		}
	}
}

// What the reference's defects cannot serve, the kit catches all the same.
func TestTheStatsChecksCatchWhatTheDefectsCannotServe(t *testing.T) {
	ist := func(s string) time.Time { at, _ := time.Parse(time.RFC3339, s); return at }
	for _, tc := range []struct {
		name, check string
		mutate      bool
		path        string
		edit        func(map[string]any)
	}{
		{"a meter against a field that is not a limit", "stats.descriptors", false, "/capabilities", func(doc map[string]any) {
			doc["metrics"].([]any)[1].(map[string]any)["against"] = "printers"
		}},
		{"a zone no tzdb has", "stats.descriptors", false, "/capabilities", func(doc map[string]any) { doc["stats_zone"] = "Mars/Olympus_Mons" }},
		{"a buffer past P400D", "stats.descriptors", false, "/capabilities", func(doc map[string]any) {
			doc["limits"].(map[string]any)["stats_buffer"] = "P401D"
		}},
		{"UTC days, not the add-on's", "stats.points", false, "/stats", eachPoint(func(p map[string]any) {
			p["start"] = ist(p["start"].(string)).Format(time.DateOnly) + "T00:00:00Z"
			p["end"] = ist(p["end"].(string)).Format(time.DateOnly) + "T00:00:00Z"
		})},
		{"a day of 23 hours", "stats.points", false, "/stats", eachPoint(func(p map[string]any) {
			p["end"] = ist(p["start"].(string)).Add(23 * time.Hour).Format(time.RFC3339)
		})},
		{"a day that starts an hour after midnight", "stats.points", false, "/stats", eachPoint(func(p map[string]any) {
			p["start"] = ist(p["start"].(string)).Add(time.Hour).Format(time.RFC3339)
		})},
		{"a negative value", "stats.totals", false, "/stats", eachPoint(func(p map[string]any) { p["value"] = -1 })},
		{"a watermark that is not a midnight", "stats.since", false, "/stats", func(doc map[string]any) {
			doc["watermark"] = ist(doc["watermark"].(string)).Add(time.Hour).Format(time.RFC3339)
		}},
		{"a gap with no reason", "stats.gaps", false, "/stats", func(doc map[string]any) {
			for _, g := range doc["gaps"].([]any) {
				g.(map[string]any)["reason"] = " "
			}
		}},
		{"usage by nobody it names", "stats.unbound", false, "/stats", func(doc map[string]any) {
			doc["unbound"] = map[string]any{"accounts": 0}
		}},
		{"a dimension naming the kit's own account id", "stats.no-people", true, "/stats", eachPoint(func(p map[string]any) {
			if dims, ok := p["dimensions"].(map[string]any); ok && dims["printer"] == "mk4-a" {
				dims["printer"] = "1001" // the kit's own account, once it exists
			}
		})},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			url, _ := serveRewriting(t, testSecret, nil, statsRewrite(tc.path, tc.edit), []string{"stats"})
			r, err := Run(context.Background(), config(url, tc.mutate))
			if err != nil && !errors.Is(err, ErrReadsFailed) {
				t.Fatal(err)
			}
			if got := r.Status(tc.check); got != "fail" {
				t.Errorf("%s says %s, want fail: %v", tc.check, got, result(r, tc.check).Detail)
			}
		})
	}
}

// Only the kit's own account is surely a person. A correct add-on whose
// resource is named like a listed service account (the mk4-a printer, run by
// an mk4-a account, as a scanner share is written by a scanner user) passes
// the MUST and draws only the advisory.
func TestAResourceNamedLikeAServiceAccountIsAdvisory(t *testing.T) {
	service := map[string]any{"id": "1500", "username": "mk4-a", "account_id": "1500",
		"state":    map[string]any{"enabled": true, "remote": false, "printers": []any{}, "after_hours": []any{}, "job_seconds": []any{"0"}},
		"standing": map[string]any{"usable": true, "waiting_on": "none"}}
	url, _ := serveRewriting(t, testSecret, nil, statsRewrite("/subjects", func(doc map[string]any) {
		doc["subjects"] = append(doc["subjects"].([]any), service)
	}), []string{"stats"})
	r, err := Run(context.Background(), config(url, true))
	if err != nil {
		t.Fatal(err, failures(r))
	}
	if got := r.Status("stats.no-people"); got != "pass" {
		t.Errorf("stats.no-people says %s, want pass: %v", got, result(r, "stats.no-people").Detail)
	}
	if f := failures(r); len(f) != 1 || f[0].ID != "stats.no-people.ids" || f[0].Level != "ADVISORY" || r.Failed() {
		t.Errorf("failures %+v (run failed %v), want only the advisory stats.no-people.ids", f, r.Failed())
	}
}

// The totals rule is the instrument's (OpenTelemetry: a counter is a
// monotonic sum, every event someone's; a gauge is a level, and a level of
// the whole may be a sum or a maximum): a counter's people sum to no more
// than the total, and each of a gauge's people is no more than it.
func TestTheTotalsRuleIsTheInstruments(t *testing.T) {
	zone, _ := time.LoadLocation("Asia/Kolkata")
	day := "2026-11-09T00:00:00+05:30"
	pt := func(subject string, v int) map[string]any {
		p := map[string]any{"start": day, "end": "2026-11-10T00:00:00+05:30", "value": json.Number(strconv.Itoa(v))}
		if subject != "" {
			p["subject"] = subject
		}
		return p
	}
	for _, tc := range []struct {
		name, instrument string
		total, a, b      int
		fails            bool
	}{
		{"people of nothing with no total", "counter", -1, 0, 0, true},
		{"a counter's people under the total", "counter", 3, 2, 1, false},
		{"a counter's people above the total, each under it", "counter", 3, 2, 2, true},
		{"a gauge's people each under the total, their sum above (a maximum)", "gauge", 3, 3, 2, false},
		{"a gauge with a person above the total", "gauge", 3, 4, 1, true},
	} {
		k, err := newKit(context.Background(), config("https://addon.example.org", false))
		if err != nil {
			t.Fatal(err)
		}
		k.m = &manifest{statsZone: "Asia/Kolkata"}
		r := statsRead{first: map[string]any{"unbound": map[string]any{"accounts": json.Number("1")}, "gaps": []any{},
			"watermark": day, "taken_at": "2026-11-10T09:00:00Z"}}
		r.watermark, _ = time.Parse(time.RFC3339, day)
		r.takeAt, _ = time.Parse(time.RFC3339, "2026-11-10T09:00:00Z")
		r.points = []map[string]any{pt("sub-a", tc.a), pt("sub-b", tc.b)}
		if tc.total >= 0 {
			r.points = append(r.points, pt("", tc.total))
		}
		k.judgeStats(map[string]any{"name": "machine.thing", "instrument": tc.instrument, "per_subject": true}, r,
			statsSetup{zone: zone, since: r.watermark.Add(-time.Hour), locals: map[string]bool{}})
		if failed := len(k.at("stats.totals").fails) > 0; failed != tc.fails {
			t.Errorf("%s: stats.totals failed %v (%v), want %v", tc.name, failed, k.at("stats.totals").fails, tc.fails)
		}
	}
}

// A day starts at its first instant: 01:00 on a day whose 00:00 a
// daylight-saving change skips (America/Santiago, 6 Sep 2026), and the kit
// accepts it as the day's midnight, as it does a 25-hour day; the previous
// day's 23:00, what Go makes of the missing 00:00, it refuses.
func TestADayWhoseMidnightIsSkippedStartsWhenTheClocksResume(t *testing.T) {
	zone, err := time.LoadLocation("America/Santiago")
	if err != nil {
		t.Fatal(err)
	}
	pt := func(start, end string) map[string]any {
		return map[string]any{"start": start, "end": end, "value": json.Number("2")}
	}
	for _, tc := range []struct {
		name  string
		point map[string]any
		fails bool
	}{
		{"the day before the change", pt("2026-09-05T00:00:00-04:00", "2026-09-06T01:00:00-03:00"), false},
		{"the day that starts at 01:00", pt("2026-09-06T01:00:00-03:00", "2026-09-07T00:00:00-03:00"), false},
		{"a 25-hour day", pt("2026-04-04T00:00:00-03:00", "2026-04-05T00:00:00-04:00"), false},
		{"the previous day's 23:00", pt("2026-09-05T23:00:00-04:00", "2026-09-07T00:00:00-03:00"), true},
	} {
		k, err := newKit(context.Background(), config("https://addon.example.org", false))
		if err != nil {
			t.Fatal(err)
		}
		k.m = &manifest{statsZone: "America/Santiago"}
		r := statsRead{first: map[string]any{"unbound": map[string]any{"accounts": json.Number("0")}, "watermark": "2026-09-06T01:00:00-03:00",
			"taken_at": "2026-09-06T15:00:00Z", "gaps": []any{map[string]any{"start": "2026-09-06T01:00:00-03:00", "end": "2026-09-07T00:00:00-03:00",
				"dimensions": map[string]any{"printer": "xl-1"}, "reason": "The xl-1 printer's job log could not be read"}}}}
		r.watermark, _ = time.Parse(time.RFC3339, "2026-09-06T01:00:00-03:00")
		r.takeAt, _ = time.Parse(time.RFC3339, "2026-09-06T15:00:00Z")
		r.points = []map[string]any{tc.point}
		k.judgeStats(map[string]any{"name": "machine.thing", "instrument": "counter", "dimensions": []any{map[string]any{"name": "printer"}}}, r,
			statsSetup{zone: zone, since: time.Date(2026, 4, 1, 0, 0, 0, 0, zone), locals: map[string]bool{}})
		for _, id := range []string{"stats.gaps", "stats.since"} {
			if fails := k.at(id).fails; len(fails) > 0 {
				t.Errorf("%s: %s failed: %v", tc.name, id, fails)
			}
		}
		if failed := len(k.at("stats.points").fails) > 0; failed != tc.fails {
			t.Errorf("%s: stats.points failed %v (%v), want %v", tc.name, failed, k.at("stats.points").fails, tc.fails)
		}
	}
}

// One add-on outlives many kit runs. Each -mutate run leaves its subject's
// account behind, disabled and still bound, so a later run's reads see
// points for an earlier kit subject: that is a Syndra subject, not an
// account name, even where the add-on named the account after it (the
// reference does). Report #51 D1: reads after a -mutate failed stats.unbound,
// and the next -mutate was refused.
func TestPointsForAnEarlierKitSubjectAreNotAccountNames(t *testing.T) {
	url, _ := serveRewriting(t, testSecret, nil, nil, reference.OptionalFeatures)
	for i, step := range []struct {
		mutate, settings bool
		item             string
	}{{true, false, "item-0001"}, {false, false, ""}, {true, true, "item-0002"}} {
		cfg := config(url, step.mutate)
		cfg.Subject, cfg.MutateSettings, cfg.Item = fmt.Sprintf("conformance-self-test-%d", i), step.settings, step.item
		r := run(t, cfg)
		for _, f := range failures(r) {
			t.Errorf("run %d (mutate %v): %s failed against one reference serving every feature: %v", i, step.mutate, f.ID, f.Detail)
		}
		if got := r.Status("stats.unbound"); got != "pass" {
			t.Errorf("run %d: stats.unbound says %s, want pass", i, got)
		}
	}
}

// The namespace does not excuse this run's own account: when the add-on named
// it from an -email that is not the subject, a point carrying that name
// instead of the subject is a local name, and the kit can prove it.
func TestAPointNamingTheKitsOwnAccountIsStillALocalName(t *testing.T) {
	url, _ := serveRewriting(t, testSecret, nil, statsRewrite("/stats", eachPoint(func(p map[string]any) {
		if p["subject"] == "conformance-self-test" {
			p["subject"] = "conformance-nickname" // the account's username, not its subject
		}
	})), []string{"stats"})
	cfg := config(url, true)
	cfg.Email = "conformance-nickname@example.org"
	r, err := Run(context.Background(), cfg)
	if err != nil && !errors.Is(err, ErrReadsFailed) {
		t.Fatal(err)
	}
	if got := r.Status("stats.unbound"); got != "fail" {
		t.Errorf("stats.unbound says %s, want fail: %v", got, result(r, "stats.unbound").Detail)
	}
}
