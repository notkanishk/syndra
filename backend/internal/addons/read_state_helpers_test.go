package addons

import "time"

// readAt is a read's taken_at, the zero time for none: for assertions only.
func readAt(s ReadState) time.Time {
	if s.TakenAt == nil {
		return time.Time{}
	}
	return *s.TakenAt
}
