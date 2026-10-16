package handlers

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"syndra/internal/db"
)

// GET /me/changes?days=30 (B10, N19). What changed in a person's own access
// lately, in their words on My access. Member-scoped: the subject is always
// the caller, so nobody reads another person's changes here, and the audit
// log stays operator-only.

type myChange struct {
	Kind      string     `json:"kind"`
	At        time.Time  `json:"at"`
	ProjectID string     `json:"project_id"`
	RoleKey   string     `json:"role_key"`
	By        string     `json:"by,omitempty"`
	Note      string     `json:"note,omitempty"`
	Via       *myViaRef  `json:"via"`
	Until     *time.Time `json:"until,omitempty"`
	// Waiting is an approval that hasn't reached Zitadel yet, and WaitReason
	// says why: role_waits (makerspace staff check this role first) or
	// halted:<reason> (the send stopped). Nothing else about the queued row is
	// a member's business.
	Waiting    bool   `json:"waiting,omitempty"`
	WaitReason string `json:"wait_reason,omitempty"`
	// RequestKind is "extend" for a request to keep a role longer.
	RequestKind string `json:"request_kind,omitempty"`
}

type myViaRef struct {
	BundleID string `json:"bundle_id,omitempty"`
	RuleID   string `json:"rule_id,omitempty"`
}

const myChangesDefaultDays, myChangesMaxDays = 30, 90

func handleMyChanges(w http.ResponseWriter, r *http.Request) {
	subject := getAdminUserID(r.Context())
	if strings.TrimSpace(subject) == "" {
		jsonErrorResponse(w, http.StatusUnauthorized, "NO_SUBJECT", "This view is a person's own.")
		return
	}
	days := myChangesDefaultDays
	if raw := r.URL.Query().Get("days"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 || n > myChangesMaxDays {
			jsonValidationErrorResponse(w, "days must be a whole number from 1 to 90", map[string]string{"days": "range=1..90"})
			return
		}
		days = n
	}
	rows, err := dbListMyChanges(r.Context(), subject, days)
	if err != nil {
		jsonErrorResponse(w, http.StatusInternalServerError, "DB_ERROR", "Couldn't read your recent changes.")
		return
	}
	jsonResponse(w, http.StatusOK, map[string]any{"items": shapeMyChanges(rows)})
}

// shapeMyChanges turns Syndra's records into what a member is told.
func shapeMyChanges(rows []db.MyChangeRow) []myChange {
	out := make([]myChange, 0, len(rows))
	for _, row := range rows {
		c := myChange{
			Kind: row.Kind, At: row.At, ProjectID: row.ProjectID, RoleKey: row.RoleKey, RequestKind: row.RequestKind,
			Note: strings.TrimSpace(row.Note), Until: row.Until,
		}
		switch row.Source {
		case "bundle":
			if row.SourceRef != "" {
				c.Via = &myViaRef{BundleID: row.SourceRef}
			}
		case "rule":
			if row.SourceRef != "" {
				c.Via = &myViaRef{RuleID: row.SourceRef}
			}
		}
		// Absent when automatic: a rule, a lifecycle step, or Syndra itself.
		if !automaticActor(row.Actor) && row.Source != "rule" && row.Source != "lifecycle_cascade" {
			c.By = row.Actor
		}
		if row.Kind == "request_approved" && row.Pending {
			c.Waiting = true
			c.WaitReason = row.WaitReason
		}
		out = append(out, c)
	}
	return out
}

func automaticActor(actor string) bool {
	a := strings.TrimSpace(actor)
	return a == "" || a == "-" || a == "system" || strings.HasPrefix(a, "system:")
}
