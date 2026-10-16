package handlers

import (
	"context"
	"os"
	"testing"

	"syndra/internal/db"
	"syndra/internal/models"
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
	dbWouldCancel = func(context.Context, db.EnqueueParams) ([]models.QueuedChange, error) { return nil, nil }
	// No bundle includes another unless a test says so (B8).
	dbBundleIncludesAnything = func(context.Context, string) (bool, error) { return false, nil }
	dbRetireBlockers = func(context.Context, string) ([]string, error) { return nil, nil }
	dbInTxLockingAccess = func(ctx context.Context, fn func(context.Context) error) error { return fn(ctx) }
	// No role waits and nobody has switched over: a change held by its own
	// mode waits, anything else is sent row by row through the stubbable
	// DrainOne, as before roles that wait. delivery tests stub their own.
	svcDeliver = func(ctx context.Context, ids []string, legacyWait bool) services.Delivery {
		d := services.Delivery{Reason: map[string]string{}}
		for _, id := range ids {
			if legacyWait {
				d.Waiting = append(d.Waiting, id)
				d.Reason[id] = db.WaitReasonManual
				continue
			}
			d.Sent = append(d.Sent, id)
			dr, err := svcDrainPropagationRow(ctx, id)
			if err != nil {
				d.Drain.Halted, d.Drain.Reason = true, err.Error()
				return d
			}
			d.Drain.Applied += dr.Applied
			if dr.Halted {
				d.Drain.Halted, d.Drain.Reason = true, dr.Reason
				return d
			}
		}
		return d
	}
	os.Exit(m.Run())
}
