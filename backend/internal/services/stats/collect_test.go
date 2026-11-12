package stats

import (
	"reflect"
	"slices"
	"testing"

	"syndra/internal/addons"
	"syndra/internal/clock"
	"syndra/internal/db"
	"syndra/internal/models"
)

// One rule names a bound person (ruling B7), for a dimension value and a gap
// reason alike: a subject id, account name or account id as a whole token, in
// any case. A point or gap whose dimension names someone is dropped and
// counted; a gap whose reason does is kept, its reason withheld, and counted.
func TestNoValueOrReasonNamesABoundPerson(t *testing.T) {
	account := "3051"
	who := peopleOn([]db.TargetBinding{{SubjectID: "sub-cena", Username: "john_cena", AccountID: &account}, {SubjectID: "sub-orton", Username: ""}, {SubjectID: "sub-rey", Username: "Rey_Mysterio"}}, nil)
	m := addons.Metric{Name: "storage.sessions", Instrument: "counter", Unit: "count", PerSubject: true,
		Dimensions: []addons.Dimension{{Name: "share", Label: "Share"}}}
	day := clock.Day(clock.Now())
	start, _ := clock.Midnight(day)
	end, _ := clock.Midnight(clock.AddDate(day, 0, 0, 1))
	v := 1.0
	point := func(share string) addons.StatPoint {
		return addons.StatPoint{Start: start, End: end, Dimensions: map[string]string{"share": share}, Value: &v}
	}
	gap := func(share, reason string) addons.StatGap {
		return addons.StatGap{Start: start, End: end, Dimensions: map[string]string{"share": share}, Reason: reason}
	}

	dropped := map[string]int{}
	kept := keep([]addons.StatPoint{point("John_Cena"), point("home-JOHN_CENA"), point("SUB-CENA"), point("makers")}, m, who, map[string]bool{}, dropped)
	if len(kept) != 1 || kept[0].Dims["share"] != "makers" {
		t.Errorf("kept %+v: a value naming a person in another case was stored", kept)
	}
	gaps := keepGaps([]addons.StatGap{
		gap("makers", "John_Cena's home is offline"), gap("makers", "account 3051 is locked"), gap("makers", "rey_mysterio is away"),
		gap("JOHN_CENA", "Auditing is off"), gap("makers", "Auditing is off on this share"),
	}, m, who, dropped)
	var reasons []string
	for _, g := range gaps {
		if g.Dims["share"] != "makers" {
			t.Errorf("kept a gap on %q: a dimension naming a person was stored", g.Dims["share"])
		}
		reasons = append(reasons, g.Reason)
	}
	if want := []string{reasonWithheld, reasonWithheld, reasonWithheld, "Auditing is off on this share"}; !slices.Equal(reasons, want) {
		t.Errorf("kept reasons %q, want %q: a reason naming a person was stored, or its gap lost", reasons, want)
	}
	if want := map[string]int{DropPersonInDimension: 7}; !reflect.DeepEqual(dropped, want) {
		t.Errorf("dropped %v, want %v", dropped, want)
	}
	// An empty username names nobody: "" is in every string.
	if who.names("makers") {
		t.Error("an empty name matched everything")
	}
}

// A name counts only as a whole token: beside the text's ends or a character
// that is neither a letter nor a digit ("_" included), never inside a longer
// word or number. Containment would drop real data — the add-on's own reason
// "Too much activity to count yet" holds "ty" and "ch".
func TestANameIsAWholeToken(t *testing.T) {
	who := func(names ...string) people {
		var b []db.TargetBinding
		for _, n := range names {
			b = append(b, db.TargetBinding{SubjectID: "sub-" + n, Username: n})
		}
		return peopleOn(b, nil)
	}
	for _, c := range []struct {
		name, text string
		want       bool
	}{
		{"john_cena", "home-JOHN_CENA", true},
		{"john_cena", "john_cena's", true},
		{"ana", "home_ana", true},
		{"ana", "ana.k-files", true},
		{"3051", "home-3051", true},
		{"3051", "3051", true},
		{"lab", "makerlab", false},
		{"ty", "activity", false},
		{"3051", "proj-30512", false},
		{"3051", "ME3051", false},
		{"ty", "Too much activity to count yet", false},
		{"ch", "Too much activity to count yet", false},
		{"a.b", "axb", false}, // a name is text, never a pattern
	} {
		if got := who(c.name).names(c.text); got != c.want {
			t.Errorf("%q names %q: %t, want %t", c.text, c.name, got, c.want)
		}
	}
	// No binding has a name: the pattern names no text, the empty one included.
	if nobody := peopleOn([]db.TargetBinding{{}}, nil); nobody.names("") || nobody.names("anything") {
		t.Error("a target with no names named someone")
	}
}

// Design 6d-2: a display name is checked like any other name. Only a bound
// person's counts (an unbound user's name is not a person this target holds),
// a name with spaces matches as a whole phrase, and a blank or one-character
// name matches nothing at all.
func TestADisplayNameIsCheckedLikeAnyOtherName(t *testing.T) {
	who := peopleOn(
		[]db.TargetBinding{{SubjectID: "sub-cena"}, {SubjectID: "sub-blank"}, {SubjectID: "sub-one"}, {SubjectID: "sub-rey"}, {SubjectID: "sub-dashes"}, {SubjectID: "sub-digits"}, {SubjectID: "sub-accent"}},
		[]models.UserProfile{
			{ID: "sub-cena", Name: "John Cena"},
			{ID: "sub-blank", Name: "   "},
			{ID: "sub-one", Name: "x"},
			{ID: "sub-dashes", Name: "--"},
			{ID: "sub-digits", Name: "42"},
			{ID: "sub-accent", Name: "Andr\u00e9 the Giant"}, // precomposed
			{ID: "sub-rey", Name: "Rey (Mysterio)"},
			{ID: "sub-unbound", Name: "The Undertaker"},
		})
	for _, c := range []struct {
		text string
		want bool
	}{
		{"John Cena", true},
		{"home of JOHN CENA", true},
		{"john cena's share", true},
		{"john  cena", false}, // one phrase, as spelled
		{"John", false},
		{"Johnny Cena", false},
		{"Rey (Mysterio) files", true},  // metacharacters are literal
		{"The Undertaker", false},       // not bound on this target
		{"x", false},                    // one character is never a name
		{"a -- b", false},               // no letter, no name
		{"room 42", false},              // digits alone are no name
		{"Andre\u0301 the Giant", true}, // decomposed spelling of the same name
		{"", false},                     // nor is blank
		{"makers", false},
	} {
		if got := who.names(c.text); got != c.want {
			t.Errorf("%q names someone: %t, want %t", c.text, got, c.want)
		}
	}

	// Through the same two places an account name goes: a value drops its
	// series and is counted; a reason is withheld, its gap kept.
	m := addons.Metric{Name: "storage.sessions", Instrument: "counter", Unit: "count", PerSubject: true,
		Dimensions: []addons.Dimension{{Name: "share", Label: "Share"}}}
	day := clock.Day(clock.Now())
	start, _ := clock.Midnight(day)
	end, _ := clock.Midnight(clock.AddDate(day, 0, 0, 1))
	v := 1.0
	dropped := map[string]int{}
	kept := keep([]addons.StatPoint{
		{Start: start, End: end, Dimensions: map[string]string{"share": "John Cena"}, Value: &v},
		{Start: start, End: end, Dimensions: map[string]string{"share": "makers"}, Value: &v},
	}, m, who, map[string]bool{}, dropped)
	if len(kept) != 1 || kept[0].Dims["share"] != "makers" || dropped[DropPersonInDimension] != 1 {
		t.Errorf("kept %+v, dropped %v: a display name in a dimension value was stored", kept, dropped)
	}
	gaps := keepGaps([]addons.StatGap{
		{Start: start, End: end, Dimensions: map[string]string{"share": "makers"}, Reason: "John Cena's home is offline"},
	}, m, who, dropped)
	if len(gaps) != 1 || gaps[0].Reason != reasonWithheld || dropped[DropPersonInDimension] != 2 {
		t.Errorf("gaps %+v, dropped %v: want the gap kept, its reason withheld, and counted", gaps, dropped)
	}
}
