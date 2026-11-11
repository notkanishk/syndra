package services

import (
	"context"
	"errors"
	"testing"

	"syndra/internal/addons"
)

// Every dormant row is a candidate for removal, so the list and the removal's
// re-check read the target now (fresh), and inside the add-on's cooldown are
// refused with the wait, never answered from a copy (owner Q1).
func TestTheDormantListReadsFreshAndRefusesInsideTheCooldown(t *testing.T) {
	stubDormant(t)
	var asked []bool
	served := dormantSubjects
	dormantSubjects = func(ctx context.Context, target string, fresh bool) addons.SubjectsResult {
		asked = append(asked, fresh)
		return served(ctx, target, fresh)
	}
	if _, err := DormantAccounts(context.Background(), "truenas"); err != nil {
		t.Fatal(err)
	}
	if len(asked) != 1 || !asked[0] {
		t.Fatalf("the dormant list asked fresh %v, want [true]", asked)
	}

	retry := 20
	for name, read := range map[string]addons.SubjectsResult{
		"refused, nothing to serve": {Outcome: addons.OutcomeUnreached, Status: 429, RetryAfter: 7},
		"answered with the copy":    {Outcome: addons.OutcomeSucceeded, Stale: &addons.Stale{Reason: addons.StaleCooldown, RetryAfter: &retry}},
	} {
		dormantSubjects = func(context.Context, string, bool) addons.SubjectsResult { return read }
		_, err := DormantAccounts(context.Background(), "truenas")
		var cooled ReadCooledDown
		if !errors.As(err, &cooled) || cooled.RetryAfter < 1 {
			t.Errorf("%s: %v, want ReadCooledDown with a wait", name, err)
		}
	}
}
