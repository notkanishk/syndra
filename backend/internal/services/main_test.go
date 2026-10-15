package services

import (
	"context"
	"os"
	"testing"

	"syndra/internal/db"
	"syndra/internal/models"
)

// TestMain answers "what would this cancel?" with nothing for tests about
// something else; the tests about it (bulk_test.go) stub their own.
func TestMain(m *testing.M) {
	svcWouldCancel = func(context.Context, db.EnqueueParams) ([]models.QueuedChange, error) { return nil, nil }
	os.Exit(m.Run())
}
