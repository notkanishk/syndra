package db

import (
	"encoding/json"
	"testing"
)

// An observation updates the fields it saw and keeps the rest. A withdraw-only
// apply sees only the lifecycle fields; if it replaced the base, the groups a
// person held before leaving would be forgotten while they are away.
func TestAnObservationKeepsTheFieldsItDidNotSee(t *testing.T) {
	ctx := liveDB(t)
	subject := freshSubject(t, ctx)
	t.Cleanup(func() { _ = ForgetMergeBase(ctx, "truenas", subject) })

	full := map[string]json.RawMessage{"group": json.RawMessage(`["lab"]`), "enabled": json.RawMessage(`true`)}
	if err := RecordMergeBase(ctx, MergeBase{Target: "truenas", SubjectID: subject, Base: full}); err != nil {
		t.Fatal(err)
	}
	lock := map[string]json.RawMessage{"enabled": json.RawMessage(`false`), "smb_enabled": json.RawMessage(`false`)}
	if err := RecordMergeBase(ctx, MergeBase{Target: "truenas", SubjectID: subject, Base: lock}); err != nil {
		t.Fatal(err)
	}

	bases, err := MergeBasesFor(ctx, "truenas")
	if err != nil {
		t.Fatal(err)
	}
	got := bases[subject].Base
	if string(got["group"]) != `["lab"]` || string(got["enabled"]) != "false" || string(got["smb_enabled"]) != "false" {
		b, _ := json.Marshal(got)
		t.Fatalf("base = %s, want group kept from the first read and the lifecycle fields from the second", b)
	}
}
