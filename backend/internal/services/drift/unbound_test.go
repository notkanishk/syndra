package drift

import (
	"context"
	"encoding/json"
	"testing"

	"syndra/internal/addons"
	"syndra/internal/db"
)

// A person who holds a mapped role and has no account is converged by the
// pass, so any grant path that skipped the cascade heals within one pass.
// One row per person, never a second while one waits, and none for someone
// whose access resolves to disabled.
func TestThePassCreatesAccountsForUnboundHolders(t *testing.T) {
	h := &addonReconcileHarness{
		read:     addons.SubjectsResult{Outcome: addons.OutcomeSucceeded, Current: true},
		mappings: []db.RoleMapping{{ProjectID: "pNet", RoleKey: "community"}, {ProjectID: "pNet", RoleKey: "staff"}},
		holders: map[string][]string{
			"pNet/community": {"u_new", "u_bound", "u_waiting", "u_held"},
			"pNet/staff":     {"u_new"},
		},
		bindings: []db.TargetBinding{{SubjectID: "u_bound", Username: "bound"}},
		waiting:  map[string]bool{"u_waiting": true},
	}
	stubAddonReconcile(t, h)
	resolveIntent = func(_ context.Context, subject, _ string) (map[string]json.RawMessage, error) {
		if subject == "u_held" {
			return map[string]json.RawMessage{"enabled": json.RawMessage(`false`)}, nil
		}
		return map[string]json.RawMessage{"enabled": json.RawMessage(`true`), "group": json.RawMessage(`["Community"]`)}, nil
	}

	n, err := convergeUnbound(context.Background(), "truenas", h.bindings)
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 || len(h.converged) != 1 || h.converged[0].SubjectID != "u_new" {
		t.Fatalf("want exactly u_new converged once, got %d %+v", n, h.converged)
	}
}
