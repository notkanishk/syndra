package db

import (
	"encoding/json"
	"testing"
)

// A whole observation replaces the base: that is what keeps the Zitadel
// sweep's bases self-pruning (a revoke Syndra performed leaves no stale key to
// read against a later re-grant). A Partial one, a withdraw-only apply's two
// lifecycle fields, updates only its own and keeps the groups a person held
// while they are away.
func TestAPartialObservationKeepsTheFieldsItDidNotSee(t *testing.T) {
	ctx := liveDB(t)
	subject := freshSubject(t, ctx)
	t.Cleanup(func() { _ = ForgetMergeBase(ctx, "truenas", subject) })
	record := func(base map[string]json.RawMessage, partial bool) map[string]json.RawMessage {
		t.Helper()
		if err := RecordMergeBase(ctx, MergeBase{Target: "truenas", SubjectID: subject, Base: base, Partial: partial}); err != nil {
			t.Fatal(err)
		}
		bases, err := MergeBasesFor(ctx, "truenas")
		if err != nil {
			t.Fatal(err)
		}
		return bases[subject].Base
	}
	full := map[string]json.RawMessage{"group": json.RawMessage(`["lab"]`), "enabled": json.RawMessage(`true`)}
	lock := map[string]json.RawMessage{"enabled": json.RawMessage(`false`), "smb_enabled": json.RawMessage(`false`)}

	record(full, false)
	if got := record(lock, true); string(got["group"]) != `["lab"]` || string(got["enabled"]) != "false" {
		b, _ := json.Marshal(got)
		t.Fatalf("partial: base = %s, want group kept and the lifecycle fields updated", b)
	}
	if got := record(lock, false); got["group"] != nil {
		b, _ := json.Marshal(got)
		t.Fatalf("whole: base = %s, want it replaced, group gone", b)
	}
}
