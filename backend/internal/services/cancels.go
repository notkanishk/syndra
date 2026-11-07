package services

import (
	"fmt"
	"strings"

	"syndra/internal/models"
)

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
	return fmt.Sprintf("Cancels %s waiting for you to send: %s.", noun, strings.Join(parts, "; "))
}
