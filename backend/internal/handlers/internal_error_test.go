package handlers

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// An internal failure's text never reaches the screen. The operator saw
// "scan role mapping: number of field descriptions must equal number of
// destinations, got 8 and 10" in a mapping dialog; it read as their mistake.
// They get a sentence and a reference, and the log line carries the cause
// under that same reference.
func TestAnInternalFailureShowsAReferenceNotItsCause(t *testing.T) {
	var logs bytes.Buffer
	previous := log.Writer()
	log.SetOutput(&logs)
	t.Cleanup(func() { log.SetOutput(previous) })

	const cause = "scan role mapping: number of field descriptions must equal number of destinations, got 8 and 10"
	rr := httptest.NewRecorder()
	jsonErrorResponse(rr, http.StatusInternalServerError, "DB_ERROR", cause)

	var body ErrorResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(rr.Body.String(), "scan role mapping") {
		t.Fatalf("the cause reached the response: %s", rr.Body.String())
	}
	ref := body.Details["request_id"]
	if len(ref) < 6 || body.Error != "DB_ERROR" {
		t.Fatalf("want the code kept and a reference, got %+v", body)
	}
	if !strings.Contains(logs.String(), "ref="+ref) || !strings.Contains(logs.String(), cause) {
		t.Fatalf("the log must carry the cause under the reference, got %q", logs.String())
	}
}

// A refusal Syndra wrote for a person is not rewritten.
func TestARefusalKeepsItsSentence(t *testing.T) {
	rr := httptest.NewRecorder()
	jsonErrorResponse(rr, http.StatusConflict, "PROJECT_REQUIRED", "Choose truenas's Zitadel project first.")
	if !strings.Contains(rr.Body.String(), "Choose truenas") {
		t.Fatalf("a 4xx keeps its message: %s", rr.Body.String())
	}
}
