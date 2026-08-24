package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"syndra/internal/addons"
)

// What the TARGET says about itself, reached at last.
//
// `health.get` was declared in the manifest, implemented by the add-on,
// dispatched by it and given a policy entry from the day the platform landed,
// and no line of backend code ever called it. The target page listed it under
// "What it can do" — a capability advertised to operators with nothing behind
// it — while the four questions it answers (a failing disk, a degraded pool, a
// stopped `cifs`) went unanswered by a system that could already answer them.

func withSystemHealth(t *testing.T, stub func(ctx context.Context, target string) addons.SystemReport) {
	t.Helper()
	original := addonsSystemHealth
	addonsSystemHealth = stub
	t.Cleanup(func() { addonsSystemHealth = original })
}

func systemHealthRequest() *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/targets/truenas/system-health", nil)
	req.SetPathValue("target", "truenas")
	rr := httptest.NewRecorder()
	handleTargetSystemHealth(rr, req)
	return rr
}

func decodeBody(t *testing.T, rr *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var body map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v (%s)", err, rr.Body.String())
	}
	return body
}

func TestSystemHealth_CarriesWhatTheTargetReported(t *testing.T) {
	withSystemHealth(t, func(context.Context, string) addons.SystemReport {
		return addons.SystemReport{
			Outcome: addons.OutcomeSucceeded,
			System:  &addons.SystemInfo{Hostname: "truenas", Version: "25.10.5"},
			Alerts: []addons.TargetAlert{{
				Level: "WARNING", Class: "SMARTUncorrectedErrors",
				Text: "1 uncorrectable errors reported for sde.",
			}},
			Pools:    []addons.PoolStatus{{Name: "tank", Status: "ONLINE", Healthy: true}},
			Services: []addons.ServiceState{{Service: "cifs", State: "RUNNING", Enabled: true}},
		}
	})

	rr := systemHealthRequest()
	if rr.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", rr.Code)
	}
	body := decodeBody(t, rr)
	if body["readable"] != true {
		t.Fatalf("a target that answered is readable: %v", body["readable"])
	}
	if len(body["alerts"].([]any)) != 1 {
		t.Fatalf("alerts: %v", body["alerts"])
	}
}

// The distinction the whole surface exists for. A target that could not be
// asked and a target with nothing wrong are opposite facts, and a 502 would
// make the page render as an error rather than as an unanswered question.
func TestSystemHealth_UnreadableIsNotHealthy(t *testing.T) {
	withSystemHealth(t, func(context.Context, string) addons.SystemReport {
		return addons.SystemReport{Outcome: addons.OutcomeUnreached, Err: errors.New("addon truenas: dial tcp: no route to host")}
	})

	rr := systemHealthRequest()
	if rr.Code != http.StatusOK {
		t.Fatalf("want 200 so the page can say what happened, got %d", rr.Code)
	}
	body := decodeBody(t, rr)
	if body["readable"] != false {
		t.Fatal("an unreached target must not read as readable")
	}
	// Not an empty list of alerts and not an empty list of pools: absent. An
	// empty list is a claim that the target reported nothing wrong.
	for _, key := range []string{"alerts", "pools", "services", "system"} {
		if _, present := body[key]; present {
			t.Fatalf("%q must be absent when nothing was read, got %v", key, body[key])
		}
	}
	if body["detail"] == "" || body["detail"] == nil {
		t.Fatal("an operator is owed the reason")
	}
}

// A partial read that does not say which part is missing is a whole read that
// is wrong. `degraded` names the sources, so "no alerts" and "the alert source
// could not be read while the other three could" stop being the same answer.
func TestSystemHealth_NamesTheSourcesThatFailed(t *testing.T) {
	withSystemHealth(t, func(context.Context, string) addons.SystemReport {
		return addons.SystemReport{
			Outcome:  addons.OutcomeSucceeded,
			Pools:    []addons.PoolStatus{{Name: "tank", Status: "ONLINE", Healthy: true}},
			Degraded: []string{"alerts"},
		}
	})

	body := decodeBody(t, systemHealthRequest())
	degraded, ok := body["degraded"].([]any)
	if !ok || len(degraded) != 1 || degraded[0] != "alerts" {
		t.Fatalf("the failed source must be named: %v", body["degraded"])
	}
	if body["readable"] != true {
		t.Fatal("three sources answered, so the read is readable and partial")
	}
}

// The target's own error text is a classification and a code by the time it
// reaches here, and `detail` must not become the place that guarantee leaks.
func TestSystemHealth_DetailIsTheBackendsSentence(t *testing.T) {
	withSystemHealth(t, func(context.Context, string) addons.SystemReport {
		return addons.SystemReport{Outcome: addons.OutcomeIndeterminate, Err: nil}
	})

	body := decodeBody(t, systemHealthRequest())
	if body["readable"] != false {
		t.Fatal("indeterminate is not readable")
	}
	// A nil error must not render as "<nil>" or panic the handler.
	if detail, _ := body["detail"].(string); detail == "<nil>" {
		t.Fatalf("a missing reason must not be printed as a Go nil: %q", detail)
	}
}
