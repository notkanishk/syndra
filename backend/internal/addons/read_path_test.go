package addons

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"testing"
	"unicode/utf8"

	"syndra/internal/db"
)

// The one read path (design §4): every effect: read operation goes through
// Read, which writes no row, takes no claim and asks nobody to confirm; the
// reply is read from `result` alone; and a reply missing what its schema
// requires is not a reading.

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
	reply := Read(context.Background(), "truenas", "holdings.list", "289471021834760193", "operator-289471021834760199", nil, false)
	assertOperationRequest(t, got, "holdings.list")
	var sent struct{ Subject, Actor string }
	_ = json.Unmarshal(got.body, &sent)
	if sent.Subject != "289471021834760193" || sent.Actor != "operator-289471021834760199" {
		t.Errorf("sent subject %q actor %q: the route names the subject, the operator acts", sent.Subject, sent.Actor)
	}
	v := reply.View()
	recs, ok := v.Result.(*Records)
	if !v.Readable || v.Shape != "records" || !ok || len(recs.Records) != 2 || recs.TakenAt.IsZero() || !recs.Current {
		t.Fatalf("the holdings did not land: %+v", v)
	}
	if r := recs.Records[0]; r.ID != "item-2207" || r.Label == "" || r.Category != "tools" || r.Due == nil || r.Since == nil || r.Standing == nil {
		t.Errorf("the record did not land: %+v", r)
	}
	// The overdue one: due before the read, waiting on the member, with the
	// next step the page shows.
	if r := recs.Records[1]; r.Due == nil || !r.Due.Before(recs.TakenAt) || r.Standing == nil || r.Standing.Usable ||
		r.Standing.WaitingOn != "member" || r.Standing.Next == "" {
		t.Errorf("the overdue record did not land with its next step: %+v", r)
	}
	if !slices.Equal(v.AlsoSays, []string{`"records[0].x-inventory-shelf": "B4"`}) {
		t.Errorf("what the add-on also says: %q", v.AlsoSays)
	}
}

// A write never goes through the read path, and nothing is sent for it.
func TestReadRefusesAnOperationThatIsNotARead(t *testing.T) {
	got := addonAnswering(t, "operation_response.json")
	noRecord(t)
	reply := Read(context.Background(), "truenas", "password.set", "s", "s", map[string]any{"password": "x"}, false)
	if !errors.Is(reply.Err, ErrNotARead) || reply.Outcome != OutcomeUnreached || got.method != "" {
		t.Fatalf("a write through Read: %+v, sent %s %s", reply, got.method, got.path)
	}
}

// Nor does a read go through Call, the write path's transport: Read alone
// sends one, as Dispatch refuses one.
func TestCallRefusesARead(t *testing.T) {
	got := addonAnswering(t, "operation_response_result.json")
	resp := Call(context.Background(), CallRequest{Target: "truenas", Operation: "storage.status", Subject: "s", Actor: "s"})
	if !errors.Is(resp.Err, ErrReadNotCalled) || resp.Outcome != OutcomeUnreached || got.method != "" {
		t.Fatalf("a read through Call: %+v, sent %s %s", resp, got.method, got.path)
	}
}

// A read needs no feature: `result` is core in contract 2, so holdings.list is
// offered by an add-on declaring operations alone, and read.
func TestAReadNeedsNoFeature(t *testing.T) {
	got := addonAnswering(t, "operation_response_records.json")
	underManifest(t, "manifest_inventory.json")
	a, _ := Get("truenas")
	registryMu.Lock()
	m := *a.manifest
	m.Features = []string{"operations"}
	a.manifest = &m
	a.ops, _ = resolveOperations(m)
	registryMu.Unlock()

	if v := Read(context.Background(), "truenas", "holdings.list", "s", "s", nil, false).View(); !v.Readable || got.path != "/operations/holdings.list" {
		t.Fatalf("holdings.list from an add-on declaring operations alone: %+v, sent %s", v, got.path)
	}
	if reads := MemberReads("truenas"); !slices.Contains(reads, "holdings.list") {
		t.Errorf("member reads %v; holdings.list is offered", reads)
	}
}

// A read is answered under `result` and nothing else: a reply carrying only
// the key that read had before contract 2 made result core (storage,
// activity, health) is not a reading, refused like any reply missing what its
// schema requires (indeterminate, with the reason), never guessed at. The same
// reply under `result` is read, so the refusal is the key's alone.
func TestAReplyUnderAnOldPerReadKeyIsNotAReading(t *testing.T) {
	ctx := context.Background()
	for _, tc := range []struct {
		fixture, op, old string
		outcome          func(ReadReply) (Outcome, error)
	}{
		{"operation_response_result.json", "storage.status", "storage", func(r ReadReply) (Outcome, error) { s := r.Storage(); return s.Outcome, s.Err }},
		{"operation_response_activity_result.json", "activity.get", "activity", func(r ReadReply) (Outcome, error) { a := r.Activity(); return a.Outcome, a.Err }},
		{"operation_response_health_result.json", "health.get", "health", func(r ReadReply) (Outcome, error) { h := r.Health(); return h.Outcome, h.Err }},
	} {
		addonAnswering(t, tc.fixture)
		if got, err := tc.outcome(Read(ctx, "truenas", tc.op, "s", "s", nil, false)); got != OutcomeSucceeded {
			t.Fatalf("%s under result: %s %v, want it read", tc.op, got, err)
		}
		var doc map[string]json.RawMessage
		_ = json.Unmarshal(contractFixture(t, tc.fixture), &doc)
		doc[tc.old] = doc["result"]
		delete(doc, "result")
		body, _ := json.Marshal(doc)
		addonServing(t, body)
		reply := Read(ctx, "truenas", tc.op, "s", "s", nil, false)
		if got, err := tc.outcome(reply); got != OutcomeIndeterminate || err == nil || !strings.Contains(err.Error(), `no "result"`) {
			t.Errorf("%s under %q alone: %s %v, want indeterminate: the reply carries no result", tc.op, tc.old, got, err)
		}
		if v := reply.View(); v.Readable {
			t.Errorf("%s under %q alone is readable: %+v", tc.op, tc.old, v)
		}
	}
}

// Absence is never a value: a result missing a key its schema requires is
// not a reading, for each shape.
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
		// Nested: a standing wherever one is, and each place, have their own.
		{"operation_response_result.json", "storage.status", "standing.usable"},
		{"operation_response_result.json", "storage.status", "standing.waiting_on"},
		{"operation_response_result.json", "storage.status", "places[0].share"},
		{"operation_response_result.json", "storage.status", "places[0].path"},
		{"operation_response_result.json", "storage.status", "places[0].access"},
		{"operation_response_result.json", "storage.status", "places[0].usage.used_bytes"},
		{"operation_response_records.json", "holdings.list", "records[0].standing.usable"},
		{"operation_response_records.json", "holdings.list", "records[1].standing.waiting_on"},
	} {
		t.Run(tc.op+" without "+tc.drop, func(t *testing.T) {
			var doc map[string]any
			_ = json.Unmarshal(contractFixture(t, tc.fixture), &doc)
			// Walk to the key's parent: "a.b" is a key, "a[1]" an item.
			node := doc["result"].(map[string]any)
			steps := strings.Split(tc.drop, ".")
			for _, step := range steps[:len(steps)-1] {
				key, index, isItem := strings.Cut(strings.TrimSuffix(step, "]"), "[")
				next := node[key]
				if isItem {
					i, _ := strconv.Atoi(index)
					next = next.([]any)[i]
				}
				node = next.(map[string]any)
			}
			delete(node, steps[len(steps)-1])
			body, _ := json.Marshal(doc)
			addonServing(t, body)
			manifest := "manifest_truenas.json"
			if tc.op == "holdings.list" {
				manifest = "manifest_inventory.json"
			}
			underManifest(t, manifest)
			reply := Read(context.Background(), "truenas", tc.op, "s", "s", nil, false)
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

// The TrueNAS add-on, against this reader: its manifest
// (manifest_truenas.json) and its answers (the *_result
// fixtures its own suite encodes, contract_test.go) land as the card needs
// them, and storage.status is the one member read.
func TestTheTrueNASAddOnLandsInTheReader(t *testing.T) {
	addonAnswering(t, "operation_response_result.json")
	underManifest(t, "manifest_truenas.json")
	if reads := MemberReads("truenas"); !slices.Equal(reads, []string{"storage.status"}) {
		t.Fatalf("member reads %v", reads)
	}
	reply := Read(context.Background(), "truenas", "storage.status", "289471021834760193", "289471021834760193", nil, false)
	st := reply.Storage()
	if st.Outcome != OutcomeSucceeded || !st.NeedsPassword || st.NeedsNewPassword || st.Usable || !st.PlacesReadable || !st.QuotaRead ||
		len(st.Places) != 1 || st.Places[0].Usage == nil || st.Standing == nil || st.Standing.WaitingOn != "member" {
		t.Fatalf("the storage card's facts did not land: %+v", st)
	}
	v := reply.View()
	status, ok := v.Result.(*StorageResult)
	if !v.Readable || v.Shape != "status" || !ok || len(status.Meters) != 1 || !status.QuotaRead || len(v.AlsoSays) != 0 {
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
		switch reply := Read(context.Background(), "truenas", tc.op, "289471021834760193", "op", nil, false); tc.op {
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
	act := Activity(context.Background(), "truenas", "s", "s", "", false)
	if act.Outcome != OutcomeRejected || act.Status != http.StatusUnprocessableEntity {
		t.Fatalf("a refused read: %+v", act)
	}
}

// quota_read is carried, never dropped: a storage profile whose usage could
// not be read in full reaches the card (view.storage) and the generic renderer
// (reads) as quota_read false, with the meters that were read kept beside it,
// so the page says "couldn't read" rather than drawing what it has as all
// there is.
func TestAnUnreadQuotaReachesThePageAsUnread(t *testing.T) {
	var doc map[string]any
	_ = json.Unmarshal(contractFixture(t, "operation_response_result.json"), &doc)
	doc["result"].(map[string]any)["quota_read"] = false
	body, _ := json.Marshal(doc)
	addonServing(t, body)
	underManifest(t, "manifest_truenas.json")

	reply := Read(context.Background(), "truenas", "storage.status", "s", "s", nil, false)
	if st := reply.Storage(); st.Outcome != OutcomeSucceeded || st.QuotaRead {
		t.Errorf("view.storage: %+v, want quota_read false", st)
	}
	v := reply.View()
	got, _ := json.Marshal(v)
	if !v.Readable || !strings.Contains(string(got), `"quota_read":false`) || !strings.Contains(string(got), `"label":"makers"`) {
		t.Errorf("reads: %s, want quota_read false beside the meter that was read", got)
	}

}

// What the add-on also says is capped whole: each entry, its key path
// included, and how many entries, the last line then saying how many more.
// The least trusted component chooses the size of both.
func TestWhatTheAddOnAlsoSaysIsCappedWhole(t *testing.T) {
	doc := map[string]any{"x-##" + strings.Repeat("é", 300): "v"} // odd prefix: the cut lands inside a rune
	for i := range 60 {
		doc[fmt.Sprintf("x-%02d", i)] = i
	}
	got := unknownKeys("", doc, keySpec{})
	if len(got) != alsoSaysMax+1 || got[alsoSaysMax] != "…and 11 more" {
		t.Fatalf("%d entries, last %q: want %d and then how many more", len(got), got[len(got)-1], alsoSaysMax)
	}
	for _, e := range got {
		if len(e) > alsoSaysCap+len("…") || !utf8.ValidString(e) {
			t.Errorf("entry of %d bytes (valid UTF-8 %v): %.40q…", len(e), utf8.ValidString(e), e)
		}
	}
	if !strings.HasPrefix(got[0], `"x-##é`) {
		t.Errorf("the long key's entry is not first: %.40q", got[0])
	}
}
