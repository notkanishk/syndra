package conformance

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"slices"
	"strconv"
	"testing"
	"time"
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
		{"a dimension naming a person", "stats.no-people", true, "/stats", eachPoint(func(p map[string]any) {
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
