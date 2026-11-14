// Package stats is Syndra's own statistics collector (addon-contract 6.6,
// design §9.1, §9.6): each declared metric of each add-on read from its
// watermark, checked, and kept, one transaction per metric per collection;
// and the daily retention (6.7): at start-up, then every 24 hours.
//
// A runner beside the drift sweep, never a step inside it: the sweep queues
// work every six hours, and a slow /stats would delay convergence while a
// failed one would read as a failed reconcile.
package stats

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"regexp"
	"slices"
	"strings"
	"sync"
	"time"
	"unicode"

	"syndra/internal/addons"
	"syndra/internal/clock"
	"syndra/internal/db"
	"syndra/internal/directory"
	"syndra/internal/models"

	"golang.org/x/text/unicode/norm"
)

// directoryUsers is who the directory knows: every current user's id, display
// name and username go in the matcher's name set. A var so a test can stand
// in for Zitadel.
var directoryUsers = func(ctx context.Context) ([]models.UserProfile, error) {
	return directory.Default.Users(ctx)
}

// Read is one page of one metric's GET /stats, since an instant, from a
// cursor ("" the first page).
type Read func(ctx context.Context, metric string, since time.Time, cursor string) (addons.StatsPage, error)

// Why a point or gap is not kept (design §9.6, §9.7): the add-on is the least
// trusted component, so what it says is checked before it is stored, and
// what is dropped is counted on the metric's row (addon_metrics.dropped).
const (
	DropNotADay             = "not_a_day"            // not one makerspace day, midnight to midnight
	DropBadValue            = "bad_value"            // no value, a negative one, or a gap with no reason
	DropNotPerSubject       = "not_per_subject"      // a subject on a metric not reported per person
	DropUnboundSubject      = "unbound_subject"      // a subject with no binding on this target
	DropUndeclaredDimension = "undeclared_dimension" // a dimension the metric does not declare
	DropPastRetention       = "past_retention"       // older than retention keeps
	DropSeries              = "series"               // a new series past constants.series_per_metric
)

// A dimension value or gap reason that names a person is not a drop: it is
// kept as sent, and who it names is recorded as named_subjects (migration
// 000094, the owner's ruling of 14 Nov 2026, superseding ruling B7's drop and
// withhold). Admins read it as stored; a member's own view leaves out what
// names anyone but them (handlers/stats.go). The contract rule stands: people
// go in `subject`, never in a dimension (kit stats.no-people); the record is
// Syndra's guard against an add-on that breaks it.

// metricTimeout bounds one metric's collection, its pages and its
// transaction: one slow metric holds only its own rows, and only this long.
const metricTimeout = 5 * time.Minute

// CollectAll collects every metric of every add-on that declares them, each
// on its own watermark (a dark metric never re-sends another) and in its own
// transaction. A failed metric is logged and keeps its watermark; the next
// one goes on. Shaped for periodic.Runner (ADDON-STATS).
func CollectAll(ctx context.Context) error {
	for _, reg := range addons.Registered() {
		metrics, buffer := addons.StatsMetrics(reg.Target)
		for _, m := range metrics {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			c, cancel := context.WithTimeout(ctx, metricTimeout)
			dropped, err := Collect(c, reg.Target, m, buffer, func(ctx context.Context, metric string, since time.Time, cursor string) (addons.StatsPage, error) {
				return addons.ReadStats(ctx, reg.Target, metric, since, cursor)
			})
			cancel()
			switch {
			case err != nil:
				log.Printf("[STATS] %s %s not collected, its watermark kept: %v", reg.Target, m.Name, err)
			case len(dropped) > 0:
				log.Printf("[STATS] %s %s collected; dropped %v", reg.Target, m.Name, dropped)
			}
		}
	}
	return nil
}

// Collect is one metric's collection (design §9.6), in one transaction:
//
//  1. since is the metric's stored watermark, or now − the add-on's buffer
//     on its first collection;
//  2. what is provisional (a day ending after the watermark, a gap ending
//     after it) is deleted, so the reply replaces it: a provisional 3 that is
//     now absent is gone, and a gap retracted is gone;
//  3. every page is checked and upserted by key (equal keys replace, nothing
//     adds), so the same (metric, since) twice changes nothing;
//  4. the watermark is written last, after the final page, floored to a
//     makerspace midnight, never past today's, never backwards.
//
// A page that fails rolls all of it back and the watermark stays. Returns
// what was dropped, by reason.
func Collect(ctx context.Context, target string, m addons.Metric, buffer time.Duration, read Read) (map[string]int, error) {
	now := clock.Now()
	today := clock.Day(now)
	bindings, err := db.ListTargetBindings(ctx, target)
	if err != nil {
		return nil, err
	}
	// A refusal to read the directory fails the collection (the watermark
	// stays): a row checked without the names would read as one that named
	// nobody, and a member would be shown what names a person.
	users, err := directoryUsers(ctx)
	if err != nil {
		return nil, fmt.Errorf("stats: the directory could not be read for display names: %w", err)
	}
	who := peopleOn(bindings, users)
	dropped := map[string]int{}
	err = db.CollectStats(ctx, target, m.Name, func(c *db.StatsTx) error {
		stored, err := c.Watermark(ctx)
		if err != nil {
			return err
		}
		// The series Syndra already knows, read before the provisional
		// window goes: a series is not forgotten because today is replaced.
		held, err := c.Series(ctx)
		if err != nil {
			return err
		}
		series := map[string]bool{}
		for _, d := range held {
			series[dimsKey(d)] = true
		}
		since := now.Add(-buffer)
		if stored != nil {
			since = *stored
			if err := c.DeleteProvisional(ctx, clock.Day(*stored)); err != nil {
				return err
			}
		}

		var first *addons.StatsPage
		unbound, cursor := 0, ""
		// Every cursor this collection has followed: an add-on answering
		// any of them again is paging in a cycle (A, B, A), not only one
		// that repeats the last, and would page until metricTimeout.
		followed := map[string]bool{}
		for {
			page, err := read(ctx, m.Name, since, cursor)
			if err != nil {
				return err
			}
			if first == nil {
				first = &page
			}
			unbound = max(unbound, page.Unbound)
			points := keep(page.Points, m, who, series, dropped)
			kept, err := c.UpsertPoints(ctx, today, page.TakenAt, points)
			if err != nil {
				return err
			}
			dropped[DropPastRetention] += len(points) - kept
			gaps := keepGaps(page.Gaps, m, who, dropped)
			if kept, err = c.UpsertGaps(ctx, today, gaps); err != nil {
				return err
			}
			dropped[DropPastRetention] += len(gaps) - kept
			if page.NextCursor == "" {
				break
			}
			if followed[page.NextCursor] {
				return errors.New("the add-on answered a cursor it had already given")
			}
			followed[page.NextCursor] = true
			cursor = page.NextCursor
		}
		for reason, n := range dropped {
			if n == 0 {
				delete(dropped, reason)
			}
		}
		watermark, err := finalThrough(first.Watermark, now)
		if err != nil {
			return err
		}
		return c.Finish(ctx, watermark, now, unbound, dropped)
	})
	if err != nil {
		return nil, err
	}
	return dropped, nil
}

// finalThrough is the watermark Syndra stores from the add-on's (B4): floored
// to the makerspace midnight of its day, and never past the start of today,
// which is still being counted whatever the add-on says.
func finalThrough(reported, now time.Time) (time.Time, error) {
	floor, err := clock.Midnight(clock.Day(reported))
	if err != nil {
		return time.Time{}, err
	}
	today, err := clock.Midnight(clock.Day(now))
	if err != nil {
		return time.Time{}, err
	}
	if floor.After(today) {
		return today, nil
	}
	return floor, nil
}

// people is who is bound on the target (the subjects a point may name) and
// the words that name a person: for everyone in the directory (current
// users, bound here or not, so a member never reads the name of someone who
// is not), their user id, display name and username; for a person bound here,
// their account name and id on this target too. One pattern over every name
// finds whether a text names anyone; one per person says who, compiled the
// first time a text names someone (rare: a conformant add-on never does), as
// compiling one per person for a few hundred costs some 60 ms a collection
// against 2 ms for the shared one.
type people struct {
	bound map[string]bool
	named *regexp.Regexp
	each  []*person
	// seen is each text already asked, by its NFC form: a page repeats one
	// share for every person and day, and asking costs ~0.1 ms at 600 people.
	seen map[string][]string
}

// person is one subject id and their names, quoted, with their pattern.
type person struct {
	subject string
	quoted  []string
	once    sync.Once
	named   *regexp.Regexp
}

func (p *person) pattern() *regexp.Regexp {
	p.once.Do(func() { p.named = namePattern(p.quoted) })
	return p.named
}

// nobody is an RE2 class with no members: the alternation when no binding
// has a name, so the pattern matches no text at all, "" included.
const nobody = `[^\x00-\x{10FFFF}]`

// namePattern is a whole-token, case-insensitive match of any of the quoted
// names (see names).
func namePattern(quoted []string) *regexp.Regexp {
	if len(quoted) == 0 {
		quoted = []string{nobody}
	}
	return regexp.MustCompile(`(?i)(?:^|[^\pL\pN\pM])(?:` + strings.Join(quoted, "|") + `)(?:[^\pL\pN\pM]|$)`)
}

func peopleOn(bindings []db.TargetBinding, users []models.UserProfile) people {
	who := people{bound: map[string]bool{}, seen: map[string][]string{}}
	var order []string
	byID := map[string][]string{}
	add := func(id, name string) {
		if id == "" || name == "" { // "" would name every text
			return
		}
		if byID[id] == nil {
			order = append(order, id)
		}
		byID[id] = append(byID[id], regexp.QuoteMeta(norm.NFC.String(name)))
	}
	for _, b := range bindings {
		who.bound[b.SubjectID] = true
		add(b.SubjectID, b.SubjectID)
		add(b.SubjectID, b.Username)
		if b.AccountID != nil {
			add(b.SubjectID, *b.AccountID)
		}
	}
	// A directory name goes through the same matcher. QuoteMeta keeps a name
	// with spaces or punctuation literal; whole-token matching holds for it as
	// for any word. A display name or username must hold a letter and be two
	// characters or more: "--" or "42" or one letter is no name, and would
	// name every text that carries it as a token.
	for _, u := range users {
		add(u.ID, u.ID)
		for _, name := range []string{u.Name, u.Username} {
			if name = strings.TrimSpace(name); nameable(name) {
				add(u.ID, name)
			}
		}
	}
	var all []string
	for _, id := range order {
		all = append(all, byID[id]...)
		who.each = append(who.each, &person{subject: id, quoted: byID[id]})
	}
	who.named = namePattern(all)
	return who
}

// nameable is a display name worth matching: two characters or more, one a letter.
func nameable(name string) bool {
	return len([]rune(name)) >= 2 && strings.IndexFunc(name, unicode.IsLetter) >= 0
}

// names is the one rule for "this text names a person", and says whom, as
// their subject ids, sorted: a name, in any case, as a whole token — at the
// text's ends or beside a character that is neither a letter nor a digit, in
// any script. "home-JOHN_CENA", "home_ana" and the reason "john_cena's share
// is offline" name them; "makerlab" does not name lab, nor "proj-30512"
// account 3051, nor "activity" ty. Not \b: RE2's \b is ASCII-only and counts
// "_" as a word character, so "home_ana" would hide ana. Each person's own
// pattern is asked, not the match of the shared one, which reports one name
// per position and would miss a second name overlapping the first.
// Both sides are NFC, so a precomposed "é" and "e" plus a combining accent are one name.
func (p people) names(text string) []string {
	text = norm.NFC.String(text)
	if out, ok := p.seen[text]; ok {
		return out
	}
	var out []string
	if p.named.MatchString(text) {
		for _, each := range p.each {
			if each.pattern().MatchString(text) {
				out = append(out, each.subject)
			}
		}
		slices.Sort(out)
	}
	p.seen[text] = out
	return out
}

// namesIn is everyone any dimension value names, sorted and once each.
func (p people) namesIn(dims map[string]string) []string {
	var out []string
	for _, v := range dims {
		out = append(out, p.names(v)...)
	}
	slices.Sort(out)
	return slices.Compact(out)
}

// keep is the points of a page worth storing, each checked (checkPoint) and
// with whoever a dimension value names (named_subjects), the series cap
// applied, and one per key (an add-on that repeats a key in a page has the
// last one kept: equal keys replace).
func keep(points []addons.StatPoint, m addons.Metric, who people, series map[string]bool, dropped map[string]int) []db.StatsPoint {
	var out []db.StatsPoint
	at := map[string]int{}
	for _, p := range points {
		day, why := checkPoint(p, m, who)
		if why != "" {
			dropped[why]++
			continue
		}
		key := dimsKey(p.Dimensions)
		if !series[key] {
			if len(series) >= addons.SeriesPerMetric {
				dropped[DropSeries]++
				continue
			}
			series[key] = true
		}
		point := db.StatsPoint{Subject: p.Subject, Dims: p.Dimensions, Day: day, Value: *p.Value, NamedSubjects: who.namesIn(p.Dimensions)}
		if i, seen := at[p.Subject+"\x00"+day+"\x00"+key]; seen {
			out[i] = point
			continue
		}
		at[p.Subject+"\x00"+day+"\x00"+key] = len(out)
		out = append(out, point)
	}
	return out
}

// checkPoint is a point's makerspace day, or why it is not kept.
func checkPoint(p addons.StatPoint, m addons.Metric, who people) (string, string) {
	day, ok := oneDay(p.Start, p.End)
	switch {
	case !ok:
		return "", DropNotADay
	case p.Value == nil || *p.Value < 0:
		return "", DropBadValue
	case p.Subject != "" && !m.PerSubject:
		return "", DropNotPerSubject
	case p.Subject != "" && !who.bound[p.Subject]:
		return "", DropUnboundSubject
	}
	if why := checkDims(p.Dimensions, m); why != "" {
		return "", why
	}
	return day, ""
}

// keepGaps is the gaps of a page worth storing: whole makerspace days, only
// declared dimensions, a reason capped at 200 characters, with whoever a
// dimension value or the reason names (named_subjects).
func keepGaps(gaps []addons.StatGap, m addons.Metric, who people, dropped map[string]int) []db.StatsGap {
	var out []db.StatsGap
	for _, g := range gaps {
		start, ok1 := midnight(g.Start)
		end, ok2 := midnight(g.End)
		why := checkDims(g.Dimensions, m)
		switch {
		case !ok1 || !ok2 || end <= start:
			dropped[DropNotADay]++
		case g.Reason == "":
			dropped[DropBadValue]++
		case why != "":
			dropped[why]++
		default:
			reason := []rune(g.Reason)
			out = append(out, db.StatsGap{Dims: g.Dimensions, Start: start, End: end, Reason: string(reason[:min(len(reason), 200)]),
				NamedSubjects: union(who.namesIn(g.Dimensions), who.names(g.Reason))})
		}
	}
	return out
}

// union is two sorted sets of subject ids as one.
func union(a, b []string) []string {
	out := slices.Concat(a, b)
	slices.Sort(out)
	return slices.Compact(out)
}

func checkDims(dims map[string]string, m addons.Metric) string {
	for name := range dims {
		if !slices.ContainsFunc(m.Dimensions, func(d addons.Dimension) bool { return d.Name == name }) {
			return DropUndeclaredDimension
		}
	}
	return ""
}

// midnight is the makerspace day t is the first instant of (ruling O1: 00:00,
// or where a change skips it the instant the clocks resume), false for any
// other instant. A day counted in another zone is not one of ours.
func midnight(t time.Time) (string, bool) {
	day := clock.Day(t)
	first, err := clock.Midnight(day)
	return day, err == nil && first.Equal(t)
}

// oneDay is the day [start, end) is, when it is exactly one makerspace day.
func oneDay(start, end time.Time) (string, bool) {
	day, ok := midnight(start)
	next, okNext := midnight(end)
	return day, ok && okNext && next == clock.AddDate(day, 0, 0, 1)
}

// dimsKey is a dimension set as one comparable string: sorted keys, as
// encoding/json writes a map.
func dimsKey(d map[string]string) string {
	if d == nil {
		d = map[string]string{}
	}
	raw, _ := json.Marshal(d)
	return string(raw)
}

// Prune is the daily retention (STATS-RETENTION, 6.7; at start-up, then every
// 24 hours, so a shortened setting takes effect within a day): what retention
// no longer keeps on today's makerspace day goes.
func Prune(ctx context.Context) error {
	points, gaps, err := db.PruneStats(ctx, clock.Day(clock.Now()))
	if err != nil {
		return fmt.Errorf("retention: %w", err)
	}
	if points > 0 || gaps > 0 {
		log.Printf("[STATS] retention deleted %d points and %d gaps", points, gaps)
	}
	return nil
}
