package handlers

import (
	"cmp"
	"context"
	"os"
	"testing"

	"syndra/internal/claimserve"
	"syndra/internal/db"
	"syndra/internal/services"
)

// TestMain opens the project gate for the package by default. Almost every
// handler test is about something else and names whatever project it likes;
// the tests ABOUT the gate (target_project_test.go) put the real rules back.
func TestMain(m *testing.M) {
	dbRequireTargetProject = func(context.Context, string) error { return nil }
	resolveMappingProject = func(_ context.Context, _ string, requested string) (string, error) { return requested, nil }
	svcConvergeProjectChange = func(context.Context, string, string, string) error { return nil }
	svcConvergeSubjectOn = func(context.Context, string, string, string) error { return nil }
	dbTargetProjects = func(context.Context) (map[string]string, error) { return map[string]string{}, nil }
	checkRoleOffered = func(context.Context, string, string) error { return nil }
	// No bundle includes another unless a test says so (B8).
	dbBundleIncludesAnything = func(context.Context, string) (bool, error) { return false, nil }
	dbRetireBlockers = func(context.Context, string) ([]string, error) { return nil, nil }
	dbInTxLockingAccess = func(ctx context.Context, fn func(context.Context) error) error { return fn(ctx) }
	// The welcome switch is off unless a test says so (it reads Postgres).
	svcWelcomeClaims = func(context.Context) (claimserve.Welcome, error) { return claimserve.Welcome{}, nil }
	// A Change is delivered as its plan said, with nothing sent anywhere: a
	// write that waits waits with its reason, every other row is sent and
	// taken. Delivery tests stub their own.
	svcDeliverChange = func(_ context.Context, ch services.Change) services.Delivery {
		d := services.Delivery{Reason: map[string]string{}}
		for _, w := range ch.Writes {
			switch {
			case w.OutboxID == "":
			case w.Delivery == services.DeliveryWaits:
				d.Waiting = append(d.Waiting, w.OutboxID)
				d.Reason[w.OutboxID] = cmp.Or(w.WaitReason, db.WaitReasonRoleWaits)
			default:
				d.Sent = append(d.Sent, w.OutboxID)
			}
		}
		return d
	}
	os.Exit(m.Run())
}
