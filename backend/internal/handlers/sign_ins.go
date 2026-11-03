package handlers

import (
	"errors"
	"log"
	"net/http"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"syndra/internal/db"
	"syndra/internal/zitadel"
)

// Sign-ins (A-10 Part 2; owner, 3 Nov: "1a with statistics").
//
// Syndra keeps two things, and any operator reads them: each person's last
// sign-in per app, and each app's daily total, naming nobody. "Who signed in,
// and when" is not kept. It is read from Zitadel's own log only through a
// deliberate lookup, with a reason, and the lookup is audited before Zitadel
// is asked.

// receivingWindow is how far back a sign-in must be for Syndra to say it is
// receiving them at all, the same 90 days Insights reads.
const receivingWindow = 90 * 24 * time.Hour

// signInsReceiving says a sign-in has reached Syndra in the last 90 days. With
// none, pages say nothing about sign-ins rather than "nobody signed in".
func signInsReceiving(r *http.Request) (bool, error) {
	latest, err := dbUsageLatestByKind(r.Context(), time.Now().Add(-receivingWindow))
	if err != nil {
		return false, err
	}
	_, ok := latest[db.UsageSignIn]
	return ok, nil
}

// handleGetUserUsage is one person's last sign-in per app, with whether
// sign-ins are arriving and since when Syndra has read them ("No sign-in to
// its app since 3 Sept"). A person nothing has been seen for gets an empty
// list, not a 404.
func handleGetUserUsage(w http.ResponseWriter, r *http.Request) {
	userID := r.PathValue("id")
	if !trimmedNonEmpty(userID) {
		jsonValidationErrorResponse(w, "id path parameter is required", map[string]string{"id": "required"})
		return
	}
	rows, err := dbGetUserUsage(r.Context(), userID)
	if err != nil {
		viewError(w, err, http.StatusInternalServerError, "INSIGHTS_ERROR")
		return
	}
	receiving, err := signInsReceiving(r)
	if err != nil {
		viewError(w, err, http.StatusInternalServerError, "INSIGHTS_ERROR")
		return
	}
	out := map[string]any{"usage": rows, "receiving": receiving}
	if receiving {
		if since, err := dbSignInsSince(r.Context()); err == nil && since != nil {
			out["receiving_since"] = since
		}
	}
	jsonResponse(w, http.StatusOK, out)
}

// handleGetApplicationUsage is one app's sign-ins, naming nobody: people in 7
// days and in 30, its last sign-in, and the daily totals for the 30-day strip.
func handleGetApplicationUsage(w http.ResponseWriter, r *http.Request) {
	appID := r.PathValue("id")
	if !trimmedNonEmpty(appID) {
		jsonValidationErrorResponse(w, "id path parameter is required", map[string]string{"id": "required"})
		return
	}
	receiving, err := signInsReceiving(r)
	if err != nil {
		viewError(w, err, http.StatusInternalServerError, "INSIGHTS_ERROR")
		return
	}
	if !receiving {
		jsonResponse(w, http.StatusOK, map[string]any{"receiving": false})
		return
	}
	now := time.Now()
	usage, err := dbGetAppUsage(r.Context(), appID, now.Add(-7*24*time.Hour), now.Add(-30*24*time.Hour))
	if err != nil {
		viewError(w, err, http.StatusInternalServerError, "INSIGHTS_ERROR")
		return
	}
	jsonResponse(w, http.StatusOK, map[string]any{
		"receiving":  true,
		"people_7d":  usage.RecentPeople,
		"people_30d": usage.WindowPeople,
		"last_at":    usage.LastAt,
		"days":       usage.Days,
	})
}

// ── Look up sign-ins ──────────────────────────────────────────────────────

const (
	signInLookupAudit = "sign_ins.looked_up"
	maxLookupReason   = 500
	// lookupReadLimit is how many sign-in events one lookup reads from
	// Zitadel's log, newest first. The log is filtered here, not in Zitadel
	// (oidc_session.added names its person only in its payload), so a busy
	// window can hold more; the answer then says it was cut off.
	// ponytail: one page of 1000; page by sequence if a whole term is wanted.
	lookupReadLimit = 1000
)

// The two events Syndra reads as a sign-in, the same two the webhook does.
var signInEventTypes = []string{"user.token.added", "oidc_session.added"}

var lookupRanges = map[int]bool{7: true, 30: true, 90: true}

type signInLookupRequest struct {
	UserID string `json:"user_id"`
	AppID  string `json:"app_id"`
	Days   int    `json:"days"`
	Reason string `json:"reason"`
}

type lookedUpSignIn struct {
	UserID    string    `json:"user_id"`
	AppID     string    `json:"app_id"`
	ProjectID string    `json:"project_id"`
	At        time.Time `json:"at"`
}

// handleLookUpSignIns reads who signed in, and when, from Zitadel's own log:
// for one person, one app, or one person on one app, over 7, 30 or 90 days.
//
// The audit entry is written FIRST, and a lookup that cannot be audited is not
// made: who looked, at whom and which app, why, and the range. The answer is
// returned and kept nowhere.
func handleLookUpSignIns(w http.ResponseWriter, r *http.Request) {
	var req signInLookupRequest
	if err := decodeJSONStrict(r.Body, &req); err != nil {
		jsonValidationErrorResponse(w, "Invalid JSON payload", map[string]string{"body": err.Error()})
		return
	}
	req.UserID, req.AppID, req.Reason = strings.TrimSpace(req.UserID), strings.TrimSpace(req.AppID), strings.TrimSpace(req.Reason)
	fields := map[string]string{}
	if req.UserID == "" && req.AppID == "" {
		fields["user_id"] = "a person or an app is required"
	}
	if !lookupRanges[req.Days] {
		fields["days"] = "must be 7, 30 or 90"
	}
	if req.Reason == "" {
		fields["reason"] = "required"
	} else if utf8.RuneCountInString(req.Reason) > maxLookupReason {
		fields["reason"] = "at most 500 characters"
	}
	if len(fields) > 0 {
		jsonValidationErrorResponse(w, "Say whose sign-ins, over which range, and why", fields)
		return
	}

	until := time.Now().UTC().Truncate(time.Second)
	since := until.Add(-time.Duration(req.Days) * 24 * time.Hour)
	actor := resolveActor(r, "")
	if err := dbInsertAuditLogDetails(r.Context(), actor, req.UserID, signInLookupAudit, req.AppID, map[string]any{
		"reason": req.Reason, "since": since, "until": until, "days": req.Days,
	}); err != nil {
		log.Printf("[SIGN-INS] lookup by %s not audited, so not made: %v", actor, err)
		jsonErrorResponse(w, http.StatusInternalServerError, "NOT_AUDITED",
			"Syndra couldn't record this lookup, so it didn't make it. Nothing was read.")
		return
	}

	events, truncated, err := zitadelSearchEvents(r.Context(), zitadel.EventSearch{
		Types: signInEventTypes, Since: since, Until: until, Limit: lookupReadLimit,
	})
	if err != nil {
		status := http.StatusBadGateway
		if errors.Is(err, errNoClient) {
			status = http.StatusServiceUnavailable
		}
		log.Printf("[SIGN-INS] Zitadel's log could not be read: %v", err)
		jsonErrorResponse(w, status, "ZITADEL_UNREACHABLE",
			"Zitadel didn't answer, so nothing was read. The lookup is still in Change history.")
		return
	}
	apps, err := directoryApplications(r.Context())
	if err != nil {
		// Still an answer: each sign-in then reads as "app unknown". Only an
		// app filter needs the directory, and it refuses rather than guess.
		log.Printf("[SIGN-INS] apps could not be read: %v", err)
		if req.AppID != "" {
			jsonErrorResponse(w, http.StatusBadGateway, "APPS_UNREADABLE",
				"Syndra couldn't read Zitadel's apps, so it can't tell which sign-ins were to this one.")
			return
		}
	}

	found := []lookedUpSignIn{}
	for _, e := range events {
		p := translateEventName(zitadelEventPayload{
			EventType: e.Type, AggregateID: e.AggregateID, AggregateType: e.AggregateType, EventPayload: e.Payload,
		})
		if p.EventType != "user_signed_in" || (req.UserID != "" && p.UserID != req.UserID) {
			continue
		}
		app := matchSignInApp(apps, p.ClientRefs)
		if req.AppID != "" && app.ID != req.AppID {
			continue
		}
		found = append(found, lookedUpSignIn{UserID: p.UserID, AppID: app.ID, ProjectID: app.ProjectID, At: e.At})
	}
	sort.SliceStable(found, func(i, j int) bool { return found[i].At.After(found[j].At) })
	jsonResponse(w, http.StatusOK, map[string]any{
		"sign_ins": found, "truncated": truncated, "since": since, "until": until,
	})
}
