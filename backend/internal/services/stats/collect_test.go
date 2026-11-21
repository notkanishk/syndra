package stats

import (
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"

	"syndra/internal/addons"
	"syndra/internal/clock"
	"syndra/internal/db"
	"syndra/internal/models"
)

// One rule names a person, for a dimension value and a gap reason
// alike: a subject id, account name or account id as a whole token, in any
// case. Nothing that names one is dropped or rewritten (the owner's ruling of
// 14 Nov 2026): the point or gap is kept as sent with whom it names,
// and nothing else is.
func TestWhatNamesAPersonIsKeptWithWhomItNames(t *testing.T) {
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
		return addons.StatGap{Start: start, End: end, Dimensions: map[string]string{"share": share}, Kind: "unread", Reason: reason}
	}

	dropped := map[string]int{}
	kept := keep([]addons.StatPoint{point("John_Cena"), point("home-JOHN_CENA"), point("SUB-CENA"), point("makers")}, m, who, map[string]bool{}, dropped)
	var named []string
	for _, p := range kept {
		named = append(named, strings.Join(p.NamedSubjects, "+"))
	}
	if len(kept) != 4 || kept[1].Dims["share"] != "home-JOHN_CENA" || !slices.Equal(named, []string{"sub-cena", "sub-cena", "sub-cena", ""}) {
		t.Errorf("kept %+v: want every point as sent, naming cena only where a value names him", kept)
	}
	gaps := keepGaps([]addons.StatGap{
		gap("makers", "John_Cena's home is offline"), gap("makers", "account 3051 is locked"), gap("makers", "rey_mysterio is away"),
		gap("JOHN_CENA", "Auditing is off"), gap("makers", "Auditing is off on this share"),
	}, m, who, dropped)
	var reasons []string
	named = nil
	for _, g := range gaps {
		reasons = append(reasons, g.Reason)
		named = append(named, strings.Join(g.NamedSubjects, "+"))
	}
	if want := []string{"John_Cena's home is offline", "account 3051 is locked", "rey_mysterio is away", "Auditing is off", "Auditing is off on this share"}; !slices.Equal(reasons, want) {
		t.Errorf("kept reasons %q, want %q: a reason was rewritten or its gap lost", reasons, want)
	}
	if want := []string{"sub-cena", "sub-cena", "sub-rey", "sub-cena", ""}; !slices.Equal(named, want) {
		t.Errorf("gaps name %q, want %q: a reason or a dimension value names whom it names", named, want)
	}
	if len(dropped) != 0 {
		t.Errorf("dropped %v: naming a person is no longer a drop", dropped)
	}
	// An empty username names nobody: "" is in every string.
	if who.names("makers") != nil {
		t.Error("an empty name matched everything")
	}
}

// A name counts only as a whole token: beside the text's ends or a character
// that is neither a letter nor a digit ("_" included), never inside a longer
// word or number. Containment would drop real data — the add-on's own reason
// "Too much activity to count yet" holds "ty" and "ch".
// 7t-6: a name two people's records both carry (A renamed on the NAS, the
// mirror still saying "alice"; B's new account given that freed name) names
// both. named_subjects is a list of whom a member's own view must not show the
// record to anyone else (handlers/stats.go namesOther), so naming both hides
// it from each of them and from everyone else: closed. Crediting neither would
// leave the list empty and show it to every member.
func TestANameTwoRecordsCarryNamesBoth(t *testing.T) {
	a, b := "3001", "3077"
	who := peopleOn([]db.TargetBinding{{SubjectID: "sub-a", Username: "alice", AccountID: &a}, {SubjectID: "sub-b", Username: "alice", AccountID: &b}}, nil)
	if got := who.names("home-alice"); !slices.Equal(got, []string{"sub-a", "sub-b"}) {
		t.Fatalf("an ambiguous name must name everyone who carries it, got %v", got)
	}
}

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
		if got := who(c.name).names(c.text) != nil; got != c.want {
			t.Errorf("%q names %q: %t, want %t", c.text, c.name, got, c.want)
		}
	}
	// No binding has a name: the pattern names no text, the empty one included.
	if nobody := peopleOn([]db.TargetBinding{{}}, nil); nobody.names("") != nil || nobody.names("anything") != nil {
		t.Error("a target with no names named someone")
	}
}

// Design 6d-2: a display name is checked like any other name, for everyone in
// the directory (7i: a member never reads the name of someone not bound on
// this target either), a name with spaces matches as a whole phrase, and a
// blank or one-character name matches nothing at all.
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
		{"The Undertaker", true},       // in the directory, bound here or not
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
		if got := who.names(c.text) != nil; got != c.want {
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
	if len(kept) != 2 || !slices.Equal(kept[0].NamedSubjects, []string{"sub-cena"}) || kept[1].NamedSubjects != nil || len(dropped) != 0 {
		t.Errorf("kept %+v, dropped %v: want both kept, the display name's naming cena", kept, dropped)
	}
	gaps := keepGaps([]addons.StatGap{
		{Start: start, End: end, Dimensions: map[string]string{"share": "makers"}, Kind: "unread", Reason: "John Cena's home is offline"},
	}, m, who, dropped)
	if len(gaps) != 1 || gaps[0].Reason != "John Cena's home is offline" || !slices.Equal(gaps[0].NamedSubjects, []string{"sub-cena"}) || len(dropped) != 0 {
		t.Errorf("gaps %+v, dropped %v: want the gap kept as sent, naming cena", gaps, dropped)
	}
}

// A gap is kept with the add-on's kind, which the page draws by. A gap the
// add-on sent is never lost: one with no kind, a kind the contract does not
// know, or no reason is kept as unread (dropped, its days would read as
// counted zeros), and still counted as a bad value.
func TestAGapIsKeptWithItsKindAndNeverLost(t *testing.T) {
	m := addons.Metric{Name: "storage.sessions", Instrument: "counter", Unit: "count", Label: "Sessions", Dimensions: []addons.Dimension{{Name: "share", Label: "Share"}}}
	start := time.Date(2026, 11, 9, 0, 0, 0, 0, clock.Loc)
	end := start.AddDate(0, 0, 1)
	dropped := map[string]int{}
	var in []addons.StatGap
	for _, k := range []string{"began", "unread", "dropped", "unobserved", "", "outage"} {
		in = append(in, addons.StatGap{Start: start, End: end, Dimensions: map[string]string{"share": k + "x"}, Kind: k, Reason: "a reason"})
	}
	in = append(in, addons.StatGap{Start: start, End: end, Dimensions: map[string]string{"share": "silent"}, Kind: "dropped", Reason: "  "})
	var kept []string
	for _, g := range keepGaps(in, m, peopleOn(nil, nil), dropped) {
		kept = append(kept, g.Kind+":"+g.Reason)
	}
	want := []string{"began:a reason", "unread:a reason", "dropped:a reason", "unobserved:a reason", "unread:a reason", "unread:a reason", "dropped:"}
	if !slices.Equal(kept, want) || dropped[DropBadValue] != 3 {
		t.Errorf("kept %v, dropped %v: want every gap kept (an unknown kind as unread, no reason as empty) and three bad values", kept, dropped)
	}
}

// A gap over the totals alone, or over one person alone (PR 246): kept with
// its scope, the person a gap is over named by it; one over a person this
// target does not bind, or over a person on a metric not counted per person,
// is dropped; one over both, or with a scope the contract does not know, is
// a bad value, the unknown scope kept over the totals alone (it claims least
// about anyone's own figures).
func TestAGapKeepsItsScopeAndItsPerson(t *testing.T) {
	who := peopleOn([]db.TargetBinding{{SubjectID: "sub-charlotte", Username: "charlotte_flair"}}, nil)
	m := addons.Metric{Name: "storage.home_bytes", Instrument: "gauge", Unit: "bytes", PerSubject: true}
	start := time.Date(2026, 11, 9, 0, 0, 0, 0, clock.Loc)
	end := start.AddDate(0, 0, 1)
	gap := func(scope, subject string) addons.StatGap {
		return addons.StatGap{Start: start, End: end, Scope: scope, Subject: subject, Kind: "unread", Reason: "a reason"}
	}
	dropped := map[string]int{}
	var kept []string
	for _, g := range keepGaps([]addons.StatGap{gap("total", ""), gap("", "sub-charlotte"), gap("", "sub-booker"), gap("total", "sub-charlotte"), gap("whole", "")}, m, who, dropped) {
		kept = append(kept, g.Scope+"/"+g.Subject+"/"+strings.Join(g.NamedSubjects, "+"))
	}
	if want := []string{"total//", "/sub-charlotte/sub-charlotte", "total//"}; !slices.Equal(kept, want) {
		t.Errorf("kept %q, want %q", kept, want)
	}
	if dropped[DropUnboundSubject] != 1 || dropped[DropBadValue] != 2 {
		t.Errorf("dropped %v, want one unbound and two bad values", dropped)
	}
	m.PerSubject = false
	if got := keepGaps([]addons.StatGap{gap("", "sub-charlotte")}, m, who, dropped); len(got) != 0 || dropped[DropNotPerSubject] != 1 {
		t.Errorf("a person's gap on a metric not counted per person: kept %+v, dropped %v", got, dropped)
	}
}

// The name set is everyone in the directory (7i), not only who is bound here:
// an unbound user's id, display name and username are each a name, recorded
// as their subject id; an account name and id only for a binding. A text
// naming two people names both, even where one name overlaps the other
// ("john cena" inside "the john cena jr"), and the result is sorted, once each.
func TestEveryoneInTheDirectoryIsAName(t *testing.T) {
	account := "3051"
	who := peopleOn([]db.TargetBinding{{SubjectID: "sub-cena", Username: "jc_nas", AccountID: &account}},
		[]models.UserProfile{
			{ID: "sub-cena", Name: "John Cena"},
			{ID: "sub-jr", Name: "Cena Jr", Username: "cenajr"},
			{ID: "sub-taker", Name: "The Undertaker", Username: "deadman"},
			{ID: "sub-x", Name: "x", Username: "7"}, // no letter or one character: no name, the id still is
			// A service account is no person: "backup" names nobody, nor its id.
			{ID: "sub-backup", Name: "Backup", Username: "backup", Kind: models.UserKindMachine},
		})
	for text, want := range map[string]string{
		"home-deadman":                "sub-taker",
		"The Undertaker's pool":       "sub-taker",
		"sub-taker":                   "sub-taker",
		"jc_nas scratch":              "sub-cena",
		"proj-3051":                   "sub-cena",
		"cenajr":                      "sub-jr",
		"the John Cena Jr share":      "sub-cena sub-jr",
		"deadman and jc_nas, deadman": "sub-cena sub-taker",
		"x 7 sub-x":                   "sub-x",
		"x and 7":                     "",
		"makers":                      "",
		"backup":                      "",
		"nightly backup share":        "",
		"sub-backup":                  "",
	} {
		if got := strings.Join(who.names(text), " "); got != want {
			t.Errorf("%q names %q, want %q", text, got, want)
		}
	}
	if who.bound["sub-taker"] || !who.bound["sub-cena"] {
		t.Error("only a binding makes a subject bound here")
	}
}

// The cost of the name set at a makerspace's size: a few hundred people, each
// with three names. Run with -bench Names -benchmem.
func BenchmarkNames(b *testing.B) {
	users := make([]models.UserProfile, 600)
	for i := range users {
		users[i] = models.UserProfile{ID: fmt.Sprintf("3%017d", i), Name: fmt.Sprintf("Wrestler Number%d", i), Username: fmt.Sprintf("ring_name_%d", i)}
	}
	b.Run("build", func(b *testing.B) {
		for b.Loop() {
			peopleOn(nil, users)
		}
	})
	who := peopleOn(nil, users)
	b.Run("a value naming nobody", func(b *testing.B) {
		for b.Loop() {
			clear(who.seen) // the first time a text is asked; after, it is a map read
			who.names("projects-archive-2026")
		}
	})
	b.Run("a value naming one", func(b *testing.B) {
		for b.Loop() {
			clear(who.seen) // the first time a text is asked; after, it is a map read
			who.names("home-ring_name_599")
		}
	})
}
