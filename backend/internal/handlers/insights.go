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
