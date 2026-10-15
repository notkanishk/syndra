package services

import (
	"context"
	"fmt"
	"strings"

	"syndra/internal/db"
	"syndra/internal/models"
)

// Never invisibly (owner, 15 Oct): a plan row that would cancel a change
// still waiting to be sent says so before anyone applies it. The apply is
// marked reviewed (db.CancellationsReviewed) because these rows said it.

// cancelsLine is the queued changes these writes would cancel, which
// withCancels names on the row. db.WouldCancel is the same predicate the
// apply cancels by.
func cancelsLine(ctx context.Context, params []db.EnqueueParams) ([]models.QueuedChange, error) {
	var found []models.QueuedChange
	for _, p := range params {
		changes, err := svcWouldCancel(ctx, p)
		if err != nil {
			return nil, err
		}
		found = append(found, changes...)
	}
	return found, nil
}

// DescribeCancels is the one sentence for queued changes an action cancels.
func DescribeCancels(changes []models.QueuedChange) string {
	if len(changes) == 0 {
		return ""
	}
	parts := make([]string, 0, len(changes))
	for _, c := range changes {
		what := "grant"
		if c.OpType == "revoke" {
			what = "removal"
		}
		part := fmt.Sprintf("the %s of %s queued %s", what, strings.Join(c.RoleKeys, ", "), formatDay(c.QueuedAt))
		if c.Reason != "" {
			part += fmt.Sprintf(" (%s)", c.Reason)
		}
		parts = append(parts, part)
	}
	noun := "a change"
	if len(changes) > 1 {
		noun = fmt.Sprintf("%d changes", len(changes))
	}
	return fmt.Sprintf("Cancels %s waiting to be sent: %s.", noun, strings.Join(parts, "; "))
}

// WithCancels names the cancellations on a row, and folds them into its
// fingerprint: a change queued after the review, which the row never named,
// makes the plan stale instead of being cancelled unseen.
func WithCancels(out BulkOutcome, changes []models.QueuedChange) BulkOutcome {
	if len(changes) == 0 {
		return out
	}
	fields := []string{out.Fingerprint, "cancels"}
	for _, c := range changes {
		fields = append(fields, c.ID)
	}
	out.Fingerprint = Fingerprint(fields...)
	line := DescribeCancels(changes)
	if out.Consequence == "" {
		out.Consequence = line
	} else {
		out.Consequence += " " + line
	}
	return out
}
