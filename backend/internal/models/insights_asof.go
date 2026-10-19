package models

import "time"

// InsightsAsOf is the access graph as Syndra last recorded it on or before a
// date (Insights "As of a date"). Day is the snapshot's own day, which can be
// earlier than the date asked about. ChangesSince counts the audit-log rows for
// access given or taken away from that day to now: the distance between this
// picture and today, not a correction of it.
type InsightsAsOf struct {
	Day          string         `json:"day"`
	TakenAt      time.Time      `json:"taken_at"`
	Topology     TopologyGraph  `json:"topology"`
	RolePeople   map[string]int `json:"role_people"`
	FirstDay     string         `json:"first_day"`
	ChangesSince int            `json:"changes_since"`
}

// TimelineDay is how many access changes the audit log shows on one local day.
type TimelineDay struct {
	Day     string `json:"day"`
	Changes int    `json:"changes"`
}

// InsightsTimeline is the scrubber's ticks. SnapshotDays is [first, last] stored
// day, both null when nothing has been stored yet.
type InsightsTimeline struct {
	Days         []TimelineDay `json:"days"`
	SnapshotDays [2]*string    `json:"snapshot_days"`
}
