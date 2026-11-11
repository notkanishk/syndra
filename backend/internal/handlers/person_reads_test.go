package handlers

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"syndra/internal/addons"
	"syndra/internal/services/addonop"
)

// An admin reading a person's holdings on the person page goes through the one
// read path (design §4): the add-on is asked about the person named by the
// route, by the operator, and nothing is dispatched, so no operations-log row
// is written for having looked. That Read itself begins no row is held in the
// addons package (TestAnOperatorsReadOfHoldingsWritesNoRow).
func TestAnAdminsReadOfHoldingsWritesNoOperationsRow(t *testing.T) {
	h := &personStorageHarness{storageHarness: storageHarness{entitled: true, bound: true}}
	stubPersonStorage(t, h)
	svcDispatchOperation = func(_ context.Context, req addonop.Request) (addonop.Result, error) {
		t.Fatalf("a read was dispatched, which writes an operations-log row: %+v", req)
		return addonop.Result{}, nil
	}
	records, err := os.ReadFile(filepath.Join("..", "..", "..", "addons", "contract", "v2", "fixtures", "operation_response_records.json"))
	if err != nil {
		t.Fatal(err)
	}
	var asked []string
	addonsMemberReads = func(string) []string { return []string{"holdings.list", "storage.status"} }
	addonsRead = func(_ context.Context, _, op, subject, actor string, _ map[string]any) addons.ReadReply {
		asked = append(asked, op+" "+subject+"/"+actor)
		if op == "storage.status" {
			return storageReply(addons.StorageStatus{Outcome: addons.OutcomeSucceeded, Usable: true})
		}
		return addons.ReadReply{Operation: op, Schema: "common/records.json", Results: true,
			Outcome: addons.OutcomeSucceeded, Status: 200, Body: records}
	}

	view := getPersonTargets(t)
	if len(asked) != 2 || asked[0] != "holdings.list u1/op-1" || asked[1] != "storage.status u1/op-1" {
		t.Fatalf("asked %v; want each read once, about u1, with the operator as actor", asked)
	}
	// A pocket (no results) feeds the storage block and is not a generic read.
	if view.Storage == nil || !view.Storage.Usable || len(view.Reads) != 1 {
		t.Fatalf("storage %+v, reads %+v", view.Storage, view.Reads)
	}
	r := view.Reads[0]
	if r.Operation != "holdings.list" || r.Shape != "records" || !r.Readable || len(r.AlsoSays) != 1 {
		t.Fatalf("the holdings read did not land on the person page: %+v", r)
	}
	if rec, ok := r.Result.(map[string]any); !ok || len(rec["records"].([]any)) != 1 {
		t.Fatalf("the records did not travel: %#v", r.Result)
	}
}
