package db

import (
	"encoding/json"
	"strings"
)

// syndraLabels are the fixed words Syndra (backend or console) writes where a
// person's reason goes: the ledger's reason, a write's Reason, an extension's
// reason. They describe what Syndra did; quoted, they would read as words
// somebody said (owner: nothing says what doesn't happen). said_guard_test.go
// fails when a fixed reason is written in the backend or ui/src and is not here.
var syndraLabels = map[string]bool{
	// Backend.
	"Approved from access request":                  true, // services.ApprovalReason
	"Kept longer, from a request":                   true,
	"Given again, from a request to keep it longer": true,
	"Bundle deletion cascade":                       true, // services/cascade.go systemParams
	"Bundle holder moved between versions":          true, // services/planner_bulk.go writeCohort
	"Bundle membership cascade":                     true,
	"Direct access removal":                         true, // removals with none, 8 Nov (on production rows)
	"drift attribution":                             true, // services/planner_ops.go writeAdopt
	"Grant expired":                                 true, // services/role_members.go
	"Mapping rule cascade":                          true, // services/rule_follow.go, planner_act.go, cascade.go
	"Mapping-rule deletion cascade":                 true,
	"Mapping-rule update cascade":                   true,
	// Console (ui/src).
	"Extended from Expiring access":                true, // useExtend
	"Extended from Home":                           true,
	"Extended from the person's page":              true,
	"Finishing an unbind that did not complete":    true, // MergeFindings
	"Marked as owned elsewhere in bulk from Drift": true, // UnexplainedAccess
}

// Said is reason as a person said it: trimmed, and "" when it is empty or one
// of Syndra's own labels (a publish's "Bundle published at vN" included). The
// one test every surface that quotes a reason reads: Pending changes, the
// audit line, Change history, the cancel step and the member feed.
func Said(reason string) string {
	r := strings.TrimSpace(reason)
	if syndraLabels[r] || strings.HasPrefix(r, "Bundle published at v") {
		return ""
	}
	return r
}

// detailsSaid is an audit row's reason as said: details.reason, or a
// cancellation's details.cancelled.reason.
func detailsSaid(details []byte) string {
	var d struct {
		Reason    string `json:"reason"`
		Cancelled struct {
			Reason string `json:"reason"`
		} `json:"cancelled"`
	}
	if json.Unmarshal(details, &d) != nil {
		return ""
	}
	if d.Reason != "" {
		return Said(d.Reason)
	}
	return Said(d.Cancelled.Reason)
}
