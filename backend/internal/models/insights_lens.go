package models

import "time"

// LensRole is one role, named the way a person would say it, and how many
// people the lens counts for it. EarliestEnd is set only by the "ending" lens.
type LensRole struct {
	ProjectID   string     `json:"project_id"`
	ProjectName string     `json:"project_name"`
	RoleKey     string     `json:"role_key"`
	RoleLabel   string     `json:"role_label"`
	People      int        `json:"people"`
	EarliestEnd *time.Time `json:"earliest_end,omitempty"`
}

// InsightsLensEnding is "whose access ends in the next N days", by role. Roles
// is empty, never a guess, when nothing has been observed (HoldersObserved
// false): an end only matters to somebody who holds the access today.
type InsightsLensEnding struct {
	Kind            string     `json:"kind"` // "ending"
	WindowDays      int        `json:"window_days"`
	HoldersObserved bool       `json:"holders_observed"`
	Roles           []LensRole `json:"roles"`
}

// InsightsLensGroup is "what does this group hold": its size, and per role how
// many of them Zitadel shows holding it. Size is a head count of the group
// itself and does not depend on anything having been observed.
type InsightsLensGroup struct {
	Kind            string     `json:"kind"`  // "group"
	Group           string     `json:"group"` // left | new_term | team:<name>
	Size            int        `json:"size"`
	HoldersObserved bool       `json:"holders_observed"`
	Roles           []LensRole `json:"roles"`
}
