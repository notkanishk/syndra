package stats

import (
	"reflect"
	"testing"

	"syndra/internal/addons"
	"syndra/internal/clock"
	"syndra/internal/db"
)

// One rule names a bound person (ruling B7), for a dimension value and a gap
// reason alike: it contains a subject id, account name or account id, in any
// case. What names someone is dropped and counted, never stored.
func TestNoValueOrReasonNamesABoundPerson(t *testing.T) {
	account := "3051"
	who := peopleOn([]db.TargetBinding{{SubjectID: "sub-cena", Username: "john_cena", AccountID: &account}, {SubjectID: "sub-orton", Username: ""}, {SubjectID: "sub-rey", Username: "Rey_Mysterio"}})
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
	if len(gaps) != 1 || gaps[0].Reason != "Auditing is off on this share" {
		t.Errorf("kept gaps %+v: a reason naming a person was stored", gaps)
	}
	if want := map[string]int{DropPersonInDimension: 7}; !reflect.DeepEqual(dropped, want) {
		t.Errorf("dropped %v, want %v", dropped, want)
	}
	// An empty username names nobody: "" is in every string.
	if who.names("makers") {
		t.Error("an empty name matched everything")
	}
}
