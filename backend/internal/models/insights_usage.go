package models

import "time"

// UsageSource says whether a kind of usage signal is arriving at all. Silence
// from a source that is not receiving says nothing about anybody, so a screen
// must show it before it shows any "unused".
type UsageSource struct {
	Receiving bool       `json:"receiving"`
	LastAt    *time.Time `json:"last_at,omitempty"`
}

// UsageSources is one entry per kind of signal the usage table can hold.
type UsageSources struct {
	SignIn UsageSource `json:"sign_in"`
	Badge  UsageSource `json:"badge"`
}

// HeldNotUsed is a role some people hold and none of those counted has used,
// in the window.
type HeldNotUsed struct {
	ProjectID string `json:"project_id"`
	RoleKey   string `json:"role_key"`
	People    int    `json:"people"`
}

// InsightsUsage is "Recent use" and "Held but not used". HeldNotUsed is empty,
// never a guess, when sign-ins are not being received or nothing has been
// observed; HoldersObserved says whether the holders were read at all.
type InsightsUsage struct {
	Sources         UsageSources  `json:"sources"`
	HeldNotUsed     []HeldNotUsed `json:"held_not_used"`
	HoldersObserved bool          `json:"holders_observed"`
	WindowDays      int           `json:"window_days"`
}
