package services

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"syndra/internal/db"
)

func TestDefaultTerms_AreTheOldConstants(t *testing.T) {
	got := DefaultTerms(2026)
	want := []Term{
		{"Spring", "2026-01-01", "2026-05-31"},
		{"Summer", "2026-06-01", "2026-07-31"},
		{"Monsoon", "2026-08-01", "2026-12-31"},
	}
	if len(got) != len(want) {
		t.Fatalf("got %v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("term %d: got %v want %v", i, got[i], want[i])
		}
	}
}

func TestValidateTerms(t *testing.T) {
	ok := []Term{{"Monsoon", "2026-08-01", "2026-12-15"}, {" Spring ", "2027-01-10", "2027-05-20"}}
	sorted, errs := ValidateTerms(ok)
	if len(errs) != 0 || sorted[0].Name != "Monsoon" || sorted[1].Name != "Spring" {
		t.Fatalf("valid: %v %v", sorted, errs)
	}
	// Saved in order, whatever order they came in; names trimmed.
	sorted, errs = ValidateTerms([]Term{ok[1], ok[0]})
	if len(errs) != 0 || sorted[0].Name != "Monsoon" || sorted[1].Name != "Spring" {
		t.Fatalf("reversed: %v %v", sorted, errs)
	}

	long := strings.Repeat("x", 41)
	many := make([]Term, 13)
	for i := range many {
		many[i] = Term{"T", time.Date(2026, time.Month(i%12)+1, 1, 0, 0, 0, 0, time.UTC).AddDate(i/12, 0, 0).Format(dateLayout),
			time.Date(2026, time.Month(i%12)+1, 2, 0, 0, 0, 0, time.UTC).AddDate(i/12, 0, 0).Format(dateLayout)}
	}
	for _, tc := range []struct {
		name  string
		terms []Term
		field string
	}{
		{"none", nil, "terms"},
		{"thirteen", many, "terms"},
		{"blank name", []Term{{"  ", "2026-01-01", "2026-02-01"}}, "terms[0].name"},
		{"long name", []Term{{long, "2026-01-01", "2026-02-01"}}, "terms[0].name"},
		{"bad start", []Term{{"A", "2026-02-30", "2026-03-01"}}, "terms[0].start"},
		{"bad end", []Term{{"A", "2026-01-01", "soon"}}, "terms[0].end"},
		{"start after end", []Term{{"A", "2026-03-01", "2026-02-01"}}, "terms[0].end"},
		{"overlap", []Term{{"A", "2026-01-01", "2026-03-01"}, {"B", "2026-03-01", "2026-04-01"}}, "terms[1].start"},
		{"overlap reported against the input's index", []Term{{"B", "2026-02-01", "2026-04-01"}, {"A", "2026-01-01", "2026-02-15"}}, "terms[0].start"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, errs := ValidateTerms(tc.terms)
			if _, has := errs[tc.field]; !has {
				t.Fatalf("want an error on %s, got %v", tc.field, errs)
			}
		})
	}
	// A day between them is fine: terms need not be gapless.
	if _, errs := ValidateTerms([]Term{{"A", "2026-01-01", "2026-02-01"}, {"B", "2026-02-02", "2026-03-01"}}); len(errs) != 0 {
		t.Fatalf("adjacent: %v", errs)
	}
}

func stubTermsSetting(t *testing.T, value string, set bool) {
	t.Helper()
	orig := termsSetting
	t.Cleanup(func() { termsSetting = orig })
	termsSetting = func(context.Context, string) (db.ConfigSetting, bool, error) {
		return db.ConfigSetting{Value: value, UpdatedBy: "op-1", UpdatedAt: time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)}, set, nil
	}
}

func TestLoadTerms_UnsetIsTheDefaultsForThisYear(t *testing.T) {
	stubTermsSetting(t, "", false)
	v, err := LoadTerms(context.Background(), at(2027, 3, 1, 9, 0))
	if err != nil || v.SetBy != nil || v.SetAt != nil || v.Terms[0].Start != "2027-01-01" || len(v.Terms) != 3 {
		t.Fatalf("%+v %v", v, err)
	}
}

func TestLoadTerms_SetCarriesWhoAndWhen(t *testing.T) {
	b, _ := json.Marshal([]Term{{"Autumn", "2026-09-01", "2026-12-20"}})
	stubTermsSetting(t, string(b), true)
	v, err := LoadTerms(context.Background(), at(2026, 10, 1, 9, 0))
	if err != nil || len(v.Terms) != 1 || v.SetBy == nil || *v.SetBy != "op-1" || v.SetAt == nil {
		t.Fatalf("%+v %v", v, err)
	}
	stubTermsSetting(t, "not json", true)
	if _, err := LoadTerms(context.Background(), time.Now()); err == nil {
		t.Fatal("a corrupt setting must be an error, not silently the defaults")
	}
}

// The window and "new this term" come from the setting: a custom term moves
// them, and today outside every term falls back to the year's defaults.
func TestWindowFor_UsesTheGivenTerms(t *testing.T) {
	custom := []Term{{"Autumn", "2026-09-15", "2026-12-20"}}
	w := windowFor(TrendTerm, at(2026, 10, 16, 10, 0), custom)
	if w.From.Format(dateLayout) != "2026-09-15" || w.To.Format(dateLayout) != "2026-12-20" || w.Label != "Autumn 2026" {
		t.Fatalf("custom: %+v", w)
	}
	// Both ends are inclusive days.
	if w := windowFor(TrendTerm, at(2026, 12, 20, 23, 59), custom); w.Label != "Autumn 2026" {
		t.Fatalf("last day: %+v", w)
	}
	// Outside every term: the defaults for that year, never a one-day window.
	if w := windowFor(TrendTerm, at(2026, 3, 1, 9, 0), custom); w.Label != "Spring 2026" {
		t.Fatalf("uncovered: %+v", w)
	}
}
