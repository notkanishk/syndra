package services

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"syndra/internal/db"
)

// Term dates are one setting (A-04 §14): the names and the first and last day
// of each term, kept under config key "terms". Trends' "This term" and the
// lens's "New this term" read them through LoadTerms, and the UI reads them
// to offer "To the end of next term".

// ConfigKeyTerms is the config_settings key the term dates live under.
const ConfigKeyTerms = "terms"

// Term is one term: both days inclusive, as YYYY-MM-DD.
type Term struct {
	Name  string `json:"name"`
	Start string `json:"start"`
	End   string `json:"end"`
}

// TermsView is what GET /terms answers: the terms, and who set them and when
// (both nil while the defaults are in force).
type TermsView struct {
	Terms []Term     `json:"terms"`
	SetBy *string    `json:"set_by"`
	SetAt *time.Time `json:"set_at"`
}

const (
	maxTerms    = 12
	maxTermName = 40
)

var termsSetting = db.GetConfigSettingMeta

// DefaultTerms is the year cut into the three terms used before the dates
// were a setting, first day to last, gapless.
func DefaultTerms(year int) []Term {
	d := func(m time.Month, day int) string {
		return time.Date(year, m, day, 0, 0, 0, 0, time.UTC).Format(dateLayout)
	}
	return []Term{
		{"Spring", d(1, 1), d(5, 31)},
		{"Summer", d(6, 1), d(7, 31)},
		{"Monsoon", d(8, 1), d(12, 31)},
	}
}

// LoadTerms reads the setting, or the defaults for now's year when it was never
// set. A stored value that can't be read is an error: it is only ever written
// through ValidateTerms, so a bad one is corruption worth hearing about.
func LoadTerms(ctx context.Context, now time.Time) (TermsView, error) {
	s, set, err := termsSetting(ctx, ConfigKeyTerms)
	if err != nil {
		return TermsView{}, err
	}
	if !set {
		return TermsView{Terms: DefaultTerms(now.Year())}, nil
	}
	var terms []Term
	if err := json.Unmarshal([]byte(s.Value), &terms); err != nil {
		return TermsView{}, fmt.Errorf("the saved term dates can't be read: %w", err)
	}
	by, at := s.UpdatedBy, s.UpdatedAt
	return TermsView{Terms: terms, SetBy: &by, SetAt: &at}, nil
}

// EncodeTerms is the stored form of validated terms.
func EncodeTerms(terms []Term) string {
	b, _ := json.Marshal(terms)
	return string(b)
}

// ValidateTerms checks what an operator sent and returns it sorted by start,
// names trimmed. Errors are keyed by the input's position ("terms[1].start").
// Terms may leave gaps but not overlap; a shared day overlaps.
func ValidateTerms(in []Term) ([]Term, map[string]string) {
	errs := map[string]string{}
	if len(in) < 1 || len(in) > maxTerms {
		errs["terms"] = fmt.Sprintf("between 1 and %d terms", maxTerms)
		return nil, errs
	}
	type parsed struct {
		Term
		idx        int
		start, end time.Time
	}
	ps := make([]parsed, 0, len(in))
	for i, t := range in {
		key := func(f string) string { return fmt.Sprintf("terms[%d].%s", i, f) }
		t.Name = strings.TrimSpace(t.Name)
		if t.Name == "" || utf8.RuneCountInString(t.Name) > maxTermName {
			errs[key("name")] = fmt.Sprintf("1 to %d characters", maxTermName)
		}
		s, serr := time.Parse(dateLayout, t.Start)
		e, eerr := time.Parse(dateLayout, t.End)
		if serr != nil {
			errs[key("start")] = "a date as YYYY-MM-DD"
		}
		if eerr != nil {
			errs[key("end")] = "a date as YYYY-MM-DD"
		}
		if serr == nil && eerr == nil && s.After(e) {
			errs[key("end")] = "on or after the start"
		}
		ps = append(ps, parsed{t, i, s, e})
	}
	if len(errs) > 0 {
		return nil, errs
	}
	sort.SliceStable(ps, func(a, b int) bool { return ps[a].start.Before(ps[b].start) })
	out := make([]Term, len(ps))
	for i, p := range ps {
		out[i] = p.Term
		if i > 0 && !p.start.After(ps[i-1].end) {
			errs[fmt.Sprintf("terms[%d].start", p.idx)] = "overlaps " + ps[i-1].Name
		}
	}
	if len(errs) > 0 {
		return nil, errs
	}
	return out, nil
}

// termAround is the term holding the local day of now, from the given terms or
// (when none holds it, e.g. the year turned and nobody updated them) the
// defaults for that year, so a report never collapses to a single day.
func termAround(terms []Term, now time.Time) (from, to time.Time, label string) {
	loc := now.Location()
	y, m, d := now.Date()
	today := time.Date(y, m, d, 0, 0, 0, 0, loc)
	for _, set := range [][]Term{terms, DefaultTerms(y)} {
		for _, t := range set {
			s, serr := time.ParseInLocation(dateLayout, t.Start, loc)
			e, eerr := time.ParseInLocation(dateLayout, t.End, loc)
			if serr == nil && eerr == nil && !today.Before(s) && !today.After(e) {
				return s, e, fmt.Sprintf("%s %d", t.Name, s.Year())
			}
		}
	}
	return today, today, "Today" // unreachable: the defaults cover the year
}
