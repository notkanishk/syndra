package models

// InsightsTrends is the read-only "how access has been moving" report
// (Insights I4). Everything in it is a count or a median over a window; nothing
// here is a decision.
type InsightsTrends struct {
	Window     TrendWindow       `json:"window"`
	Weekly     []TrendWeek       `json:"weekly"`
	Totals     TrendTotals       `json:"totals"`
	Requests   TrendRequests     `json:"requests"`
	Sending    TrendSending      `json:"sending"`
	Drift      TrendDrift        `json:"drift"`
	EndingSoon []TrendEndingSoon `json:"ending_soon"`
	Upkeep     TrendUpkeep       `json:"upkeep"`
}

// TrendWindow is the period asked about. From and To are local dates
// (YYYY-MM-DD), both inclusive.
type TrendWindow struct {
	From  string `json:"from"`
	To    string `json:"to"`
	Label string `json:"label"`
}

// TrendWeek is one Monday-started week of changes, counted from the audit log.
type TrendWeek struct {
	WeekStart     string `json:"week_start"`
	ThroughBundle int    `json:"through_bundle"`
	ByRequest     int    `json:"by_request"`
	ByHand        int    `json:"by_hand"`
	TakenAway     int    `json:"taken_away"`
}

type TrendTotals struct {
	Given     int `json:"given"`
	TakenAway int `json:"taken_away"`
}

type TrendRequests struct {
	CameIn              int                `json:"came_in"`
	Approved            int                `json:"approved"`
	Declined            int                `json:"declined"`
	Withdrawn           int                `json:"withdrawn"`
	DeclinedReasons     []TrendReason      `json:"declined_reasons"`
	MedianHoursToDecide *float64           `json:"median_hours_to_decide"`
	OutsideBundles      []TrendOutsideRole `json:"outside_bundles"`
}

type TrendReason struct {
	Reason string `json:"reason"`
	N      int    `json:"n"`
}

// TrendOutsideRole is a role people ask for that no bundle gives.
type TrendOutsideRole struct {
	ProjectID string `json:"project_id"`
	RoleKey   string `json:"role_key"`
	N         int    `json:"n"`
}

// TrendSending is how long an approved request took to reach Zitadel. Sample is
// how many sends the median is over; finished outbox rows are pruned, so it can
// be well short of the number of approvals.
type TrendSending struct {
	MedianMinutesApprovalToSent *float64 `json:"median_minutes_approval_to_sent"`
	Sample                      int      `json:"sample"`
}

type TrendDrift struct {
	Found              int      `json:"found"`
	Cleared            int      `json:"cleared"`
	MedianHoursToClear *float64 `json:"median_hours_to_clear"`
}

// TrendEndingSoon counts people whose direct grant in a project ends within 30,
// 60 and 90 days (cumulative, distinct people).
type TrendEndingSoon struct {
	ProjectID   string `json:"project_id"`
	ProjectName string `json:"project_name"`
	D30         int    `json:"d30"`
	D60         int    `json:"d60"`
	D90         int    `json:"d90"`
}

type TrendUpkeep struct {
	PeopleWhoLeftHolding int `json:"people_who_left_holding"`
	DriftOpen            int `json:"drift_open"`
	RolesNobodyHas       int `json:"roles_nobody_has"`
	BundlesNothingNew    int `json:"bundles_nothing_new"`
	Overlaps             int `json:"overlaps"`
}
