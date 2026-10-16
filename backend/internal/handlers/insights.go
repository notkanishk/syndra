package handlers

import (
	"errors"
	"net/http"

	"syndra/internal/models"
	"syndra/internal/services"
)

// Insights are read-only: what is odd about how access is arranged, and who
// holds one thing and not another. Operator-only; nothing here writes.

func handleInsightsFindings(w http.ResponseWriter, r *http.Request) {
	findings, err := svcInsightsFindings(r.Context())
	if err != nil {
		viewError(w, err, http.StatusInternalServerError, "INSIGHTS_ERROR")
		return
	}
	jsonResponse(w, http.StatusOK, findings)
}

// handleInsightsWho answers ?has=<ref>&without=<ref>. `has` is required;
// `without` is optional. A ref is role:<project>:<key> or bundle:<id>.
func handleInsightsWho(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	if !q.Has("has") {
		jsonValidationErrorResponse(w, "has is required", map[string]string{"has": "required"})
		return
	}
	has, err := services.ParseInsightRef(q.Get("has"))
	if err != nil {
		jsonValidationErrorResponse(w, err.Error(), map[string]string{"has": "malformed reference"})
		return
	}
	var without services.InsightRef
	if q.Has("without") {
		if without, err = services.ParseInsightRef(q.Get("without")); err != nil {
			jsonValidationErrorResponse(w, err.Error(), map[string]string{"without": "malformed reference"})
			return
		}
	}
	who, err := svcInsightsWho(r.Context(), has, without)
	if err != nil {
		if errors.Is(err, services.ErrInvalidInsightRef) {
			jsonValidationErrorResponse(w, err.Error(), nil)
			return
		}
		viewError(w, err, http.StatusInternalServerError, "INSIGHTS_ERROR")
		return
	}
	jsonResponse(w, http.StatusOK, who)
}

// handleInsightsWhatIf answers POST {"change": {...}}: who would gain and lose
// what if it were made. It makes no change, issues no plan, queues nothing.
func handleInsightsWhatIf(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Change models.InsightsWhatIfChange `json:"change"`
	}
	if err := decodeJSONStrict(r.Body, &req); err != nil {
		jsonValidationErrorResponse(w, "Invalid JSON payload", map[string]string{"body": err.Error()})
		return
	}
	res, err := svcInsightsWhatIf(r.Context(), req.Change)
	switch {
	case errors.Is(err, services.ErrInvalidWhatIf):
		jsonValidationErrorResponse(w, err.Error(), map[string]string{"change": "invalid"})
		return
	case errors.Is(err, services.ErrWhatIfNotFound):
		jsonErrorResponse(w, http.StatusNotFound, "NOT_FOUND", err.Error())
		return
	case err != nil:
		viewError(w, err, http.StatusInternalServerError, "INSIGHTS_ERROR")
		return
	}
	jsonResponse(w, http.StatusOK, res)
}

// handleInsightsTrends answers ?period=30d|term|year (default term): how access
// has been moving over that window. Anything else is a 400, not a quiet default.
func handleInsightsTrends(w http.ResponseWriter, r *http.Request) {
	period, err := services.ParseTrendPeriod(r.URL.Query().Get("period"))
	if err != nil {
		jsonValidationErrorResponse(w, err.Error(), map[string]string{"period": "must be 30d, term or year"})
		return
	}
	res, err := svcInsightsTrends(r.Context(), period)
	if err != nil {
		viewError(w, err, http.StatusInternalServerError, "INSIGHTS_ERROR")
		return
	}
	jsonResponse(w, http.StatusOK, res)
}

// handleInsightsAsOf answers ?date=YYYY-MM-DD with the latest daily snapshot on
// or before it. A date before the first snapshot is a 404 that says when the
// record begins (first_day is null when there is no snapshot at all).
func handleInsightsAsOf(w http.ResponseWriter, r *http.Request) {
	day, err := services.ParseAsOfDate(r.URL.Query().Get("date"))
	if err != nil {
		jsonValidationErrorResponse(w, err.Error(), map[string]string{"date": "must be YYYY-MM-DD, no later than today"})
		return
	}
	res, err := svcInsightsAsOf(r.Context(), day)
	var none *services.NoSnapshotError
	if errors.As(err, &none) {
		// Its own shape: ErrorResponse.Details is string-only and first_day can be null.
		type details struct {
			FirstDay *string `json:"first_day"`
		}
		jsonResponse(w, http.StatusNotFound, struct {
			Error   string  `json:"error"`
			Message string  `json:"message"`
			Details details `json:"details"`
		}{"NO_SNAPSHOT", err.Error(), details{none.FirstDay}})
		return
	}
	if err != nil {
		viewError(w, err, http.StatusInternalServerError, "INSIGHTS_ERROR")
		return
	}
	jsonResponse(w, http.StatusOK, res)
}

// handleInsightsTimeline answers ?from=&to= (inclusive days, at most 400): the
// scrubber's per-day change counts and the first and last snapshot day.
func handleInsightsTimeline(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	from, to, err := services.ParseTimelineRange(q.Get("from"), q.Get("to"))
	if err != nil {
		jsonValidationErrorResponse(w, err.Error(), map[string]string{"from": "YYYY-MM-DD", "to": "YYYY-MM-DD"})
		return
	}
	res, err := svcInsightsTimeline(r.Context(), from, to)
	if err != nil {
		viewError(w, err, http.StatusInternalServerError, "INSIGHTS_ERROR")
		return
	}
	jsonResponse(w, http.StatusOK, res)
}

// handleInsightsUsage answers ?days=1..365 (default 90): whether sign-ins are
// being received at all, and which roles are held by people who have not used
// their project in that time. Anything else for days is a 400.
func handleInsightsUsage(w http.ResponseWriter, r *http.Request) {
	days, err := services.ParseUsageDays(r.URL.Query().Get("days"))
	if err != nil {
		jsonValidationErrorResponse(w, err.Error(), map[string]string{"days": "must be a whole number from 1 to 365"})
		return
	}
	res, err := svcInsightsUsage(r.Context(), days)
	if err != nil {
		viewError(w, err, http.StatusInternalServerError, "INSIGHTS_ERROR")
		return
	}
	jsonResponse(w, http.StatusOK, res)
}

// handleGetUserUsage is one person's last use of each project, by kind. A person
// nothing has been seen for gets [], not a 404: absence of a row is not absence
// of the person.
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
	jsonResponse(w, http.StatusOK, rows)
}
