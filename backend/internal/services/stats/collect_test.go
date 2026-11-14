package stats

import (
	"slices"
	"testing"

	"syndra/internal/addons"
	"syndra/internal/clock"
	"syndra/internal/db"
	"syndra/internal/models"
)

// One rule names a bound person, for a dimension value and a gap reason
// alike: a subject id, account name or account id as a whole token, in any
// case. Nothing that names one is dropped or rewritten (the owner's ruling of
// 14 Nov 2026): the point or gap is kept as sent and flagged names_person,
// and nothing else is.
func TestWhatNamesABoundPersonIsKeptAndFlagged(t *testing.T) {
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
	var flags []bool
	for _, p := range kept {
		flags = append(flags, p.NamesPerson)
	}
	if len(kept) != 4 || kept[1].Dims["share"] != "home-JOHN_CENA" || !slices.Equal(flags, []bool{true, true, true, false}) {
		t.Errorf("kept %+v: want every point as sent, flagged only where a value names a person", kept)
	}
	gaps := keepGaps([]addons.StatGap{
		gap("makers", "John_Cena's home is offline"), gap("makers", "account 3051 is locked"), gap("makers", "rey_mysterio is away"),
		gap("JOHN_CENA", "Auditing is off"), gap("makers", "Auditing is off on this share"),
	}, m, who, dropped)
	var reasons []string
	flags = nil
	for _, g := range gaps {
		reasons = append(reasons, g.Reason)
		flags = append(flags, g.NamesPerson)
	}
	if want := []string{"John_Cena's home is offline", "account 3051 is locked", "rey_mysterio is away", "Auditing is off", "Auditing is off on this share"}; !slices.Equal(reasons, want) {
		t.Errorf("kept reasons %q, want %q: a reason was rewritten or its gap lost", reasons, want)
	}
	if want := []bool{true, true, true, true, false}; !slices.Equal(flags, want) {
		t.Errorf("gap flags %v, want %v: a reason or a dimension value naming a person flags its gap", flags, want)
	}
	if len(dropped) != 0 {
		t.Errorf("dropped %v: naming a person is no longer a drop", dropped)
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
		[]db.TargetBinding{{SubjectID: "sub-cena"}, {SubjectID: "sub-blank"}, {SubjectID: "sub-one"}, {SubjectID: "sub-rey"}, {SubjectID: "sub-dashes"}, {SubjectID: "sub-digits"}, {SubjectID: "sub-accent"}, {SubjectID: "sub-raj"}, {SubjectID: "sub-ram"}},
		[]models.UserProfile{
			{ID: "sub-cena", Name: "John Cena"},
			{ID: "sub-blank", Name: "   "},
			{ID: "sub-one", Name: "x"},
			{ID: "sub-dashes", Name: "--"},
			{ID: "sub-digits", Name: "42"},
			{ID: "sub-raj", Name: "Raj\u0915"},
			{ID: "sub-ram", Name: "\u0930\u093e\u092e"},
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
		{"Rey (Mysterio) files", true}, // metacharacters are literal
		{"The Undertaker", false},      // not bound on this target
		{"x", false},                   // one character is never a name
		// A combining mark is part of the word: a nukta letter (NFC decomposes
		// U+0958 to U+0915 U+093C) and a vowel sign continue the name's token.
		{"Raj\u0958", false},
		{"\u0930\u093e\u092e\u093e", false},
		{"Raj\u0915 files", true},
		{"\u0930\u093e\u092e files", true},
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

	// Through the same two places an account name goes: a value flags its
	// point, a reason its gap; both kept as sent.
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
	if len(kept) != 2 || !kept[0].NamesPerson || kept[1].NamesPerson || len(dropped) != 0 {
		t.Errorf("kept %+v, dropped %v: want both kept, the display name's flagged", kept, dropped)
	}
	gaps := keepGaps([]addons.StatGap{
		{Start: start, End: end, Dimensions: map[string]string{"share": "makers"}, Reason: "John Cena's home is offline"},
	}, m, who, dropped)
	if len(gaps) != 1 || gaps[0].Reason != "John Cena's home is offline" || !gaps[0].NamesPerson || len(dropped) != 0 {
		t.Errorf("gaps %+v, dropped %v: want the gap kept as sent and flagged", gaps, dropped)
	}
}
