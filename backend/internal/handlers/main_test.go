package handlers

import (
	"context"
	"os"
	"testing"

	"syndra/internal/db"
	"syndra/internal/models"
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
	os.Exit(m.Run())
}
