package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	"syndra/internal/addons"
	"syndra/internal/clock"
	"syndra/internal/db"
	"syndra/internal/textsort"
)

// Statistics, read (addon-contract 6.8, design §9.7, §9.8). Who sees what:
//
//   - a member, their own per-person points, on /me/targets (usage), the
//     subject always the actor and never a parameter, less what names anyone
//     but them (named_subjects): such a series is left out and counted, such
//     a gap's reason replaced and its dimensions dropped (memberView);
//   - an admin, everything as stored: one person on GET /users/{id}/targets
//     (the same view, personalUsage), the target-wide series on GET
//     /targets/{target}/stats, and on GET /insights/stats each per-person
//     series' spread and the people in it with their values; never a member.
//
// Usage is traceable to a person, for admins only (the owner's ruling of 14
// Nov 2026, superseding design §9.7's "never whose"). No view audit (decision
// 14; the owner declined one again with this ruling). Every day is a
// makerspace day; a counter's sums are over days (temporal), never across
// series. Pages render from the stored descriptors, so this holds while the
// add-on is down.

// statsUsage is a target's statistics as one page shows them. Reported false
// is "this add-on does not report statistics", said rather than implied.
type statsUsage struct {
	Reported bool `json:"reported"`
	// KeptFrom, on a person's page, is the first day whose per-person points
	// this target still keeps (retention): a day before it is deleted, never
	// zero. Every series starts there at the earliest.
	KeptFrom string `json:"kept_from,omitempty"`
	// Today is the makerspace day this response was built on (YYYY-MM-DD). A
	// window "in the last n days" counts n to it, so a response cached across
	// midnight never miscounts against the reader's own clock.
	Today string `json:"today,omitempty"`
	// Window30dFrom and Window12mFrom are the first day each window's numbers
	// are read from (YYYY-MM-DD): the window's own start, or KeptFrom when
	// retention is shorter. The page names each window from them.
	Window30dFrom string            `json:"window_30d_from,omitempty"`
	Window12mFrom string            `json:"window_12m_from,omitempty"`
	Metrics       []statsMetricView `json:"metrics,omitempty"`
}

// statsMetricView is one metric: its descriptor, through when Syndra holds it
// final, its series and its gaps.
type statsMetricView struct {
	addons.Metric
	// Watermark is the makerspace midnight before which the metric is final;
	// nil before its first collection.
	Watermark *time.Time `json:"watermark"`
	// CountedThrough is the watermark as a day: the last final one, nil with
	// it. A day after it may still change, and one with no point is not yet
	// a zero.
	CountedThrough *string         `json:"counted_through"`
	Series         []statsSeries   `json:"series"`
	Gaps           []statsGapView  `json:"gaps"`
	Collected      *statsCollected `json:"collected,omitempty"`
	// LeftOut, on a member's own view, is how many of their series are left
	// out because a dimension value names a person: said, so the rest never
	// reads as all of their usage.
	LeftOut int `json:"left_out,omitempty"`
}

// statsSeries is one dimension set's days. A counter carries its 30-day and
// 12-month sums and its months; a gauge its latest day. Days are sparse: a
// counter's missing day inside an observed window is 0, a gauge's is no new
// sample, and a gap is never either.
type statsSeries struct {
	Dimensions map[string]string `json:"dimensions"`
	Sum30d     *float64          `json:"sum_30d,omitempty"`
	Sum12m     *float64          `json:"sum_12m,omitempty"`
	Latest     *statsDay         `json:"latest,omitempty"`
	Days       []statsDay        `json:"days"`
	Months     []statsMonth      `json:"months,omitempty"`
}

type statsDay struct {
	Day   string  `json:"day"`
	Value float64 `json:"value"`
}

type statsMonth struct {
	Month string  `json:"month"`
	Value float64 `json:"value"`
}

// statsGapView is a window nobody could observe, [start, end) in days.
type statsGapView struct {
	Start      string            `json:"start"`
	End        string            `json:"end"`
	Dimensions map[string]string `json:"dimensions,omitempty"`
	Reason     string            `json:"reason"`
}

// statsCollected is an admin's line about the last collection: when, how many
// accounts Syndra does not manage are in the totals, and what was dropped.
type statsCollected struct {
	At              *time.Time     `json:"at"`
	UnboundAccounts int            `json:"unbound_accounts"`
	Dropped         map[string]int `json:"dropped"`
}

// statsWindows is the two windows every page shows, in makerspace days:
// today, the first of the last 30 days, and the first day of the 12th month
// back (this month included).
func statsWindows(now time.Time) (today, from30, from12m string) {
	today = clock.Day(now)
	return today, clock.AddDate(today, 0, 0, -29), clock.AddDate(today[:8]+"01", 0, -11, 0)
}

// reasonWithheld is a gap's reason, on a member's own view, in place of one
// that names someone else.
const reasonWithheld = "The add-on's reason named a person."

// namesOther reports whether a row's named_subjects name anyone but subject:
// what a member's own view never shows. Their own name hides nothing of theirs.
func namesOther(named []string, subject string) bool {
	return slices.ContainsFunc(named, func(s string) bool { return s != subject })
}

// statsUsageFor is one target's statistics for a person (subject), or its
// totals (subject ""): a person's page carries per-person metrics only, and
// only that person's points; the totals page every metric and the collection
// line. memberView is the member's own page: a series with a point that names
// anyone but them is left out (and counted in LeftOut), and such a gap's
// reason is replaced and its dimensions dropped; an admin's page shows
// everything as stored.
//
// A series left out, not relabelled: what is named is the series', not one
// dimension's, so a relabelled row would be a number under a label that is
// neither the add-on's nor true, and two would be indistinguishable. Leaving
// it out and saying how many keeps every word on the page true.
//
// A gap that names someone else keeps its window with no dimensions: the days
// stay said as uncounted (dropped, they would read as counted, a counter's as
// 0), at the cost of covering the member's whole metric rather than one line.
func statsUsageFor(ctx context.Context, target, subject string, memberView bool, now time.Time) (statsUsage, error) {
	stored, err := dbStatsMetrics(ctx, target)
	if err != nil || len(stored) == 0 {
		return statsUsage{}, err
	}
	today, from30, from12m := statsWindows(now)
	out := statsUsage{Reported: true, Today: today, Window30dFrom: from30, Window12mFrom: from12m, Metrics: []statsMetricView{}}
	if subject != "" {
		// A person's points are read no earlier than retention keeps them, so
		// a sum never counts a day the daily prune has yet to delete.
		if out.KeptFrom, err = dbStatsKeptFrom(ctx, today, target); err != nil {
			return statsUsage{}, err
		}
		out.Window30dFrom, out.Window12mFrom = max(from30, out.KeptFrom), max(from12m, out.KeptFrom) // YYYY-MM-DD orders as text
	}
	days, err := dbStatsDays(ctx, target, subject, out.Window30dFrom)
	if err != nil {
		return statsUsage{}, err
	}
	months, err := dbStatsMonths(ctx, target, subject, out.Window12mFrom)
	if err != nil {
		return statsUsage{}, err
	}
	gaps, err := dbStatsGaps(ctx, target, from12m)
	if err != nil {
		return statsUsage{}, err
	}
	// A member's series is left out when any of its points read names someone else.
	leftOut := map[string]bool{}
	if memberView {
		for _, v := range slices.Concat(days, months) {
			if namesOther(v.NamedSubjects, subject) {
				leftOut[v.Metric+"\x00"+dimsText(v.Dims)] = true
			}
		}
	}
	for _, s := range stored {
		var m addons.Metric
		if err := json.Unmarshal(s.Descriptor, &m); err != nil {
			return statsUsage{}, err
		}
		if subject != "" && !m.PerSubject {
			continue
		}
		view := statsMetricView{Metric: m, Watermark: s.Watermark, Series: []statsSeries{}, Gaps: []statsGapView{}}
		if s.Watermark != nil { // a makerspace midnight: the day before it is the last final one
			through := clock.AddDate(clock.Day(*s.Watermark), 0, 0, -1)
			view.CountedThrough = &through
		}
		if subject == "" {
			view.Collected = &statsCollected{At: s.LastCollectedAt, UnboundAccounts: s.UnboundAccounts, Dropped: s.Dropped}
		}
		bySeries := map[string]*statsSeries{}
		left := map[string]bool{}
		series := func(dims map[string]string) *statsSeries {
			key := dimsText(dims)
			if leftOut[m.Name+"\x00"+key] {
				left[key] = true
				return &statsSeries{} // read, never shown
			}
			if bySeries[key] == nil {
				bySeries[key] = &statsSeries{Dimensions: dims, Days: []statsDay{}}
			}
			return bySeries[key]
		}
		counter := m.Instrument == addons.InstrumentCounter
		for _, d := range days {
			if d.Metric != m.Name {
				continue
			}
			ss := series(d.Dims)
			ss.Days = append(ss.Days, statsDay{Day: d.When, Value: d.Value})
			if counter {
				ss.Sum30d = addTo(ss.Sum30d, d.Value)
			} else {
				ss.Latest = &statsDay{Day: d.When, Value: d.Value} // days come oldest first
			}
		}
		if counter {
			for _, mo := range months {
				if mo.Metric == m.Name {
					ss := series(mo.Dims)
					ss.Months = append(ss.Months, statsMonth{Month: mo.When, Value: mo.Value})
					ss.Sum12m = addTo(ss.Sum12m, mo.Value)
				}
			}
		}
		for _, ss := range bySeries {
			if counter { // a counter with no day in a window counted none there
				ss.Sum30d, ss.Sum12m = addTo(ss.Sum30d, 0), addTo(ss.Sum12m, 0)
			}
			view.Series = append(view.Series, *ss)
		}
		// Series by their dimension values as a person reads them.
		textsort.ByName(view.Series, func(s statsSeries) string { return dimsText(s.Dimensions) }, func(s statsSeries) string { return "" })
		for _, g := range gaps {
			// A person's page carries a gap over the whole metric ({}) or over
			// a part of one of their own series (every key=value of the gap in
			// that series: {"share":"makers"} covers their makers/smb), never
			// the target's others: a gap on a share only someone else used
			// would say that share exists here.
			if g.Metric != m.Name || subject != "" && len(g.Dims) > 0 && !slices.ContainsFunc(view.Series, func(s statsSeries) bool { return within(g.Dims, s.Dimensions) }) {
				continue
			}
			gap := statsGapView{Start: g.Start, End: g.End, Dimensions: g.Dims, Reason: g.Reason}
			if memberView && namesOther(g.NamedSubjects, subject) {
				gap.Dimensions, gap.Reason = nil, reasonWithheld
			}
			view.Gaps = append(view.Gaps, gap)
		}
		view.LeftOut = len(left)
		out.Metrics = append(out.Metrics, view)
	}
	textsort.ByName(out.Metrics, func(m statsMetricView) string { return m.Label }, func(m statsMetricView) string { return m.Name })
	return out, nil
}

// within reports whether every key=value of part is also in whole.
func within(part, whole map[string]string) bool {
	for k, v := range part {
		if w, ok := whole[k]; !ok || w != v {
			return false
		}
	}
	return true
}

func addTo(sum *float64, v float64) *float64 {
	if sum == nil {
		return &v
	}
	total := *sum + v
	return &total
}

// dimsText is a dimension set's values in its keys' order, for sorting.
func dimsText(dims map[string]string) string {
	raw, _ := json.Marshal(dims)
	return string(raw)
}

// personalUsage is the usage block on a person's view of a target: the actor's
// own on /me/targets (memberView), one person's on an admin's person page. Filled by those
// two views alone, never by describeTargetFor, which the password and
// member-action paths also call for their state checks and which must not pay
// four statistics queries for a page nobody renders. Syndra's own store,
// never the add-on, so a target that is down still shows its history.
// Best-effort: a page that cannot read it says less rather than failing.
func personalUsage(ctx context.Context, target, subject string, memberView bool) *statsUsage {
	usage, err := statsUsageFor(ctx, target, subject, memberView, clock.Now())
	if err != nil {
		log.Printf("[STATS] could not read %s's usage on %s: %v (non-fatal)", subject, target, err)
		return nil
	}
	return &usage
}

// handleTargetStats is GET /api/v1/targets/{target}/stats (admin): the
// target-wide series by dimension, the collection line, and how long
// per-person points are kept here.
func handleTargetStats(w http.ResponseWriter, r *http.Request) {
	target := r.PathValue("target")
	if _, err := addonsGet(target); err != nil {
		jsonErrorResponse(w, http.StatusNotFound, "TARGET_NOT_FOUND", target+" is not a connected system.")
		return
	}
	usage, err := statsUsageFor(r.Context(), target, "", false, clock.Now())
	if err != nil {
		jsonErrorResponse(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}
	days, set, err := dbStatsRetention(r.Context(), target)
	if err != nil {
		jsonErrorResponse(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}
	jsonResponse(w, http.StatusOK, map[string]any{
		"target": target, "usage": usage,
		"retention": map[string]any{"subject_days": days, "set": set},
	})
}

// statsRetentionRequest is the one write of per-person retention. Days is
// required: a number sets it, null clears it back to the ceiling. Raw, because
// a *int cannot tell {"days": null} from {}, and {} must not silently clear.
type statsRetentionRequest struct {
	Days json.RawMessage `json:"days"`
}

// handleSetStatsRetention is PUT /api/v1/targets/{target}/stats-retention
// (admin), the one write place: how many days this target keeps per-person
// points, 1 to 400 (the schema's CHECK, the rule's one place), audited. It
// shortens per-person retention only; totals keep five years (ruling O4).
func handleSetStatsRetention(w http.ResponseWriter, r *http.Request) {
	target := r.PathValue("target")
	var req statsRetentionRequest
	if err := decodeJSONStrict(r.Body, &req); err != nil {
		jsonValidationErrorResponse(w, "Invalid JSON payload", map[string]string{"body": err.Error()})
		return
	}
	if len(req.Days) == 0 {
		jsonValidationErrorResponse(w, "days is required: a number of days, or null to clear it", map[string]string{"days": "required"})
		return
	}
	var days *int
	if err := json.Unmarshal(req.Days, &days); err != nil {
		jsonValidationErrorResponse(w, "Invalid JSON payload", map[string]string{"days": err.Error()})
		return
	}
	if _, err := addonsGet(target); err != nil {
		jsonErrorResponse(w, http.StatusNotFound, "TARGET_NOT_FOUND", target+" is not a connected system.")
		return
	}
	actor := resolveActor(r, "")
	switch err := dbSetStatsRetention(r.Context(), target, days, actor); {
	case errors.Is(err, db.ErrStatsRetentionInvalid):
		jsonResponse(w, http.StatusUnprocessableEntity, ErrorResponse{Error: "VALIDATION_FAILED", Message: err.Error(),
			Details: map[string]string{"days": "out_of_range"}})
		return
	case err != nil:
		jsonErrorResponse(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}
	log.Printf("[TARGETS] %s set %s's per-person statistics retention to %s", actor, target, retentionText(days))
	kept, set, err := dbStatsRetention(r.Context(), target)
	if err != nil {
		jsonErrorResponse(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}
	jsonResponse(w, http.StatusOK, map[string]any{"target": target, "subject_days": kept, "set": set})
}

func retentionText(days *int) string {
	if days == nil {
		return "the ceiling (cleared)"
	}
	return strconv.Itoa(*days) + " days"
}

// insightsStatsRow is one per-person series of one metric: its spread over
// the members with a value in the last 30 days (how many, the median, the
// 90th percentile; no floor, an admin's read) and every person with a value
// in the last 12 months (the owner's ruling of 14 Nov 2026: an admin can trace
// usage to a person). One series' people only: a counter is never summed
// across series, nor a gauge's values.
type insightsStatsRow struct {
	Dimensions map[string]string `json:"dimensions"`
	Members    int               `json:"members"`
	P50        float64           `json:"p50"`
	P90        float64           `json:"p90"`
	People     []insightsPerson  `json:"people"`
}

// insightsPerson is one person's value of a series: a counter's sum of their
// days or a gauge's latest value, in each window, read from the days the
// spread reads (StatsDistributions, the one definition). Last30d is a counter's
// 0 when they counted none there, a gauge's null when there was no sample.
// Name is the directory's display name, "" when it cannot be read; the id is
// never the label.
type insightsPerson struct {
	UserID  string   `json:"user_id"`
	Name    string   `json:"name"`
	Last30d *float64 `json:"last_30d"`
	Last12m float64  `json:"last_12m"`
}

type insightsStatsMetric struct {
	Target string `json:"target"`
	System string `json:"system"`
	// Window30dFrom and Window12mFrom are the first day each window covers:
	// the window's `from`, or later when this target's retention is shorter
	// (the read's own clamp).
	Window30dFrom string `json:"window_30d_from"`
	Window12mFrom string `json:"window_12m_from"`
	addons.Metric
	Series []insightsStatsRow `json:"series"`
}

// handleInsightsStats is GET /api/v1/insights/stats (admin): every per-person
// metric of every target, each series' spread over the last 30 days and its
// people over the last 30 days and 12 months (or the days its target keeps,
// when fewer): a counter's sum or a gauge's latest value per person. The UI
// groups them by unit.
func handleInsightsStats(w http.ResponseWriter, r *http.Request) {
	today, from30, from12m := statsWindows(clock.Now())
	rows, err := dbStatsDistributions(r.Context(), today, from30, from12m)
	if err != nil {
		jsonErrorResponse(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}
	names := map[string]string{}
	if len(rows) > 0 {
		// The directory the person pages name people from. Unreadable, every
		// name is "" and the page says it could not name them; never an id.
		users, err := directoryUsers(r.Context())
		if err != nil {
			log.Printf("[STATS] insights could not read the directory for names: %v (non-fatal)", err)
		}
		for _, u := range users {
			names[u.ID] = strings.TrimSpace(u.Name)
		}
	}
	out := []insightsStatsMetric{}
	at := map[string]int{}
	descriptors := map[string]map[string]addons.Metric{}
	for _, row := range rows {
		if descriptors[row.Target] == nil {
			stored, err := dbStatsMetrics(r.Context(), row.Target)
			if err != nil {
				jsonErrorResponse(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
				return
			}
			descriptors[row.Target] = map[string]addons.Metric{}
			for _, s := range stored {
				var m addons.Metric
				if json.Unmarshal(s.Descriptor, &m) == nil {
					descriptors[row.Target][s.Metric] = m
				}
			}
		}
		m, ok := descriptors[row.Target][row.Metric]
		if !ok || !m.PerSubject {
			continue
		}
		key := row.Target + "\x00" + row.Metric
		if _, seen := at[key]; !seen {
			system, _ := systemName(r.Context(), row.Target)
			at[key] = len(out)
			out = append(out, insightsStatsMetric{Target: row.Target, System: system, Window30dFrom: row.From, Window12mFrom: row.From12m, Metric: m, Series: []insightsStatsRow{}})
		}
		people := make([]insightsPerson, 0, len(row.People))
		for _, p := range row.People {
			last30 := p.Last30d
			if last30 == nil && m.Instrument == addons.InstrumentCounter { // a counter with no day in the window counted none there
				last30 = addTo(nil, 0)
			}
			people = append(people, insightsPerson{UserID: p.Subject, Name: names[p.Subject], Last30d: last30, Last12m: p.Last12m})
		}
		i := at[key]
		out[i].Series = append(out[i].Series, insightsStatsRow{Dimensions: row.Dims, Members: row.Members, P50: row.P50, P90: row.P90, People: people})
	}
	textsort.ByName(out, func(m insightsStatsMetric) string { return m.System + " " + m.Label }, func(m insightsStatsMetric) string { return m.Target + m.Name })
	jsonResponse(w, http.StatusOK, map[string]any{"window_days": 30, "from": from30, "from_12m": from12m, "metrics": out})
}
