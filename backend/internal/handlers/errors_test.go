package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const zitadelRaw = "rpc error: code = Unavailable desc = connection refused 192.0.2.7:8080 zitadel.example.org"

func envelope(t *testing.T, rr *httptest.ResponseRecorder) ErrorResponse {
	t.Helper()
	var e ErrorResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &e); err != nil {
		t.Fatalf("not an envelope: %v: %s", err, rr.Body.String())
	}
	return e
}

// A Zitadel or add-on failure never carries its own text to the screen, keeps
// a stable code, and hands out a reference.
func TestUpstreamFailureNeverShowsRawText(t *testing.T) {
	for _, tc := range []struct {
		status int
		code   string
	}{
		{http.StatusBadGateway, "ZITADEL_ERROR"},
		{http.StatusBadGateway, "DRAIN_ERROR"},
		{http.StatusServiceUnavailable, "ADDON_UNREACHABLE"},
		{http.StatusInternalServerError, "DB_ERROR"},
	} {
		rr := httptest.NewRecorder()
		if tc.status == http.StatusInternalServerError {
			jsonErrorResponse(rr, tc.status, tc.code, zitadelRaw)
		} else {
			writeUpstreamStatus(rr, tc.status, errors.New(zitadelRaw), tc.code)
		}
		body := rr.Body.String()
		if rr.Code != tc.status || strings.Contains(body, "rpc error") || strings.Contains(body, "192.0.2.7") {
			t.Fatalf("%s leaked or changed status: %d %s", tc.code, rr.Code, body)
		}
		e := envelope(t, rr)
		if e.Error != tc.code || e.Message == "" || e.Details["request_id"] == "" {
			t.Fatalf("%s: want code, sentence and reference: %s", tc.code, body)
		}
	}
}

// A sentence a handler wrote, such as the one that says a change may have
// applied, is not upstream text and stays.
func TestAuthoredSentenceAtFiveHundredsSurvives(t *testing.T) {
	rr := httptest.NewRecorder()
	jsonErrorResponse(rr, http.StatusServiceUnavailable, "TARGET_UNCONFIRMED", "The change may have applied.")
	if e := envelope(t, rr); e.Message != "The change may have applied." {
		t.Fatalf("authored sentence replaced: %+v", e)
	}
}
