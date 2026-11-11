package addons

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"slices"
	"strings"
	"testing"

	"syndra/internal/db"
)

// The one read path (feature results, design §4): every effect: read operation
// goes through Read, which writes no row, takes no claim and asks nobody to
// confirm; `result` is read only from an add-on declaring results; and a reply
// missing what its schema requires is not a reading.

// underManifest swaps the installed add-on's manifest for a fixture's, accepted
// as Refresh accepts one: validated, then intersected with policy.
func underManifest(t *testing.T, fixture string) {
	t.Helper()
	m, err := decodeManifest(contractFixture(t, fixture))
	if err != nil {
		t.Fatal(err)
	}
	if err := m.validate(); err != nil {
		t.Fatalf("fixtures/%s is refused: %v", fixture, err)
	}
	ops, unknown := resolveOperations(m)
	if len(unknown) > 0 {
		t.Fatalf("fixtures/%s declares operations the policy lacks: %v", fixture, unknown)
	}
	a, err := Get("truenas")
	if err != nil {
		t.Fatal(err)
	}
	registryMu.Lock()
	a.manifest, a.ops = &m, ops
	registryMu.Unlock()
}

// noRecord fails the test on any operations-log row or claim.
func noRecord(t *testing.T) {
	t.Helper()
	begin, claim := dbBeginAddonOperation, dbClaimAddonOperation
	t.Cleanup(func() { dbBeginAddonOperation, dbClaimAddonOperation = begin, claim })
	dbBeginAddonOperation = func(_ context.Context, p db.AddonOperationParams) (string, error) {
		t.Fatalf("a read began an operations-log row: %+v", p)
		return "", nil
	}
	dbClaimAddonOperation = func(_ context.Context, id, _, op, _ string) (db.AddonOperation, error) {
		t.Fatalf("a read claimed operation record %s (%s)", id, op)
		return db.AddonOperation{}, nil
	}
}

// The fourth target as a manifest and a binary: holdings.list is read for a
// person by an operator through Read, leaves as the request schema says, and
// lands as records, with the x- key quoted under what the add-on also says.
func TestAnOperatorsReadOfHoldingsWritesNoRow(t *testing.T) {
	got := addonAnswering(t, "operation_response_records.json")
	underManifest(t, "manifest_inventory.json")
	noRecord(t)

	if reads := MemberReads("truenas"); !slices.Equal(reads, []string{"holdings.list"}) {
		t.Fatalf("member reads %v, want holdings.list alone", reads)
	}
	reply := Read(context.Background(), "truenas", "holdings.list", "289471021834760193", "operator-289471021834760199", nil)
	assertOperationRequest(t, got, "holdings.list")
	var sent struct{ Subject, Actor string }
	_ = json.Unmarshal(got.body, &sent)
	if sent.Subject != "289471021834760193" || sent.Actor != "operator-289471021834760199" {
		t.Errorf("sent subject %q actor %q: the route names the subject, the operator acts", sent.Subject, sent.Actor)
	}
	v := reply.View()
	recs, ok := v.Result.(*Records)
	if !v.Readable || v.Shape != "records" || !ok || len(recs.Records) != 1 || recs.TakenAt.IsZero() || !recs.Current {
		t.Fatalf("the holdings did not land: %+v", v)
	}
	if r := recs.Records[0]; r.ID != "item-2207" || r.Label == "" || r.Category != "tools" || r.Due == nil || r.Since == nil || r.Standing == nil {
		t.Errorf("the record did not land: %+v", r)
	}
	if !slices.Equal(v.AlsoSays, []string{`"records[0].x-inventory-shelf": "B4"`}) {
		t.Errorf("what the add-on also says: %q", v.AlsoSays)
	}
}

// A write never goes through the read path, and nothing is sent for it.
func TestReadRefusesAnOperationThatIsNotARead(t *testing.T) {
	got := addonAnswering(t, "operation_response.json")
	noRecord(t)
	reply := Read(context.Background(), "truenas", "password.set", "s", "s", map[string]any{"password": "x"})
	if !errors.Is(reply.Err, ErrNotARead) || reply.Outcome != OutcomeUnreached || got.method != "" {
		t.Fatalf("a write through Read: %+v, sent %s %s", reply, got.method, got.path)
	}
}

// A read with no legacy pocket is unavailable on an add-on without results,
// with a reason an operator reads, and is never sent.
func TestAReadWithNoPocketNeedsResults(t *testing.T) {
	got := addonAnswering(t, "operation_response_records.json")
	underManifest(t, "manifest_inventory.json")
	a, _ := Get("truenas")
	registryMu.Lock()
	m := *a.manifest
	m.Features = []string{"operations"}
	a.manifest = &m
	a.ops, _ = resolveOperations(m)
	registryMu.Unlock()

	var unavailable *ErrOperationUnavailable
	if reply := Read(context.Background(), "truenas", "holdings.list", "s", "s", nil); !errors.As(reply.Err, &unavailable) ||
		!strings.Contains(unavailable.Reason, "results") || got.method != "" {
		t.Fatalf("holdings.list without results: %+v, sent %s", reply, got.path)
	}
	if reads := MemberReads("truenas"); len(reads) != 0 {
		t.Errorf("member reads %v; an unavailable read is not read", reads)
	}
}

// `result` is read only from an add-on that declares results, and a results
// add-on is not read from a pocket: the feature is the promise of the shape.
func TestResultIsReadOnlyFromAResultsAddOn(t *testing.T) {
	ctx := context.Background()
	addonAnswering(t, "operation_response_result.json") // result, no pocket; goodManifest has no results
	if st := Read(ctx, "truenas", "storage.status", "s", "s", nil).Storage(); st.Outcome != OutcomeIndeterminate {
		t.Errorf("a result from an add-on without results was read: %+v", st)
	}
	addonAnswering(t, "operation_response_storage.json") // the pocket, from an add-on declaring results
	declareResults(t)
	if st := Read(ctx, "truenas", "storage.status", "s", "s", nil).Storage(); st.Outcome != OutcomeIndeterminate {
		t.Errorf("a pocket from an add-on declaring results was read: %+v", st)
	}
	if v := Read(ctx, "truenas", "storage.status", "s", "s", nil).View(); v.Readable {
		t.Errorf("a results add-on answering a pocket is readable: %+v", v)
	}
}

// Absence is never a value: a result missing a key its schema requires is
// not a reading, from a results add-on, for each shape.
func TestAResultMissingWhatItsSchemaRequiresIsNotAReading(t *testing.T) {
	for _, tc := range []struct {
		fixture, op, drop string
	}{
		{"operation_response_result.json", "storage.status", "taken_at"},
		{"operation_response_result.json", "storage.status", "current"},
		{"operation_response_result.json", "storage.status", "places_readable"},
		{"operation_response_result.json", "storage.status", "meters[0].unit"},
		{"operation_response_activity_result.json", "activity.get", "uncovered"},
		{"operation_response_activity_result.json", "activity.get", "events[0].success"},
		{"operation_response_records.json", "holdings.list", "records[0].label"},
	} {
		t.Run(tc.op+" without "+tc.drop, func(t *testing.T) {
			var doc map[string]any
			_ = json.Unmarshal(contractFixture(t, tc.fixture), &doc)
			result := doc["result"].(map[string]any)
			if list, key, nested := strings.Cut(tc.drop, "[0]."); nested {
				delete(result[list].([]any)[0].(map[string]any), key)
			} else {
				delete(result, tc.drop)
			}
			body, _ := json.Marshal(doc)
			addonServing(t, body)
			manifest := "manifest_truenas.json"
			if tc.op == "holdings.list" {
				manifest = "manifest_inventory.json"
			}
			underManifest(t, manifest)
			reply := Read(context.Background(), "truenas", tc.op, "s", "s", nil)
			if v := reply.View(); v.Readable {
				t.Errorf("read as a reading: %+v", v)
			}
			switch tc.op {
			case "storage.status":
				if st := reply.Storage(); st.Outcome != OutcomeIndeterminate {
					t.Errorf("storage: %+v", st)
				}
			case "activity.get":
				if act := reply.Activity(); act.Outcome != OutcomeIndeterminate {
					t.Errorf("activity: %+v", act)
				}
			}
		})
	}
}

// The TrueNAS add-on with results, against this reader: its manifest
// (manifest_truenas.json declares results) and its answers (the *_result
// fixtures its own suite encodes, contract_test.go) land as the card needs
// them, and storage.status is the one member read.
func TestTheTrueNASAddOnWithResultsLandsInTheReader(t *testing.T) {
	addonAnswering(t, "operation_response_result.json")
	underManifest(t, "manifest_truenas.json")
	if reads := MemberReads("truenas"); !slices.Equal(reads, []string{"storage.status"}) {
		t.Fatalf("member reads %v", reads)
	}
	reply := Read(context.Background(), "truenas", "storage.status", "289471021834760193", "289471021834760193", nil)
	st := reply.Storage()
	if st.Outcome != OutcomeSucceeded || !st.NeedsPassword || st.NeedsNewPassword || st.Usable || !st.PlacesReadable ||
		len(st.Places) != 1 || st.Places[0].Usage == nil || st.Standing == nil || st.Standing.WaitingOn != "member" {
		t.Fatalf("the storage card's facts did not land: %+v", st)
	}
	v := reply.View()
	status, ok := v.Result.(*Status)
	if !v.Readable || v.Shape != "status" || !ok || len(status.Meters) != 1 || len(v.AlsoSays) != 0 {
		t.Fatalf("the status did not land: %+v", v)
	}
	if m := status.Meters[0]; m.Label != "makers" || m.Used != 19756849561 || m.Limit == nil || *m.Limit != 53687091200 || m.Unit != "bytes" {
		t.Errorf("the meter did not land: %+v", m)
	}

	for _, tc := range []struct{ fixture, op string }{
		{"operation_response_activity_result.json", "activity.get"},
		{"operation_response_health_result.json", "health.get"},
	} {
		addonAnswering(t, tc.fixture)
		underManifest(t, "manifest_truenas.json")
		switch reply := Read(context.Background(), "truenas", tc.op, "289471021834760193", "op", nil); tc.op {
		case "activity.get":
			if act := reply.Activity(); act.Outcome != OutcomeSucceeded || len(act.Events) != 1 || act.Events[0].Resource != "makers" || act.Observation == nil {
				t.Errorf("activity: %+v", act)
			}
		case "health.get":
			if sys := reply.Health(); sys.Outcome != OutcomeSucceeded || len(sys.Pools) != 1 || sys.System == nil {
				t.Errorf("health: %+v", sys)
			}
		}
	}
}

// A refusal is the add-on's status, carried, so a caller can tell 422 (no
// bound account) from an outage.
func TestARefusedReadCarriesItsStatus(t *testing.T) {
	addonServingStatus(t, http.StatusUnprocessableEntity, []byte(`{"type":"urn:syndra:addon:error:OPERATION_FAILED","title":"Operation failed",`+
		`"status":422,"detail":"No account is bound to this subject yet.","code":"OPERATION_FAILED","retry":"never"}`))
	act := Activity(context.Background(), "truenas", "s", "")
	if act.Outcome != OutcomeRejected || act.Status != http.StatusUnprocessableEntity {
		t.Fatalf("a refused read: %+v", act)
	}
}
