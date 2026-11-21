package services

import (
	"context"
	"errors"
	"testing"
	"time"

	"syndra/internal/addons"
)

// Every dormant row is a candidate for removal, so the list and the removal's
// re-check read the target now (fresh), and inside the add-on's cooldown are
// NotRead cooldown with the wait, never answered from a copy (owner Q1).
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
		var nr addons.NotRead
		if !errors.As(err, &nr) || nr.Why.Reason != addons.ReasonCooldown || nr.Why.RetryAfter < 1 {
			t.Errorf("%s: %v, want NotRead cooldown with a wait", name, err)
		}
	}
}

// A read that is not a reading travels as NotRead with the read's own why,
// for the handler's 503; one cut at the cap is answered, and says so.
func TestTheDormantListSaysWhyItWasNotRead(t *testing.T) {
	stubDormant(t)
	down := addons.SubjectsResult{Outcome: addons.OutcomeUnreached, Err: errors.New("dial tcp: connection refused")}
	want := down.Read().Why()
	if want.Reason == "" {
		t.Fatal("the fixture is not a read with a why")
	}
	dormantSubjects = func(context.Context, string, bool) addons.SubjectsResult { return down }
	_, err := DormantAccounts(context.Background(), "truenas")
	var nr addons.NotRead
	if !errors.As(err, &nr) || nr.Why != want || errors.Is(err, ErrReadTruncated) {
		t.Errorf("down: %v, want NotRead %+v", err, want)
	}

	dormantSubjects = func(context.Context, string, bool) addons.SubjectsResult {
		return addons.SubjectsResult{Outcome: addons.OutcomeSucceeded, Current: true, TakenAt: time.Now(), Truncated: true}
	}
	_, err = DormantAccounts(context.Background(), "truenas")
	if !errors.Is(err, ErrReadTruncated) || !errors.As(err, &nr) {
		t.Errorf("cut at the cap: %v, want ErrReadTruncated with a NotRead", err)
	}
}
